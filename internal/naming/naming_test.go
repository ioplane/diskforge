package naming

import "testing"

func TestValidSemverTag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "initial release", value: "v0.1.0", want: true},
		{name: "stable release", value: "v1.0.0", want: true},
		{name: "multi digit components", value: "v10.20.30", want: true},
		{name: "missing prefix", value: "0.1.0", want: false},
		{name: "uppercase prefix", value: "V0.1.0", want: false},
		{name: "leading zero major", value: "v01.2.3", want: false},
		{name: "leading zero minor", value: "v1.02.3", want: false},
		{name: "leading zero patch", value: "v1.2.03", want: false},
		{name: "prerelease", value: "v0.1.0-rc.1", want: false},
		{name: "build metadata", value: "v0.1.0+build.1", want: false},
		{name: "ambiguous short version", value: "v1.2", want: false},
		{name: "ambiguous long version", value: "v1.2.3.4", want: false},
		{name: "path", value: "v1/2/3", want: false},
		{name: "space", value: "v1.2.3 ", want: false},
		{name: "empty", value: "", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := ValidSemverTag(test.value); got != test.want {
				t.Fatalf("ValidSemverTag(%q) = %t, want %t", test.value, got, test.want)
			}
		})
	}
}

func TestArtifactName(t *testing.T) {
	t.Parallel()

	got, err := ArtifactName("v0.1.0", "linux", "amd64", "tar.gz")
	if err != nil {
		t.Fatalf("ArtifactName() error = %v", err)
	}

	const want = "diskforge_0.1.0_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("ArtifactName() = %q, want %q", got, want)
	}
}

func TestArtifactNameRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		version   string
		goos      string
		goarch    string
		extension string
	}{
		{name: "invalid version", version: "1.0.0", goos: "linux", goarch: "amd64", extension: "tar.gz"},
		{name: "prerelease", version: "v0.1.0-rc.1", goos: "linux", goarch: "amd64", extension: "tar.gz"},
		{name: "uppercase operating system", version: "v1.0.0", goos: "Linux", goarch: "amd64", extension: "tar.gz"},
		{name: "operating system path", version: "v1.0.0", goos: "../linux", goarch: "amd64", extension: "tar.gz"},
		{name: "architecture path", version: "v1.0.0", goos: "linux", goarch: "amd64/x", extension: "tar.gz"},
		{name: "architecture space", version: "v1.0.0", goos: "linux", goarch: "amd 64", extension: "tar.gz"},
		{name: "architecture underscore", version: "v1.0.0", goos: "linux", goarch: "x86_64", extension: "tar.gz"},
		{name: "leading extension dot", version: "v1.0.0", goos: "linux", goarch: "amd64", extension: ".tar.gz"},
		{name: "extension traversal", version: "v1.0.0", goos: "linux", goarch: "amd64", extension: "../gz"},
		{name: "extension path", version: "v1.0.0", goos: "linux", goarch: "amd64", extension: "tar/gz"},
		{name: "uppercase extension", version: "v1.0.0", goos: "linux", goarch: "amd64", extension: "TAR.GZ"},
		{name: "empty operating system", version: "v1.0.0", goos: "", goarch: "amd64", extension: "tar.gz"},
		{name: "empty architecture", version: "v1.0.0", goos: "linux", goarch: "", extension: "tar.gz"},
		{name: "empty extension", version: "v1.0.0", goos: "linux", goarch: "amd64", extension: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got, err := ArtifactName(test.version, test.goos, test.goarch, test.extension); err == nil {
				t.Fatalf("ArtifactName() = %q, want error", got)
			}
		})
	}
}

func TestValidLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  bool
	}{
		{value: "vda", want: true},
		{value: "dm-0", want: true},
		{value: "386", want: true},
		{value: "dm_0", want: false},
		{value: "VDA", want: false},
		{value: "-vda", want: false},
		{value: "vda-", want: false},
		{value: "", want: false},
	}

	for _, test := range tests {
		if got := ValidLabel(test.value); got != test.want {
			t.Errorf("ValidLabel(%q) = %t, want %t", test.value, got, test.want)
		}
	}
}
