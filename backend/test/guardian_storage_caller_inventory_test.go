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
)

// The guardian cutover (#2756) left users.students_guardians in place as a
// rollback-only mirror of People Directory's relationship, Care Plan's pickup
// permission and Identity & Access's portal access, kept by triggers. Current
// application providers read and write those facts through the owners or the
// tenant-safe guardian-link projection, never through the mirror and never
// through its routing counter. Migrations own the compatibility SQL, the
// backfill CLI drives the migration package, and test-support packages build
// fixtures; none of them is a provider.
func TestGuardianStorageCallerInventory(t *testing.T) {
	t.Parallel()
	mirror := regexp.MustCompile(`\bstudents_guardians\b|\bstudents_guardians_compatibility_writes\b`)
	for _, root := range []string{"api", "services", "modules", "workflows", "database/repositories", "models", "seed", "auth"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if strings.HasSuffix(entry.Name(), "test") && path != filepath.Join("..", root) {
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
				if match := mirror.FindString(value); match != "" {
					t.Errorf("%s: application references the rollback-only guardian mirror: %q", positions.Position(literal.Pos()), match)
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatalf("inventory %s: %v", root, err)
		}
	}
}
