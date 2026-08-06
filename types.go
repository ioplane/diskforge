// Package diskforge provides fail-closed disk image inspection and writing.
package diskforge

// Mode selects the safety policy for a whole-disk write.
type Mode string

const (
	// ModeRescue requires every target descendant to be unused.
	ModeRescue Mode = "rescue"
	// ModeLive applies the in-memory root-disk replacement policy.
	ModeLive Mode = "live"
)

// TargetIdentity is the stable identity and observed capacity of a block device.
type TargetIdentity struct {
	CanonicalPath string   `json:"canonical_path"`
	Serial        string   `json:"serial"`
	WWN           string   `json:"wwn"`
	SizeBytes     int64    `json:"size_bytes"`
	KName         string   `json:"kernel_name"`
	IsPartition   bool     `json:"is_partition"`
	Descendants   []string `json:"descendants"`
}

// ImageIdentity describes a verified source and its expanded disk size.
type ImageIdentity struct {
	Path              string `json:"path"`
	SHA256            string `json:"sha256"`
	Format            string `json:"format"`
	CompressedBytes   int64  `json:"compressed_bytes"`
	UncompressedBytes int64  `json:"uncompressed_bytes"`
}

// HostObservation contains read-only Linux state consumed by safety policy.
type HostObservation struct {
	EUID                  int             `json:"euid"`
	RootDisk              string          `json:"root_disk"`
	MountedDevices        map[string]bool `json:"mounted_devices"`
	SwapDevices           map[string]bool `json:"swap_devices"`
	SourceFilesystem      string          `json:"source_filesystem"`
	SourceBackingDevices  []string        `json:"source_backing_devices"`
	MemoryAvailableBytes  int64           `json:"memory_available_bytes"`
	SysRqTriggerAvailable bool            `json:"sysrq_trigger_available"`
}

// Inspection is an immutable safety decision consumed by a write operation.
type Inspection struct {
	Mode              Mode           `json:"mode"`
	Target            TargetIdentity `json:"target"`
	Image             ImageIdentity  `json:"image"`
	ConfirmationToken string         `json:"confirmation_token"`
}

// InspectRequest identifies the target and source to evaluate without writing.
type InspectRequest struct {
	Mode          Mode   `json:"mode"`
	TargetPath    string `json:"target_path"`
	ImagePath     string `json:"image_path"`
	SHA256        string `json:"sha256"`
	ExpectedBytes int64  `json:"expected_bytes"`
}

// StageRequest describes one bounded atomic image download.
type StageRequest struct {
	URL          string `json:"url"`
	Destination  string `json:"destination"`
	SHA256       string `json:"sha256"`
	MaximumBytes int64  `json:"maximum_bytes"`
}

// StagedImage owns the descriptor retained after a verified atomic download.
// Call Close when the metadata has been consumed.
type StagedImage struct {
	Path            string `json:"path"`
	SHA256          string `json:"sha256"`
	Format          string `json:"format"`
	CompressedBytes int64  `json:"compressed_bytes"`

	close func() error
}

// WriteRequest describes a guarded whole-disk write or complete dry run.
type WriteRequest struct {
	Mode          Mode   `json:"mode"`
	TargetPath    string `json:"target_path"`
	ImagePath     string `json:"image_path"`
	SHA256        string `json:"sha256"`
	ExpectedBytes int64  `json:"expected_bytes"`
	Confirmation  string `json:"confirmation,omitempty"`
	Reboot        bool   `json:"reboot"`
	DryRun        bool   `json:"dry_run"`
}

// Progress reports monotonically increasing expanded bytes written.
type Progress struct {
	WrittenBytes  int64 `json:"written_bytes"`
	ExpectedBytes int64 `json:"expected_bytes"`
}

// WriteResult reports either a verified dry run or a durable write.
type WriteResult struct {
	WrittenBytes int64      `json:"written_bytes"`
	DryRun       bool       `json:"dry_run"`
	Inspection   Inspection `json:"inspection"`
}
