// Package linux implements Linux host observation and guarded disk execution.
package linux

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/ioplane/diskforge/internal/policy"
)

const (
	deviceRoot            = "/dev"
	sectorBytes     int64 = 512
	kilobyteBytes   int64 = 1024
	mountFieldCount       = 6
	swapFieldCount        = 2
)

var (
	ErrInvalidBlockDevice = errors.New("invalid block device")
	ErrInvalidProcfs      = errors.New("invalid procfs data")
	ErrUnresolvedDevice   = errors.New("unresolved block device")
)

// Observer is the read-only procfs and sysfs boundary used before policy runs.
type Observer struct {
	ProcRoot string
	SysRoot  string
}

type mountRecord struct {
	device     string
	mountpoint string
	filesystem string
	readOnly   bool
}

func (observer Observer) blockRoot() string {
	return filepath.Join(observer.SysRoot, "class", "block")
}

func readTrimmed(path string) (string, error) {
	// #nosec G304 -- procfs and sysfs roots are explicit Observer dependencies.
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read kernel state %q: %w", path, err)
	}

	return strings.TrimSpace(string(content)), nil
}

func (observer Observer) topLevelNames() ([]string, error) {
	entries, err := os.ReadDir(observer.blockRoot())
	if err != nil {
		return nil, fmt.Errorf("read sysfs block directory: %w", err)
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	return names, nil
}

func (observer Observer) blockPath(name string) (string, error) {
	direct := filepath.Join(observer.blockRoot(), name)
	if _, err := os.Stat(direct); err == nil {
		return direct, nil
	}

	parents, err := observer.topLevelNames()
	if err != nil {
		return "", err
	}
	for _, parent := range parents {
		candidate := filepath.Join(observer.blockRoot(), parent, name)
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("%w: %q is absent from sysfs", ErrUnresolvedDevice, name)
}

func (observer Observer) partitionParent(name string) (string, bool, error) {
	parents, err := observer.topLevelNames()
	if err != nil {
		return "", false, err
	}
	for _, parent := range parents {
		partition := filepath.Join(observer.blockRoot(), parent, name, "partition")
		if _, statErr := os.Stat(partition); statErr == nil {
			return parent, true, nil
		}
	}

	return "", false, nil
}

func sortedEntries(path string) ([]string, error) {
	directory, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read block relations %q: %w", path, err)
	}

	result := make([]string, 0, len(directory))
	for _, entry := range directory {
		result = append(result, entry.Name())
	}
	slices.Sort(result)

	return result, nil
}

func (observer Observer) related(start string) ([]string, error) {
	seen := map[string]bool{}
	if err := observer.walkRelated(start, true, seen); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(seen))
	for name := range seen {
		result = append(result, name)
	}
	slices.Sort(result)

	return result, nil
}

func (observer Observer) walkRelated(
	name string,
	includePartitions bool,
	seen map[string]bool,
) error {
	if seen[name] {
		return nil
	}
	seen[name] = true

	path, err := observer.blockPath(name)
	if err != nil {
		return err
	}
	parent, found, err := observer.partitionParent(name)
	if err != nil {
		return err
	}
	if found {
		if walkErr := observer.walkRelated(parent, false, seen); walkErr != nil {
			return walkErr
		}
	}

	slaves, err := sortedEntries(filepath.Join(path, "slaves"))
	if err != nil {
		return err
	}
	for _, slave := range slaves {
		if walkErr := observer.walkRelated(slave, false, seen); walkErr != nil {
			return walkErr
		}
	}
	if !includePartitions {
		return nil
	}

	partitions, err := partitionChildren(path)
	if err != nil {
		return err
	}
	for _, partition := range partitions {
		if walkErr := observer.walkRelated(partition, false, seen); walkErr != nil {
			return walkErr
		}
	}

	return nil
}

func partitionChildren(path string) ([]string, error) {
	children, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("read block children %q: %w", path, err)
	}

	partitions := []string{}
	for _, child := range children {
		marker := filepath.Join(path, child.Name(), "partition")
		if _, statErr := os.Stat(marker); statErr == nil {
			partitions = append(partitions, child.Name())
		}
	}

	return partitions, nil
}

