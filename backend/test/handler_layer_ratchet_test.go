package test

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHandlerLayerRatchet enforces .claude/rules/backend-conventions.md
// Rule 1: HTTP handlers in api/ MUST NOT access repositories — not as struct
// fields, not via service `.XxxRepository()` getters, not by constructing
// queries on a db handle, and not by importing database/repositories.
// Issue #584 counted 71 violations in January 2026; by June the count had
// grown past 200 because detection was docs-only. Like the service ratchet
// above, this test makes every count shrink-only.
//
// Five patterns, each with its own per-file allowlist:
//
//	R1 — repository-typed declarations (struct fields, constructor params)
//	     anywhere under api/ except the test composition hooks
//	     (api/*_test_helpers.go, #4005), which build the retained repositories
//	     for tests and serve no request.
//	R2 — service repository-getter calls (`.XxxRepository()`) anywhere in the
//	     backend except database/ and test/: the getters are scheduled for
//	     deletion from the PersonService interface, so non-api consumers
//	     (authorize policies, schedule services) are ratcheted too.
//	R3 — query construction (`.NewSelect(` etc., `ExecContext` etc.) under api/.
//	R4 — imports of database/repositories under api/ (api/base.go and
//	     api/testutil/ are the sanctioned wiring/testing exceptions).
//	R5 — test-export handler accessors (Rule 5): niladic methods returning
//	     http.HandlerFunc under api/. 308 of these wrappers (~611 LOC) were
//	     deleted in the 2026-07 audit B3 batch; tests drive the resource's
//	     public Router() instead. Routers register private handlers directly,
//	     so a method that merely returns one exists only to leak it to tests.
//
// Allowlist rules (identical to serviceQueryRatchetAllowlist):
//
//   - A file NOT in the allowlist must have ZERO hits.
//   - A file may never exceed its allowed count.
//   - When refactoring removes hits, the test fails until the entry is
//     lowered/removed — the ratchet only turns one way. Never raise a number.
//
// The handlers #2580 moved out of api/ are scanned too (#4071): R1, R3, R4
// and R5 apply to every non-test .go file under modules/*/http,
// modules/*/inbound and workflows/*/http, nested module layouts included
// (modules/careplan/requestfeed/http). Their hits have registers of their own
// (moduleHTTP*Allowlist), first measured on 2026-10-15 on top of development
// 46c01f7234, with the same semantics. No file there is exempt: test hooks in
// those trees count. R2 already scans the whole backend.
//
// Known limitation: R1 matches the textual "name pkg.SomethingRepository"
// declaration shape, so it is bypassable via type aliases and embedded
// fields. The ratchet catches the realistic copy-paste case, which is its
// job — a green run is not proof that no repository reaches a handler.
var (
	// R1: "name pkg.SomethingRepository" declaration pairs — struct fields and
	// constructor params. Service-typed fields never match (their types end in
	// "Service"); getter call sites never match (no name/type whitespace pair).
	apiRepoDeclPattern = regexp.MustCompile(`(^|\(|,\s*)\s*[A-Za-z_]\w*\s+\*?\w+\.\w*Repository\b`)

	// R2: repo-getter invocation on any value. The leading dot excludes the
	// getter *definitions* (method declarations have no receiver dot on the
	// same token).
	repoGetterCallPattern = regexp.MustCompile(`\.\w*Repository\(\)`)

	// R4: import of the repositories tree (prefix also catches /base).
	repoImportPattern = regexp.MustCompile(`"github\.com/moto-nrw/project-phoenix/database/repositories`)

	// R5: niladic method returning http.HandlerFunc — the test-export wrapper
	// shape (`func (rs *X) Name() http.HandlerFunc { return rs.x }`), matched
	// on its declaration line so one-line and gofmt-split bodies both count.
	// Deliberately name-agnostic: the audit found 11 wrappers dodging the
	// `*Handler` suffix.
	handlerAccessorPattern = regexp.MustCompile(`^func \([^)]+\) \w+\(\) http\.HandlerFunc \{`)
)

// R1 — repository-typed declarations in api/ (fields + params).
// Seeded 2026-06-11 from the issue #584 audit; shrink-only. Empty since the
// device authenticators take a SchoolLookup served by SchoolService.
var apiRepoDeclAllowlist = map[string]int{}

// R2 — `.XxxRepository()` getter calls, backend-wide minus database/ + test/.
// Target: empty — the three PersonService getters are scheduled for deletion.
var repoGetterCallAllowlist = map[string]int{}

// R3 — query construction in api/. Empty: handlers construct no queries.
var apiQueryConstructionAllowlist = map[string]int{}

// R4 — database/repositories imports in api/ (beyond base.go + testutil).
// Empty: only the sanctioned wiring exceptions import repositories.
var apiRepoImportAllowlist = map[string]int{}

