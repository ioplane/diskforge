// Package version reports immutable binary build metadata.
package version

import "runtime"

// GoReleaser injects these immutable values with linker -X flags.
var version = "dev"

var commit = "none" //nolint:gochecknoglobals // Linker-injected release metadata.

var buildDate = "unknown" //nolint:gochecknoglobals // Linker-injected release metadata.

// Info is the stable JSON representation of binary build metadata.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

// Current returns linker values together with the running Go platform.
func Current() Info {
	return Info{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}
}