// Target reads stable block identity without opening the target writable.
func (observer Observer) Target(path string) (policy.TargetIdentity, error) {
	cleaned := filepath.Clean(path)
	name := filepath.Base(cleaned)
	if cleaned != filepath.Join(deviceRoot, name) || name == "." || name == string(os.PathSeparator) {
		return policy.TargetIdentity{}, fmt.Errorf("%w: expected canonical /dev/<name> path", ErrInvalidBlockDevice)
	}

	sysPath, err := observer.blockPath(name)
	if err != nil {
		return policy.TargetIdentity{}, err
	}
	sectorsText, err := readTrimmed(filepath.Join(sysPath, "size"))
	if err != nil {
		return policy.TargetIdentity{}, err
	}
	sectors, err := strconv.ParseInt(sectorsText, 10, 64)
	if err != nil || sectors < 0 || sectors > (int64(^uint64(0)>>1)/sectorBytes) {
		return policy.TargetIdentity{}, fmt.Errorf("%w: sector count for %s", ErrInvalidBlockDevice, name)
	}
	descendants, err := observer.related(name)
	if err != nil {
		return policy.TargetIdentity{}, err
	}
	_, partitionErr := os.Stat(filepath.Join(sysPath, "partition"))

	return policy.TargetIdentity{
		CanonicalPath: cleaned,
		Serial:        readOptional(sysPath, "device/serial", "serial"),
		WWN:           readOptional(sysPath, "device/wwid", "wwid"),
		SizeBytes:     sectors * sectorBytes,
		KName:         name,
		IsPartition:   partitionErr == nil,
		Descendants:   descendants,
	}, nil
}

func readOptional(root string, paths ...string) string {
	for _, relative := range paths {
		value, err := readTrimmed(filepath.Join(root, relative))
		if err == nil && value != "" {
			return value
		}
	}

	return ""
}

