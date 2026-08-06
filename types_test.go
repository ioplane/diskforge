package diskforge_test

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ioplane/diskforge"
)

func TestPublicIdentityJSONUsesStableFieldNames(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(diskforge.TargetIdentity{
		CanonicalPath: "/dev/vda",
		KName:         "vda",
		Serial:        "SER123",
		WWN:           "WWN456",
		SizeBytes:     1024,
		Descendants:   []string{"vda", "vda1"},
	})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	for _, field := range []string{
		`"canonical_path"`,
		`"kernel_name"`,
		`"serial"`,
		`"wwn"`,
		`"size_bytes"`,
		`"descendants"`,
	} {
		if !strings.Contains(string(encoded), field) {
			t.Fatalf("json.Marshal() = %s, missing %s", encoded, field)
		}
	}
}

func TestGateErrorSupportsCodeAndCauseMatching(t *testing.T) {
	t.Parallel()

	err := &diskforge.GateError{
		Code:    diskforge.GateNotRoot,
		Message: "root is required",
		Cause:   io.ErrClosedPipe,
	}

	if !errors.Is(err, &diskforge.GateError{Code: diskforge.GateNotRoot}) {
		t.Fatal("errors.Is() did not match the gate code")
	}
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal("errors.Is() did not match the wrapped cause")
	}
	if got := err.Error(); got != "not_root: root is required" {
		t.Fatalf("GateError.Error() = %q", got)
	}
}
