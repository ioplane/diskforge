package version

import (
	"bytes"
	"encoding/json"
	"runtime"
	"testing"
)

//nolint:paralleltest // The test temporarily replaces linker-injected globals.
func TestCurrentReportsLinkerAndRuntimeMetadata(t *testing.T) {
	originalVersion, originalCommit, originalDate := version, commit, buildDate
	t.Cleanup(func() {
		version, commit, buildDate = originalVersion, originalCommit, originalDate
	})
	version = "v1.2.3"
	commit = "0123456789abcdef"
	buildDate = "2026-08-06T00:00:00Z"

	info := Current()
	if info.Version != version || info.Commit != commit || info.BuildDate != buildDate {
		t.Fatalf("Current() = %#v", info)
	}
	if info.GoVersion != runtime.Version() || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		t.Fatalf("Current() runtime = %#v", info)
	}
	encoded, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	const want = `"version":"v1.2.3"`
	if !bytes.Contains(encoded, []byte(want)) {
		t.Fatalf("json.Marshal(Current()) = %s", encoded)
	}
}
