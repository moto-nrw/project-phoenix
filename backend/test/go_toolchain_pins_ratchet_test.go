package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var (
	goDirectivePattern = regexp.MustCompile(`(?m)^go ([0-9]+\.[0-9]+\.[0-9]+)$`)
	goImagePattern     = regexp.MustCompile(`(?m)^FROM golang:([^[:space:]]+)`)
	airInstallPattern  = regexp.MustCompile(`go install github\.com/air-verse/air@v([0-9]+\.[0-9]+\.[0-9]+)`)
	linterCIPattern    = regexp.MustCompile(`(?m)^\s+version: v([0-9]+\.[0-9]+\.[0-9]+)\s*(?:#.*)?$`)
)

var supportedDevboxSystems = []string{"aarch64-darwin", "aarch64-linux", "x86_64-linux"}

func TestGoToolchainPinsRatchet(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	goVersion := readGoVersion(t, filepath.Join(repoRoot, "backend", "go.mod"))

	for _, modulePath := range []string{
		"backend/tools/go.mod",
		"scripts/backend-architecture/go.mod",
	} {
		if got := readGoVersion(t, filepath.Join(repoRoot, modulePath)); got != goVersion {
			t.Errorf("%s declares Go %s; want %s from backend/go.mod", modulePath, got, goVersion)
		}
	}

	packages := readDevboxPackages(t, filepath.Join(repoRoot, "devbox.json"))
	requireDevboxPin(t, packages, "go", goVersion)
	linterVersion := requireDevboxPackageVersion(t, packages, "golangci-lint")
	airVersion := requireDevboxPackageVersion(t, packages, "air")
	requireDevboxPackageVersion(t, packages, "govulncheck")
	assertDevboxLock(t, filepath.Join(repoRoot, "devbox.lock"), packages)

	for _, dockerfile := range []string{"backend/Dockerfile", "backend/Dockerfile.dev"} {
		assertDockerGoImage(t, filepath.Join(repoRoot, dockerfile), dockerfile, goVersion)
	}
	assertDockerAirVersion(t, filepath.Join(repoRoot, "backend/Dockerfile.dev"), airVersion)

	assertCILinterVersion(t, filepath.Join(repoRoot, ".github/workflows/lint.yml"), linterVersion)
	assertCIUsesMainGoMod(t, filepath.Join(repoRoot, ".github/workflows"))
	assertLocalHooksUsePinnedRunner(t, repoRoot)
	assertRunnerResolvesExactBinary(t, filepath.Join(repoRoot, "scripts/run-go-toolchain.sh"))
}

func assertLocalHooksUsePinnedRunner(t *testing.T, repoRoot string) {
	t.Helper()
	checks := map[string][]string{
		"lefthook.yml": {
			`DIFF=$(../scripts/run-go-toolchain.sh go tool goimports -d . 2>&1)`,
			"run: cd backend && ../scripts/run-go-toolchain.sh go vet ./...",
			"run: cd backend && ../scripts/run-go-toolchain.sh golangci-lint run --timeout 5m",
			"run: cd backend && ../scripts/run-go-toolchain.sh go test ./test -run '^TestGoToolchainPinsRatchet$' -count=1",
			"run: scripts/run-go-toolchain.sh scripts/backend-architecture.sh check",
			"run: scripts/run-go-toolchain.sh scripts/check-deadcode.sh",
			"run: cd backend && ../scripts/run-go-toolchain.sh govulncheck ./...",
		},
		".claude/hooks/format-go.sh": {
			`cd "${project_root}/backend"`,
			`"${project_root}/scripts/run-go-toolchain.sh" go tool goimports -w "${file_path}"`,
		},
	}
	for path, requiredCommands := range checks {
		content := string(readToolchainFile(t, filepath.Join(repoRoot, path)))
		for _, command := range requiredCommands {
			if !activeLinePattern(command).MatchString(content) {
				t.Errorf("%s does not invoke pinned command %q", path, command)
			}
		}
	}
}

