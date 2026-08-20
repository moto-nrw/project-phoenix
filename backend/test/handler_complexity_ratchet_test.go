package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/uudashr/gocognit"
)

// TestHandlerComplexityRatchet enforces .claude/rules/backend-conventions.md
// Rule 4: no function under api/ may exceed a gocognit cognitive-complexity
// score of 15. The rule was review-only since June 2026 and the violation
// count grew from ~53 to 79 in a month — this ratchet makes it shrink-only,
// like the other ratchets in this package.
//
// Allowlist semantics (per function, not per file):
//
//   - A function NOT in the allowlist must score ≤ 15.
//   - An allowlisted function may never exceed its recorded score.
//   - When refactoring lowers a score (or drops it to ≤ 15), the test fails
//     until the entry is lowered/removed — the ratchet only turns one way.
//     Never raise a number.
//
// Keys are "relative/file.go:FuncName" with methods rendered exactly like the
// gocognit CLI ("(*Resource).applyDeviations"), so `gocognit -over 15 api/`
// output maps 1:1 onto entries here.
const handlerComplexityThreshold = 15

// Seeded 2026-07-12 from `gocognit -over 15 api/` (non-test files only), the
// issue #575 B0 baseline. 75 functions.
var handlerComplexityAllowlist = map[string]int{}

func TestHandlerComplexityRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	scores, err := scanHandlerComplexity(backendRoot)
	if err != nil {
		t.Fatalf("complexity scan failed: %v", err)
	}

	violations := complexityViolations(scores, handlerComplexityAllowlist)
	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Handler-complexity ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// complexityViolations compares per-function scores over the threshold against
// the allowlist. It cannot reuse ratchetViolations: entries here are capped
// scores, not hit counts, and an allowlisted function dropping to or below the
// threshold disappears from the scan entirely (which is the "ratchet down"
// signal, not a stale-entry error in the per-file sense).
func complexityViolations(scores, allowlist map[string]int) []string {
	var violations []string
	for fn, got := range scores {
		allowed, ok := allowlist[fn]
		switch {
		case !ok:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d (limit %d), and is not in the allowlist.\n  Move branching/orchestration into a service method (Rule 4) — do not add new entries.",
				fn, got, handlerComplexityThreshold))
		case got > allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowed %d.\n  Complexity grew. Simplify the function — never raise the allowlist.", fn, got, allowed))
		case got < allowed:
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s scores %d, allowlist says %d.\n  Nice — ratchet the entry down to %d so the improvement cannot regress.", fn, got, allowed, got))
		}
	}
	for fn, allowed := range allowlist {
		if _, ok := scores[fn]; !ok {
			violations = append(violations, fmt.Sprintf(
				"[gocognit] %s is allowlisted at %d but now scores ≤ %d (or was deleted/renamed).\n  Remove the entry.",
				fn, allowed, handlerComplexityThreshold))
		}
	}
	return violations
}

// scanHandlerComplexity returns gocognit scores above the threshold for every
// function in non-test .go files under api/, keyed "relpath:FuncName" with
// methods rendered like the gocognit CLI ("(*Resource).name").
func scanHandlerComplexity(backendRoot string) (map[string]int, error) {
	scores := make(map[string]int)
	apiRoot := filepath.Join(backendRoot, "api")
	err := filepath.WalkDir(apiRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if strings.HasPrefix(d.Name(), ".") && path != apiRoot {
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

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", rel, parseErr)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if score := gocognit.Complexity(fn); score > handlerComplexityThreshold {
				scores[rel+":"+funcDisplayName(fn)] = score
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return scores, nil
}

// funcDisplayName renders a function name the way the gocognit CLI does:
// plain functions as "name", methods as "(recv).name" / "(*recv).name".
func funcDisplayName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	return "(" + typeExprString(fn.Recv.List[0].Type) + ")." + fn.Name.Name
}

func typeExprString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + typeExprString(e.X)
	case *ast.IndexExpr: // generic receiver
		return typeExprString(e.X)
	case *ast.IndexListExpr:
		return typeExprString(e.X)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
