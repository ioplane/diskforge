package linux

import (
	"context"
	"fmt"
	"io"

	"github.com/ioplane/diskforge/internal/image"
	"github.com/ioplane/diskforge/internal/policy"
)

// Request contains every destructive choice supplied by the caller.
type Request struct {
	Mode          policy.Mode
	TargetPath    string
	ImagePath     string
	SHA256        string
	ExpectedBytes int64
	Confirmation  string
	Reboot        bool
}

// Target is an opened whole-disk descriptor owned by Execute.
type Target interface {
	io.Writer
	Sync() error
	Fd() uintptr
	Close() error
}

type inspectionBoundary interface {
	Inspect(ctx context.Context, request Request) (policy.Inspection, error)
	VerifyImage(path string, digest string) (*image.Verified, error)
	Confirm(provided string, expected string) error
}

type liveBoundary interface {
	Mlockall() error
	Swapoff() error
	EnableSysRq() error
	RemountReadOnly() error
	SysRqSync() error
	SysRqReboot() error
}

type writeBoundary interface {
	OpenTarget(path string, mode policy.Mode, expected policy.TargetIdentity) (Target, error)
	WriteVerified(
		ctx context.Context,
		source *image.Verified,
		target Target,
		expectedBytes int64,
		progress func(image.Progress),
	) (image.Result, error)
	Fdatasync(target Target) error
	FlushBlock(target Target) error
}

// Boundary is the privileged dependency used by Execute.
type Boundary interface {
	inspectionBoundary
	liveBoundary
	writeBoundary
}

// Prepare completes observation, verification, and immutable identity checks.
func Prepare(
	ctx context.Context,
	request Request,
	system Boundary,
) (policy.Inspection, *image.Verified, error) {
	inspection, err := system.Inspect(ctx, request)
	if err != nil {
		return policy.Inspection{}, nil, err
	}
	verified, err := system.VerifyImage(request.ImagePath, request.SHA256)
	if err != nil {
		return policy.Inspection{}, nil, err
	}
	if !sameIdentity(request, inspection, verified) {
		_ = verified.Close()

		return policy.Inspection{}, nil, &policy.GateError{
			Code:    policy.GateIdentityChanged,
			Message: "target or image identity changed between inspection and verification",
		}
	}

	return inspection, verified, nil
}

func sameIdentity(request Request, inspection policy.Inspection, verified *image.Verified) bool {
	return verified != nil &&
		inspection.Mode == request.Mode &&
		inspection.Target.CanonicalPath == request.TargetPath &&
		inspection.Image.Path == request.ImagePath &&
		inspection.Image.SHA256 == request.SHA256 &&
		inspection.Image.UncompressedBytes == request.ExpectedBytes &&
		verified.Path == inspection.Image.Path &&
		verified.SHA256 == inspection.Image.SHA256 &&
		verified.CompressedBytes == inspection.Image.CompressedBytes
}

// Execute applies every non-destructive gate before opening the target.
func Execute(
	ctx context.Context,
	request Request,
	system Boundary,
	progress func(image.Progress),
) (image.Result, error) {
	inspection, verified, err := Prepare(ctx, request, system)
	if err != nil {
		return image.Result{}, err
	}
	defer verified.Close()

	if confirmErr := system.Confirm(request.Confirmation, inspection.ConfirmationToken); confirmErr != nil {
		return image.Result{}, confirmErr
	}
	if request.Mode == policy.ModeLive && !request.Reboot {
		return image.Result{}, &policy.GateError{
			Code:    policy.GateLiveReboot,
			Message: "live overwrite requires explicit immediate reboot",
		}
	}
	if prepareErr := prepareLive(request.Mode, system); prepareErr != nil {
		return image.Result{}, prepareErr
	}

	target, err := system.OpenTarget(request.TargetPath, request.Mode, inspection.Target)
	if err != nil {
		return image.Result{}, err
	}
	defer target.Close()

	result, err := system.WriteVerified(ctx, verified, target, request.ExpectedBytes, progress)
	if err != nil {
		return image.Result{}, err
	}
	if err := system.Fdatasync(target); err != nil {
		return image.Result{}, err
	}
	if err := system.FlushBlock(target); err != nil {
		return image.Result{}, err
	}
	if err := rebootLive(request.Mode, system); err != nil {
		return image.Result{}, err
	}

	return result, nil
}

func prepareLive(mode policy.Mode, system Boundary) error {
	if mode != policy.ModeLive {
		return nil
	}
	for _, operation := range []struct {
		name string
		run  func() error
	}{
		{name: "lock process memory", run: system.Mlockall},
		{name: "disable swap", run: system.Swapoff},
		{name: "enable SysRq", run: system.EnableSysRq},
		{name: "remount filesystems read-only", run: system.RemountReadOnly},
	} {
		if err := operation.run(); err != nil {
			return fmt.Errorf("%s: %w", operation.name, err)
		}
	}

	return nil
}

func rebootLive(mode policy.Mode, system Boundary) error {
	if mode != policy.ModeLive {
		return nil
	}
	if err := system.SysRqSync(); err != nil {
		return fmt.Errorf("SysRq sync: %w", err)
	}
	if err := system.SysRqReboot(); err != nil {
		return fmt.Errorf("SysRq reboot: %w", err)
	}

	return nil
}
