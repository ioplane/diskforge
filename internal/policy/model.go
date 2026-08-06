// Package policy applies pure fail-closed disk safety decisions.
package policy

import "fmt"

// Mode selects an internal safety policy.
type Mode string

const (
	ModeRescue Mode = "rescue"
	ModeLive   Mode = "live"
)

// TargetIdentity is the internal target identity transport value.
type TargetIdentity struct {
	CanonicalPath string
	Serial        string
	WWN           string
	SizeBytes     int64
	KName         string
	IsPartition   bool
	Descendants   []string
}

// ImageIdentity is the internal verified image transport value.
type ImageIdentity struct {
	Path              string
	SHA256            string
	Format            string
	CompressedBytes   int64
	UncompressedBytes int64
}

// HostObservation is the internal read-only host state transport value.
type HostObservation struct {
	EUID                  int
	RootDisk              string
	MountedDevices        map[string]bool
	SwapDevices           map[string]bool
	SourceFilesystem      string
	SourceBackingDevices  []string
	MemoryAvailableBytes  int64
	SysRqTriggerAvailable bool
}

// Inspection is an internal immutable safety decision.
type Inspection struct {
	Mode              Mode
	Target            TargetIdentity
	Image             ImageIdentity
	ConfirmationToken string
}

// GateCode is an internal machine-readable safety refusal.
type GateCode string

const (
	GateInvalidTarget      GateCode = "invalid_target"
	GateInvalidDigest      GateCode = "invalid_digest"
	GateInvalidImageSize   GateCode = "invalid_image_size"
	GateInvalidMode        GateCode = "invalid_mode"
	GateNotRoot            GateCode = "not_root"
	GateTargetPartition    GateCode = "target_partition"
	GateTargetTooSmall     GateCode = "target_too_small"
	GateTargetMounted      GateCode = "target_mounted"
	GateTargetSwap         GateCode = "target_swap"
	GateSourceOnTarget     GateCode = "source_on_target"
	GateLiveTargetNotRoot  GateCode = "live_target_not_root"
	GateLiveSourceNotTmpfs GateCode = "live_source_not_tmpfs"
	GateLiveMemory         GateCode = "live_memory"
	GateLiveSysRq          GateCode = "live_sysrq"
)

// GateError is an internal fail-closed refusal.
type GateError struct {
	Code    GateCode
	Message string
}

// Error returns the stable code followed by the human-readable message.
func (e *GateError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
