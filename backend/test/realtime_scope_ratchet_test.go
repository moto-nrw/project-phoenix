package test

import (
	"go/ast"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// Production producers must name a school. BroadcastToAll remains available on
// the local hub for compatibility tests, not as an unaddressed producer port.
func TestRealtimeProducersRequireSchool(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		_, file, err := parseGoSourceCached(path, nil)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			target, ok := node.(*ast.SelectorExpr)
			if ok && target.Sel.Name == "BroadcastToAll" {
				t.Errorf("%s: producer must use a school-scoped broadcaster", path)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
