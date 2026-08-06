package policy

import (
	"errors"
	"testing"
)

func TestConfirmationTokenIsStableAndHumanAuditable(t *testing.T) {
	t.Parallel()

	target := TargetIdentity{
		CanonicalPath: "/dev/vda",
		Serial:        "SER123",
		WWN:           "WWN456",
		SizeBytes:     42_949_672_960,
	}
	image := ImageIdentity{
		SHA256:            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		UncompressedBytes: 8_589_934_592,
	}

	token, err := ConfirmationToken(target, image)
	if err != nil {
		t.Fatalf("ConfirmationToken() error = %v", err)
	}

	const want = "confirm-v1-vda-0123456789ab-ca99718141349949"
	if token != want {
		t.Fatalf("ConfirmationToken() = %q, want %q", token, want)
	}
}

func TestConfirmationTokenChangesForEveryBoundIdentityField(t *testing.T) {
	t.Parallel()

	baseTarget := TargetIdentity{
		CanonicalPath: "/dev/vda",
		Serial:        "SER123",
		WWN:           "WWN456",
		SizeBytes:     42_949_672_960,
	}
	baseImage := ImageIdentity{
		SHA256:            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		UncompressedBytes: 8_589_934_592,
	}
	base, err := ConfirmationToken(baseTarget, baseImage)
	if err != nil {
		t.Fatalf("ConfirmationToken() error = %v", err)
	}

	tests := map[string]struct {
		target TargetIdentity
		image  ImageIdentity
	}{
		"canonical path": {
			target: TargetIdentity{
				CanonicalPath: "/dev/vdb",
				Serial:        "SER123",
				WWN:           "WWN456",
				SizeBytes:     42_949_672_960,
			},
			image: baseImage,
		},
		"serial": {
			target: TargetIdentity{
				CanonicalPath: "/dev/vda",
				Serial:        "SER124",
				WWN:           "WWN456",
				SizeBytes:     42_949_672_960,
			},
			image: baseImage,
		},
		"wwn": {
			target: TargetIdentity{
				CanonicalPath: "/dev/vda",
				Serial:        "SER123",
				WWN:           "WWN457",
				SizeBytes:     42_949_672_960,
			},
			image: baseImage,
		},
		"target bytes": {
			target: TargetIdentity{
				CanonicalPath: "/dev/vda",
				Serial:        "SER123",
				WWN:           "WWN456",
				SizeBytes:     42_949_672_961,
			},
			image: baseImage,
		},
		"image bytes": {
			target: baseTarget,
			image: ImageIdentity{
				SHA256:            baseImage.SHA256,
				UncompressedBytes: baseImage.UncompressedBytes + 1,
			},
		},
		"digest": {
			target: baseTarget,
			image: ImageIdentity{
				SHA256:            "1123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
				UncompressedBytes: baseImage.UncompressedBytes,
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			got, tokenErr := ConfirmationToken(test.target, test.image)
			if tokenErr != nil {
				t.Fatalf("ConfirmationToken() error = %v", tokenErr)
			}
			if got == base {
				t.Fatalf("ConfirmationToken() did not change for %s", name)
			}
		})
	}
}

func TestConfirmationTokenRejectsMalformedIdentity(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		target TargetIdentity
		image  ImageIdentity
		code   GateCode
	}{
		"relative target": {
			target: TargetIdentity{CanonicalPath: "dev/vda", SizeBytes: 2},
			image: ImageIdentity{
				SHA256:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				UncompressedBytes: 1,
			},
			code: GateInvalidTarget,
		},
		"malformed digest": {
			target: TargetIdentity{CanonicalPath: "/dev/vda", SizeBytes: 2},
			image:  ImageIdentity{SHA256: "not-a-digest", UncompressedBytes: 1},
			code:   GateInvalidDigest,
		},
		"uppercase digest": {
			target: TargetIdentity{CanonicalPath: "/dev/vda", SizeBytes: 2},
			image: ImageIdentity{
				SHA256:            "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
				UncompressedBytes: 1,
			},
			code: GateInvalidDigest,
		},
		"control character in serial": {
			target: TargetIdentity{
				CanonicalPath: "/dev/vda",
				Serial:        "SER123\nWWN456",
				SizeBytes:     2,
			},
			image: ImageIdentity{
				SHA256:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				UncompressedBytes: 1,
			},
			code: GateInvalidTarget,
		},
		"nonportable device label": {
			target: TargetIdentity{CanonicalPath: "/dev/dm_0", SizeBytes: 2},
			image: ImageIdentity{
				SHA256:            "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				UncompressedBytes: 1,
			},
			code: GateInvalidTarget,
		},
		"nonpositive image": {
			target: TargetIdentity{CanonicalPath: "/dev/vda", SizeBytes: 2},
			image: ImageIdentity{
				SHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			},
			code: GateInvalidImageSize,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			_, err := ConfirmationToken(test.target, test.image)
			var gate *GateError
			if !errors.As(err, &gate) {
				t.Fatalf("ConfirmationToken() error = %v, want *GateError", err)
			}
			if gate.Code != test.code {
				t.Fatalf("GateError.Code = %q, want %q", gate.Code, test.code)
			}
		})
	}
}
