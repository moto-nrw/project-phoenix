package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// traegerViewPostgres is the only package that reads across the schools of a
// Träger for the Träger-Portal (ADR 0084 section 1).
const traegerViewPostgres = "modules/traegerview/internal/adapters/postgres"

// traegerSchoolFilter is the predicate every query of that package carries,
// bound to the school set of the session's Träger as its first argument.
const traegerSchoolFilter = "= ANY(?0)"

// TestTraegerViewQueriesFilterOnTheTraegerSchools is the ratchet of ADR 0084
// section 1. The Träger projection reads in an administrative transaction, so
// RLS does not narrow it; the filter on the schools of the Träger is the
// boundary. Every database call of the package must be NewRaw over a
// package-level constant that contains the filter, with the school set bound
// as pgdialect.Array in first position. Any other query builder, a computed
// statement or a missing filter fails. The administrative boundary itself is
// pinned in transaction_boundaries.json (traeger_view_transaction.go).
func TestTraegerViewQueriesFilterOnTheTraegerSchools(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	problems, queries := traegerViewScopeProblems(t, filepath.Join(root, traegerViewPostgres))
	if queries == 0 {
		t.Fatalf("%s holds no query; the ratchet must not pass vacuously", traegerViewPostgres)
	}
	for _, problem := range problems {
		t.Error(problem)
	}
}

func traegerViewScopeProblems(t *testing.T, dir string) ([]string, int) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	constants := traegerViewStringConstants(files)
	var problems []string
	queries := 0
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !traegerViewDatabaseCall(selector.Sel.Name) {
				return true
			}
			position := fset.Position(call.Pos()).String()
			if selector.Sel.Name != "NewRaw" {
				problems = append(problems, position+": only NewRaw over a filtered constant may query in the Träger projection, found "+selector.Sel.Name)
				return true
			}
			queries++
			problems = append(problems, traegerViewRawProblems(position, call, constants)...)
			return true
		})
	}
	for name, value := range constants {
		if strings.Contains(strings.ToUpper(value), "SELECT") && !strings.Contains(value, traegerSchoolFilter) {
			problems = append(problems, "query constant "+name+" lacks the filter "+traegerSchoolFilter)
		}
	}
	return problems, queries
}

func traegerViewRawProblems(position string, call *ast.CallExpr, constants map[string]string) []string {
	if len(call.Args) < 2 {
		return []string{position + ": NewRaw must bind the school set of the Träger as its first argument"}
	}
	var problems []string
	name, ok := call.Args[0].(*ast.Ident)
	query, known := "", false
	if ok {
		query, known = constants[name.Name]
	}
	if !known {
		problems = append(problems, position+": NewRaw must run a package-level string constant")
	} else if !strings.Contains(query, traegerSchoolFilter) {
		problems = append(problems, position+": query "+name.Name+" lacks the filter "+traegerSchoolFilter)
	}
	bind, ok := call.Args[1].(*ast.CallExpr)
	if !ok {
		return append(problems, position+": the first argument must be pgdialect.Array(<school set>)")
	}
	if fn, ok := bind.Fun.(*ast.SelectorExpr); !ok || fn.Sel.Name != "Array" || len(bind.Args) != 1 {
		problems = append(problems, position+": the first argument must be pgdialect.Array(<school set>)")
	}
	return problems
}

func traegerViewDatabaseCall(name string) bool {
	switch name {
	case "NewRaw", "NewSelect", "NewInsert", "NewUpdate", "NewDelete", "NewMerge", "NewValues", "NewTruncateTable",
		"ExecContext", "QueryContext", "QueryRowContext", "Exec", "Query", "QueryRow":
		return true
	}
	return false
}

func traegerViewStringConstants(files []*ast.File) map[string]string {
	constants := map[string]string{}
	for _, file := range files {
		for _, decl := range file.Decls {
			general, ok := decl.(*ast.GenDecl)
			if !ok || general.Tok != token.CONST {
				continue
			}
			for _, spec := range general.Specs {
				value := spec.(*ast.ValueSpec)
				for i, name := range value.Names {
					if i >= len(value.Values) {
						continue
					}
					if literal, ok := value.Values[i].(*ast.BasicLit); ok && literal.Kind == token.STRING {
						if text, err := strconv.Unquote(literal.Value); err == nil {
							constants[name.Name] = text
						}
					}
				}
			}
		}
	}
	return constants
}

// TestTraegerViewScopeRatchetRejectsUnfilteredQueries proves the scanner
// fails on the shapes it exists to stop.
func TestTraegerViewScopeRatchetRejectsUnfilteredQueries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := `package postgres

const unfiltered = "SELECT id FROM platform.schools WHERE deleted_at IS NULL"
const filtered = "SELECT id FROM platform.schools WHERE id = ANY(?0)"

func reads(db DB, schools []int64, query string) {
	db.NewRaw(unfiltered, pgdialect.Array(schools))
	db.NewRaw(filtered, schools)
	db.NewRaw(query, pgdialect.Array(schools))
	db.NewSelect()
}
`
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	problems, queries := traegerViewScopeProblems(t, dir)
	if queries != 3 || len(problems) != 5 {
		t.Fatalf("want 3 raw queries and 5 problems (unfiltered call and constant, unbound set, computed statement, builder), got %d and %q", queries, problems)
	}
}
