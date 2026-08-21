package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGDPRLogPIIRatchet adds a narrow CI tripwire for the project rule "no
// student names at Info level or above" (root CLAUDE.md Critical Pattern 7).
//
// Issue #2062: the IoT check-in handler logged the kiosk greeting ("Hallo
// Max!") on its Info line, so every RFID scan wrote a child's first name into
// journald and from there into Loki for the whole retention window. Nothing
// caught it — the rule existed only in prose.
//
// Its guarantee is syntactic: an Info-or-higher log call must not directly read
// a forbidden name selector in its arguments. It is not an information-flow
// analysis and does not claim to detect values hidden behind local aliases,
// logger.With context, or whole-object serialization. The IoT route tests cover
// the actual check-in and check-out log output end to end. Debug is deliberately
// exempt because production filters it out via LOG_LEVEL=info.
//
// There is no allowlist and there must never be one.
func TestGDPRLogPIIRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	violations, err := scanDirectPIISelectorsInLogCalls(backendRoot)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("Found %d log call(s) at Info level or above that directly reference forbidden name selectors:\n\n%s\n\n"+
			"Log the ID instead; if the name is genuinely needed for local debugging, log it at Debug:\n\n"+
			"  // FORBIDDEN\n"+
			"  logger.InfoContext(ctx, \"checkin complete\", slog.String(\"message\", result.GreetingMsg))\n\n"+
			"  // CORRECT\n"+
			"  logger.InfoContext(ctx, \"checkin complete\", slog.Int64(\"student_id\", student.ID))",
			len(violations), strings.Join(violations, "\n"))
	}
}

// logMethodsAtInfoOrAbove are the slog methods whose output survives the
// production LOG_LEVEL=info filter.
var logMethodsAtInfoOrAbove = map[string]bool{
	"Info": true, "InfoContext": true,
	"Warn": true, "WarnContext": true,
	"Error": true, "ErrorContext": true,
}

// forbiddenLogSelectors are fields and methods that yield a person's name or a
// greeting built from one. Matching the selector keeps the check independent of the
// receiver type and of the attribute key.
var forbiddenLogSelectors = map[string]bool{
	"FirstName":   true,
	"LastName":    true,
	"GetFullName": true,
	"GreetingMsg": true,
}

// scanDirectPIISelectorsInLogCalls parses every non-test .go file under
// backendRoot and reports direct forbidden-selector reads in Info+ log calls.
func scanDirectPIISelectorsInLogCalls(backendRoot string) ([]string, error) {
	var violations []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(backendRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return fmt.Errorf("parse %s: %w", path, parseErr)
		}
		rel, relErr := filepath.Rel(backendRoot, path)
		if relErr != nil {
			rel = path
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fun, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !isLogCallAtInfoOrAbove(call, fun.Sel.Name) {
				return true
			}
			for _, field := range forbiddenFieldsIn(call.Args) {
				violations = append(violations, fmt.Sprintf("  %s:%d: logs %s",
					filepath.ToSlash(rel), fset.Position(call.Lparen).Line, field))
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(violations)
	return violations, nil
}

// isLogCallAtInfoOrAbove also covers Log and LogAttrs, whose second argument
// selects the level. A dynamic level is treated conservatively because it can
// resolve to Info or above at runtime.
func isLogCallAtInfoOrAbove(call *ast.CallExpr, method string) bool {
	if logMethodsAtInfoOrAbove[method] {
		return true
	}
	if method != "Log" && method != "LogAttrs" {
		return false
	}
	if len(call.Args) < 2 {
		return true
	}
	return !isSlogDebugLevel(call.Args[1])
}

func isSlogDebugLevel(expr ast.Expr) bool {
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return isSlogDebugLevel(paren.X)
	}
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "LevelDebug" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "slog"
}

// forbiddenFieldsIn returns forbidden selector names read anywhere in args,
// including nested inside fmt.Sprintf or string concatenation.
func forbiddenFieldsIn(args []ast.Expr) []string {
	seen := map[string]bool{}
	var found []string
	for _, arg := range args {
		ast.Inspect(arg, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && forbiddenLogSelectors[sel.Sel.Name] && !seen[sel.Sel.Name] {
				seen[sel.Sel.Name] = true
				found = append(found, sel.Sel.Name)
			}
			return true
		})
	}
	sort.Strings(found)
	return found
}

func TestGDPRLogPIIRatchetCoversExplicitLevelsAndFullNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		source     string
		wantFields []string
	}{
		{
			name:       "Log at Info",
			source:     `logger.Log(ctx, slog.LevelInfo, "student", "name", person.GetFullName())`,
			wantFields: []string{"GetFullName"},
		},
		{
			name:       "LogAttrs at Warn",
			source:     `slog.LogAttrs(ctx, slog.LevelWarn, "student", slog.String("name", person.FirstName))`,
			wantFields: []string{"FirstName"},
		},
		{
			name:       "LogAttrs with dynamic level",
			source:     `logger.LogAttrs(ctx, level, "student", slog.String("name", person.LastName))`,
			wantFields: []string{"LastName"},
		},
		{
			name:   "LogAttrs at Debug",
			source: `logger.LogAttrs(ctx, slog.LevelDebug, "student", slog.String("name", person.GetFullName()))`,
		},
		{
			name:   "Debug convenience method",
			source: `logger.Debug("student", "name", person.GetFullName())`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			source := "package sample\nfunc check() {\n" + tt.source + "\n}\n"
			if err := os.WriteFile(filepath.Join(root, "sample.go"), []byte(source), 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			violations, err := scanDirectPIISelectorsInLogCalls(root)
			if err != nil {
				t.Fatalf("scan fixture: %v", err)
			}
			if len(violations) != len(tt.wantFields) {
				t.Fatalf("got violations %v, want fields %v", violations, tt.wantFields)
			}
			joined := strings.Join(violations, "\n")
			for _, field := range tt.wantFields {
				if !strings.Contains(joined, "logs "+field) {
					t.Errorf("violations %q do not report %s", joined, field)
				}
			}
		})
	}
}
