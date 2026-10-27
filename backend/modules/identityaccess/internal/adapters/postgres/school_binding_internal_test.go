package postgres

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var accountTenantInsert = regexp.MustCompile(`(?i)insert\s+into\s+auth\.account_tenants`)

// TestEveryAccountTenantInsertRefusesCaterers keeps ADR 0077 from eroding:
// every production statement that maps an account to a school runs through
// bindAccountToSchool, which refuses a caterer account in the same statement.
// A new mapping write outside it fails here, as does a bun model or table
// expression that could insert into auth.account_tenants without the check.
// Migrations backfill history and are not scanned.
func TestEveryAccountTenantInsertRefusesCaterers(t *testing.T) {
	t.Parallel()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("backend root not found at %s: %v", root, err)
	}
	guarded := 0
	walkErr := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if strings.HasPrefix(name, ".") || name == "testdata" || name == "migrations" || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		guarded += scanAccountTenantWrites(t, root, path)
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
	if guarded == 0 {
		t.Fatal("found no guarded auth.account_tenants insert; the scan no longer sees bindAccountToSchool")
	}
}

// scanAccountTenantWrites reports every unguarded mapping write of one file
// and returns how many guarded ones it holds.
func scanAccountTenantWrites(t *testing.T, root, path string) int {
	t.Helper()
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, path, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	rel, _ := filepath.Rel(root, path)
	// Test fixtures seed memberships directly; they serve no request.
	fixture := strings.HasPrefix(filepath.ToSlash(rel), "test/")
	guarded := 0
	var stack []ast.Node
	ast.Inspect(file, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		stack = append(stack, node)
		switch value := node.(type) {
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return true
			}
			text := value.Value
			switch {
			case accountTenantInsert.MatchString(text):
				if insideBindCall(stack) {
					guarded++
				} else if !fixture {
					t.Errorf("%s: auth.account_tenants insert outside bindAccountToSchool (ADR 0077)", files.Position(value.Pos()))
				}
			case !fixture && strings.Contains(text, "table:auth.account_tenants"):
				t.Errorf("%s: a bun model of auth.account_tenants can insert without the caterer refusal (ADR 0077)", files.Position(value.Pos()))
			case !fixture && strings.Trim(text, "`\"") == "auth.account_tenants" && insideCall(stack, "ModelTableExpr"):
				t.Errorf("%s: a model insert into auth.account_tenants bypasses the caterer refusal (ADR 0077)", files.Position(value.Pos()))
			}
		}
		return true
	})
	return guarded
}

func insideBindCall(stack []ast.Node) bool { return insideCall(stack, "bindAccountToSchool") }

func insideCall(stack []ast.Node, name string) bool {
	for i := len(stack) - 1; i >= 0; i-- {
		call, ok := stack[i].(*ast.CallExpr)
		if !ok {
			continue
		}
		if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == name {
			return true
		}
	}
	return false
}
