package test

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestModuleHTTPORMRatchet enforces the #2580 boundary rule that the inbound
// (HTTP) role of a module never touches the ORM: no package under
// modules/*/http, modules/*/inbound or workflows/*/http may import
// github.com/uptrace/bun.
//
// Why: the migration forbids ORM types in public contracts and requires the
// HTTP layer to receive a transaction middleware / injected transaction runner
// instead of a database handle. Today most of the offenders hold a *bun.DB
// purely to hand it to api/common.ProtectedTenantGroup, but some already start
// their own transactions at the request boundary
// (modules/careplan/inbound/parent/enrollment_handlers.go in three places,
// modules/dataimport/inbound/compose/runtime.go). Both shapes make the inbound
// role depend on the persistence technology and put transaction control in the
// place least able to reason about it.
//
// Scan: non-test .go files under modules/*/http/**, modules/*/inbound/** and
// workflows/*/http/**, at any depth, with "legacy" subdirectories included —
// they are part of the problem. Keys are repo-relative slash paths starting at
// "modules/" or "workflows/". Counting is AST-based over the import block: an
// import of "github.com/uptrace/bun" or any path below it counts as one hit,
// so a file's value is in practice 1.
//
// Allowlist semantics (identical to the other ratchets in this package):
//
//   - A file NOT in the allowlist must have ZERO hits.
//   - A file may never exceed its allowed count.
//   - When a refactor removes the import, the test fails until the entry is
//     lowered/removed — the ratchet only turns one way. Never raise a number,
//     never add an entry.
//
// Target is 0 entries. The seed below freezes the state of 2026-09-18 at
// commit 19feca2822, measured with an AST walk over the scan area described
// above (23 files, all with exactly one bun import).
var moduleHTTPORMAllowlist = map[string]int{
	"modules/birthdays/http/api.go":                           1,
	"modules/careplan/inbound/parent/api.go":                  1,
	"modules/careplan/inbound/parent/enrollment_handlers.go":  1,
	"modules/classday/http/api.go":                            1,
	"modules/communication/http/parentannouncements/api.go":   1,
	"modules/communication/http/parentmessages/api.go":        1,
	"modules/communication/http/staffmessages/api.go":         1,
	"modules/dataimport/inbound/compose/runtime.go":           1,
	"modules/delivery/http/notifications/api.go":              1,
	"modules/delivery/http/sse/api.go":                        1,
	"modules/delivery/http/sse/resource.go":                   1,
	"modules/emergencysnapshot/http/api.go":                   1,
	"modules/filestorage/http/files/api.go":                   1,
	"modules/statistics/http/api.go":                          1,
	"modules/workforce/inbound/absencetypes.go":               1,
	"modules/workforce/inbound/shiftplanning/resources.go":    1,
	"modules/workforce/inbound/substitutions.go":              1,
	"modules/workforce/inbound/timetracking/api.go":           1,
	"modules/workforce/inbound/timetracking/staff_admin.go":   1,
	"modules/workforce/inbound/timetracking/staff_notices.go": 1,
}

// moduleHTTPORMImportPath is the ORM module path; imports of this path or of
// any package below it (bun/dialect, bun/extra/..., ...) count.
const moduleHTTPORMImportPath = "github.com/uptrace/bun"

func TestModuleHTTPORMRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := moduleHTTPORMScan(backendRoot)
	if err != nil {
		t.Fatalf("http-orm ratchet scan failed: %v", err)
	}

	violations := ratchetViolations("http-orm", counts, moduleHTTPORMAllowlist,
		"Take an injected transaction runner instead of a *bun.DB")
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module HTTP ORM ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleHTTPORMScan counts bun imports per non-test .go file in the inbound
// role of every module and workflow, keyed by repo-relative slash path.
func moduleHTTPORMScan(backendRoot string) (map[string]int, error) {
	counts := make(map[string]int)
	for _, top := range []string{"modules", "workflows"} {
		root := filepath.Join(backendRoot, top)
		if _, statErr := os.Stat(root); os.IsNotExist(statErr) {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") && path != root {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if !moduleHTTPORMInScope(rel) {
				return nil
			}
			hits, countErr := moduleHTTPORMCountImports(path)
			if countErr != nil {
				return fmt.Errorf("parse %s: %w", rel, countErr)
			}
			if hits > 0 {
				counts[rel] = hits
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return counts, nil
}

// moduleHTTPORMInScope reports whether a repo-relative path sits in the
// inbound role: modules/<module>/http/**, modules/<module>/inbound/** or
// workflows/<workflow>/http/**. "legacy" subdirectories are deliberately not
// excluded.
func moduleHTTPORMInScope(rel string) bool {
	parts := strings.Split(rel, "/")
	if len(parts) < 4 {
		return false
	}
	switch parts[0] {
	case "modules":
		return parts[2] == "http" || parts[2] == "inbound"
	case "workflows":
		return parts[2] == "http"
	}
	return false
}

// moduleHTTPORMCountImports parses only the import block and counts the ORM
// imports. Parsing instead of grepping keeps commented-out and string-literal
// occurrences out of the count.
func moduleHTTPORMCountImports(path string) (int, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return 0, err
	}
	hits := 0
	for _, imp := range file.Imports {
		importPath, unquoteErr := strconv.Unquote(imp.Path.Value)
		if unquoteErr != nil {
			continue
		}
		if importPath == moduleHTTPORMImportPath ||
			strings.HasPrefix(importPath, moduleHTTPORMImportPath+"/") {
			hits++
		}
	}
	return hits, nil
}
