//go:build linux

package linux

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/ioplane/diskforge/internal/image"
	"github.com/ioplane/diskforge/internal/policy"
	"golang.org/x/sys/unix"
)

const (
	remountPollAttempts = 600
	remountPollInterval = 50 * time.Millisecond
)

var ErrRemountIncomplete = errors.New("block-backed mounts remain writable")

// System implements the privileged Linux boundary without shell commands.
type System struct {
	Observer Observer
}

// DefaultSystem observes the running Linux host's procfs and sysfs.
func DefaultSystem() *System {
	return &System{Observer: Observer{ProcRoot: "/proc", SysRoot: "/sys"}}
}

// Inspect gathers a complete snapshot and applies pure fail-closed policy.
func (system *System) Inspect(
	_ context.Context,
	request Request,
) (policy.Inspection, error) {
	target, err := system.Observer.Target(request.TargetPath)
	if err != nil {
		return policy.Inspection{}, err
	}
	mounted, err := system.Observer.MountedDevices()
	if err != nil {
		return policy.Inspection{}, err
	}
	swaps, err := system.Observer.SwapDevices()
	if err != nil {
		return policy.Inspection{}, err
	}
	filesystem, backing, err := system.Observer.Source(request.ImagePath)
	if err != nil {
		return policy.Inspection{}, err
	}
	// #nosec G304 -- inspection must stat the caller-selected image path.
	info, err := os.Stat(request.ImagePath)
	if err != nil {
		return policy.Inspection{}, fmt.Errorf("stat image source: %w", err)
	}

	host := policy.HostObservation{
		EUID:                 os.Geteuid(),
		MountedDevices:       mounted,
		SwapDevices:          swaps,
		SourceFilesystem:     filesystem,
		SourceBackingDevices: backing,
	}
	if request.Mode == policy.ModeLive {
		if err := system.observeLive(&host); err != nil {
			return policy.Inspection{}, err
		}
	}

	return policy.Inspect(
		request.Mode,
		target,
		policy.ImageIdentity{
			Path:              request.ImagePath,
			SHA256:            request.SHA256,
			CompressedBytes:   info.Size(),
			UncompressedBytes: request.ExpectedBytes,
		},
		host,
	)
}

func (system *System) observeLive(host *policy.HostObservation) error {
	var err error
	host.RootDisk, err = system.Observer.RootDisk()
	if err != nil {
		return err
	}
	host.MemoryAvailableBytes, err = system.Observer.MemoryAvailableBytes()
	if err != nil {
		return err
	}
	host.SysRqTriggerAvailable, err = system.Observer.SysRqTriggerAvailable()

	return err
}

// VerifyImage opens and completely verifies one source descriptor.
func (system *System) VerifyImage(path, digest string) (*image.Verified, error) {
	return image.Verify(path, digest)
}

// Confirm compares the complete operator token in constant time.
func (system *System) Confirm(provided, expected string) error {
	if len(provided) != len(expected) || subtle.ConstantTimeCompare(
		[]byte(provided),
		[]byte(expected),
	) != 1 {
		return &policy.GateError{
			Code:    policy.GateConfirmation,
			Message: "confirmation token does not match inspected target and image",
		}
	}

	return nil
}

// Mlockall prevents live-mode code and data from being paged out.
func (system *System) Mlockall() error {
	var limit unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &limit); err != nil {
		return fmt.Errorf("read memlock limit: %w", err)
	}
	limit.Cur = limit.Max
	if err := unix.Setrlimit(unix.RLIMIT_MEMLOCK, &limit); err != nil {
		return fmt.Errorf("raise memlock limit: %w", err)
	}
	if err := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE); err != nil {
		return fmt.Errorf("lock process memory: %w", err)
	}

	return nil
}

// Swapoff disables every active swap area reported by procfs.
func (system *System) Swapoff() error {
	records, err := readSwapRecords(system.Observer.ProcRoot)
	if err != nil {
		return err
	}
	for _, record := range records {
		if err := swapoff(record.source); err != nil {
			return fmt.Errorf("swapoff %s: %w", record.source, err)
		}
	}

	return nil
}

func swapoff(path string) error {
	pointer, err := unix.BytePtrFromString(path)
	if err != nil {
		return fmt.Errorf("encode swap path: %w", err)
	}
	// #nosec G103 -- x/sys v0.47.0 has no typed Swapoff wrapper.
	_, _, errno := unix.Syscall(unix.SYS_SWAPOFF, uintptr(unsafe.Pointer(pointer)), 0, 0)
	runtime.KeepAlive(pointer)
	if errno != 0 {
		return errno
	}

	return nil
}

func writeControl(path, value string) error {
	// #nosec G304 -- paths are fixed procfs control endpoints under Observer.ProcRoot.
	file, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open kernel control %q: %w", path, err)
	}
	if _, err := file.WriteString(value); err != nil {
		_ = file.Close()

		return fmt.Errorf("write kernel control %q: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close kernel control %q: %w", path, err)
	}

	return nil
}

// EnableSysRq enables the kernel SysRq control interface.
func (system *System) EnableSysRq() error {
	return writeControl(filepath.Join(system.Observer.ProcRoot, "sys", "kernel", "sysrq"), "1\n")
}

