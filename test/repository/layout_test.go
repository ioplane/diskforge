package repository_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestContainerDefinitionsHaveOwnedLocation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, path := range []string{
		"deployments/containers/development.Containerfile",
		"deployments/containers/release.Containerfile",
	} {
		assertPathExists(t, root, path)
	}

	for _, path := range []string{
		"Containerfile.dev",
		"Containerfile.release",
	} {
		assertPathAbsent(t, root, path)
	}
}

func TestIntegrationTestHasOwnedLocation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, path := range []string{
		"test/integration/diskforge_test.go",
		"test/integration/testdata/proc-swaps",
	} {
		assertPathExists(t, root, path)
	}

	for _, path := range []string{
		"integration_test.go",
		"testdata",
	} {
		assertPathAbsent(t, root, path)
	}
}

func TestToolConfigurationHasOwnedLocation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, path := range []string{
		".config/cspell.json",
		".config/golangci.yml",
		".config/goreleaser.yaml",
		".config/markdownlint-cli2.yaml",
		".config/yamllint.yml",
	} {
		assertPathExists(t, root, path)
	}

	for _, path := range []string{
		".cspell.json",
		".golangci.yml",
		".goreleaser.yaml",
		".markdownlint-cli2.yaml",
		".yamllint.yml",
	} {
		assertPathAbsent(t, root, path)
	}
}

func TestReleaseStateAndArtifactsHaveOwnedLocation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, path := range []string{
		".config/release-please/config.json",
		".config/release-please/manifest.json",
	} {
		assertPathExists(t, root, path)
	}

	for _, path := range []string{
		".release-please-manifest.json",
		"release-please-config.json",
		"dist",
	} {
		assertPathAbsent(t, root, path)
	}
}

func TestCommunityAndArchitectureDocsHaveOwnedLocation(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	for _, path := range []string{
		".github/CODE_OF_CONDUCT.md",
		".github/CONTRIBUTING.md",
		".github/GOVERNANCE.md",
		".github/MAINTAINERS.md",
		".github/SECURITY.md",
		".github/SUPPORT.md",
		"docs/architecture/README.md",
	} {
		assertPathExists(t, root, path)
	}

	for _, path := range []string{
		"CODE_OF_CONDUCT.md",
		"CONTRIBUTING.md",
		"GOVERNANCE.md",
		"MAINTAINERS.md",
		"SECURITY.md",
		"SUPPORT.md",
		"docs/architecture.md",
	} {
		assertPathAbsent(t, root, path)
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()

	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the layout test path")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func assertPathExists(t *testing.T, root, path string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Errorf("required repository path %q: %v", path, err)
	}
}

func assertPathAbsent(t *testing.T, root, path string) {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(path))); err == nil {
		t.Errorf("obsolete repository path %q still exists", path)
	} else if !os.IsNotExist(err) {
		t.Errorf("inspect obsolete repository path %q: %v", path, err)
	}
}
