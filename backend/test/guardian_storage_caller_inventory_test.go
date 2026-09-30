package test

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

	"github.com/stretchr/testify/assert"
)

// The Contract (#2757) removed the users.students_guardians rollback mirror
// the guardian cutover (#2756) had kept, with its routing and mirror triggers
// and write counter. Application code, fixtures and behavior tests alike read
// and write People Directory's relationship, Care Plan's pickup permission and
// Identity & Access's portal access through the owners or the tenant-safe
// guardian-link projection. Historical migrations, the operator-only backfill
// command and the restore helpers keep the name for the frozen
// Expand/Backfill/Cutover contracts, which recreate the mirror inside
// disposable clones.
func TestGuardianStorageCallerInventory(t *testing.T) {
	t.Parallel()
	root := ".."
	exempt := map[string]bool{
		"cmd/backfill.go":                                true, // operator-only backfill command
		"cmd/backfill_test.go":                           true, // drives that command on a restored pre-cutover clone
		"test/guardian_storage_cutover.go":               true, // historical restore helpers
		"test/fixtures_guardians.go":                     true, // fixtures write the restored pre-cutover table
		"test/guardian_storage_caller_inventory_test.go": true, // this pattern
		"database/repositories/studentintegration/student_caller_inventory_test.go": true, // negative sample
	}
	files := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() {
			if relative == "database/migrations" || relative == "internal/architecture" || entry.Name() == "testdata" || entry.Name() == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || exempt[relative] {
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
				t.Errorf("%s: invalid string: %v", positions.Position(literal.Pos()), err)
				return true
			}
			if referencesRetiredGuardianStorage(value) {
				t.Errorf("retired guardian storage literal at %s: %q", positions.Position(literal.Pos()), value)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatalf("inventory: %v", err)
	}
	t.Logf("Inventory: %d Go files, zero retired guardian storage callers", files)
}

var guardianStorageSQLComment = regexp.MustCompile(`(?m)^\s*--[^\n]*`)

var guardianStorageReference = regexp.MustCompile(`(?i)\bstudents_guardians(?:_compatibility_writes|_id_seq)?\b|\b(?:route_students_guardians_compatibility|mirror_student_guardian_relationship|mirror_student_guardian_pickup_permission|mirror_guardian_student_access|enforce_single_primary_student_guardian)\b`)

func referencesRetiredGuardianStorage(value string) bool {
	value = guardianStorageSQLComment.ReplaceAllString(value, "")
	value = strings.ReplaceAll(value, `"`, "")
	return guardianStorageReference.MatchString(value)
}

func TestGuardianStorageCallerInventoryRecognizesStorageNotOwnerTables(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`SELECT * FROM "users"."students_guardians"`, `UPDATE users.students_guardians SET permissions = '{}'`,
		`students_guardians_compatibility_writes`, `route_students_guardians_compatibility`,
		`DELETE FROM students_guardians`,
	} {
		assert.True(t, referencesRetiredGuardianStorage(value), "missed retired storage: %s", value)
	}
	for _, value := range []string{
		`users.student_guardian_relationships`, `users.student_guardian_pickup_permissions`,
		`auth.guardian_student_access`, `json:"student_guardians"`, `alias:student_guardian`,
		"-- users.students_guardians is historical\nSELECT 42",
	} {
		assert.False(t, referencesRetiredGuardianStorage(value), "not a retired storage reference: %s", value)
	}
}
