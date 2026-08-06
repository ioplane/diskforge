package diskforge

import "fmt"

// GateCode is a stable machine-readable safety refusal.
type GateCode string

const (
	// GateInvalidTarget rejects a malformed or nonportable target identity.
	GateInvalidTarget GateCode = "invalid_target"
	// GateInvalidDigest rejects a malformed SHA-256 digest.
	GateInvalidDigest GateCode = "invalid_digest"
	// GateInvalidImageSize rejects a nonpositive expanded image size.
	GateInvalidImageSize GateCode = "invalid_image_size"
	// GateInvalidMode rejects an unsupported write mode.
	GateInvalidMode GateCode = "invalid_mode"
	// GateNotRoot rejects an operation without root privileges.
	GateNotRoot GateCode = "not_root"
	// GateTargetPartition rejects a partition where a whole disk is required.
	GateTargetPartition GateCode = "target_partition"
	// GateTargetTooSmall rejects a target smaller than the expanded image.
	GateTargetTooSmall GateCode = "target_too_small"
	// GateTargetMounted rejects a mounted rescue-mode target descendant.
	GateTargetMounted GateCode = "target_mounted"
	// GateTargetSwap rejects an active rescue-mode swap descendant.
	GateTargetSwap GateCode = "target_swap"
	// GateSourceOnTarget rejects a source backed by the target disk.
	GateSourceOnTarget GateCode = "source_on_target"
	// GateLiveTargetNotRoot rejects a live-mode target other than the root disk.
	GateLiveTargetNotRoot GateCode = "live_target_not_root"
	// GateLiveSourceNotTmpfs rejects a live-mode source outside tmpfs.
	GateLiveSourceNotTmpfs GateCode = "live_source_not_tmpfs"
	// GateLiveMemory rejects insufficient live-mode memory headroom.
	GateLiveMemory GateCode = "live_memory"
	// GateLiveSysRq rejects an unavailable live-mode SysRq trigger.
	GateLiveSysRq GateCode = "live_sysrq"
	// GateLiveReboot reports the terminal live-mode reboot requirement.
	GateLiveReboot GateCode = "live_reboot_required"
	// GateConfirmation rejects a confirmation token mismatch.
	GateConfirmation GateCode = "confirmation_mismatch"
	// GateIdentityChanged rejects a target identity changed after inspection.
	GateIdentityChanged GateCode = "identity_changed"
)

// GateError reports a fail-closed refusal and its optional underlying cause.
type GateError struct {
	Code    GateCode `json:"code"`
	Message string   `json:"message"`
	Cause   error    `json:"-"`
}

// Error returns the stable code followed by the human-readable message.
func (e *GateError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Is matches another GateError by non-empty gate code.
func (e *GateError) Is(target error) bool {
	other, ok := target.(*GateError)

	return ok && other.Code != "" && e.Code == other.Code
}

// Unwrap returns the operational cause, when present.
func (e *GateError) Unwrap() error {
	return e.Cause
}
