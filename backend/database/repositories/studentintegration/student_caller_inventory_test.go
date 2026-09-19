package studentintegration_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Inventory source literals rather than grep output: comments, relationship
// tables, migration compatibility SQL and test fixtures are not app callers.
// Runtime integration separately proves that the retained Student model tag
// cannot make generic repository operations fall back to the rollback view.
func TestStudentCutoverCallerInventory(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	oldTable := regexp.MustCompile(`\busers\s*"?\s*\.\s*"?students\b`)
	sqlComments := regexp.MustCompile(`(?m)--[^\n]*`)
	files, literals, modelTags := 0, 0, 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if relative == "cmd/backfill.go" || relative == "cmd/migrate_student_compatibility.go" {
			return nil // Operator-only migration/rollback commands, not application callers.
		}
		if entry.IsDir() {
			if relative == "database/migrations" || relative == "test" || relative == "internal/architecture" || entry.Name() == "testdata" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		positions := token.NewFileSet()
		file, err := parser.ParseFile(positions, path, nil, 0)
		if err != nil {
			return err
		}
		files++
		ast.Inspect(file, func(node ast.Node) bool {
			literal, ok := node.(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			literals++
			if relative == "models/users/student.go" && value == "bun:\"schema:users,table:students\"" {
				modelTags++
				return true
			}
			if oldTable.MatchString(sqlComments.ReplaceAllString(value, "")) || strings.Contains(value, "table:students\"") || strings.Contains(value, "table:students,") {
				t.Errorf("old application table literal at %s: %q", positions.Position(literal.Pos()), value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if modelTags != 1 {
		t.Fatalf("expected one documented compatibility DTO tag, got %d", modelTags)
	}
	t.Logf("Inventory: %d application Go files, %d string literals, %d compatibility DTO tag, zero old application table callers", files, literals, modelTags)
}
