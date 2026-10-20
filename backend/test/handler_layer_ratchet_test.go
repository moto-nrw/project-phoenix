package test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestHandlerLayerRatchet enforces .claude/rules/backend-conventions.md
// Rule 1: HTTP handlers in api/ MUST NOT access repositories — not as struct
// fields and not by constructing queries on a db handle. Issue #584 counted
// 71 violations in January 2026; by June the count had grown past 200
// because detection was docs-only. This test makes every count shrink-only.
//
// Three patterns, each with its own per-file allowlist:
//
//	R1 — repository-typed declarations (struct fields, constructor params)
//	     anywhere under the production build of api/.
//	R3 — query construction (`.NewSelect(` etc., `ExecContext` etc.) under api/.
//	R5 — test-export handler accessors (Rule 5): niladic methods returning
//	     http.HandlerFunc under api/. 308 of these wrappers (~611 LOC) were
//	     deleted in the 2026-07 audit B3 batch; tests drive the resource's
//	     public Router() instead. Routers register private handlers directly,
//	     so a method that merely returns one exists only to leak it to tests.
//
// R2 (`.XxxRepository()` getter calls) and R4 (database/repositories
// imports) are gone (#4427): the getters and the production repository
// packages no longer exist, so neither shape compiles.
//
// Allowlist rules (identical to the other ratchets in this package):
//
//   - A file NOT in the allowlist must have ZERO hits.
//   - A file may never exceed its allowed count.
//   - When refactoring removes hits, the test fails until the entry is
//     lowered/removed — the ratchet only turns one way. Never raise a number.
//
// The handlers #2580 moved out of api/ are scanned too (#4071): R1, R3
// and R5 apply to every non-test .go file under modules/*/http,
// modules/*/inbound and workflows/*/http, nested module layouts included
// (modules/careplan/requestfeed/http). Their hits have registers of their own
// (moduleHTTP*Allowlist), first measured on 2026-10-15 on top of development
// c49b81c3a9, with the same semantics. Every file in the default production
// build counts.
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

	// R5: niladic method returning http.HandlerFunc — the test-export wrapper
	// shape (`func (rs *X) Name() http.HandlerFunc { return rs.x }`), matched
	// on its declaration line so one-line and gofmt-split bodies both count.
	// Deliberately name-agnostic: the audit found 11 wrappers dodging the
	// `*Handler` suffix.
	handlerAccessorPattern = regexp.MustCompile(`^func \([^)]+\) \w+\(\) http\.HandlerFunc \{`)

	// R3: query builder construction on a db/tx handle.
	queryBuilderPattern = regexp.MustCompile(`\.New(Select|Update|Insert|Delete|Raw)\(`)
	// R3: driver-level statement execution on a db/tx handle — both the
	// context-taking and context-free variants (Exec/Query/QueryRow compile
	// fine on bun.DB too). The trailing ([^)]|$) requires an argument (or a
	// multi-line call continuing past the line end) so the no-arg
	// `r.URL.Query()` accessor all over api/ does not match.
	driverExecPattern = regexp.MustCompile(`\.(Exec|Query|QueryRow)(Context)?\(([^)]|$)`)
)

// R1 — repository-typed declarations in api/ (fields + params).
// Seeded 2026-06-11 from the issue #584 audit; shrink-only. Empty since the
// device authenticators take a SchoolLookup served by SchoolService.
var apiRepoDeclAllowlist = map[string]int{}

// R3 — query construction in api/. Empty: handlers construct no queries.
var apiQueryConstructionAllowlist = map[string]int{}

// R5 — test-export handler accessors in api/. Emptied by audit B3 (2026-07);
// tests use the resource's public Router() per backend-conventions.md Rule 5.
var apiHandlerAccessorAllowlist = map[string]int{}

// R1, R3 and R5 under modules/*/http, modules/*/inbound and
// workflows/*/http (#4071). Shrink-only; no new entry.
var (
	moduleHTTPRepoDeclAllowlist          = map[string]int{}
	moduleHTTPQueryConstructionAllowlist = map[string]int{}
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
			name:      "R3 query construction in api/",
			scan:      scanAPIQueryConstruction,
			allowlist: apiQueryConstructionAllowlist,
			fix:       "queries belong in database/repositories, orchestration in services (Rules 1+11) — never on a db handle in a handler",
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
	return scanTree(backendRoot, filepath.Join(backendRoot, "api"), func(line string) int {
		return len(apiRepoDeclPattern.FindAllString(line, -1))
	})
}

func scanAPIQueryConstruction(backendRoot string) (map[string]int, error) {
	return scanTree(backendRoot, filepath.Join(backendRoot, "api"), func(line string) int {
		return len(queryBuilderPattern.FindAllString(line, -1)) +
			len(driverExecPattern.FindAllString(line, -1))
	})
}

func scanAPIHandlerAccessors(backendRoot string) (map[string]int, error) {
	return scanTree(backendRoot, filepath.Join(backendRoot, "api"), func(line string) int {
		return len(handlerAccessorPattern.FindAllString(line, -1))
	})
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
	return scanSources(backendRoot, isModuleHandlerFile, countLine)
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

// scanTree counts pattern hits per non-test .go file under root, skipping
// hidden directories and comment-only lines.
func scanTree(backendRoot, root string, countLine func(string) int) (map[string]int, error) {
	prefix, err := filepath.Rel(backendRoot, root)
	if err != nil {
		return nil, err
	}
	prefix = filepath.ToSlash(prefix) + "/"
	return scanSources(backendRoot, func(rel string) bool {
		return strings.HasPrefix(rel, prefix)
	}, countLine)
}

// scanSources counts pattern hits per file of the default production build
// whose backend-relative path is selected, skipping hidden directories and
// comment-only lines. It filters the shared goSourceIndex before reading a
// line, so every check shares one tree walk.
func scanSources(backendRoot string, selected func(string) bool, countLine func(string) int) (map[string]int, error) {
	files, err := goSourceIndex(backendRoot)
	if err != nil {
		return nil, err
	}
	active, err := productionGoSources(backendRoot)
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, f := range files {
		if strings.HasSuffix(f.rel, "_test.go") || !selected(f.rel) || inDirNamed(f.rel) {
			continue
		}
		if !active[filepath.Join(backendRoot, filepath.FromSlash(f.rel))] {
			continue
		}
		if hits := countContentHits(f.content, countLine); hits > 0 {
			counts[f.rel] = hits
		}
	}
	return counts, nil
}

func countContentHits(content []byte, countLine func(string) int) int {
	hits := 0
	for _, raw := range bytes.Split(content, []byte("\n")) {
		line := strings.TrimSuffix(string(raw), "\r")
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		hits += countLine(line)
	}
	return hits
}
