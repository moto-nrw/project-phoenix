package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAPITestsDoNotCallBroadSetup(t *testing.T) {
	t.Parallel()

	backendRoot := filepath.Clean("..")
	var callers []string
	err := filepath.WalkDir(backendRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, string(filepath.Separator)+"api"+string(filepath.Separator)+"testutil"+string(filepath.Separator)) {
			return nil
		}

		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(file, func(node ast.Node) bool {
			function, ok := node.(*ast.FuncDecl)
			if !ok || function.Body == nil {
				return true
			}

			name := function.Name.Name
			relativePath := strings.TrimPrefix(path, backendRoot+string(filepath.Separator))
			if relativePath == "api/base_utility_test.go" && name == "setupAPIRootRoute" {
				return false
			}

			ast.Inspect(function.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "SetupAPITest" {
					callers = append(callers, relativePath+"#"+name)
				}
				return true
			})
			return false
		})
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, callers, "API tests must use route/module-sized builders instead of SetupAPITest")
}
