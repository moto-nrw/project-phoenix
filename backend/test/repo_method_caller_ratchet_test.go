package test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestRepoInterfaceMethodCallerRatchet implements regrowth guard #5 of the
// 2026-07-05 code-reduction audit: `deadcode` cannot see interface-dispatched
// methods, which let ~2,000 LOC of dead repository methods accumulate
// unnoticed (deleted in slice A3). This ratchet re-runs the audit's detection
// continuously.
//
// Mechanism: every method declared in a models/*/repository.go interface (plus
// models/config/setting_value_repository.go) must have at least one textual
// caller `.Name(` in non-test Go outside models/ and database/ — i.e. a
// service, api, cmd, seed, or scheduler call site. A method with ZERO textual
// hits anywhere is either dead or test-only and must appear in the allowlist
// below.
//
// The check is conservative by construction: name collisions (a live method
// elsewhere sharing the name) inflate the count and mark the method live, so
// the ratchet can under-report but never false-fail. A green run is therefore
// not proof of liveness — it only stops the zero-textual-caller set from
// growing.
//
// The allowlist is intentionally empty: repository interfaces may expose only
// methods with production callers. Tests must verify behavior through live
// methods instead of preserving test-only repository API.
var repoMethodZeroCallerAllowlist = map[string]string{}

func TestRepoInterfaceMethodCallerRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	methodNames := collectRepoInterfaceMethods(t, backendRoot)
	if len(methodNames) == 0 {
		t.Fatal("no repository interface methods found — scan is broken")
	}

	callerFiles := collectCallerFileContents(t, backendRoot)

	var violations []string
	for _, name := range methodNames {
		needle := "." + name + "("
		found := false
		for _, content := range callerFiles {
			if strings.Contains(content, needle) {
				found = true
				break
			}
		}
		_, listed := repoMethodZeroCallerAllowlist[name]
		switch {
		case !found && !listed:
			violations = append(violations, "zero-caller repository interface method not in allowlist: "+name+
				"\n  Delete the method (impl + interface decl + dedicated tests), or allowlist it with a reason if it is a deliberate test helper.")
		case found && listed:
			violations = append(violations, "allowlisted method now has a production caller: "+name+
				"\n  Remove the allowlist entry so the improvement cannot regress.")
		}
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Repository interface-method caller ratchet failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// collectRepoInterfaceMethods parses method declarations out of every
// interface block in models/*/repository.go (+ the config setting-value
// repository file), returning the deduplicated, sorted method-name list.
func collectRepoInterfaceMethods(t *testing.T, backendRoot string) []string {
	t.Helper()

	files, globErr := filepath.Glob(filepath.Join(backendRoot, "models", "*", "repository.go"))
	if globErr != nil {
		t.Fatalf("glob models repositories: %v", globErr)
	}
	files = append(files, filepath.Join(backendRoot, "models", "config", "setting_value_repository.go"))

	ifaceRe := regexp.MustCompile(`(?ms)^type \w+ interface \{(.*?)^\}`)
	declRe := regexp.MustCompile(`(?m)^\t([A-Z]\w*)\(`)

	seen := map[string]bool{}
	for _, f := range files {
		data, readErr := os.ReadFile(f) // #nosec G304 -- test scans repo-local source files
		if readErr != nil {
			continue
		}
		for _, iface := range ifaceRe.FindAllStringSubmatch(string(data), -1) {
			for _, decl := range declRe.FindAllStringSubmatch(iface[1], -1) {
				seen[decl[1]] = true
			}
		}
	}

	names := make([]string, 0, len(seen))
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// collectCallerFileContents loads every non-test .go file outside models/ and
// database/ (the caller universe: services, api, auth, cmd, seed, scheduler,
// simulate, tenant, realtime, internal, ...).
func collectCallerFileContents(t *testing.T, backendRoot string) []string {
	t.Helper()

	skipDirs := map[string]bool{"models": true, "database": true}
	var contents []string
	err := filepath.WalkDir(backendRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[rel] || (strings.HasPrefix(d.Name(), ".") && rel != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, readErr := os.ReadFile(path) // #nosec G304 -- test scans repo-local source files
		if readErr != nil {
			return readErr
		}
		contents = append(contents, string(data))
		return nil
	})
	if err != nil {
		t.Fatalf("walking backend for caller files: %v", err)
	}
	return contents
}
