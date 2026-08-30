package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var routeBuilderName = regexp.MustCompile(`^(?:setup|build).*(?:Route|Module)$`)

func TestAPITestsUseNarrowRouteBuilders(t *testing.T) {
	t.Parallel()

	backendRoot := filepath.Clean("..")
	var violations []string
	err := filepath.WalkDir(backendRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
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
			isBuilder := routeBuilderName.MatchString(name)
			if isBuilder && containsServiceFactory(function.Type.Results) {
				violations = append(violations, relativePath+"#"+name+" returns services.Factory")
			}

			ast.Inspect(function.Body, func(node ast.Node) bool {
				selector, ok := node.(*ast.SelectorExpr)
				if ok && selector.Sel.Name == "SetupAPITest" {
					isSmokeTest := relativePath == "api/testutil/helpers_test.go" && name == "TestSetupAPITest"
					if !isBuilder && !isSmokeTest {
						violations = append(violations, relativePath+"#"+name+" calls SetupAPITest directly")
					}
				}
				return true
			})
			return false
		})
		return nil
	})
	require.NoError(t, err)
	require.Empty(t, violations, "API tests must hide SetupAPITest behind route/module builders that do not return services.Factory")
}

func containsServiceFactory(fields *ast.FieldList) bool {
	if fields == nil {
		return false
	}
	found := false
	ast.Inspect(fields, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "Factory" {
			if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "services" {
				found = true
				return false
			}
		}
		return !found
	})
	return found
}
