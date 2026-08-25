// Package test: wall-clock enforcement. See .claude/rules/calendar-dates.md.
//
// PostgreSQL TIME columns carry a clock value, not an instant. ActivityInstance
// uses time.Time for those columns, so assignments derived from time.Now or
// timezone.Now must pass through timezone.WallClock before persistence.
package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestActivityInstanceWallClockRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	violations, err := findRawActivityInstanceWallClocks(backendRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) > 0 {
		t.Errorf("Found %d ActivityInstance wall-clock assignment(s) derived from a current instant:\n\n%s\n\n"+
			"schedule.activity_instances.start_time/end_time are PostgreSQL TIME columns. "+
			"Normalize dynamic values with timezone.WallClock, or use a fixed UTC-anchored clock in tests.",
			len(violations), strings.Join(violations, "\n"))
	}
}

func TestActivityInstanceWallClockRatchetDetectsAliasedInstantSources(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	source := `package sample

import (
	stdtime "time"
	tz "github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

func examples() {
	raw := stdtime.Now()
	berlinNow := tz.Now()
	_ = &schedule.ActivityInstance{StartTime: raw}
	_ = &schedule.ActivityInstance{EndTime: berlinNow.Add(90 * stdtime.Minute)}
	_ = &schedule.ActivityInstance{StartTime: tz.WallClock(berlinNow)}
	_ = &schedule.ActivityInstance{StartTime: stdtime.Date(2000, 1, 1, 10, 0, 0, 0, stdtime.UTC)}
}
`
	if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}

	violations, err := findRawActivityInstanceWallClocks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(violations) != 2 {
		t.Fatalf("expected raw time.Now and timezone.Now assignments to fail while normalized/fixed clocks pass; got %v", violations)
	}
}

func findRawActivityInstanceWallClocks(backendRoot string) ([]string, error) {
	fset := token.NewFileSet()
	var violations []string
	err := filepath.Walk(backendRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}
		rel, _ := filepath.Rel(backendRoot, path)
		if rel == filepath.Join("test", "wall_clock_verification_test.go") {
			return nil
		}
		found, scanErr := findRawActivityInstanceWallClocksInFile(path, rel, fset)
		violations = append(violations, found...)
		return scanErr
	})
	return violations, err
}

func findRawActivityInstanceWallClocksInFile(path, rel string, fset *token.FileSet) ([]string, error) {
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	timePackages, timezonePackages := timeImportNames(file)
	var violations []string
	ast.Inspect(file, func(node ast.Node) bool {
		body, ok := functionBody(node)
		if !ok {
			return true
		}
		instantVars := currentInstantVariables(body, timePackages, timezonePackages)
		violations = append(violations, wallClockViolations(body, rel, fset, instantVars, timePackages, timezonePackages)...)
		return false
	})
	return violations, nil
}

func wallClockViolations(body *ast.BlockStmt, rel string, fset *token.FileSet, instantVars, timePackages, timezonePackages map[string]bool) []string {
	var violations []string
	ast.Inspect(body, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok || expressionName(literal.Type) != "ActivityInstance" {
			return true
		}
		for _, elt := range literal.Elts {
			field, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := field.Key.(*ast.Ident)
			if !ok || (name.Name != "StartTime" && name.Name != "EndTime") {
				continue
			}
			if expressionUsesCurrentInstant(field.Value, instantVars, timePackages, timezonePackages) {
				pos := fset.Position(field.Value.Pos())
				violations = append(violations, formatViolation(rel, pos.Line,
					name.Name+" must normalize the current instant with timezone.WallClock"))
			}
		}
		return true
	})
	return violations
}

func functionBody(node ast.Node) (*ast.BlockStmt, bool) {
	switch fn := node.(type) {
	case *ast.FuncDecl:
		return fn.Body, fn.Body != nil
	case *ast.FuncLit:
		return fn.Body, true
	default:
		return nil, false
	}
}

func currentInstantVariables(body *ast.BlockStmt, timePackages, timezonePackages map[string]bool) map[string]bool {
	instantVars := map[string]bool{}
	changed := true
	for changed {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch stmt := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range stmt.Lhs {
					if i >= len(stmt.Rhs) || !expressionUsesCurrentInstant(stmt.Rhs[i], instantVars, timePackages, timezonePackages) {
						continue
					}
					if id, ok := lhs.(*ast.Ident); ok && !instantVars[id.Name] {
						instantVars[id.Name] = true
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range stmt.Names {
					if i < len(stmt.Values) && expressionUsesCurrentInstant(stmt.Values[i], instantVars, timePackages, timezonePackages) && !instantVars[name.Name] {
						instantVars[name.Name] = true
						changed = true
					}
				}
			}
			return true
		})
	}
	return instantVars
}

func expressionUsesCurrentInstant(expr ast.Expr, instantVars, timePackages, timezonePackages map[string]bool) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return instantVars[e.Name]
	case *ast.ParenExpr:
		return expressionUsesCurrentInstant(e.X, instantVars, timePackages, timezonePackages)
	case *ast.CallExpr:
		if isImportedCall(e.Fun, timezonePackages, "WallClock") {
			return false
		}
		if isImportedCall(e.Fun, timePackages, "Now") || isImportedCall(e.Fun, timezonePackages, "Now") {
			return true
		}
		selector, ok := e.Fun.(*ast.SelectorExpr)
		return ok && expressionUsesCurrentInstant(selector.X, instantVars, timePackages, timezonePackages)
	case *ast.SelectorExpr:
		return expressionUsesCurrentInstant(e.X, instantVars, timePackages, timezonePackages)
	case *ast.StarExpr:
		return expressionUsesCurrentInstant(e.X, instantVars, timePackages, timezonePackages)
	default:
		return false
	}
}

func isImportedCall(expr ast.Expr, packageNames map[string]bool, functionName string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != functionName {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && packageNames[pkg.Name]
}

func timeImportNames(file *ast.File) (map[string]bool, map[string]bool) {
	timePackages := map[string]bool{}
	timezonePackages := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := pathpkg.Base(importPath)
		if spec.Name != nil && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		switch {
		case importPath == "time":
			timePackages[name] = true
		case strings.HasSuffix(importPath, "/internal/timezone"):
			timezonePackages[name] = true
		}
	}
	return timePackages, timezonePackages
}

func expressionName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.StarExpr:
		return expressionName(e.X)
	default:
		return ""
	}
}
