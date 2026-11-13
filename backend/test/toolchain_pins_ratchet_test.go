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
	alpineImagePattern = regexp.MustCompile(`(?m)^FROM alpine:([^[:space:]]+)`)
	pnpmInstallPattern = regexp.MustCompile(`npm install -g pnpm@([^[:space:]]+)`)
	nodeImagePattern   = regexp.MustCompile(`(?m)^FROM node:([^[:space:]]+)`)
	nodeCIPattern      = regexp.MustCompile(`(?m)^\s+node-version:\s*"?([^"[:space:]]+)"?`)
	airInstallPattern  = regexp.MustCompile(`go install github\.com/air-verse/air@v([0-9]+\.[0-9]+\.[0-9]+)`)
	linterCIPattern    = regexp.MustCompile(`(?m)^\s+version: v([0-9]+\.[0-9]+\.[0-9]+)\s*(?:#.*)?$`)
	// A release reaches nixpkgs days before nixhub indexes it, and devbox
	// search only offers indexed versions. Until then Go and golangci-lint may
	// be pinned to the nixpkgs commit that carries the release. The commit
	// fixes the exact version, which this test cannot read: it checks Go's
	// minor line through the attribute name and nothing for the linter.
	nixpkgsPinPattern = regexp.MustCompile(`^github:NixOS/nixpkgs/[0-9a-f]{40}#(.+)$`)
	// The postgres image carries the exact minor and alpine release plus the
	// index digest. A floating tag such as 17-alpine moved to 17.12 overnight
	// and turned every pull request red (#6916).
	postgresPinPattern = regexp.MustCompile(`^postgres:[0-9]+\.[0-9]+-alpine[0-9]+\.[0-9]+@sha256:[0-9a-f]{64}$`)
	// Compose and workflow image lines, tagged or not, and versioned references
	// in commands; "postgres:5432" host-port pairs have no dot or dash.
	postgresImageLinePattern = regexp.MustCompile(`(?m)^[\t ]*image:[\t ]*["']?(postgres(?:[:@][^\s"']*)?)["']?[\t ]*(?:#.*)?$`)
	postgresVersionedPattern = regexp.MustCompile("postgres:[0-9]+[.-][^\\s\"'`)]*")
)

// postgresImageFiles are the files that start a postgres container: the
// deployed environments, CI, local development and the script tests.
var postgresImageFiles = []string{
	".github/workflows/*.yml",
	"docker-compose.example.yml",
	"environments/*.compose.yml",
	"docs/operations/*.compose.yml",
	"scripts/*.mjs",
	"scripts/test-support/*.mjs",
}

var supportedToolchainSystems = []string{"aarch64-darwin", "aarch64-linux", "x86_64-linux"}