func activeLinePattern(line string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[\t ]*` + regexp.QuoteMeta(line) + `[\t ]*$`)
}

func assertRunnerResolvesExactBinary(t *testing.T, path string) {
	t.Helper()
	content := string(readToolchainFile(t, path))
	for _, required := range []string{
		`command_path="$tool_bin/$requested_command"`,
		`if [[ ! -x "$command_path" ]]; then`,
		`exec "$command_path" "$@"`,
	} {
		if !activeLinePattern(required).MatchString(content) {
			t.Errorf("%s does not enforce exact Devbox command resolution: missing %q", path, required)
		}
	}
}

func readGoVersion(t *testing.T, path string) string {
	t.Helper()
	content := readToolchainFile(t, path)
	match := goDirectivePattern.FindStringSubmatch(string(content))
	if match == nil {
		t.Fatalf("%s has no exact three-part Go directive", path)
	}
	return match[1]
}

func readDevboxPackages(t *testing.T, path string) map[string]string {
	t.Helper()
	var config struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &config); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	packages := make(map[string]string, len(config.Packages))
	for _, packageSpec := range config.Packages {
		name, version, ok := strings.Cut(packageSpec, "@")
		if ok {
			packages[name] = version
		}
	}
	return packages
}

func requireDevboxPin(t *testing.T, packages map[string]string, name, want string) {
	t.Helper()
	if got := requireDevboxPackageVersion(t, packages, name); got != want {
		t.Errorf("devbox pins %s %s; want %s from backend/go.mod", name, got, want)
	}
}

func requireDevboxPackageVersion(t *testing.T, packages map[string]string, name string) string {
	t.Helper()
	version, ok := packages[name]
	if !ok || version == "" || version == "latest" {
		t.Fatalf("devbox.json has no explicit %s@<version> pin", name)
	}
	return version
}

func assertDevboxLock(t *testing.T, path string, declared map[string]string) {
	t.Helper()
	var lock struct {
		Packages map[string]struct {
			Version string                     `json:"version"`
			Systems map[string]json.RawMessage `json:"systems"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &lock); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	for _, name := range []string{"go", "golangci-lint", "govulncheck", "air"} {
		version := requireDevboxPackageVersion(t, declared, name)
		entry, ok := lock.Packages[name+"@"+version]
		if !ok || entry.Version != version {
			t.Errorf("%s does not resolve %s@%s", path, name, version)
			continue
		}
		for _, system := range supportedDevboxSystems {
			if _, ok := entry.Systems[system]; !ok {
				t.Errorf("%s has no %s resolution for %s@%s", path, system, name, version)
			}
		}
	}
}

func assertDockerGoImage(t *testing.T, path, label, goVersion string) {
	t.Helper()
	match := goImagePattern.FindStringSubmatch(string(readToolchainFile(t, path)))
	if match == nil {
		t.Fatalf("%s has no pinned Go builder image", label)
	}
	want := goVersion + "-alpine3.24"
	if match[1] != want {
		t.Errorf("%s uses golang:%s; want golang:%s", label, match[1], want)
	}
}

func assertDockerAirVersion(t *testing.T, path, want string) {
	t.Helper()
	match := airInstallPattern.FindStringSubmatch(string(readToolchainFile(t, path)))
	if match == nil {
		t.Fatalf("%s has no pinned Air install", path)
	}
	if match[1] != want {
		t.Errorf("%s installs Air %s; want Devbox version %s", path, match[1], want)
	}
}

func assertCILinterVersion(t *testing.T, path, want string) {
	t.Helper()
	match := linterCIPattern.FindStringSubmatch(string(readToolchainFile(t, path)))
	if match == nil {
		t.Fatalf("%s has no pinned golangci-lint action version", path)
	}
	if match[1] != want {
		t.Errorf("CI pins golangci-lint %s; want Devbox version %s", match[1], want)
	}
}

func assertCIUsesMainGoMod(t *testing.T, workflowDir string) {
	t.Helper()
	workflowPaths, err := filepath.Glob(filepath.Join(workflowDir, "*.yml"))
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	for _, path := range workflowPaths {
		content := string(readToolchainFile(t, path))
		setupCount := strings.Count(content, "actions/setup-go@")
		goModCount := strings.Count(content, "go-version-file: backend/go.mod")
		if setupCount != goModCount {
			t.Errorf("%s has %d setup-go steps but %d backend/go.mod version sources", path, setupCount, goModCount)
		}
		if regexp.MustCompile(`(?m)^[\t ]*go-version:[\t ]*`).MatchString(content) {
			t.Errorf("%s hardcodes go-version; use go-version-file: backend/go.mod", path)
		}
	}
}

func readToolchainFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository configuration paths
	if err != nil {
		t.Fatal(fmt.Errorf("read %s: %w", path, err))
	}
	return content
}
