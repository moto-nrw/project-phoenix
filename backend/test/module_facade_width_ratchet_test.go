package test

import (
	"fmt"
	"go/ast"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestModuleFacadeWidthRatchet enforces the module contract of the #2580
// architecture migration: a module publishes small concrete facades with
// deliberately limited methods, never a generic CRUD API. An interface that
// keeps growing is the migration failure mode this ratchet exists to stop —
// the package boundary moved, but the god-service moved with it. The three
// widest facades at seed time were organizationtenancy.Provisioning with 33
// methods, careplan.StudentSchedulesCommand with 32, and
// peopledirectory.GuardianProvider with 31: a consumer that depends on one of
// them depends on everything the module can do.
//
// The rule: no exported interface in a module's or workflow's public package
// declares more than 12 methods.
//
// Scanned surface — the public face only. A package counts as a root package
// when it holds non-test .go files directly and neither it nor any directory
// above it (below modules/ or workflows/) is named internal, legacy, compose,
// http, inbound, adapters, domain, application or ports. Those are wiring,
// transport and implementation detail, not the facade, and the walk prunes
// them, so nested submodules with their own public package (careplan's
// requestfeed, schoolcalendar's portal) are found without being named here.
//
// Measurement (go/ast): every exported *ast.TypeSpec whose type is an
// *ast.InterfaceType; the width is the number of *ast.Field entries in
// Methods.List whose type is an *ast.FuncType. Embedded interfaces are
// deliberately NOT counted — they are *ast.Ident / *ast.SelectorExpr, not
// FuncType. The repository's Capability = Query + Command composition shape
// would otherwise be charged twice for methods the embedded facades already
// carry, and those embedded facades are the entries this ratchet holds. The
// consequence: a Capability that embeds two allowlisted halves scores 0 here,
// and the halves are where the declared width is ratcheted. The composite
// itself is measured by its method set in TestModuleFacadeMethodSetRatchet
// (module_facade_method_set_ratchet_test.go, ADR 0178).
//
// Allowlist semantics (per interface, like handler_complexity_ratchet_test.go):
//
//   - An interface NOT in the allowlist must declare ≤ 12 methods.
//   - An allowlisted interface may never exceed its recorded width.
//   - When a split lowers a width (or drops it to ≤ 12), the test fails until
//     the entry is lowered/removed — the ratchet only turns one way.
//     Additions or raises require a new-numbered ADR cited by the entry
//     (ADR 0070).
//
// Keys are "relative/file.go:InterfaceName", paths starting at "modules/" or
// "workflows/".
const moduleFacadeThreshold = 12

// moduleFacadeNonRootDirs are directory names that are never a module's public
// face. The walk prunes them, so nothing below them is scanned either.
var moduleFacadeNonRootDirs = map[string]bool{
	"adapters":    true,
	"application": true,
	"compose":     true,
	"domain":      true,
	"http":        true,
	"inbound":     true,
	"internal":    true,
	"legacy":      true,
	"ports":       true,
}

// moduleFacadeAllowlist freezes the facades that were already too wide when
// the ratchet was introduced. Shrink-only.
//
// Seeded 2026-09-18 at commit 19feca2822 by walking the root packages under
// backend/modules/ and backend/workflows/ with go/ast, exactly as
// moduleFacadeScan below does. 36 entries out of 421 exported interfaces in
// those packages (mean width 4.37; 46 above 10 methods, 11 above 20).
// To re-derive a number, run the ratchet — every drift is reported with the
// width measured today:
//
//	cd backend && ../scripts/run-go-toolchain.sh go test ./test/ -run TestModuleFacadeWidthRatchet
var moduleFacadeAllowlist = map[string]int{}

func TestModuleFacadeWidthRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatalf("Could not find backend root: %v", err)
		return
	}

	widths, err := moduleFacadeScan(backendRoot)
	if err != nil {
		t.Fatalf("module-facade scan failed: %v", err)
	}

	violations := moduleFacadeViolations(widths, moduleFacadeAllowlist, moduleFacadeDeclaredMeasure)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Module-facade width ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// moduleFacadeMeasure names a measurement in its violation messages.
type moduleFacadeMeasure struct {
	register string // the bracketed tag
	verb     string // "declares" or "has a method set of"
}

// moduleFacadeDeclaredMeasure names the declared-method measurement.
var moduleFacadeDeclaredMeasure = moduleFacadeMeasure{register: "facade width", verb: "declares"}

