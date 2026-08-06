package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
)

const (
	destinationDirectoryMode os.FileMode = 0o755
	stagedImageMode          os.FileMode = 0o600
)

// Stage downloads into a same-directory partial and atomically publishes it.
func Stage(
	ctx context.Context,
	client *http.Client,
	sourceURL string,
	destination string,
	expectedSHA256 string,
	maximumBytes int64,
) (*Verified, error) {
	expected, err := validateStageInputs(ctx, client, expectedSHA256, maximumBytes)
	if err != nil {
		return nil, err
	}

	response, err := download(ctx, client, sourceURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if err := validateResponse(response, maximumBytes); err != nil {
		return nil, err
	}

	return publishResponse(
		response.Body,
		response.ContentLength,
		destination,
		expected,
		maximumBytes,
	)
}

func validateStageInputs(
	ctx context.Context,
	client *http.Client,
	expectedSHA256 string,
	maximumBytes int64,
) (string, error) {
	if ctx == nil {
		return "", fmt.Errorf("%w: context is required", ErrInvalidArgument)
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if client == nil {
		return "", fmt.Errorf("%w: HTTP client is required", ErrInvalidArgument)
	}
	if maximumBytes <= 0 || maximumBytes == math.MaxInt64 {
		return "", fmt.Errorf(
			"%w: maximum download size must be positive and bounded",
			ErrInvalidArgument,
		)
	}

	return validateDigest(expectedSHA256)
}

func download(ctx context.Context, client *http.Client, sourceURL string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("create download request: %w", err)
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}

	return response, nil
}

func validateResponse(response *http.Response, maximumBytes int64) error {
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("%w: HTTP status %d", ErrDownloadRejected, response.StatusCode)
	}
	if response.ContentLength > maximumBytes {
		return fmt.Errorf("%w: download exceeds %d bytes", ErrSizeMismatch, maximumBytes)
	}

	return nil
}

func publishResponse(
	body io.Reader,
	contentLength int64,
	destination string,
	expectedSHA256 string,
	maximumBytes int64,
) (*Verified, error) {
	directory := filepath.Dir(destination)
	if mkdirErr := os.MkdirAll(directory, destinationDirectoryMode); mkdirErr != nil {
		return nil, fmt.Errorf("create destination directory: %w", mkdirErr)
	}

	partial, err := os.CreateTemp(directory, "."+filepath.Base(destination)+".partial-*")
	if err != nil {
		return nil, fmt.Errorf("create partial image: %w", err)
	}
	partialPath := partial.Name()
	keepOpen := false
	defer func() {
		if !keepOpen {
			_ = partial.Close()
		}
		_ = os.Remove(partialPath)
	}()
	if chmodErr := partial.Chmod(stagedImageMode); chmodErr != nil {
		return nil, fmt.Errorf("set partial image permissions: %w", chmodErr)
	}

	hasher := sha256.New()
	written, err := io.Copy(
		io.MultiWriter(partial, hasher),
		io.LimitReader(body, maximumBytes+1),
	)
	if err != nil {
		return nil, fmt.Errorf("download image: %w", err)
	}
	if written > maximumBytes {
		return nil, fmt.Errorf("%w: download exceeds %d bytes", ErrSizeMismatch, maximumBytes)
	}
	if contentLength >= 0 && written != contentLength {
		return nil, fmt.Errorf(
			"%w: download length mismatch: got %d, want %d",
			ErrSizeMismatch,
			written,
			contentLength,
		)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != expectedSHA256 {
		return nil, fmt.Errorf("%w: got %s", ErrDigestMismatch, actual)
	}
	if syncErr := partial.Sync(); syncErr != nil {
		return nil, fmt.Errorf("sync partial image: %w", syncErr)
	}

	verified, err := verifyDescriptor(partial, destination, expectedSHA256)
	if err != nil {
		return nil, err
	}
	if renameErr := os.Rename(partialPath, destination); renameErr != nil {
		return nil, fmt.Errorf("publish staged image: %w", renameErr)
	}
	if syncErr := syncDirectory(directory); syncErr != nil {
		return nil, syncErr
	}

	keepOpen = true

	return verified, nil
}

func syncDirectory(path string) error {
	// #nosec G304 -- path is the parent of the caller-selected stage destination.
	directory, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open destination directory: %w", err)
	}
	if err := directory.Sync(); err != nil {
		_ = directory.Close()

		return fmt.Errorf("sync destination directory: %w", err)
	}
	if err := directory.Close(); err != nil {
		return fmt.Errorf("close destination directory: %w", err)
	}

	return nil
}