func unescapeMount(value string) string {
	return strings.NewReplacer(
		`\040`, " ",
		`\011`, "\t",
		`\012`, "\n",
		`\134`, `\`,
	).Replace(value)
}

func (observer Observer) mounts() ([]mountRecord, error) {
	path := filepath.Join(observer.ProcRoot, "self", "mountinfo")
	// #nosec G304 -- procfs root is an explicit Observer dependency.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open mountinfo: %w", err)
	}
	defer file.Close()

	records := []mountRecord{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		record, err := parseMountRecord(scanner.Text())
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan mountinfo: %w", err)
	}

	return records, nil
}

func parseMountRecord(line string) (mountRecord, error) {
	fields := strings.Fields(line)
	separator := slices.Index(fields, "-")
	if len(fields) < mountFieldCount || separator < 0 || separator+3 >= len(fields) {
		return mountRecord{}, fmt.Errorf("%w: mountinfo line %q", ErrInvalidProcfs, line)
	}

	return mountRecord{
		device:     fields[2],
		mountpoint: unescapeMount(fields[4]),
		filesystem: fields[separator+1],
		readOnly: containsOption(fields[5], "ro") ||
			containsOption(fields[separator+3], "ro"),
	}, nil
}

func containsOption(options, expected string) bool {
	return slices.Contains(strings.Split(options, ","), expected)
}

// BlockMountsReadOnly proves that a block-backed mount exists and all are read-only.
func (observer Observer) BlockMountsReadOnly() (bool, error) {
	mounts, err := observer.mounts()
	if err != nil {
		return false, err
	}
	found := false
	for _, mount := range mounts {
		_, backedByBlock, err := observer.nameByDeviceNumber(mount.device)
		if err != nil {
			return false, err
		}
		if !backedByBlock {
			continue
		}
		found = true
		if !mount.readOnly {
			return false, nil
		}
	}

	return found, nil
}

func (observer Observer) nameByDeviceNumber(device string) (string, bool, error) {
	parents, err := observer.topLevelNames()
	if err != nil {
		return "", false, err
	}
	for _, parent := range parents {
		parentPath := filepath.Join(observer.blockRoot(), parent)
		paths := []string{parentPath}
		children, err := os.ReadDir(parentPath)
		if err != nil {
			return "", false, fmt.Errorf("read block device %q: %w", parentPath, err)
		}
		for _, child := range children {
			paths = append(paths, filepath.Join(parentPath, child.Name()))
		}
		for _, path := range paths {
			value, readErr := readTrimmed(filepath.Join(path, "dev"))
			if readErr == nil && value == device {
				return filepath.Base(path), true, nil
			}
		}
	}

	return "", false, nil
}

func (observer Observer) expand(result map[string]bool, name string) error {
	related, err := observer.related(name)
	if err != nil {
		return err
	}
	for _, device := range related {
		result[device] = true
	}

	return nil
}

// MountedDevices returns mounted device names plus backing dependencies.
func (observer Observer) MountedDevices() (map[string]bool, error) {
	records, err := observer.mounts()
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, record := range records {
		name, found, err := observer.nameByDeviceNumber(record.device)
		if err != nil {
			return nil, err
		}
		if found {
			if err := observer.expand(result, name); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// SwapDevices returns active swap device names plus backing dependencies.
// File-backed swap is resolved through the filesystem that contains the file.
func (observer Observer) SwapDevices() (map[string]bool, error) {
	path := filepath.Join(observer.ProcRoot, "swaps")
	// #nosec G304 -- procfs root is an explicit Observer dependency.
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open swaps: %w", err)
	}
	defer file.Close()

	result := map[string]bool{}
	scanner := bufio.NewScanner(file)
	first := true
	for scanner.Scan() {
		if first {
			first = false
			continue
		}
		fields := strings.Fields(scanner.Text())
		if len(fields) < swapFieldCount {
			return nil, fmt.Errorf("%w: swaps line %q", ErrInvalidProcfs, scanner.Text())
		}
		source := unescapeMount(fields[0])
		switch fields[1] {
		case "partition":
			if err := observer.expand(result, filepath.Base(source)); err != nil {
				return nil, err
			}
		case "file":
			_, backing, err := observer.Source(source)
			if err != nil {
				return nil, fmt.Errorf("resolve swap file %q: %w", source, err)
			}
			for _, name := range backing {
				result[name] = true
			}
		default:
			return nil, fmt.Errorf(
				"%w: unsupported swap type %q",
				ErrInvalidProcfs,
				fields[1],
			)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan swaps: %w", err)
	}

	return result, nil
}

// RootDisk resolves the root mount through partitions and device-mapper slaves.
func (observer Observer) RootDisk() (string, error) {
	records, err := observer.mounts()
	if err != nil {
		return "", err
	}
	for _, record := range records {
		if record.mountpoint == "/" {
			return observer.physicalRootDisk(record.device)
		}
	}

	return "", fmt.Errorf("%w: root mount is absent from mountinfo", ErrInvalidProcfs)
}

func (observer Observer) physicalRootDisk(device string) (string, error) {
	name, found, err := observer.nameByDeviceNumber(device)
	if err != nil {
		return "", fmt.Errorf("resolve root device %s: %w", device, err)
	}
	if !found {
		return "", fmt.Errorf("%w: root device %s", ErrUnresolvedDevice, device)
	}
	related, err := observer.related(name)
	if err != nil {
		return "", err
	}

	candidates := []string{}
	for _, candidate := range related {
		path, err := observer.blockPath(candidate)
		if err != nil {
			return "", err
		}
		_, partitionErr := os.Stat(filepath.Join(path, "partition"))
		slaves, err := sortedEntries(filepath.Join(path, "slaves"))
		if err != nil {
			return "", err
		}
		if partitionErr != nil && len(slaves) == 0 && !strings.HasPrefix(candidate, "dm-") {
			candidates = append(candidates, candidate)
		}
	}
	if len(candidates) != 1 {
		return "", fmt.Errorf("%w: root resolves to %d physical disks", ErrUnresolvedDevice, len(candidates))
	}

	return filepath.Join(deviceRoot, candidates[0]), nil
}

func pathWithin(path, mountpoint string) bool {
	if mountpoint == "/" {
		return filepath.IsAbs(path)
	}

	return path == mountpoint || strings.HasPrefix(path, mountpoint+string(os.PathSeparator))
}

// Source returns the source filesystem and all backing block-device names.
func (observer Observer) Source(path string) (string, []string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) {
		return "", nil, fmt.Errorf("%w: source path must be absolute", ErrInvalidBlockDevice)
	}
	records, err := observer.mounts()
	if err != nil {
		return "", nil, err
	}

	var selected *mountRecord
	for index := range records {
		record := &records[index]
		if pathWithin(cleaned, record.mountpoint) &&
			(selected == nil || len(record.mountpoint) > len(selected.mountpoint)) {
			selected = record
		}
	}
	if selected == nil {
		return "", nil, fmt.Errorf("%w: no mount contains source %q", ErrUnresolvedDevice, cleaned)
	}

	name, found, err := observer.nameByDeviceNumber(selected.device)
	if err != nil {
		return "", nil, err
	}
	if !found {
		return selected.filesystem, nil, nil
	}
	backing, err := observer.related(name)

	return selected.filesystem, backing, err
}

// MemoryAvailableBytes parses the kernel's reclaim-aware memory estimate.
func (observer Observer) MemoryAvailableBytes() (int64, error) {
	path := filepath.Join(observer.ProcRoot, "meminfo")
	// #nosec G304 -- procfs root is an explicit Observer dependency.
	file, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open meminfo: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 3 && fields[0] == "MemAvailable:" && fields[2] == "kB" {
			kilobytes, parseErr := strconv.ParseInt(fields[1], 10, 64)
			if parseErr != nil || kilobytes < 0 {
				return 0, fmt.Errorf("%w: MemAvailable", ErrInvalidProcfs)
			}

			return kilobytes * kilobyteBytes, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan meminfo: %w", err)
	}

	return 0, fmt.Errorf("%w: MemAvailable is absent", ErrInvalidProcfs)
}

// SysRqTriggerAvailable verifies that control and trigger interfaces exist.
func (observer Observer) SysRqTriggerAvailable() (bool, error) {
	if _, err := readTrimmed(filepath.Join(observer.ProcRoot, "sys", "kernel", "sysrq")); err != nil {
		return false, err
	}
	info, err := os.Stat(filepath.Join(observer.ProcRoot, "sysrq-trigger"))
	if err != nil {
		return false, fmt.Errorf("stat SysRq trigger: %w", err)
	}

	return info.Mode().Perm()&0o222 != 0, nil
}
