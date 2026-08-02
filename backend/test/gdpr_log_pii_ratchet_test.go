package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestGDPRLogPIIRatchet turns the project rule "no student names at Info level
// or above" (root CLAUDE.md Critical Pattern 7) into a CI gate instead of a
// docs-only promise.
//
// Issue #2062: the IoT check-in handler logged the kiosk greeting ("Hallo
// Max!") on its Info line, so every RFID scan wrote a child's first name into
// journald and from there into Loki for the whole retention window. Nothing
// caught it — the rule existed only in prose.
//
// The check is AST-based, so formatting cannot hide a violation: a log call at
// Info/Warn/Error must not read a person-name field or the check-in greeting in
// any of its arguments, whatever the attribute key. Debug is deliberately
// exempt — production filters it out via LOG_LEVEL=info.
//
// There is no allowlist and there must never be one.
func TestGDPRLogPIIRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	violations, err := scanLogCallsForPII(backendRoot)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(violations) > 0 {
		t.Errorf("Found %d log call(s) at Info level or above that reference personal data:\n\n%s\n\n"+
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

// forbiddenLogSelectors are the fields that yield a person's name or a greeting
// built from one. Matching the selector keeps the check independent of the
// receiver type and of the attribute key.
var forbiddenLogSelectors = map[string]bool{
	"FirstName":   true,
	"LastName":    true,
	"GreetingMsg": true,
}

// scanLogCallsForPII parses every non-test .go file under backendRoot and
// reports "path:line: field" for each Info+ log call reading a forbidden field.
func scanLogCallsForPII(backendRoot string) ([]string, error) {
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
			if !ok || !logMethodsAtInfoOrAbove[fun.Sel.Name] {
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

// forbiddenFieldsIn returns the forbidden field names read anywhere in args,
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