// R5 — test-export handler accessors in api/. Emptied by audit B3 (2026-07);
// tests use the resource's public Router() per backend-conventions.md Rule 5.
var apiHandlerAccessorAllowlist = map[string]int{}

// R1, R3, R4 and R5 under modules/*/http, modules/*/inbound and
// workflows/*/http (#4071). Shrink-only; no new entry.
var (
	moduleHTTPRepoDeclAllowlist          = map[string]int{}
	moduleHTTPQueryConstructionAllowlist = map[string]int{}
	moduleHTTPRepoImportAllowlist        = map[string]int{}
	moduleHTTPHandlerAccessorAllowlist   = map[string]int{}
)

func TestHandlerLayerRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	checks := []struct {
		name      string
		scan      func(string) (map[string]int, error)
		allowlist map[string]int
		fix       string
	}{
		{
			name:      "R1 repository-typed declarations in api/",
			scan:      scanAPIRepoDecls,
			allowlist: apiRepoDeclAllowlist,
			fix:       "handlers depend on services, never on repositories (Rule 1) — add/extend a service method and drop the repo field/param",
		},
		{
			name:      "R2 repository-getter calls",
			scan:      scanRepoGetterCalls,
			allowlist: repoGetterCallAllowlist,
			fix:       "call a real service method instead of reaching through `.XxxRepository()` — the getters are being removed",
		},
		{
			name:      "R3 query construction in api/",
			scan:      scanAPIQueryConstruction,
			allowlist: apiQueryConstructionAllowlist,
			fix:       "queries belong in database/repositories, orchestration in services (Rules 1+11) — never on a db handle in a handler",
		},
		{
			name:      "R4 database/repositories imports in api/",
			scan:      scanAPIRepoImports,
			allowlist: apiRepoImportAllowlist,
			fix:       "only api/base.go (wiring) and api/testutil/ may import repositories",
		},
		{
			name:      "R5 test-export handler accessors in api/",
			scan:      scanAPIHandlerAccessors,
			allowlist: apiHandlerAccessorAllowlist,
			fix:       "delete the wrapper and test through the resource's public Router() (Rule 5) — internal-package tests may call the private handler directly",
		},
		{
			name:      "R1 repository-typed declarations in module handlers",
			scan:      moduleHTTPScan(apiRepoDeclPattern),
			allowlist: moduleHTTPRepoDeclAllowlist,
			fix:       "handlers depend on their owner's capability, never on repositories (Rule 1)",
		},
		{
			name: "R3 query construction in module handlers",
			scan: func(backendRoot string) (map[string]int, error) {
				return scanModuleHTTPTrees(backendRoot, func(line string) int {
					return len(queryBuilderPattern.FindAllString(line, -1)) + len(driverExecPattern.FindAllString(line, -1))
				})
			},
			allowlist: moduleHTTPQueryConstructionAllowlist,
			fix:       "queries belong in the owner's postgres adapter, orchestration in its application layer (Rules 1+11)",
		},
		{
			name:      "R4 database/repositories imports in module handlers",
			scan:      moduleHTTPScan(repoImportPattern),
			allowlist: moduleHTTPRepoImportAllowlist,
			fix:       "a module handler reaches data through its owner's capability, never through database/repositories",
		},
		{
			name:      "R5 test-export handler accessors in module handlers",
			scan:      moduleHTTPScan(handlerAccessorPattern),
			allowlist: moduleHTTPHandlerAccessorAllowlist,
			fix:       "delete the wrapper and test through the resource's public Router() (Rule 5)",
		},
	}

	var violations []string
	for _, c := range checks {
		counts, scanErr := c.scan(backendRoot)
		if scanErr != nil {
			t.Fatalf("ratchet scan %s failed: %v", c.name, scanErr)
		}
		violations = append(violations, ratchetViolations(c.name, counts, c.allowlist, c.fix)...)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Handler-layer ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

func ratchetViolations(check string, counts, allowlist map[string]int, fix string) []string {
	var violations []string
	for file, got := range counts {
		allowed, ok := allowlist[file]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d hit(s), but the file is not in the allowlist.\n  %s.", check, file, got, fix))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d hit(s), allowed %d.\n  %s. Never raise the allowlist.", check, file, got, allowed, fix))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: %d hit(s), allowlist says %d.\n  Nice — hits were removed. Ratchet down the allowlist entry so the improvement cannot regress.",
				check, file, got, allowed))
		}
	}
	for file, allowed := range allowlist {
		if _, ok := counts[file]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[%s] %s: allowlisted with %d hit(s) but has none (deleted or cleaned up?).\n  Remove the entry.", check, file, allowed))
		}
	}
	return violations
}

