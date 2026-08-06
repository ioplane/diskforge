// Package image verifies, stages, and streams disk image sources.
package image

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// Format identifies the encoded source representation.
type Format string

const (
	FormatRaw  Format = "raw"
	FormatZstd Format = "zstd"
)

var (
	ErrInvalidArgument   = errors.New("invalid image argument")
	ErrInvalidDigest     = errors.New("invalid image SHA-256 digest")
	ErrInvalidSource     = errors.New("invalid image source")
	ErrDigestMismatch    = errors.New("image SHA-256 mismatch")
	ErrDownloadRejected  = errors.New("image download rejected")
	ErrSizeMismatch      = errors.New("image size mismatch")
	ErrSourceChanged     = errors.New("verified image source changed")
	ErrUnsupportedFormat = errors.New("unsupported image format")
)

// Verified owns a read-only descriptor whose complete content matched SHA-256.
type Verified struct {
	Path            string
	SHA256          string
	Format          Format
	CompressedBytes int64

	mu   sync.Mutex
	file *os.File
}

// Close releases the held descriptor. Repeated calls are safe.
func (v *Verified) Close() error {
	if v == nil {
		return nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	if v.file == nil {
		return nil
	}

	err := v.file.Close()
	v.file = nil

	return err
}

// Verify opens path once and retains that descriptor after complete hashing.
func Verify(path, expectedSHA256 string) (*Verified, error) {
	expected, err := validateDigest(expectedSHA256)
	if err != nil {
		return nil, err
	}

	// #nosec G304 -- opening the caller-selected local image is this API's purpose.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image source: %w", err)
	}
	keepOpen := false
	defer func() {
		if !keepOpen {
			_ = file.Close()
		}
	}()

	verified, err := verifyDescriptor(file, path, expected)
	if err != nil {
		return nil, err
	}
	keepOpen = true

	return verified, nil
}

func validateDigest(value string) (string, error) {
	decoded, err := hex.DecodeString(value)
	if err != nil ||
		len(decoded) != sha256.Size ||
		len(value) != sha256.Size*2 ||
		value != strings.ToLower(value) {
		return "", fmt.Errorf(
			"%w: must contain exactly 64 lowercase hexadecimal characters",
			ErrInvalidDigest,
		)
	}

	return value, nil
}

func verifyDescriptor(file *os.File, path, expectedSHA256 string) (*Verified, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat image source: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: must be a regular file", ErrInvalidSource)
	}
	if _, seekErr := file.Seek(0, io.SeekStart); seekErr != nil {
		return nil, fmt.Errorf("rewind image source: %w", seekErr)
	}

	hasher := sha256.New()
	bytesRead, err := io.Copy(hasher, file)
	if err != nil {
		return nil, fmt.Errorf("hash complete image: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedSHA256 {
		return nil, fmt.Errorf("%w: got %s", ErrDigestMismatch, actual)
	}

	format, err := descriptorFormat(file)
	if err != nil {
		return nil, err
	}

	return &Verified{
		Path:            path,
		SHA256:          actual,
		Format:          format,
		CompressedBytes: bytesRead,
		file:            file,
	}, nil
}

func descriptorFormat(file *os.File) (Format, error) {
	zstdMagic := [4]byte{0x28, 0xb5, 0x2f, 0xfd}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind image source: %w", err)
	}

	header := make([]byte, len(zstdMagic))
	headerBytes, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read image header: %w", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewind image source: %w", err)
	}

	if headerBytes == len(zstdMagic) && [4]byte(header) == zstdMagic {
		return FormatZstd, nil
	}

	return FormatRaw, nil
}