func TestToolchainPinsRatchet(t *testing.T) {
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
	assertDevboxUsesDefaultGoCache(t, filepath.Join(repoRoot, "devbox.json"))
	nixpkgsPins := readDevboxNixpkgsPins(t, filepath.Join(repoRoot, "devbox.json"))
	lockedPackages := []string{"pnpm_10", "govulncheck", "air", "gitleaks"}
	major, rest, _ := strings.Cut(goVersion, ".")
	minor, _, _ := strings.Cut(rest, ".")
	if goPin, ok := nixpkgsPins["go_"+major+"_"+minor]; ok {
		assertDevboxLockResolvesFlake(t, filepath.Join(repoRoot, "devbox.lock"), goPin)
	} else {
		requireDevboxPin(t, packages, "go", goVersion)
		lockedPackages = append(lockedPackages, "go")
	}
	if linterPin, ok := nixpkgsPins["golangci-lint"]; ok {
		assertDevboxLockResolvesFlake(t, filepath.Join(repoRoot, "devbox.lock"), linterPin)
	} else {
		linterVersion := requireDevboxPackageVersion(t, packages, "golangci-lint")
		lockedPackages = append(lockedPackages, "golangci-lint")
		assertCILinterVersion(t, filepath.Join(repoRoot, ".github/workflows/lint.yml"), linterVersion)
	}
	pnpmVersion := requireDevboxPackageVersion(t, packages, "pnpm_10")
	for _, manifestPath := range []string{"package.json", "frontend/package.json"} {
		assertPnpmPackageManagerVersion(t, filepath.Join(repoRoot, manifestPath), pnpmVersion)
	}
	nodeMajor := requireDevboxNodeMajor(t, packages)
	for _, dockerfile := range []string{"frontend/Dockerfile", "frontend/Dockerfile.prod"} {
		assertDockerPnpmVersion(t, filepath.Join(repoRoot, dockerfile), pnpmVersion)
		assertDockerNodeMajor(t, filepath.Join(repoRoot, dockerfile), nodeMajor)
	}
	assertCINodeMajor(t, filepath.Join(repoRoot, ".github/workflows"), nodeMajor)
	airVersion := requireDevboxPackageVersion(t, packages, "air")
	requireDevboxPackageVersion(t, packages, "govulncheck")
	requireDevboxPackageVersion(t, packages, "gitleaks")
	assertDevboxLock(t, filepath.Join(repoRoot, "devbox.lock"), packages, lockedPackages)

	for _, dockerfile := range []string{"backend/Dockerfile", "backend/Dockerfile.dev"} {
		assertDockerGoImage(t, filepath.Join(repoRoot, dockerfile), dockerfile, goVersion)
	}
	assertDockerAirVersion(t, filepath.Join(repoRoot, "backend/Dockerfile.dev"), airVersion)

	assertCIUsesMainGoMod(t, filepath.Join(repoRoot, ".github/workflows"))
	assertPostgresImagePins(t, repoRoot)
	assertLocalHooksUsePinnedRunner(t, repoRoot)
	assertRunnerResolvesExactBinary(t, filepath.Join(repoRoot, "scripts/run-go-toolchain.sh"))
}

func assertPnpmPackageManagerVersion(t *testing.T, path, want string) {
	t.Helper()
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &manifest); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	version, ok := strings.CutPrefix(manifest.PackageManager, "pnpm@")
	if !ok {
		t.Fatalf("%s does not pin pnpm in packageManager", path)
	}
	version, _, _ = strings.Cut(version, "+")
	if version != want {
		t.Errorf("%s pins pnpm %s; want %s from devbox.json", path, version, want)
	}
}

func assertDevboxUsesDefaultGoCache(t *testing.T, path string) {
	t.Helper()
	var config struct {
		Env map[string]string `json:"env"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &config); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	for _, name := range []string{"GOPATH", "GOMODCACHE"} {
		if value, ok := config.Env[name]; ok {
			t.Errorf("devbox must not set %s=%q; use Go's shared default cache", name, value)
		}
	}
}

func assertLocalHooksUsePinnedRunner(t *testing.T, repoRoot string) {
	t.Helper()
	checks := map[string][]string{
		"lefthook.yml": {
			// The hook checks the staged files, not the whole tree, and keeps
			// goimports' stderr out of the diff variable — but it still has to
			// go through the pinned runner and propagate tool failures.
			`DIFF=$(../scripts/run-go-toolchain.sh go tool goimports -d $FILES) || exit $?`,
			"run: bash scripts/pre-push.sh --hook",
		},
		".claude/hooks/format-go.sh": {
			`cd "${project_root}/backend"`,
			`"${project_root}/scripts/run-go-toolchain.sh" go tool goimports -w "${file_path}"`,
		},
	}
	// Keep the pinned runner contract when the hook delegates to shared scripts.
	// The final argument selects a quality mode or base revision.
	prefixChecks := map[string][]string{
		"scripts/pre-push.sh": {
			"cached backend-lint scripts/run-go-toolchain.sh scripts/check-backend-quality.sh lint",
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

	for path, requiredPrefixes := range prefixChecks {
		content := string(readToolchainFile(t, filepath.Join(repoRoot, path)))
		for _, prefix := range requiredPrefixes {
			if !activeLinePrefixPattern(prefix).MatchString(content) {
				t.Errorf("%s does not invoke pinned command %q with a trailing value", path, prefix)
			}
		}
	}
}

func activeLinePattern(line string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[\t ]*` + regexp.QuoteMeta(line) + `[\t ]*$`)
}

