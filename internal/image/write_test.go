package image

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

type syncBuffer struct {
	bytes.Buffer

	syncCalls int
	syncError error
}

func (b *syncBuffer) Sync() error {
	b.syncCalls++

	return b.syncError
}

func sourceFile(t *testing.T, content []byte, format Format) *Verified {
	t.Helper()

	path := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(path, digestOf(content))
	if err != nil {
		t.Fatal(err)
	}
	if verified.Format != format {
		t.Fatalf("Verify() format = %q, want %q", verified.Format, format)
	}
	t.Cleanup(func() {
		if closeErr := verified.Close(); closeErr != nil {
			t.Errorf("Verified.Close() error = %v", closeErr)
		}
	})

	return verified
}

func zstdBytes(t *testing.T, content []byte) []byte {
	t.Helper()

	encoder, err := zstd.NewWriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { encoder.Close() })

	return encoder.EncodeAll(content, nil)
}

func TestWriteStreamsRawAndZstdWithMonotonicProgress(t *testing.T) {
	t.Parallel()

	content := bytes.Repeat([]byte("disk-sector"), 700_000)
	for name, source := range map[string]*Verified{
		"raw":  sourceFile(t, content, FormatRaw),
		"zstd": sourceFile(t, zstdBytes(t, content), FormatZstd),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			destination := &syncBuffer{}
			progress := []Progress{}

			result, err := Write(
				t.Context(),
				source,
				destination,
				int64(len(content)),
				func(update Progress) { progress = append(progress, update) },
			)
			if err != nil {
				t.Fatalf("Write() error = %v", err)
			}
			if !bytes.Equal(destination.Bytes(), content) {
				t.Fatal("Write() output differs from source")
			}
			if result.WrittenBytes != int64(len(content)) || destination.syncCalls != 1 {
				t.Fatalf("Write() = %#v, sync calls = %d", result, destination.syncCalls)
			}
			if len(progress) < 2 || progress[0].WrittenBytes != 0 {
				t.Fatalf("progress = %#v", progress)
			}
			for index := 1; index < len(progress); index++ {
				if progress[index].WrittenBytes < progress[index-1].WrittenBytes {
					t.Fatalf("progress moved backwards: %#v", progress)
				}
			}
			if progress[len(progress)-1].WrittenBytes != int64(len(content)) {
				t.Fatalf("final progress = %#v", progress[len(progress)-1])
			}
		})
	}
}

func TestWriteRejectsCorruptAndWrongLengthStreams(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		source        *Verified
		expectedBytes int64
	}{
		"corrupt zstd": {source: sourceFile(t, []byte{0x28, 0xb5, 0x2f, 0xfd, 0xff}, FormatZstd), expectedBytes: 4},
		"too few":      {source: sourceFile(t, []byte("short"), FormatRaw), expectedBytes: 6},
		"too many":     {source: sourceFile(t, []byte("longer"), FormatRaw), expectedBytes: 5},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			destination := &syncBuffer{}
			if _, err := Write(t.Context(), test.source, destination, test.expectedBytes, nil); err == nil {
				t.Fatal("Write() error = nil")
			}
			if destination.syncCalls != 0 {
				t.Fatalf("Sync called %d times after failed write", destination.syncCalls)
			}
			if int64(destination.Len()) > test.expectedBytes {
				t.Fatalf("wrote %d bytes, maximum is %d", destination.Len(), test.expectedBytes)
			}
		})
	}
}

func TestWritePropagatesCancellationAndSyncFailure(t *testing.T) {
	t.Parallel()

	source := sourceFile(t, []byte("complete"), FormatRaw)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Write(ctx, source, &syncBuffer{}, 8, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("Write() error = %v, want context.Canceled", err)
	}

	syncError := errors.New("sync failed")
	destination := &syncBuffer{syncError: syncError}
	if _, err := Write(t.Context(), source, destination, 8, nil); !errors.Is(err, syncError) {
		t.Fatalf("Write() error = %v, want sync failure", err)
	}
}

func TestWriteUsesHeldDescriptorAfterPathReplacement(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "image.raw")
	original := []byte("verified-original")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(path, digestOf(original))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := verified.Close(); closeErr != nil {
			t.Errorf("Verified.Close() error = %v", closeErr)
		}
	})
	if err := os.Rename(path, path+".verified"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("untrusted-replace"), 0o600); err != nil {
		t.Fatal(err)
	}

	destination := &syncBuffer{}
	if _, err := Write(t.Context(), verified, destination, int64(len(original)), nil); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if !bytes.Equal(destination.Bytes(), original) {
		t.Fatalf("written bytes = %q, want held verified source %q", destination.Bytes(), original)
	}
}

func TestWriteRejectsInPlaceMutationOfHeldDescriptor(t *testing.T) {
	t.Parallel()

	directory := t.TempDir()
	path := filepath.Join(directory, "image.raw")
	original := []byte("verified-original")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	verified, err := Verify(path, digestOf(original))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := verified.Close(); closeErr != nil {
			t.Errorf("Verified.Close() error = %v", closeErr)
		}
	})
	if err := os.WriteFile(path, []byte("tampered-original"), 0o600); err != nil {
		t.Fatal(err)
	}

	destination := &syncBuffer{}
	if _, err := Write(t.Context(), verified, destination, int64(len(original)), nil); err == nil {
		t.Fatal("Write() error = nil after source mutation")
	}
	if destination.Len() != 0 || destination.syncCalls != 0 {
		t.Fatalf("destination changed before refusal: bytes=%d sync=%d", destination.Len(), destination.syncCalls)
	}
}

func FuzzWriteNeverExceedsExpected(f *testing.F) {
	f.Add([]byte("complete"), uint16(8))
	f.Add([]byte("too-long"), uint16(3))
	f.Add([]byte{}, uint16(0))

	f.Fuzz(func(t *testing.T, content []byte, expectedSeed uint16) {
		expectedBytes := int64(expectedSeed%1024) + 1
		path := filepath.Join(t.TempDir(), "source.img")
		if err := os.WriteFile(path, content, 0o600); err != nil {
			t.Fatal(err)
		}
		source, err := Verify(path, digestOf(content))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if closeErr := source.Close(); closeErr != nil {
				t.Errorf("Verified.Close() error = %v", closeErr)
			}
		}()

		destination := &syncBuffer{}
		_, writeErr := Write(t.Context(), source, destination, expectedBytes, nil)
		if int64(destination.Len()) > expectedBytes {
			t.Fatalf("wrote %d bytes, maximum is %d", destination.Len(), expectedBytes)
		}
		if writeErr != nil && destination.syncCalls != 0 {
			t.Fatalf("Sync called %d times after failed write", destination.syncCalls)
		}
		if writeErr == nil && destination.syncCalls != 1 {
			t.Fatalf("Sync called %d times after successful write", destination.syncCalls)
		}
	})
}
