package policy

import (
	"fmt"
	"path/filepath"
	"slices"
)

const (
	// MinimumLiveAvailableBytes is the RAM headroom required before mlockall.
	MinimumLiveAvailableBytes int64 = 512 * 1024 * 1024
	deviceIdentityFieldCount        = 2
	tmpfsFilesystem                 = "tmpfs"
)

func refuse(code GateCode, format string, arguments ...any) error {
	return &GateError{Code: code, Message: fmt.Sprintf(format, arguments...)}
}

func deviceNames(target *TargetIdentity) map[string]struct{} {
	names := make(map[string]struct{}, len(target.Descendants)+deviceIdentityFieldCount)
	for _, value := range target.Descendants {
		if value != "" {
			names[filepath.Base(value)] = struct{}{}
		}
	}
	for _, value := range []string{target.KName, target.CanonicalPath} {
		if value != "" {
			names[filepath.Base(value)] = struct{}{}
		}
	}

	return names
}

func intersects(devices map[string]bool, candidates map[string]struct{}) (string, bool) {
	activeDevices := make([]string, 0, len(devices))
	for device, active := range devices {
		if active {
			activeDevices = append(activeDevices, device)
		}
	}
	slices.Sort(activeDevices)

	for _, device := range activeDevices {
		if _, found := candidates[filepath.Base(device)]; found {
			return device, true
		}
	}

	return "", false
}

// Inspect applies pure fail-closed policy to an observed Linux host.
func Inspect(
	mode Mode,
	target TargetIdentity,
	image ImageIdentity,
	host HostObservation,
) (Inspection, error) {
	if mode != ModeRescue && mode != ModeLive {
		return Inspection{}, refuse(GateInvalidMode, "unsupported write mode %q", mode)
	}
	if host.EUID != 0 {
		return Inspection{}, refuse(GateNotRoot, "whole-disk inspection requires root")
	}
	if target.IsPartition {
		return Inspection{}, refuse(GateTargetPartition, "target is a partition")
	}
	if target.SizeBytes < image.UncompressedBytes {
		return Inspection{}, refuse(
			GateTargetTooSmall,
			"target has %d bytes but image expands to %d bytes",
			target.SizeBytes,
			image.UncompressedBytes,
		)
	}

	token, err := ConfirmationToken(target, image)
	if err != nil {
		return Inspection{}, err
	}

	targetDevices := deviceNames(&target)
	if sourceDevice, found := intersects(mapFromSlice(host.SourceBackingDevices), targetDevices); found {
		return Inspection{}, refuse(
			GateSourceOnTarget,
			"source is backed by target descendant %s",
			sourceDevice,
		)
	}

	if err := inspectMode(mode, target, host, targetDevices); err != nil {
		return Inspection{}, err
	}

	target.Descendants = slices.Clone(target.Descendants)

	return Inspection{
		Mode:              mode,
		Target:            target,
		Image:             image,
		ConfirmationToken: token,
	}, nil
}

func inspectMode(
	mode Mode,
	target TargetIdentity,
	host HostObservation,
	targetDevices map[string]struct{},
) error {
	if mode == ModeRescue {
		if mounted, found := intersects(host.MountedDevices, targetDevices); found {
			return refuse(GateTargetMounted, "target descendant %s is mounted", mounted)
		}
		if swap, found := intersects(host.SwapDevices, targetDevices); found {
			return refuse(GateTargetSwap, "target descendant %s is active swap", swap)
		}

		return nil
	}

	if filepath.Clean(host.RootDisk) != target.CanonicalPath {
		return refuse(GateLiveTargetNotRoot, "live target must be the root disk")
	}
	if host.SourceFilesystem != tmpfsFilesystem {
		return refuse(GateLiveSourceNotTmpfs, "live source must reside on tmpfs")
	}
	if host.MemoryAvailableBytes < MinimumLiveAvailableBytes {
		return refuse(
			GateLiveMemory,
			"live mode requires at least %d available bytes",
			MinimumLiveAvailableBytes,
		)
	}
	if !host.SysRqTriggerAvailable {
		return refuse(GateLiveSysRq, "live mode requires a writable SysRq trigger")
	}

	return nil
}

func mapFromSlice(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}

	return result
}
