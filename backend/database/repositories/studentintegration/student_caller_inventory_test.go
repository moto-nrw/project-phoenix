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
// Contract removes the last compatibility DTO table tag as well: no runtime
// provider may retain a binding to the old view, archive or dependent view.
func TestStudentCutoverCallerInventory(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "..")
	files, literals := 0, 0
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
			if referencesOldStudentStorage(value) {
				t.Errorf("old application table literal at %s: %q", positions.Position(literal.Pos()), value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Inventory: %d application Go files, %d string literals, zero old application table callers or compatibility DTO tags", files, literals)
}

var studentStorageSQLComment = regexp.MustCompile(`(?m)^\s*--[^\n]*`)
var studentStorageReference = regexp.MustCompile(`(?i)(?:\busers\s*\.\s*|\btable:|\b(?:from|join|update|into|table)\s+)(?:students(?:_legacy)?|expired_privacy_consents)\b|\b(?:student_compatibility_reads|student_compatibility_writes|route_student_compatibility)\b`)

func referencesOldStudentStorage(value string) bool {
	value = studentStorageSQLComment.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, `"`, "")
	return studentStorageReference.MatchString(value)
}

func TestStudentStorageCallerInventoryRecognizesStorageNotWireNames(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`SELECT * FROM "users"."students"`, `UPDATE users.students_legacy SET sick = false`,
		`SELECT * FROM students`, `bun:"schema:users,table:students"`,
		`users.expired_privacy_consents`, `student_compatibility_reads`,
	} {
		if !referencesOldStudentStorage(value) {
			t.Errorf("missed retired storage: %s", value)
		}
	}
	for _, value := range []string{`json:"students"`, `users.students_guardians`, `users.student_profiles`, "-- users.students is historical\nSELECT 42"} {
		if referencesOldStudentStorage(value) {
			t.Errorf("not a retired storage reference: %s", value)
		}
	}
}
