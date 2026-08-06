package image

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

const (
	copyBufferBytes    = 4 * 1024 * 1024
	decoderMemoryBytes = 64 * 1024 * 1024
)

// Progress reports monotonically increasing expanded bytes written.
type Progress struct {
	WrittenBytes  int64
	ExpectedBytes int64
}

// Result reports a completed, synchronized write.
type Result struct {
	WrittenBytes int64
}

// SyncWriter is a destination that can durably synchronize completed bytes.
type SyncWriter interface {
	io.Writer
	Sync() error
}

type contextReader struct {
	done         <-chan struct{}
	contextError func() error
	reader       io.Reader
}

func (r contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.done:
		return 0, r.contextError()
	default:
		return r.reader.Read(buffer)
	}
}

func readerWithContext(ctx context.Context, reader io.Reader) contextReader {
	return contextReader{done: ctx.Done(), contextError: ctx.Err, reader: reader}
}

type progressWriter struct {
	destination io.Writer
	expected    int64
	written     int64
	callback    func(Progress)
}

func (w *progressWriter) Write(buffer []byte) (int, error) {
	written, err := w.destination.Write(buffer)
	w.written += int64(written)
	if written > 0 && w.callback != nil {
		w.callback(Progress{WrittenBytes: w.written, ExpectedBytes: w.expected})
	}

	return written, err
}

// Write streams one held verified source into an already gated destination.
func Write(
	ctx context.Context,
	source *Verified,
	destination SyncWriter,
	expectedBytes int64,
	progress func(Progress),
) (Result, error) {
	if err := validateWriteInputs(ctx, source, destination, expectedBytes); err != nil {
		return Result{}, err
	}

	source.mu.Lock()
	defer source.mu.Unlock()
	if source.file == nil {
		return Result{}, fmt.Errorf("verified source descriptor is unavailable: %w", os.ErrClosed)
	}
	if err := reverifySource(ctx, source); err != nil {
		return Result{}, err
	}
	reader, closeDecoder, err := expandedReader(ctx, source)
	if err != nil {
		return Result{}, err
	}
	defer closeDecoder()

	if progress != nil {
		progress(Progress{ExpectedBytes: expectedBytes})
	}
	reporter := &progressWriter{
		destination: destination,
		expected:    expectedBytes,
		callback:    progress,
	}
	written, err := io.CopyBuffer(
		reporter,
		io.LimitReader(reader, expectedBytes),
		make([]byte, copyBufferBytes),
	)
	if err != nil {
		return Result{}, fmt.Errorf("write expanded image: %w", err)
	}
	if written != expectedBytes {
		return Result{}, fmt.Errorf(
			"%w: expanded image is too short: got %d bytes, want %d",
			ErrSizeMismatch,
			written,
			expectedBytes,
		)
	}

	extra := make([]byte, 1)
	extraBytes, err := io.ReadFull(reader, extra)
	if extraBytes > 0 {
		return Result{}, fmt.Errorf("%w: expanded image exceeds %d bytes", ErrSizeMismatch, expectedBytes)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return Result{}, fmt.Errorf("finish expanded image: %w", err)
	}
	if err := destination.Sync(); err != nil {
		return Result{WrittenBytes: written}, fmt.Errorf("sync destination: %w", err)
	}

	return Result{WrittenBytes: written}, nil
}

func validateWriteInputs(
	ctx context.Context,
	source *Verified,
	destination SyncWriter,
	expectedBytes int64,
) error {
	if ctx == nil {
		return fmt.Errorf("%w: context is required", ErrInvalidArgument)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if source == nil {
		return fmt.Errorf("%w: verified source is required", ErrInvalidArgument)
	}
	if destination == nil {
		return fmt.Errorf("%w: destination is required", ErrInvalidArgument)
	}
	if expectedBytes <= 0 {
		return fmt.Errorf(
			"%w: expected expanded byte count must be positive",
			ErrInvalidArgument,
		)
	}

	return nil
}

func reverifySource(ctx context.Context, source *Verified) error {
	info, err := source.file.Stat()
	if err != nil {
		return fmt.Errorf("stat verified source: %w", err)
	}
	if info.Size() != source.CompressedBytes {
		return fmt.Errorf(
			"%w: got %d bytes, want %d",
			ErrSourceChanged,
			info.Size(),
			source.CompressedBytes,
		)
	}
	if _, err := source.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind verified source: %w", err)
	}

	hasher := sha256.New()
	if _, err := io.CopyBuffer(
		hasher,
		readerWithContext(ctx, source.file),
		make([]byte, copyBufferBytes),
	); err != nil {
		return fmt.Errorf("reverify source content: %w", err)
	}
	actual := hex.EncodeToString(hasher.Sum(nil))
	if actual != source.SHA256 {
		return fmt.Errorf("%w: got SHA-256 %s", ErrSourceChanged, actual)
	}
	if _, err := source.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind verified source: %w", err)
	}

	return nil
}

func expandedReader(ctx context.Context, source *Verified) (io.Reader, func(), error) {
	reader := readerWithContext(ctx, source.file)
	if source.Format == FormatRaw {
		return reader, func() {}, nil
	}
	if source.Format != FormatZstd {
		return nil, func() {}, fmt.Errorf("%w: %q", ErrUnsupportedFormat, source.Format)
	}

	decoder, err := zstd.NewReader(
		reader,
		zstd.WithDecoderConcurrency(1),
		zstd.WithDecoderMaxMemory(decoderMemoryBytes),
		zstd.WithDecoderMaxWindow(decoderMemoryBytes),
		zstd.WithDecoderLowmem(true),
	)
	if err != nil {
		return nil, func() {}, fmt.Errorf("create zstd decoder: %w", err)
	}

	return decoder, decoder.Close, nil
}
