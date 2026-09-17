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

// runnerOwnedFiles are the files in this package that legitimately write to a
// stream: a person ran `migrate status` or `migrate validate` and is waiting for
// the answer on stdout, and the runner's own lines are the ones this whole
// contract exists to make room for.
//
// Everything else in the package is migration code, whether or not bun
// registers it directly — a helper a migration calls prints into the same run as
// the migration itself, so an exclusion list rather than a `000`/`001` filename
// filter is what keeps `migration_helpers.go` and the backfills inside the
// contract.
var runnerOwnedFiles = map[string]bool{
	"00_migrations.go":     true, // PrintMigrationPlanTo, the collision scanner
	"main.go":              true, // MigrateStatus prints the plan a person asked for
	"reset.go":             true,
	"runner_logging.go":    true,
	"migration_logging.go": true,
}

// migrationSourceFiles returns every non-test Go file in this directory that is
// migration code rather than runner code.
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
		if runnerOwnedFiles[name] {
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
// It works alongside TestMigrationLogOutputMatchesRegisteredVersion (#3299),
// which stays: that test reads every string literal, so it still catches a
// hardcoded version drifting inside an error message, which is not a log line
// and therefore not this test's subject.
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

			// The builtin, which needs no package qualifier.
			if ident, ok := call.Fun.(*ast.Ident); ok && (ident.Name == "println" || ident.Name == "print") {
				t.Errorf("%s:%d: the %s builtin writes to stderr.\n"+
					"Route content through migrationLog(); delete a pure progress line.",
					name, fset.Position(call.Pos()).Line, ident.Name)
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
			// fmt.Errorf and fmt.Sprintf build values; the Print family writes
			// to a stream, and so does the Fprint family whenever its writer is
			// a standard stream rather than a buffer.
			if pkg.Name != "fmt" && pkg.Name != "log" {
				return true
			}
			if strings.HasPrefix(selector.Sel.Name, "Fprint") && len(call.Args) > 0 {
				if writer, ok := call.Args[0].(*ast.SelectorExpr); ok {
					if base, ok := writer.X.(*ast.Ident); ok && base.Name == "os" &&
						(writer.Sel.Name == "Stdout" || writer.Sel.Name == "Stderr") {
						t.Errorf("%s:%d: %s.%s writes to os.%s.\n"+
							"Route content through migrationLog(); delete a pure progress line.",
							name, fset.Position(call.Pos()).Line, pkg.Name, selector.Sel.Name, writer.Sel.Name)
					}
				}
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

			// A field is written either as a loose key/value pair, where keys
			// sit at even offsets after the message, or as an attr —
			// slog.String("migration", v) — whose key is its first argument.
			// Reading only the loose form is how the pre-cleanup shape of
			// 001015340_push_subscriptions_school_portal.go would have slipped
			// through this test.
			for i, arg := range args[1:] {
				key, ok := attrKey(arg)
				if !ok && i%2 == 0 {
					key, ok = stringLiteral(arg)
				}
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

// attrKey returns the key of a slog attr constructor — slog.String("migration",
// v), slog.Int64("rows", n) — which carries its key as a call argument rather
// than as a loose element of the variadic list.
func attrKey(expr ast.Expr) (string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return "", false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	if pkg, ok := selector.X.(*ast.Ident); !ok || pkg.Name != "slog" {
		return "", false
	}
	return stringLiteral(call.Args[0])
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
