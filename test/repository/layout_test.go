package repository_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestTrackedTopLevelLayout(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	unexpected := unexpectedTopLevelEntries(
		trackedRepositoryPaths(t, root),
		allowedTopLevelEntries(),
	)
	if len(unexpected) != 0 {
		t.Fatalf("unexpected tracked top-level entries: %v", unexpected)
	}
}

func TestUnexpectedTopLevelEntries(t *testing.T) {
	t.Parallel()

	tracked := []string{
		"README.md",
		"unexpected/file.go",
		"unexpected/second.go",
	}
	want := []string{"unexpected"}
	if got := unexpectedTopLevelEntries(tracked, allowedTopLevelEntries()); !slices.Equal(got, want) {
		t.Fatalf("unexpectedTopLevelEntries() = %v, want %v", got, want)
	}
}

func TestDependabotTracksContainerDefinitions(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, ".github", "dependabot.yml"))
	if err != nil {
		t.Fatalf("read Dependabot configuration: %v", err)
	}

	const dockerConfiguration = "package-ecosystem: docker\n    directory: /deployments/containers"
	if !strings.Contains(string(content), dockerConfiguration) {
		t.Fatalf("Dependabot Docker configuration does not use /deployments/containers")
	}
}

func TestWorkflowJobsUseRightSizedBlacksmithRunners(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	workflowDirectory := filepath.Join(root, ".github", "workflows")
	policies := map[string]string{
		"ci.yml":             "blacksmith-4vcpu-ubuntu-2404",
		"release-please.yml": "blacksmith-2vcpu-ubuntu-2404",
		"release.yml":        "blacksmith-4vcpu-ubuntu-2404",
		"scorecard.yml":      "blacksmith-2vcpu-ubuntu-2404",
		"security.yml":       "blacksmith-2vcpu-ubuntu-2404",
	}

	entries, err := os.ReadDir(workflowDirectory)
	if err != nil {
		t.Fatalf("read workflow directory: %v", err)
	}

	seen := make(map[string]struct{}, len(policies))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || (filepath.Ext(name) != ".yml" && filepath.Ext(name) != ".yaml") {
			continue
		}

		want, ok := policies[name]
		if !ok {
			t.Errorf("workflow %q has no Blacksmith runner policy", name)
			continue
		}
		seen[name] = struct{}{}

		content, readErr := os.ReadFile(filepath.Join(workflowDirectory, name))
		if readErr != nil {
			t.Errorf("read workflow %q: %v", name, readErr)
			continue
		}

		labels := workflowRunnerLabels(string(content))
		if len(labels) == 0 {
			t.Errorf("workflow %q has no literal runs-on label", name)
			continue
		}
		for _, got := range labels {
			if got != want {
				t.Errorf("workflow %q runs on %q, want %q", name, got, want)
			}
		}
	}

	for name := range policies {
		if _, ok := seen[name]; !ok {
			t.Errorf("runner policy references missing workflow %q", name)
		}
	}
}

func workflowRunnerLabels(content string) []string {
	var labels []string
	for line := range strings.SplitSeq(content, "\n") {
		value, ok := strings.CutPrefix(strings.TrimSpace(line), "runs-on:")
		if !ok {
			continue
		}
		labels = append(labels, strings.Trim(strings.TrimSpace(value), "\"'"))
	}

	return labels
}

func TestReleaseArchivePreservesSecurityDocumentPath(t *testing.T) {
	t.Parallel()

	root := repositoryRoot(t)
	content, err := os.ReadFile(filepath.Join(root, ".config", "goreleaser.yaml"))
	if err != nil {
		t.Fatalf("read GoReleaser configuration: %v", err)
	}

	configuration := string(content)
	if !strings.Contains(configuration, "- .github/SECURITY.md") {
		t.Fatal("release archive does not preserve .github/SECURITY.md")
	}
	if strings.Contains(configuration, "dst: SECURITY.md") {
		t.Fatal("release archive flattens .github/SECURITY.md and breaks relative links")
	}
}

func allowedTopLevelEntries() []string {
	return []string{
		".config",
		".containerignore",
		".editorconfig",
		".gitattributes",
		".github",
		".gitignore",
		"CHANGELOG.md",
		"LICENSE",
		"NOTICE",
		"README.md",
		"cmd",
		"compose.yaml",
		"deployments",
		"diskforge.go",
		"diskforge_internal_test.go",
		"diskforge_test.go",
		"docs",
		"errors.go",
		"go.mod",
		"go.sum",
		"internal",
		"test",
		"types.go",
		"types_test.go",
	}
}

func trackedRepositoryPaths(t *testing.T, root string) []string {
	t.Helper()

	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		if os.IsNotExist(err) {
			t.Skip("tracked layout requires a Git working tree")

			return nil
		}
		t.Fatalf("inspect Git metadata: %v", err)
	}

	command := exec.CommandContext(t.Context(), "git", "-C", root, "ls-files", "-z")
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list tracked repository paths: %v", err)
	}

	fields := bytes.Split(output, []byte{0})
	paths := make([]string, 0, len(fields))
	for _, field := range fields {
		if len(field) != 0 {
			paths = append(paths, string(field))
		}
	}

	return paths
}

func unexpectedTopLevelEntries(tracked, allowed []string) []string {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, entry := range allowed {
		allowedSet[entry] = struct{}{}
	}

	unexpectedSet := make(map[string]struct{})
	for _, path := range tracked {
		topLevel, _, _ := strings.Cut(path, "/")
		if _, ok := allowedSet[topLevel]; !ok {
			unexpectedSet[topLevel] = struct{}{}
		}
	}

	unexpected := make([]string, 0, len(unexpectedSet))
	for entry := range unexpectedSet {
		unexpected = append(unexpected, entry)
	}
	sort.Strings(unexpected)

	return unexpected
}

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
