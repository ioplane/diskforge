package diskforge

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ioplane/diskforge/internal/image"
	linuxsystem "github.com/ioplane/diskforge/internal/linux"
	"github.com/ioplane/diskforge/internal/policy"
)

type facadeTarget struct {
	bytes.Buffer
}

func (target *facadeTarget) Sync() error  { return nil }
func (target *facadeTarget) Fd() uintptr  { return 42 }
func (target *facadeTarget) Close() error { return nil }

type facadeBoundary struct {
	calls      []string
	inspectErr error
}

func (boundary *facadeBoundary) Inspect(
	_ context.Context,
	request linuxsystem.Request,
) (policy.Inspection, error) {
	boundary.calls = append(boundary.calls, "inspect")
	if boundary.inspectErr != nil {
		return policy.Inspection{}, boundary.inspectErr
	}

	// #nosec G101 -- the confirmation value is a deterministic test fixture.
	return policy.Inspection{
		Mode: request.Mode,
		Target: policy.TargetIdentity{
			CanonicalPath: request.TargetPath,
			SizeBytes:     4096,
		},
		Image: policy.ImageIdentity{
			Path:              request.ImagePath,
			SHA256:            request.SHA256,
			CompressedBytes:   1024,
			UncompressedBytes: request.ExpectedBytes,
		},
		ConfirmationToken: "confirm-v1-vda-aaaaaaaaaaaa-testbinding0000",
	}, nil
}

func (boundary *facadeBoundary) VerifyImage(path, digest string) (*image.Verified, error) {
	boundary.calls = append(boundary.calls, "verify")

	return &image.Verified{
		Path:            path,
		SHA256:          digest,
		Format:          image.FormatRaw,
		CompressedBytes: 1024,
	}, nil
}

func (boundary *facadeBoundary) Confirm(string, string) error {
	boundary.calls = append(boundary.calls, "confirm")

	return nil
}

func (boundary *facadeBoundary) Mlockall() error        { return nil }
func (boundary *facadeBoundary) Swapoff() error         { return nil }
func (boundary *facadeBoundary) EnableSysRq() error     { return nil }
func (boundary *facadeBoundary) RemountReadOnly() error { return nil }
func (boundary *facadeBoundary) SysRqSync() error       { return nil }
func (boundary *facadeBoundary) SysRqReboot() error     { return nil }

func (boundary *facadeBoundary) OpenTarget(
	string,
	policy.Mode,
	policy.TargetIdentity,
) (linuxsystem.Target, error) {
	boundary.calls = append(boundary.calls, "open")

	return &facadeTarget{}, nil
}

func (boundary *facadeBoundary) WriteVerified(
	_ context.Context,
	_ *image.Verified,
	_ linuxsystem.Target,
	expectedBytes int64,
	progress func(image.Progress),
) (image.Result, error) {
	boundary.calls = append(boundary.calls, "write")
	if progress != nil {
		progress(image.Progress{ExpectedBytes: expectedBytes})
		progress(image.Progress{WrittenBytes: expectedBytes, ExpectedBytes: expectedBytes})
	}

	return image.Result{WrittenBytes: expectedBytes}, nil
}

func (boundary *facadeBoundary) Fdatasync(linuxsystem.Target) error { return nil }
func (boundary *facadeBoundary) FlushBlock(linuxsystem.Target) error {
	return nil
}

func facadeWriteRequest() WriteRequest {
	return WriteRequest{
		Mode:          ModeRescue,
		TargetPath:    "/dev/vda",
		ImagePath:     "/images/image.raw",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedBytes: 2048,
		Confirmation:  "confirm-v1-vda-aaaaaaaaaaaa-testbinding0000",
	}
}

func TestEngineDryRunVerifiesWithoutOpeningTarget(t *testing.T) {
	t.Parallel()

	boundary := &facadeBoundary{}
	engine, err := New(withBoundary(boundary))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	request := facadeWriteRequest()
	request.DryRun = true
	result, err := engine.Write(t.Context(), request)
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !result.DryRun || result.Inspection.ConfirmationToken == "" {
		t.Fatalf("Write() = %#v", result)
	}
	if !reflect.DeepEqual(boundary.calls, []string{"inspect", "verify"}) {
		t.Fatalf("calls = %#v", boundary.calls)
	}
}

func TestEngineMapsInternalGateErrors(t *testing.T) {
	t.Parallel()

	boundary := &facadeBoundary{
		inspectErr: &policy.GateError{Code: policy.GateNotRoot, Message: "root is required"},
	}
	engine, err := New(withBoundary(boundary))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	_, err = engine.Inspect(t.Context(), InspectRequest{
		Mode:          ModeRescue,
		TargetPath:    "/dev/vda",
		ImagePath:     "/images/image.raw",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ExpectedBytes: 2048,
	})
	var gate *GateError
	if !errors.As(err, &gate) || gate.Code != GateNotRoot || !errors.Is(err, &GateError{Code: GateNotRoot}) {
		t.Fatalf("Inspect() error = %v", err)
	}
}

func TestEngineConvertsProgress(t *testing.T) {
	t.Parallel()

	boundary := &facadeBoundary{}
	updates := []Progress{}
	engine, err := New(
		withBoundary(boundary),
		WithProgress(func(progress Progress) { updates = append(updates, progress) }),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	result, err := engine.Write(t.Context(), facadeWriteRequest())
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if result.WrittenBytes != 2048 || len(updates) != 2 || updates[1].WrittenBytes != 2048 {
		t.Fatalf("result=%#v updates=%#v", result, updates)
	}
}
