package diskforge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ioplane/diskforge"
)

func TestConfirmationTokenUsesPublicIdentityContract(t *testing.T) {
	t.Parallel()

	token, err := diskforge.ConfirmationToken(
		diskforge.TargetIdentity{
			CanonicalPath: "/dev/vda",
			Serial:        "SER123",
			WWN:           "WWN456",
			SizeBytes:     42_949_672_960,
		},
		diskforge.ImageIdentity{
			SHA256:            "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			UncompressedBytes: 8_589_934_592,
		},
	)
	if err != nil {
		t.Fatalf("ConfirmationToken() error = %v", err)
	}
	// #nosec G101 -- the confirmation value is a deterministic public API fixture.
	if token != "confirm-v1-vda-0123456789ab-ca99718141349949" {
		t.Fatalf("ConfirmationToken() = %q", token)
	}
}

func TestEngineStagePublishesVerifiedImageWithExplicitOwnership(t *testing.T) {
	t.Parallel()

	content := bytes.Repeat([]byte("disk-image"), 1024)
	digest := sha256.Sum256(content)
	server := httptest.NewServer(httpHandler(content))
	t.Cleanup(server.Close)
	client := server.Client()
	client.Timeout = time.Minute

	engine, err := diskforge.New(diskforge.WithHTTPClient(client))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	destination := filepath.Join(t.TempDir(), "image.raw")
	staged, err := engine.Stage(t.Context(), diskforge.StageRequest{
		URL:          server.URL,
		Destination:  destination,
		SHA256:       hex.EncodeToString(digest[:]),
		MaximumBytes: int64(len(content)),
	})
	if err != nil {
		t.Fatalf("Stage() error = %v", err)
	}
	if staged.Path != destination || staged.Format != "raw" || staged.CompressedBytes != int64(len(content)) {
		t.Fatalf("Stage() = %#v", staged)
	}
	got, err := os.ReadFile(destination)
	if err != nil || !bytes.Equal(got, content) {
		t.Fatalf("staged bytes differ: size=%d error=%v", len(got), err)
	}
	if err := staged.Close(); err != nil {
		t.Fatalf("StagedImage.Close() error = %v", err)
	}
	if err := staged.Close(); err != nil {
		t.Fatalf("second StagedImage.Close() error = %v", err)
	}
}

func TestEngineRejectsCancellationBeforeIO(t *testing.T) {
	t.Parallel()

	engine, err := diskforge.New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := engine.Inspect(ctx, diskforge.InspectRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Inspect() error = %v, want context.Canceled", err)
	}
	if _, err := engine.Stage(ctx, diskforge.StageRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Stage() error = %v, want context.Canceled", err)
	}
	if _, err := engine.Write(ctx, diskforge.WriteRequest{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Write() error = %v, want context.Canceled", err)
	}
}

func TestNewRejectsUnboundedOrNilHTTPClient(t *testing.T) {
	t.Parallel()

	if _, err := diskforge.New(diskforge.WithHTTPClient(nil)); err == nil {
		t.Fatal("New(WithHTTPClient(nil)) error = nil")
	}
	server := httptest.NewServer(httpHandler(nil))
	t.Cleanup(server.Close)
	client := server.Client()
	if _, err := diskforge.New(diskforge.WithHTTPClient(client)); err == nil {
		t.Fatal("New(WithHTTPClient(unbounded)) error = nil")
	}
}

type fixedHandler struct {
	content []byte
}

func httpHandler(content []byte) fixedHandler {
	return fixedHandler{content: content}
}

func (handler fixedHandler) ServeHTTP(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Length", strconv.Itoa(len(handler.content)))
	_, _ = response.Write(handler.content)
}