// RemountReadOnly requests SysRq remount and proves all block mounts read-only.
func (system *System) RemountReadOnly() error {
	if err := writeControl(filepath.Join(system.Observer.ProcRoot, "sysrq-trigger"), "u"); err != nil {
		return fmt.Errorf("SysRq remount read-only: %w", err)
	}
	for range remountPollAttempts {
		readOnly, err := system.Observer.BlockMountsReadOnly()
		if err != nil {
			return err
		}
		if readOnly {
			return nil
		}
		time.Sleep(remountPollInterval)
	}

	return ErrRemountIncomplete
}

// OpenTarget opens a whole block device with no symlink traversal.
func (system *System) OpenTarget(
	path string,
	mode policy.Mode,
	expected policy.TargetIdentity,
) (Target, error) {
	flags := unix.O_WRONLY | unix.O_CLOEXEC | unix.O_NOFOLLOW
	if mode == policy.ModeRescue {
		flags |= unix.O_EXCL
	}
	descriptor, err := unix.Open(path, flags, 0)
	if err != nil {
		return nil, fmt.Errorf("open block target: %w", err)
	}

	var stat unix.Stat_t
	if err := unix.Fstat(descriptor, &stat); err != nil {
		_ = unix.Close(descriptor)

		return nil, fmt.Errorf("stat block target: %w", err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFBLK {
		_ = unix.Close(descriptor)

		return nil, fmt.Errorf("%w: target descriptor is not a block device", ErrInvalidBlockDevice)
	}
	if err := system.validateOpenedTarget(descriptor, expected); err != nil {
		_ = unix.Close(descriptor)

		return nil, err
	}

	return os.NewFile(uintptr(descriptor), path), nil
}

func (system *System) validateOpenedTarget(
	descriptor int,
	expected policy.TargetIdentity,
) error {
	current, err := system.Observer.Target(expected.CanonicalPath)
	if err != nil {
		return identityChanged("observe opened target: %v", err)
	}
	if !sameTarget(current, expected) {
		return identityChanged("sysfs target identity no longer matches confirmation")
	}

	major, minor, err := system.Observer.deviceNumber(expected.KName)
	if err != nil {
		return identityChanged("read opened target device number: %v", err)
	}
	var stat unix.Stat_t
	if err := unix.Fstat(descriptor, &stat); err != nil {
		return fmt.Errorf("stat opened target: %w", err)
	}
	if unix.Major(stat.Rdev) != major || unix.Minor(stat.Rdev) != minor {
		return identityChanged("opened descriptor does not match confirmed sysfs device")
	}

	return nil
}

func (observer Observer) deviceNumber(name string) (uint32, uint32, error) {
	path, err := observer.blockPath(name)
	if err != nil {
		return 0, 0, err
	}
	value, err := readTrimmed(filepath.Join(path, "dev"))
	if err != nil {
		return 0, 0, err
	}
	majorText, minorText, found := strings.Cut(value, ":")
	if !found {
		return 0, 0, fmt.Errorf("%w: device number %q", ErrInvalidProcfs, value)
	}
	parsedMajor, err := strconv.ParseUint(majorText, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: major device number %q", ErrInvalidProcfs, majorText)
	}
	parsedMinor, err := strconv.ParseUint(minorText, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: minor device number %q", ErrInvalidProcfs, minorText)
	}

	return uint32(parsedMajor), uint32(parsedMinor), nil
}

func sameTarget(current, expected policy.TargetIdentity) bool {
	return current.CanonicalPath == expected.CanonicalPath &&
		current.Serial == expected.Serial &&
		current.WWN == expected.WWN &&
		current.SizeBytes == expected.SizeBytes &&
		current.KName == expected.KName &&
		current.IsPartition == expected.IsPartition &&
		slices.Equal(current.Descendants, expected.Descendants)
}

func identityChanged(format string, arguments ...any) error {
	return &policy.GateError{
		Code:    policy.GateIdentityChanged,
		Message: fmt.Sprintf(format, arguments...),
	}
}

// WriteVerified streams the held verified source to the opened target.
func (system *System) WriteVerified(
	ctx context.Context,
	source *image.Verified,
	target Target,
	expectedBytes int64,
	progress func(image.Progress),
) (image.Result, error) {
	return image.Write(ctx, source, target, expectedBytes, progress)
}

// Fdatasync durably flushes file data for the block descriptor.
func (system *System) Fdatasync(target Target) error {
	if err := unix.Fdatasync(int(target.Fd())); err != nil {
		return fmt.Errorf("fdatasync target: %w", err)
	}

	return nil
}

// FlushBlock invalidates buffered block data after the durable write.
func (system *System) FlushBlock(target Target) error {
	if err := unix.IoctlSetInt(int(target.Fd()), unix.BLKFLSBUF, 0); err != nil {
		return fmt.Errorf("flush block buffers: %w", err)
	}

	return nil
}

// SysRqSync requests a global kernel filesystem sync.
func (system *System) SysRqSync() error {
	return writeControl(filepath.Join(system.Observer.ProcRoot, "sysrq-trigger"), "s")
}

// SysRqReboot requests an immediate kernel reboot.
func (system *System) SysRqReboot() error {
	return writeControl(filepath.Join(system.Observer.ProcRoot, "sysrq-trigger"), "b")
}
