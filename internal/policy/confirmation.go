package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ioplane/diskforge/internal/naming"
)

// ConfirmationToken binds destructive consent to target and image identity.
func ConfirmationToken(target TargetIdentity, image ImageIdentity) (string, error) {
	if !filepath.IsAbs(target.CanonicalPath) ||
		filepath.Clean(target.CanonicalPath) != target.CanonicalPath ||
		!strings.HasPrefix(target.CanonicalPath, "/dev/") ||
		target.SizeBytes <= 0 ||
		!naming.ValidLabel(filepath.Base(target.CanonicalPath)) ||
		!validIdentityText(target.Serial) ||
		!validIdentityText(target.WWN) {
		return "", &GateError{
			Code:    GateInvalidTarget,
			Message: "target must be a canonical absolute /dev path with positive capacity",
		}
	}
	if image.UncompressedBytes <= 0 {
		return "", &GateError{
			Code:    GateInvalidImageSize,
			Message: "uncompressed image size must be positive",
		}
	}

	decoded, err := hex.DecodeString(image.SHA256)
	if err != nil ||
		len(decoded) != sha256.Size ||
		len(image.SHA256) != sha256.Size*2 ||
		image.SHA256 != strings.ToLower(image.SHA256) {
		return "", &GateError{
			Code:    GateInvalidDigest,
			Message: "SHA-256 digest must contain exactly 64 lowercase hexadecimal characters",
		}
	}

	record := fmt.Sprintf(
		"v1\n%s\n%s\n%s\n%d\n%d\n%s\n",
		target.CanonicalPath,
		target.Serial,
		target.WWN,
		target.SizeBytes,
		image.UncompressedBytes,
		image.SHA256,
	)
	binding := sha256.Sum256([]byte(record))

	return fmt.Sprintf(
		"confirm-v1-%s-%s-%s",
		filepath.Base(target.CanonicalPath),
		image.SHA256[:12],
		hex.EncodeToString(binding[:8]),
	), nil
}

func validIdentityText(value string) bool {
	const maxIdentityLength = 256

	if len(value) > maxIdentityLength {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x20 || value[index] > 0x7e {
			return false
		}
	}

	return true
}
