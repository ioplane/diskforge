package linux

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ioplane/diskforge/internal/image"
	"github.com/ioplane/diskforge/internal/policy"
)

var errInjected = errors.New("injected failure")

type fakeTarget struct {
	bytes.Buffer
}

func (target *fakeTarget) Sync() error  { return nil }
func (target *fakeTarget) Fd() uintptr  { return 42 }
func (target *fakeTarget) Close() error { return nil }

type recordingSystem struct {
	calls          []string
	failAt         string
	target         *fakeTarget
	openedIdentity policy.TargetIdentity
	verifiedDigest string
}

func (system *recordingSystem) call(name string) error {
	system.calls = append(system.calls, name)
	if system.failAt == name {
		return errInjected
	}

	return nil
}

func (system *recordingSystem) Inspect(_ context.Context, request Request) (policy.Inspection, error) {
	if err := system.call("inspect"); err != nil {
		return policy.Inspection{}, err
	}

	// #nosec G101 -- all values are synthetic identity and confirmation fixtures.
	return policy.Inspection{
		Mode: request.Mode,
		Target: policy.TargetIdentity{
			CanonicalPath: "/dev/vda",
			SizeBytes:     42_949_672_960,
		},
		Image: policy.ImageIdentity{
			Path:              request.ImagePath,
			SHA256:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			CompressedBytes:   1024,
			UncompressedBytes: 34_359_738_368,
		},
		ConfirmationToken: "confirm-v1-vda-aaaaaaaaaaaa-testbinding0000",
	}, nil
}

func (system *recordingSystem) VerifyImage(path, digest string) (*image.Verified, error) {
	if err := system.call("verify"); err != nil {
		return nil, err
	}
	if system.verifiedDigest != "" {
		digest = system.verifiedDigest
	}

	return &image.Verified{
		Path:            path,
		SHA256:          digest,
		Format:          image.FormatZstd,
		CompressedBytes: 1024,
	}, nil
}

func (system *recordingSystem) Confirm(string, string) error {
	return system.call("confirmation")
}

func (system *recordingSystem) Mlockall() error        { return system.call("mlockall") }
func (system *recordingSystem) Swapoff() error         { return system.call("swapoff") }
func (system *recordingSystem) EnableSysRq() error     { return system.call("sysrq-enable") }
func (system *recordingSystem) RemountReadOnly() error { return system.call("remount-readonly") }

func (system *recordingSystem) OpenTarget(
	_ string,
	_ policy.Mode,
	expected policy.TargetIdentity,
) (Target, error) {
	if err := system.call("target-open"); err != nil {
		return nil, err
	}
	system.target = &fakeTarget{}
	system.openedIdentity = expected

	return system.target, nil
}

