package linux

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func linuxFixture(t *testing.T) Observer {
	t.Helper()

	root := t.TempDir()
	if err := os.CopyFS(filepath.Join(root, "proc"), os.DirFS("testdata/proc")); err != nil {
		t.Fatalf("copy proc fixture: %v", err)
	}
	write := func(relative, content string) {
		t.Helper()

		path := filepath.Join(root, "sys", "class", "block", relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir fixture: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}
	write("vda/dev", "252:0\n")
	write("vda/size", "83886080\n")
	write("vda/device/serial", "VIRTIO-SERIAL\n")
	write("vda/device/wwid", "virtio-WWN\n")
	write("vda/vda1/dev", "252:1\n")
	write("vda/vda1/size", "2048\n")
	write("vda/vda1/partition", "1\n")
	write("vda/vda2/dev", "252:2\n")
	write("vda/vda2/size", "67108864\n")
	write("vda/vda2/partition", "2\n")
	write("sda/dev", "8:0\n")
	write("sda/size", "167772160\n")
	write("sda/device/serial", "SCSI-SERIAL\n")
	write("sda/wwid", "naa.5000\n")
	write("nvme0n1/dev", "259:0\n")
	write("nvme0n1/size", "335544320\n")
	write("nvme0n1/device/serial", "NVME-SERIAL\n")
	write("nvme0n1/wwid", "eui.1234\n")
	write("dm-0/dev", "253:0\n")
	write("dm-0/size", "67108864\n")
	write("dm-0/slaves/vda2", "\n")
	write("dm-0/slaves/dm-1", "\n")
	write("dm-1/dev", "253:1\n")
	write("dm-1/size", "67108864\n")
	write("dm-1/slaves/dm-0", "\n")

	return Observer{
		ProcRoot: filepath.Join(root, "proc"),
		SysRoot:  filepath.Join(root, "sys"),
	}
}

func TestObserverReadsBlockIdentityAndDependencies(t *testing.T) {
	t.Parallel()

	observer := linuxFixture(t)
	target, err := observer.Target("/dev/vda")
	if err != nil {
		t.Fatalf("Target() error = %v", err)
	}
	if target.CanonicalPath != "/dev/vda" || target.KName != "vda" {
		t.Fatalf("Target() identity = %#v", target)
	}
	if target.SizeBytes != 42_949_672_960 || target.Serial != "VIRTIO-SERIAL" || target.WWN != "virtio-WWN" {
		t.Fatalf("Target() hardware fields = %#v", target)
	}
	if !reflect.DeepEqual(target.Descendants, []string{"vda", "vda1", "vda2"}) {
		t.Fatalf("Target().Descendants = %#v", target.Descendants)
	}

	partition, err := observer.Target("/dev/vda2")
	if err != nil || !partition.IsPartition {
		t.Fatalf("Target(vda2) = %#v, %v", partition, err)
	}
	for path, serial := range map[string]string{
		"/dev/sda":     "SCSI-SERIAL",
		"/dev/nvme0n1": "NVME-SERIAL",
	} {
		identity, identityErr := observer.Target(path)
		if identityErr != nil || identity.Serial != serial || identity.WWN == "" {
			t.Fatalf("Target(%s) = %#v, %v", path, identity, identityErr)
		}
	}

	dm, err := observer.Target("/dev/dm-0")
	if err != nil {
		t.Fatalf("Target(dm-0) error = %v", err)
	}
	if !reflect.DeepEqual(dm.Descendants, []string{"dm-0", "dm-1", "vda", "vda2"}) {
		t.Fatalf("Target(dm-0).Descendants = %#v", dm.Descendants)
	}
}

func TestObserverReadsHostSafetyState(t *testing.T) {
	t.Parallel()

	observer := linuxFixture(t)
	mounted, err := observer.MountedDevices()
	if err != nil {
		t.Fatalf("MountedDevices() error = %v", err)
	}
	for _, name := range []string{"dm-0", "vda", "vda2"} {
		if !mounted[name] {
			t.Fatalf("MountedDevices()[%q] = false; all = %#v", name, mounted)
		}
	}

	swaps, err := observer.SwapDevices()
	if err != nil || !swaps["vda1"] {
		t.Fatalf("SwapDevices() = %#v, %v", swaps, err)
	}
	rootDisk, err := observer.RootDisk()
	if err != nil || rootDisk != "/dev/vda" {
		t.Fatalf("RootDisk() = %q, %v", rootDisk, err)
	}
	filesystem, backing, err := observer.Source("/dev/shm/image.raw.zst")
	if err != nil || filesystem != "tmpfs" || len(backing) != 0 {
		t.Fatalf("Source(tmpfs) = %q, %#v, %v", filesystem, backing, err)
	}
	filesystem, backing, err = observer.Source("/boot/image.raw.zst")
	if err != nil || filesystem != "xfs" || !reflect.DeepEqual(backing, []string{"vda", "vda2"}) {
		t.Fatalf("Source(/boot) = %q, %#v, %v", filesystem, backing, err)
	}
	memory, err := observer.MemoryAvailableBytes()
	if err != nil || memory != 1_073_741_824 {
		t.Fatalf("MemoryAvailableBytes() = %d, %v", memory, err)
	}
	available, err := observer.SysRqTriggerAvailable()
	if err != nil || !available {
		t.Fatalf("SysRqTriggerAvailable() = %t, %v", available, err)
	}
}

func TestObserverProvesEveryBlockMountReadOnly(t *testing.T) {
	t.Parallel()

	observer := linuxFixture(t)
	readOnly, err := observer.BlockMountsReadOnly()
	if err != nil || readOnly {
		t.Fatalf("BlockMountsReadOnly() before SysRq = %t, %v", readOnly, err)
	}

	mountinfo := filepath.Join(observer.ProcRoot, "self", "mountinfo")
	content, err := os.ReadFile(mountinfo)
	if err != nil {
		t.Fatalf("read mountinfo: %v", err)
	}
	content = []byte(strings.ReplaceAll(string(content), " rw\n", " ro\n"))
	// #nosec G703 -- mountinfo is an exact path inside t.TempDir().
	if writeErr := os.WriteFile(mountinfo, content, 0o600); writeErr != nil {
		t.Fatalf("write mountinfo: %v", writeErr)
	}

	readOnly, err = observer.BlockMountsReadOnly()
	if err != nil || !readOnly {
		t.Fatalf("BlockMountsReadOnly() after SysRq = %t, %v", readOnly, err)
	}
}