// moduleFacadeViolations compares the measured widths above the threshold
// against the allowlist. Like complexityViolations, and unlike
// ratchetViolations, entries are capped values rather than hit counts: an
// allowlisted facade that shrinks to the threshold disappears from the scan
// entirely, which is the ratchet-down signal, not a deleted file.
func moduleFacadeViolations(widths, allowlist map[string]int, m moduleFacadeMeasure) []string {
	var violations []string
	for iface, got := range widths {
		allowed, ok := allowlist[iface]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s %s %d methods (limit %d), and is not in the allowlist.\n  Split the facade along a capability boundary — #2580 asks for small concrete facades, not generic CRUD APIs. Do not add new entries.",
				m.register, iface, m.verb, got, moduleFacadeThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s %s %d methods, allowed %d.\n  The facade grew. Put the new behaviour behind its own capability — never raise the allowlist.",
				m.register, iface, m.verb, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[%s] %s %s %d methods, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.",
				m.register, iface, m.verb, got, allowed, got))
		}
	}
	for iface, allowed := range allowlist {
		if _, ok := widths[iface]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[%s] %s is allowlisted at %d but now %s ≤ %d methods (or was deleted/renamed).\n  Remove the entry.",
				m.register, iface, allowed, m.verb, moduleFacadeThreshold))
		}
	}
	return violations
}

// moduleFacadeScan returns the width of every exported interface wider than
// the threshold in the root packages under modules/ and workflows/, keyed
// "relpath:InterfaceName".
func moduleFacadeScan(backendRoot string) (map[string]int, error) {
	widths := make(map[string]int)
	err := moduleFacadeWalkRootFiles(backendRoot, func(path, rel string) error {
		return moduleFacadeMeasureFile(path, rel, widths)
	})
	if err != nil {
		return nil, err
	}
	return widths, nil
}

// moduleFacadeRootDirs returns the backend-relative directories of the root
// packages, sorted.
func moduleFacadeRootDirs(backendRoot string) ([]string, error) {
	seen := make(map[string]bool)
	err := moduleFacadeWalkRootFiles(backendRoot, func(_, rel string) error {
		seen[path.Dir(rel)] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	dirs := make([]string, 0, len(seen))
	for dir := range seen {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs, nil
}

// moduleFacadeWalkRootFiles calls visit for every non-test .go file of a root
// package under modules/ and workflows/ (see the file header), with its
// backend-relative slash path.
func moduleFacadeWalkRootFiles(backendRoot string, visit func(path, rel string) error) error {
	for _, tree := range []string{"modules", "workflows"} {
		treeRoot := filepath.Join(backendRoot, tree)
		err := filepath.WalkDir(treeRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if path == treeRoot {
					return nil
				}
				if moduleFacadeNonRootDirs[d.Name()] || strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			if filepath.Dir(path) == treeRoot {
				// No package lives directly in modules/ or workflows/.
				return nil
			}
			rel, relErr := filepath.Rel(backendRoot, path)
			if relErr != nil {
				return relErr
			}
			return visit(path, filepath.ToSlash(rel))
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// moduleFacadeMeasureFile records every exported interface in one file whose
// declared-method count exceeds the threshold.
func moduleFacadeMeasureFile(path, rel string, widths map[string]int) error {
	_, file, err := parseGoSourceCached(path, nil) // #nosec G304 -- test scans repo-local source files
	if err != nil {
		return fmt.Errorf("parse %s: %w", rel, err)
	}
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || !spec.Name.IsExported() {
			return true
		}
		iface, ok := spec.Type.(*ast.InterfaceType)
		if !ok {
			return true
		}
		if methods := moduleFacadeMethodCount(iface); methods > moduleFacadeThreshold {
			widths[rel+":"+spec.Name.Name] = methods
		}
		return true
	})
	return nil
}

// moduleFacadeMethodCount counts declared methods. Embedded interfaces and
// type-set elements are not FuncType fields and are deliberately excluded —
// see the file header.
func moduleFacadeMethodCount(iface *ast.InterfaceType) int {
	if iface.Methods == nil {
		return 0
	}
	count := 0
	for _, field := range iface.Methods.List {
		if _, isMethod := field.Type.(*ast.FuncType); isMethod {
			count++
		}
	}
	return count
}
