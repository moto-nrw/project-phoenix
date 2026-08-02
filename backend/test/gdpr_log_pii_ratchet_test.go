package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// TestGDPRLogPIIRatchet enforces the project rule "no student names at Info
// level or above" (root CLAUDE.md Critical Pattern 7, backend/CLAUDE.md
// GDPR/Privacy Patterns) as a CI gate instead of a docs-only promise.
//
// Issue #2062: the IoT check-in handler logged the kiosk greeting
// ("Hallo Max!") as a `message` attribute on its Info line, so every RFID scan
// wrote a child's first name into journald and, from there, into Loki for the
// whole retention window. Nothing caught it — the rule existed only in prose.
//
// Two checks, both AST-based so they cannot be fooled by formatting:
//
//	P1 (backend-wide) — a log call at Info/Warn/Error must not reference a
//	    person-name field or the check-in greeting in ANY of its arguments.
//	    Matched on the selector, so `slog.String("k", p.FirstName)`,
//	    `"k", person.LastName`, and `fmt.Sprintf("%s", x.GreetingMsg)` all
//	    count regardless of the key.
//
//	P2 (IoT scope) — under api/iot/ and services/iot/ a log call at
//	    Info/Warn/Error must not use a name-bearing attribute key. This catches
//	    the indirect shape P1 misses: a full name assembled into a local
//	    variable earlier and then logged. It is scoped to the IoT tree because
//	    a bare "name" key is legitimate elsewhere (enrollment phases, calendar
//	    periods, form schemas all log their own object's name).
//
// Both allowlists are empty and must stay empty. Names belong at Debug, which
// production filters out via LOG_LEVEL=info.
func TestGDPRLogPIIRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	t.Run("no_person_name_values_in_info_or_above", func(t *testing.T) {
		violations, err := scanLogCalls(backendRoot, backendRoot, checkForbiddenValueSelectors)
		if err != nil {
			t.Fatalf("scan failed: %v", err)
		}
		if len(violations) > 0 {
			t.Errorf("Found %d log call(s) at Info level or above that reference personal data:\n\n%s\n\n"+
				"Names must never reach retained production logs (#2062). Log the ID and,\n"+
				"if the name is genuinely needed for local debugging, log it at Debug:\n\n"+
				"  // FORBIDDEN\n"+
				"  logger.InfoContext(ctx, \"checkin complete\", slog.String(\"message\", result.GreetingMsg))\n\n"+
				"  // CORRECT\n"+
				"  logger.InfoContext(ctx, \"checkin complete\", slog.Int64(\"student_id\", student.ID))\n"+
				"  logger.DebugContext(ctx, \"checkin greeting\", slog.String(\"message\", result.GreetingMsg))",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("no_name_bearing_log_keys_in_iot", func(t *testing.T) {
		var violations []string
		for _, subtree := range []string{
			filepath.Join("api", "iot"),
			filepath.Join("services", "iot"),
		} {
			found, err := scanLogCalls(backendRoot, filepath.Join(backendRoot, subtree), checkForbiddenAttrKeys)
			if err != nil {
				t.Fatalf("scan failed: %v", err)
			}
			violations = append(violations, found...)
		}
		sort.Strings(violations)
		if len(violations) > 0 {
			t.Errorf("Found %d log call(s) at Info level or above using a name-bearing key in the IoT path:\n\n%s\n\n"+
				"Forbidden keys: %s\n"+
				"The kiosk response may carry the child's name; the log line may not.\n"+
				"Use student_id/staff_id/person_id, or drop to Debug.",
				len(violations), strings.Join(violations, "\n"), strings.Join(sortedKeys(forbiddenLogAttrKeys), ", "))
		}
	})
}

// logMethodsAtInfoOrAbove are the slog methods whose output survives the
// production LOG_LEVEL=info filter. Debug is deliberately absent.
var logMethodsAtInfoOrAbove = map[string]bool{
	"Info": true, "InfoContext": true,
	"Warn": true, "WarnContext": true,
	"Error": true, "ErrorContext": true,
}

// forbiddenValueSelectors are struct fields / methods that yield a person's
// name or a greeting built from one. Matching on the selector name keeps the
// check independent of the receiver's type and of the attribute key.
var forbiddenValueSelectors = map[string]bool{
	"FirstName":   true,
	"LastName":    true,
	"GreetingMsg": true,
}

// forbiddenLogAttrKeys are attribute keys that promise a human name. Suffixed
// keys such as room_name / activity_name / device_name are objects, not people,
// and are matched exactly so they stay allowed.
var forbiddenLogAttrKeys = map[string]bool{
	"name":          true,
	"student_name":  true,
	"person_name":   true,
	"staff_name":    true,
	"child_name":    true,
	"guardian_name": true,
	"first_name":    true,
	"last_name":     true,
	"full_name":     true,
	"greeting":      true,
	"message":       true,
}

// logCallChecker inspects one Info/Warn/Error call and returns a reason string
// when it violates the rule, or "" when it is clean.
type logCallChecker func(call *ast.CallExpr) string

// scanLogCalls parses every non-test .go file under root and applies check to
// each log call at Info level or above. Violations are reported as
// "path:line: reason", with path relative to backendRoot.
func scanLogCalls(backendRoot, root string, check logCallChecker) ([]string, error) {
	var violations []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".") && name != "." {
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
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !logMethodsAtInfoOrAbove[sel.Sel.Name] {
				return true
			}
			if reason := check(call); reason != "" {
				violations = append(violations,
					fmt.Sprintf("  %s:%d: %s", filepath.ToSlash(rel), fset.Position(call.Lparen).Line, reason))
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

// checkForbiddenValueSelectors reports a person-name field read anywhere in the
// call's arguments, including nested inside fmt.Sprintf or string concatenation.
func checkForbiddenValueSelectors(call *ast.CallExpr) string {
	var found []string
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if ok && forbiddenValueSelectors[sel.Sel.Name] {
				found = append(found, sel.Sel.Name)
			}
			return true
		})
	}
	if len(found) == 0 {
		return ""
	}
	return fmt.Sprintf("logs personal data field(s) %s", strings.Join(dedupe(found), ", "))
}

// checkForbiddenAttrKeys reports a forbidden attribute key. Every string
// literal in the call is compared, which covers both the slog.Attr style
// (slog.String("student_name", v)) and the key-value style
// (logger.Info("msg", "student_name", v)) — the log message itself never
// matches, since the keys are single lowercase tokens.
func checkForbiddenAttrKeys(call *ast.CallExpr) string {
	var found []string
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err == nil && forbiddenLogAttrKeys[value] {
				found = append(found, value)
			}
			return true
		})
	}
	if len(found) == 0 {
		return ""
	}
	return fmt.Sprintf("uses name-bearing log key(s) %s", strings.Join(dedupe(found), ", "))
}

func dedupe(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
