package policy

import (
	"errors"
	"testing"
)

func validInspectionInputs() (TargetIdentity, ImageIdentity, HostObservation) {
	target := TargetIdentity{
		CanonicalPath: "/dev/vda",
		KName:         "vda",
		Serial:        "SER123",
		WWN:           "WWN456",
		SizeBytes:     42_949_672_960,
		Descendants:   []string{"vda", "vda1", "vda2"},
	}
	image := ImageIdentity{
		Path:              "/images/polyexit.raw.zst",
		SHA256:            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		CompressedBytes:   2_147_483_648,
		UncompressedBytes: 34_359_738_368,
		Format:            "zstd",
	}
	host := HostObservation{
		EUID:                  0,
		RootDisk:              "/dev/vda",
		MountedDevices:        map[string]bool{},
		SwapDevices:           map[string]bool{},
		SourceFilesystem:      "xfs",
		SourceBackingDevices:  []string{"sda1"},
		MemoryAvailableBytes:  MinimumLiveAvailableBytes,
		SysRqTriggerAvailable: true,
	}

	return target, image, host
}

func TestInspectRejectsFirstUnsafeCondition(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		mode   Mode
		mutate func(*TargetIdentity, *ImageIdentity, *HostObservation)
		code   GateCode
	}{
		"non-root caller": {
			mode: ModeRescue,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.EUID = 1000
			},
			code: GateNotRoot,
		},
		"partition target": {
			mode: ModeRescue,
			mutate: func(target *TargetIdentity, _ *ImageIdentity, _ *HostObservation) {
				target.IsPartition = true
			},
			code: GateTargetPartition,
		},
		"undersized target": {
			mode: ModeRescue,
			mutate: func(target *TargetIdentity, image *ImageIdentity, _ *HostObservation) {
				target.SizeBytes = image.UncompressedBytes - 1
			},
			code: GateTargetTooSmall,
		},
		"mounted descendant": {
			mode: ModeRescue,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.MountedDevices["vda2"] = true
			},
			code: GateTargetMounted,
		},
		"swap descendant": {
			mode: ModeRescue,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.SwapDevices["vda2"] = true
			},
			code: GateTargetSwap,
		},
		"wrong live root disk": {
			mode: ModeLive,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.RootDisk = "/dev/vdb"
			},
			code: GateLiveTargetNotRoot,
		},
		"live source outside tmpfs": {
			mode:   ModeLive,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, _ *HostObservation) {},
			code:   GateLiveSourceNotTmpfs,
		},
		"source backed by target": {
			mode: ModeRescue,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.SourceBackingDevices = []string{"vda2"}
			},
			code: GateSourceOnTarget,
		},
		"insufficient live RAM": {
			mode: ModeLive,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.SourceFilesystem = "tmpfs"
				host.MemoryAvailableBytes = MinimumLiveAvailableBytes - 1
			},
			code: GateLiveMemory,
		},
		"SysRq unavailable": {
			mode: ModeLive,
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, host *HostObservation) {
				host.SourceFilesystem = "tmpfs"
				host.SysRqTriggerAvailable = false
			},
			code: GateLiveSysRq,
		},
		"malformed digest": {
			mode: ModeRescue,
			mutate: func(_ *TargetIdentity, image *ImageIdentity, _ *HostObservation) {
				image.SHA256 = "bad"
			},
			code: GateInvalidDigest,
		},
		"unsupported mode": {
			mode:   Mode("automatic"),
			mutate: func(_ *TargetIdentity, _ *ImageIdentity, _ *HostObservation) {},
			code:   GateInvalidMode,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			target, image, host := validInspectionInputs()
			test.mutate(&target, &image, &host)
			_, err := Inspect(test.mode, target, image, host)
			var gate *GateError
			if !errors.As(err, &gate) {
				t.Fatalf("Inspect() error = %v, want *GateError", err)
			}
			if gate.Code != test.code {
				t.Fatalf("GateError.Code = %q, want %q", gate.Code, test.code)
			}
		})
	}
}

func TestInspectAcceptsRescueAndLivePolicies(t *testing.T) {
	t.Parallel()

	for _, mode := range []Mode{ModeRescue, ModeLive} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()

			target, image, host := validInspectionInputs()
			if mode == ModeLive {
				image.Path = "/dev/shm/polyexit.raw.zst"
				host.SourceFilesystem = "tmpfs"
				host.MountedDevices["vda2"] = true
				host.SwapDevices["vda2"] = true
			}

			inspection, err := Inspect(mode, target, image, host)
			if err != nil {
				t.Fatalf("Inspect() error = %v", err)
			}
			if inspection.Mode != mode || inspection.Target.CanonicalPath != "/dev/vda" {
				t.Fatalf("Inspect() = %#v", inspection)
			}
			if inspection.ConfirmationToken == "" {
				t.Fatal("Inspect() returned an empty confirmation token")
			}
		})
	}
}

func TestInspectClonesMutableIdentityFields(t *testing.T) {
	t.Parallel()

	target, image, host := validInspectionInputs()
	inspection, err := Inspect(ModeRescue, target, image, host)
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}

	target.Descendants[0] = "mutated"
	if got := inspection.Target.Descendants[0]; got != "vda" {
		t.Fatalf("Inspection.Target.Descendants[0] = %q, want %q", got, "vda")
	}
}