// activeLinePrefixPattern matches a pinned command that ends in exactly one
// further argument, whose value is not pinned.
func activeLinePrefixPattern(prefix string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^[\t ]*` + regexp.QuoteMeta(prefix) + `[\t ]+\S+[\t ]*$`)
}

func assertRunnerResolvesExactBinary(t *testing.T, path string) {
	t.Helper()
	content := string(readToolchainFile(t, path))
	for _, required := range []string{
		`export CGO_ENABLED=${CGO_ENABLED:-0}`,
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
			if _, exists := packages[name]; exists {
				t.Fatalf("%s declares %s more than once", path, name)
			}
			packages[name] = version
		}
	}
	return packages
}

// readDevboxNixpkgsPins maps nixpkgs attribute names to their commit pins.
func readDevboxNixpkgsPins(t *testing.T, path string) map[string]string {
	t.Helper()
	var config struct {
		Packages []string `json:"packages"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &config); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	pins := make(map[string]string)
	for _, packageSpec := range config.Packages {
		if match := nixpkgsPinPattern.FindStringSubmatch(packageSpec); match != nil {
			pins[match[1]] = packageSpec
		}
	}
	return pins
}

func assertDevboxLockResolvesFlake(t *testing.T, path, packageSpec string) {
	t.Helper()
	var lock struct {
		Packages map[string]struct {
			Resolved string `json:"resolved"`
		} `json:"packages"`
	}
	if err := json.Unmarshal(readToolchainFile(t, path), &lock); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	if entry, ok := lock.Packages[packageSpec]; !ok || entry.Resolved != packageSpec {
		t.Errorf("%s does not resolve %s", path, packageSpec)
	}
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

func assertDevboxLock(t *testing.T, path string, declared map[string]string, names []string) {
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
	for _, name := range names {
		version := requireDevboxPackageVersion(t, declared, name)
		entry, ok := lock.Packages[name+"@"+version]
		if !ok || entry.Version != version {
			t.Errorf("%s does not resolve %s@%s", path, name, version)
			continue
		}
		for _, system := range supportedToolchainSystems {
			if _, ok := entry.Systems[system]; !ok {
				t.Errorf("%s has no %s resolution for %s@%s", path, system, name, version)
			}
		}
	}
}

func assertDockerGoImage(t *testing.T, path, label, goVersion string) {
	t.Helper()
	content := string(readToolchainFile(t, path))
	match := goImagePattern.FindStringSubmatch(content)
	if match == nil {
		t.Fatalf("%s has no pinned Go builder image", label)
	}
	want := goVersion + "-alpine3.24"
	if match[1] != want {
		t.Errorf("%s uses golang:%s; want golang:%s", label, match[1], want)
	}
	_, alpineVersion, ok := strings.Cut(match[1], "-alpine")
	if !ok {
		t.Fatalf("%s Go builder has no Alpine version", label)
	}
	runtimes := alpineImagePattern.FindAllStringSubmatch(content, -1)
	if label == "backend/Dockerfile" && len(runtimes) != 1 {
		t.Fatalf("%s must have exactly one pinned Alpine runtime base", label)
	}
	for _, runtime := range runtimes {
		if runtime[1] != alpineVersion {
			t.Errorf("%s uses runtime alpine:%s; want alpine:%s from builder", label, runtime[1], alpineVersion)
		}
	}
}

func assertDockerPnpmVersion(t *testing.T, path, want string) {
	t.Helper()
	matches := pnpmInstallPattern.FindAllStringSubmatch(string(readToolchainFile(t, path)), -1)
	if len(matches) == 0 {
		t.Fatalf("%s has no pinned pnpm install", path)
	}
	for _, match := range matches {
		if match[1] != want {
			t.Errorf("%s installs pnpm %s; want %s from packageManager and devbox.json", path, match[1], want)
		}
	}
}

// requireDevboxNodeMajor is the Node major of the one nodejs_<major>
// package in devbox.json. The frontend images and CI run the same major:
// a major the images alone move to can break the image build only on
// development (Node 25 dropped Corepack, #6483). The images therefore
// install pnpm through npm.
func requireDevboxNodeMajor(t *testing.T, packages map[string]string) string {
	t.Helper()
	var majors []string
	for name := range packages {
		if major, ok := strings.CutPrefix(name, "nodejs_"); ok {
			majors = append(majors, major)
		}
	}
	if len(majors) != 1 {
		t.Fatalf("devbox.json declares %d nodejs_<major> packages; want exactly one", len(majors))
	}
	return majors[0]
}

func assertDockerNodeMajor(t *testing.T, path, want string) {
	t.Helper()
	matches := nodeImagePattern.FindAllStringSubmatch(string(readToolchainFile(t, path)), -1)
	if len(matches) == 0 {
		t.Fatalf("%s has no node base image", path)
	}
	for _, match := range matches {
		if major, _, _ := strings.Cut(match[1], "-"); major != want {
			t.Errorf("%s uses node:%s; want Node %s from devbox.json", path, match[1], want)
		}
	}
}

func assertCINodeMajor(t *testing.T, workflowDir, want string) {
	t.Helper()
	workflowPaths, err := filepath.Glob(filepath.Join(workflowDir, "*.yml"))
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	for _, path := range workflowPaths {
		for _, match := range nodeCIPattern.FindAllStringSubmatch(string(readToolchainFile(t, path)), -1) {
			if major, _, _ := strings.Cut(match[1], "."); major != want {
				t.Errorf("%s sets node-version %s; want Node %s from devbox.json", path, match[1], want)
			}
		}
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

// assertPostgresImagePins requires every postgres container to run the image
// production pins, so CI tests the version production serves.
func assertPostgresImagePins(t *testing.T, repoRoot string) {
	t.Helper()
	const source = "environments/production.compose.yml"
	distinct := map[string]bool{}
	for _, ref := range postgresImageRefs(string(readToolchainFile(t, filepath.Join(repoRoot, source)))) {
		distinct[ref] = true
	}
	if len(distinct) != 1 {
		t.Fatalf("%s names %d distinct postgres images; want exactly one", source, len(distinct))
	}
	var pin string
	for ref := range distinct {
		pin = ref
	}
	if !postgresPinPattern.MatchString(pin) {
		t.Fatalf("%s uses %s; want postgres:<major>.<minor>-alpine<x>.<y>@sha256:<digest>", source, pin)
	}
	for _, pattern := range postgresImageFiles {
		paths, err := filepath.Glob(filepath.Join(repoRoot, pattern))
		if err != nil {
			t.Fatalf("list %s: %v", pattern, err)
		}
		for _, path := range paths {
			reported := map[string]bool{}
			for _, ref := range postgresImageRefs(string(readToolchainFile(t, path))) {
				if ref != pin && !reported[ref] {
					reported[ref] = true
					relative, _ := filepath.Rel(repoRoot, path)
					t.Errorf("%s uses %s; want %s from %s", relative, ref, pin, source)
				}
			}
		}
	}
}

func postgresImageRefs(content string) []string {
	var refs []string
	for _, match := range postgresImageLinePattern.FindAllStringSubmatch(content, -1) {
		refs = append(refs, match[1])
	}
	return append(refs, postgresVersionedPattern.FindAllString(content, -1)...)
}

func readToolchainFile(t *testing.T, path string) []byte {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository configuration paths
	if err != nil {
		t.Fatal(fmt.Errorf("read %s: %w", path, err))
	}
	return content
}