func scanAPIRepoDecls(backendRoot string) (map[string]int, error) {
	counts, err := scanTree(backendRoot, filepath.Join(backendRoot, "api"), nil, func(line string) int {
		return len(apiRepoDeclPattern.FindAllString(line, -1))
	})
	if err != nil {
		return nil, err
	}
	for file := range counts {
		if isAPITestHookFile(file) {
			delete(counts, file)
		}
	}
	return counts, nil
}

// isAPITestHookFile reports whether a backend-relative path is one of the
// root's test composition hooks: the api/*_test_helpers.go files that moved
// out of the legacy composition (#4005). They compose the graph tests run
// against and are no request handlers. #4094 takes them out of the production
// build; this exemption goes with it.
func isAPITestHookFile(rel string) bool {
	return strings.HasPrefix(rel, "api/") && !strings.Contains(strings.TrimPrefix(rel, "api/"), "/") &&
		strings.HasSuffix(rel, "_test_helpers.go")
}

func scanRepoGetterCalls(backendRoot string) (map[string]int, error) {
	skipDirs := map[string]bool{"database": true, "test": true}
	return scanTree(backendRoot, backendRoot, skipDirs, func(line string) int {
		return len(repoGetterCallPattern.FindAllString(line, -1))
	})
}

func scanAPIQueryConstruction(backendRoot string) (map[string]int, error) {
	return scanTree(backendRoot, filepath.Join(backendRoot, "api"), nil, func(line string) int {
		return len(queryBuilderPattern.FindAllString(line, -1)) +
			len(driverExecPattern.FindAllString(line, -1))
	})
}

func scanAPIHandlerAccessors(backendRoot string) (map[string]int, error) {
	return scanTree(backendRoot, filepath.Join(backendRoot, "api"), nil, func(line string) int {
		return len(handlerAccessorPattern.FindAllString(line, -1))
	})
}

func scanAPIRepoImports(backendRoot string) (map[string]int, error) {
	counts, err := scanTree(backendRoot, filepath.Join(backendRoot, "api"), nil, func(line string) int {
		return len(repoImportPattern.FindAllString(line, -1))
	})
	if err != nil {
		return nil, err
	}
	delete(counts, "api/base.go")
	for file := range counts {
		if strings.HasPrefix(file, "api/testutil/") {
			delete(counts, file)
		}
	}
	return counts, nil
}

// moduleHTTPScan counts the hits of one pattern per file under the module
// handler trees.
func moduleHTTPScan(pattern *regexp.Regexp) func(string) (map[string]int, error) {
	return func(backendRoot string) (map[string]int, error) {
		return scanModuleHTTPTrees(backendRoot, func(line string) int {
			return len(pattern.FindAllString(line, -1))
		})
	}
}

// scanModuleHTTPTrees counts hits per non-test .go file below modules/*/http,
// modules/*/inbound and workflows/*/http.
func scanModuleHTTPTrees(backendRoot string, countLine func(string) int) (map[string]int, error) {
	counts := map[string]int{}
	for _, top := range []string{"modules", "workflows"} {
		tree, err := scanTree(backendRoot, filepath.Join(backendRoot, top), nil, countLine)
		if err != nil {
			return nil, err
		}
		for file, hits := range tree {
			if isModuleHandlerFile(file) {
				counts[file] = hits
			}
		}
	}
	return counts, nil
}

// isModuleHandlerFile reports whether a backend-relative path lies below a
// handler tree #2580 moved out of api/: a directory named http below a module
// or workflow, or one named inbound below a module, at any nesting depth.
func isModuleHandlerFile(rel string) bool {
	parts := strings.Split(rel, "/")
	if len(parts) < 4 || parts[0] != "modules" && parts[0] != "workflows" {
		return false
	}
	for _, part := range parts[2 : len(parts)-1] {
		if part == "http" || part == "inbound" && parts[0] == "modules" {
			return true
		}
	}
	return false
}

// scanTree counts pattern hits per non-test .go file under root, skipping the
// named top-level directories (relative to backendRoot) and comment-only lines.
func scanTree(backendRoot, root string, skipDirs map[string]bool, countLine func(string) int) (map[string]int, error) {
	counts := make(map[string]int)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if skipDirs[rel] || strings.HasPrefix(d.Name(), ".") && rel != "." {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		hits, scanErr := countFileHits(path, countLine)
		if scanErr != nil {
			return scanErr
		}
		if hits > 0 {
			counts[rel] = hits
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return counts, nil
}

func countFileHits(path string, countLine func(string) int) (int, error) {
	f, err := os.Open(path) // #nosec G304 -- test scans repo-local source files
	if err != nil {
		return 0, err
	}
	defer func() { _ = f.Close() }()

	hits := 0
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		hits += countLine(line)
	}
	return hits, scanner.Err()
}
