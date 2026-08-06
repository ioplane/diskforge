package image

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func digestOf(content []byte) string {
	digest := sha256.Sum256(content)

	return hex.EncodeToString(digest[:])
}

func TestVerifyReadsCompleteRawAndZstdSources(t *testing.T) {
	t.Parallel()

	for name, test := range map[string]struct {
		content []byte
		format  Format
	}{
		"raw":  {content: []byte("raw disk bytes"), format: FormatRaw},
		"zstd": {content: append([]byte{0x28, 0xb5, 0x2f, 0xfd}, []byte("frame")...), format: FormatZstd},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "image")
			if err := os.WriteFile(path, test.content, 0o600); err != nil {
				t.Fatal(err)
			}

			verified, err := Verify(path, digestOf(test.content))
			if err != nil {
				t.Fatalf("Verify() error = %v", err)
			}
			t.Cleanup(func() {
				if closeErr := verified.Close(); closeErr != nil {
					t.Errorf("Verified.Close() error = %v", closeErr)
				}
			})

			if verified.Path != path || verified.SHA256 != digestOf(test.content) {
				t.Fatalf("Verify() = %#v", verified)
			}
			if verified.CompressedBytes != int64(len(test.content)) || verified.Format != test.format {
				t.Fatalf("Verify() = %#v", verified)
			}
		})
	}
}

func TestVerifyRejectsMalformedAndWrongDigest(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "image.raw")
	if err := os.WriteFile(path, []byte("complete"), 0o600); err != nil {
		t.Fatal(err)
	}

	for name, digest := range map[string]string{
		"malformed": "not-a-sha256",
		"uppercase": strings.Repeat("A", 64),
		"wrong":     strings.Repeat("0", 64),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := Verify(path, digest); err == nil {
				t.Fatal("Verify() error = nil")
			}
		})
	}
}

func TestVerifiedCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	content := []byte("complete")
	path := filepath.Join(t.TempDir(), "image.raw")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}

	verified, err := Verify(path, digestOf(content))
	if err != nil {
		t.Fatal(err)
	}
	if err := verified.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := verified.Close(); err != nil {
		t.Fatalf("second Close() error = %v", err)
	}

	if _, err := Write(t.Context(), verified, &syncBuffer{}, int64(len(content)), nil); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Write() error = %v, want os.ErrClosed", err)
	}
}
