package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Rollback SQL belongs only to migrations. Current application providers must
// not reference either the compatibility view or its archived base table.
// The migration CLI descriptions and test fixture builders are not providers.
func TestRequestChildStorageCallerInventory(t *testing.T) {
	t.Parallel()
	legacy := regexp.MustCompile(`\brequest_child_offerings(?:_legacy)?\b`)
	for _, root := range []string{"api", "services", "modules", "database/repositories", "models"} {
		err := filepath.WalkDir(filepath.Join("..", root), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, path, nil, 0)
			if err != nil {
				return err
			}
			wireTags := make(map[*ast.BasicLit]bool)
			ast.Inspect(file, func(node ast.Node) bool {
				if field, ok := node.(*ast.Field); ok && field.Tag != nil {
					tag, err := strconv.Unquote(field.Tag.Value)
					// Retained JSON response and audit keys are not storage callers.
					wireTags[field.Tag] = err == nil && reflect.StructTag(tag).Get("bun") == ""
				}
				literal, ok := node.(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING || wireTags[literal] {
					return true
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Errorf("%s: invalid string: %v", positions.Position(literal.Pos()), err)
				} else if legacy.MatchString(value) {
					t.Errorf("%s: application references rollback-only request-child storage", positions.Position(literal.Pos()))
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
