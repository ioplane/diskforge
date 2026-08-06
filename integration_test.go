//go:build integration && linux

package diskforge_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ioplane/diskforge"
	"golang.org/x/sys/unix"
)

const (
	integrationImageBytes = 1024 * 1024
	integrationDiskBytes  = 2 * integrationImageBytes
	loopDeviceMajor       = 7
)

//nolint:paralleltest // Loop allocation is host-global.
func TestEngineWritesRawImageToIsolatedLoopDevice(t *testing.T) {
	temporaryDirectory := t.TempDir()
	sourcePath := filepath.Join(temporaryDirectory, "diskforge-integration.raw")
	backingPath := filepath.Join(temporaryDirectory, "diskforge-loop-backing.raw")
	imageContent := bytes.Repeat([]byte("diskforge-integration\n"), integrationImageBytes/22)
	imageContent = append(imageContent, bytes.Repeat([]byte{0xa5}, integrationImageBytes-len(imageContent))...)

	writeSynchronizedFile(t, sourcePath, imageContent)
	createSynchronizedBackingFile(t, backingPath, integrationDiskBytes)
	targetPath := attachLoopDevice(t, backingPath)
	digestBytes := sha256.Sum256(imageContent)
	digest := hex.EncodeToString(digestBytes[:])

	engine, err := diskforge.New()
	if err != nil {
		t.Fatalf("create engine: %v", err)
	}
	request := diskforge.InspectRequest{
		Mode:          diskforge.ModeRescue,
		TargetPath:    targetPath,
		ImagePath:     sourcePath,
		SHA256:        digest,
		ExpectedBytes: int64(len(imageContent)),
	}
	inspection, err := engine.Inspect(context.Background(), request)
	if err != nil {
		t.Fatalf("inspect isolated loop device: %v", err)
	}

	result, err := engine.Write(context.Background(), diskforge.WriteRequest{
		Mode:          request.Mode,
		TargetPath:    request.TargetPath,
		ImagePath:     request.ImagePath,
		SHA256:        request.SHA256,
		ExpectedBytes: request.ExpectedBytes,
		Confirmation:  inspection.ConfirmationToken,
	})
	if err != nil {
		t.Fatalf("write isolated loop device: %v", err)
	}
	if result.WrittenBytes != int64(len(imageContent)) {
		t.Fatalf("written bytes = %d, want %d", result.WrittenBytes, len(imageContent))
	}

	written, err := os.ReadFile(backingPath)
	if err != nil {
		t.Fatalf("read loop backing file: %v", err)
	}
	if !bytes.Equal(written[:len(imageContent)], imageContent) {
		t.Fatal("loop backing file does not contain the verified image")
	}
	if !bytes.Equal(written[len(imageContent):], make([]byte, integrationDiskBytes-len(imageContent))) {
		t.Fatal("write extended beyond the expected image size")
	}
}

func writeSynchronizedFile(t *testing.T, path string, content []byte) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("create image fixture: %v", err)
	}
	if _, err := file.Write(content); err != nil {
		_ = file.Close()
		t.Fatalf("write image fixture: %v", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatalf("synchronize image fixture: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close image fixture: %v", err)
	}
}

func createSynchronizedBackingFile(t *testing.T, path string, size int64) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatalf("create loop backing file: %v", err)
	}
	if err := file.Truncate(size); err != nil {
		_ = file.Close()
		t.Fatalf("size loop backing file: %v", err)
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		t.Fatalf("synchronize loop backing file: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close loop backing file: %v", err)
	}
}

func attachLoopDevice(t *testing.T, backingPath string) string {
	t.Helper()
	controlDescriptor, err := unix.Open("/dev/loop-control", unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open loop control: %v", err)
	}
	defer unix.Close(controlDescriptor)

	deviceNumber, err := unix.IoctlRetInt(controlDescriptor, unix.LOOP_CTL_GET_FREE)
	if err != nil {
		t.Fatalf("allocate loop device number: %v", err)
	}
	devicePath := fmt.Sprintf("/dev/loop%d", deviceNumber)
	createdNode := createLoopDeviceNode(t, devicePath, deviceNumber)

	backingDescriptor, err := unix.Open(backingPath, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open loop backing file: %v", err)
	}
	defer unix.Close(backingDescriptor)

	loopDescriptor, err := unix.Open(devicePath, unix.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("open allocated loop device: %v", err)
	}
	configuration := unix.LoopConfig{Fd: checkedUint32(t, backingDescriptor, "backing descriptor")}
	if err := unix.IoctlLoopConfigure(loopDescriptor, &configuration); err != nil {
		_ = unix.Close(loopDescriptor)
		t.Fatalf("configure allocated loop device: %v", err)
	}
	if err := unix.Close(loopDescriptor); err != nil {
		t.Fatalf("close configured loop device: %v", err)
	}

	t.Cleanup(func() {
		descriptor, openErr := unix.Open(devicePath, unix.O_RDWR|unix.O_CLOEXEC, 0)
		if openErr == nil {
			if detachErr := unix.IoctlSetInt(descriptor, unix.LOOP_CLR_FD, 0); detachErr != nil &&
				!errors.Is(detachErr, unix.ENXIO) {
				t.Errorf("detach loop device: %v", detachErr)
			}
			if closeErr := unix.Close(descriptor); closeErr != nil {
				t.Errorf("close loop device during cleanup: %v", closeErr)
			}
		} else if !errors.Is(openErr, unix.ENOENT) {
			t.Errorf("open loop device during cleanup: %v", openErr)
		}
		if createdNode {
			if removeErr := os.Remove(devicePath); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				t.Errorf("remove loop device node: %v", removeErr)
			}
		}
	})

	return devicePath
}

func createLoopDeviceNode(t *testing.T, path string, deviceNumber int) bool {
	t.Helper()
	if _, err := os.Lstat(path); err == nil {
		return false
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("inspect loop device node: %v", err)
	}
	device := unix.Mkdev(loopDeviceMajor, checkedUint32(t, deviceNumber, "loop device number"))
	if err := unix.Mknod(path, unix.S_IFBLK|0o600, checkedInt(t, device, "encoded device number")); err != nil {
		t.Fatalf("create loop device node: %v", err)
	}

	return true
}

func checkedUint32(t *testing.T, value int, field string) uint32 {
	t.Helper()
	if value < 0 || uint64(value) > uint64(^uint32(0)) {
		t.Fatalf("%s %d cannot be represented as uint32", field, value)
	}

	return uint32(value) //nolint:gosec // Bounds are proven immediately above.
}

func checkedInt(t *testing.T, value uint64, field string) int {
	t.Helper()
	if value > uint64(^uint(0)>>1) {
		t.Fatalf("%s %d cannot be represented as int", field, value)
	}

	return int(value) //nolint:gosec // Bounds are proven immediately above.
}
