package image

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestStagePublishesOnlyCompleteVerifiedDownload(t *testing.T) {
	t.Parallel()

	content := []byte("complete staged image")
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Length", "21")
		_, _ = response.Write(content)
	}))
	t.Cleanup(server.Close)
	destination := filepath.Join(t.TempDir(), "image.raw.zst")

	verified, err := Stage(
		t.Context(),
		server.Client(),
		server.URL,
		destination,
		digestOf(content),
		int64(len(content)),
	)
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	t.Cleanup(func() {
		if closeErr := verified.Close(); closeErr != nil {
			t.Errorf("Verified.Close() error = %v", closeErr)
		}
	})

	if verified.Path != destination || verified.CompressedBytes != int64(len(content)) {
		t.Fatalf("Stage() = %#v", verified)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("published content = %q, %v", got, err)
	}
}

func TestStagePreservesDestinationAndRemovesPartialOnFailure(t *testing.T) {
	t.Parallel()

	tests := map[string]http.HandlerFunc{
		"non-2xx": func(response http.ResponseWriter, _ *http.Request) {
			response.WriteHeader(http.StatusForbidden)
		},
		"truncated": func(response http.ResponseWriter, _ *http.Request) {
			response.Header().Set("Content-Length", "100")
			_, _ = response.Write([]byte("short"))
		},
		"wrong digest": func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte("wrong"))
		},
		"over limit": func(response http.ResponseWriter, _ *http.Request) {
			_, _ = response.Write([]byte("expected-plus-one"))
		},
	}

	for name, handler := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(handler)
			t.Cleanup(server.Close)
			directory := t.TempDir()
			destination := filepath.Join(directory, "image.raw.zst")
			if err := os.WriteFile(destination, []byte("existing"), 0o600); err != nil {
				t.Fatal(err)
			}

			_, err := Stage(
				t.Context(),
				server.Client(),
				server.URL,
				destination,
				digestOf([]byte("expected")),
				int64(len("expected")),
			)
			if err == nil {
				t.Fatal("Stage() error = nil")
			}
			got, readErr := os.ReadFile(destination)
			if readErr != nil || string(got) != "existing" {
				t.Fatalf("destination after failure = %q, %v", got, readErr)
			}
			partials, globErr := filepath.Glob(filepath.Join(directory, ".image.raw.zst.partial-*"))
			if globErr != nil || len(partials) != 0 {
				t.Fatalf("partial files = %#v, %v", partials, globErr)
			}
		})
	}
}

func TestStageHonorsCancellationBeforeCreatingPartial(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = response.Write([]byte("unexpected"))
	}))
	t.Cleanup(server.Close)
	directory := t.TempDir()
	destination := filepath.Join(directory, "image.raw")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Stage(ctx, server.Client(), server.URL, destination, digestOf(nil), 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Stage() error = %v, want context.Canceled", err)
	}
	partials, globErr := filepath.Glob(filepath.Join(directory, ".image.raw.partial-*"))
	if globErr != nil || len(partials) != 0 {
		t.Fatalf("partial files = %#v, %v", partials, globErr)
	}
}