func TestExecuteBindsOpenToInspectedTargetIdentity(t *testing.T) {
	t.Parallel()

	system := &recordingSystem{}
	if _, err := Execute(t.Context(), liveRequest(), system, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if system.openedIdentity.CanonicalPath != "/dev/vda" || system.openedIdentity.SizeBytes == 0 {
		t.Fatalf("OpenTarget() expected identity = %#v", system.openedIdentity)
	}
}

func (system *recordingSystem) WriteVerified(
	context.Context,
	*image.Verified,
	Target,
	int64,
	func(image.Progress),
) (image.Result, error) {
	if err := system.call("write"); err != nil {
		return image.Result{}, err
	}

	return image.Result{WrittenBytes: 34_359_738_368}, nil
}

func (system *recordingSystem) Fdatasync(Target) error  { return system.call("fdatasync") }
func (system *recordingSystem) FlushBlock(Target) error { return system.call("block-flush") }
func (system *recordingSystem) SysRqSync() error        { return system.call("sysrq-sync") }
func (system *recordingSystem) SysRqReboot() error      { return system.call("sysrq-reboot") }

func liveRequest() Request {
	return Request{
		Mode:          policy.ModeLive,
		TargetPath:    "/dev/vda",
		ImagePath:     "/dev/shm/image.raw.zst",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedBytes: 34_359_738_368,
		Confirmation:  "confirm-v1-vda-aaaaaaaaaaaa-testbinding0000",
		Reboot:        true,
	}
}

func TestExecuteRejectsChangedVerifiedIdentity(t *testing.T) {
	t.Parallel()

	system := &recordingSystem{
		verifiedDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	_, err := Execute(t.Context(), liveRequest(), system, nil)
	var gate *policy.GateError
	if !errors.As(err, &gate) || gate.Code != policy.GateIdentityChanged {
		t.Fatalf("Execute() error = %v, want GateIdentityChanged", err)
	}
	if !reflect.DeepEqual(system.calls, []string{"inspect", "verify"}) {
		t.Fatalf("calls = %#v", system.calls)
	}
}

func TestSystemConfirmationIsExact(t *testing.T) {
	t.Parallel()

	system := DefaultSystem()
	if err := system.Confirm("token", "token"); err != nil {
		t.Fatalf("Confirm() error = %v", err)
	}
	var gate *policy.GateError
	if err := system.Confirm("TOKEN", "token"); !errors.As(err, &gate) || gate.Code != policy.GateConfirmation {
		t.Fatalf("Confirm() error = %v, want GateConfirmation", err)
	}
}

func TestExecuteEnforcesExactLiveOrder(t *testing.T) {
	t.Parallel()

	system := &recordingSystem{}
	result, err := Execute(t.Context(), liveRequest(), system, nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if result.WrittenBytes != 34_359_738_368 {
		t.Fatalf("Execute() = %#v", result)
	}
	want := []string{
		"inspect", "verify", "confirmation", "mlockall", "swapoff",
		"sysrq-enable", "remount-readonly", "target-open", "write",
		"fdatasync", "block-flush", "sysrq-sync", "sysrq-reboot",
	}
	if !reflect.DeepEqual(system.calls, want) {
		t.Fatalf("calls = %#v, want %#v", system.calls, want)
	}
}

func TestExecuteStopsAtEveryFailure(t *testing.T) {
	t.Parallel()

	steps := []string{
		"inspect", "verify", "confirmation", "mlockall", "swapoff",
		"sysrq-enable", "remount-readonly", "target-open", "write",
		"fdatasync", "block-flush", "sysrq-sync", "sysrq-reboot",
	}
	for index, step := range steps {
		t.Run(step, func(t *testing.T) {
			t.Parallel()

			system := &recordingSystem{failAt: step}
			if _, err := Execute(t.Context(), liveRequest(), system, nil); !errors.Is(err, errInjected) {
				t.Fatalf("Execute() error = %v, want injected failure", err)
			}
			if !reflect.DeepEqual(system.calls, steps[:index+1]) {
				t.Fatalf("calls = %#v, want %#v", system.calls, steps[:index+1])
			}
		})
	}
}

func TestExecuteRescueOmitsLiveOperations(t *testing.T) {
	t.Parallel()

	system := &recordingSystem{}
	request := liveRequest()
	request.Mode = policy.ModeRescue
	request.ImagePath = "/images/image.raw.zst"
	request.Reboot = false
	if _, err := Execute(t.Context(), request, system, nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	want := []string{
		"inspect", "verify", "confirmation", "target-open", "write",
		"fdatasync", "block-flush",
	}
	if !reflect.DeepEqual(system.calls, want) {
		t.Fatalf("calls = %#v, want %#v", system.calls, want)
	}
}

func TestExecuteRequiresExplicitLiveReboot(t *testing.T) {
	t.Parallel()

	system := &recordingSystem{}
	request := liveRequest()
	request.Reboot = false
	_, err := Execute(t.Context(), request, system, nil)
	var gate *policy.GateError
	if !errors.As(err, &gate) || gate.Code != policy.GateLiveReboot {
		t.Fatalf("Execute() error = %v, want GateLiveReboot", err)
	}
	if !reflect.DeepEqual(system.calls, []string{"inspect", "verify", "confirmation"}) {
		t.Fatalf("calls = %#v", system.calls)
	}
}
