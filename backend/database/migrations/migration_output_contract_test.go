package migrations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// migrationSourceFiles returns the migration files in this directory — the
// numerically prefixed ones bun registers, excluding tests and the runner's own
// files (00_migrations.go, main.go, reset.go), which are allowed to write CLI
// output because a person asked for it.
func migrationSourceFiles(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		if !strings.HasPrefix(name, "000") && !strings.HasPrefix(name, "001") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestMigrationsDoNotPrintTheirOwnProgress is the contract the runner's log line
// depends on (#3300).
//
// The runner writes one slog line per migration — version, description and
// duration_ms — so a migration printing "Migration 1.2.3: doing the thing..."
// only duplicates it, in a second format, on a second stream. 261 files wrote
// their progress with fmt, 159 mixed fmt and log.Printf (stderr, no version),
// and the backend's slog-only rule never reached this package because nothing
// enforced it here. This test is that enforcement.
//
// Output that carries content — a row count, a repair total — is still wanted.
// It goes through slog with fields, which this test permits and fmt/log cannot
// express.
//
// This replaces TestMigrationLogOutputMatchesRegisteredVersion (#3299), which
// checked that a migration's self-announcement named the version it registers.
// Forbidding the announcement outright is strictly stronger: a line that cannot
// exist cannot drift from the registered version.
func TestMigrationsDoNotPrintTheirOwnProgress(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()

	for _, name := range migrationSourceFiles(t) {
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", name, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok {
				return true
			}
			// fmt.Errorf and fmt.Sprintf build values; only the Print family
			// writes to a stream.
			if pkg.Name != "fmt" && pkg.Name != "log" {
				return true
			}
			if !strings.HasPrefix(selector.Sel.Name, "Print") {
				return true
			}

			t.Errorf("%s:%d: %s.%s writes the migration's own output.\n"+
				"The runner already logs one line per migration (version, description, duration_ms).\n"+
				"Delete a pure progress line; route output that carries content through slog with fields.",
				name, fset.Position(call.Pos()).Line, pkg.Name, selector.Sel.Name)
			return true
		})
	}
}

// selfAnnouncement matches the message of a line a migration writes about its
// own execution rather than about what it did to the data: "migration
// starting", "migration finished", "migration rollback failed",
// "migration 1.15.332: creating documents schema".
var selfAnnouncement = regexp.MustCompile(
	`^migration\s+(\d+(\.\d+)+\b|(starting|started|finished|completed|complete|running|rolling|rolled|rollback|begin|beginning)\b)`)

// logMethods are the slog levels, plain and context-carrying.
var logMethods = map[string]bool{
	"Debug": true, "Info": true, "Warn": true, "Error": true,
	"DebugContext": true, "InfoContext": true, "WarnContext": true, "ErrorContext": true,
}

// runnerOwnedKeys are the fields the runner's own line already carries. A
// migration repeating them is how "Migration 1.6.17" came to appear twice while
// 1.6.17.1 never appeared at all (#3299) — the number is written in two places
// and only one of them is checked.
var runnerOwnedKeys = map[string]bool{
	"migration": true,
	"version":   true,
}

// TestMigrationsDoNotRestateTheRunnersLine keeps the #3300 contract honest for
// the migrations that already logged through slog.
//
// Removing fmt and log.Printf is only half the cleanup: 22 files announced
// themselves through slog instead, which produces the same duplicate line in a
// different API and which TestMigrationsDoNotPrintTheirOwnProgress cannot see.
// A migration logs what it did to the data — rows repaired, records that need a
// human — and leaves "which migration, how long" to the runner.
func TestMigrationsDoNotRestateTheRunnersLine(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()

	for _, name := range migrationSourceFiles(t) {
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("failed to parse %s: %v", name, err)
		}

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !logMethods[selector.Sel.Name] {
				return true
			}

			args := call.Args
			if strings.HasSuffix(selector.Sel.Name, "Context") && len(args) > 0 {
				args = args[1:] // drop ctx
			}
			if len(args) == 0 {
				return true
			}
			line := fset.Position(call.Pos()).Line

			if msg, ok := stringLiteral(args[0]); ok && selfAnnouncement.MatchString(strings.ToLower(msg)) {
				t.Errorf("%s:%d: log line %q announces the migration itself.\n"+
					"The runner logs version, description and duration_ms for every migration.\n"+
					"Delete the line, or say what it did to the data.",
					name, line, msg)
			}

			// Keys sit at even offsets after the message.
			for i := 1; i < len(args); i += 2 {
				key, ok := stringLiteral(args[i])
				if ok && runnerOwnedKeys[strings.ToLower(key)] {
					t.Errorf("%s:%d: log field %q repeats what the runner's line already carries.\n"+
						"A second copy of the version is a second thing to keep in sync (#3299).",
						name, line, key)
				}
			}
			return true
		})
	}
}

// stringLiteral unquotes an interpreted string literal argument.
func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
