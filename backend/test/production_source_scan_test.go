package test

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandlerScannersUseProductionBuildNotHookFileNames(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.test/handler-scan\n\ngo 1.25\n"), 0o600))
	api := filepath.Join(root, "api")
	require.NoError(t, os.Mkdir(api, 0o700))
	source := "package api\ntype holder struct {\n repo store.ExampleRepository\n}\nfunc handle() {" + strings.Repeat("if true {}\n", 16) + "}\n"
	active := "ordinary_test_helpers.go"
	require.NoError(t, os.WriteFile(filepath.Join(api, active), []byte(source), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(api, "tagged.go"), []byte("//go:build testhooks\n\n"+source), 0o600))

	declarations, err := scanAPIRepoDecls(root)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"api/" + active: 1}, declarations)
	scores, err := scanHandlerComplexity(root)
	require.NoError(t, err)
	require.Equal(t, map[string]int{"api/" + active + ":handle": 16}, scores)
}

func TestProductionBuildExcludesTestHooks(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	require.NoError(t, err)
	active, err := productionGoSources(root)
	require.NoError(t, err)
	for path := range active {
		if strings.HasSuffix(path, "_test_helpers.go") {
			t.Errorf("test hooks compile in production: %s", path)
		}
		_, source, err := parseGoSourceCached(path, nil)
		require.NoError(t, err)
		for _, declaration := range source.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Name.IsExported() && strings.Contains(function.Name.Name, "ForTest") {
				t.Errorf("test-only export compiles in production: %s in %s", function.Name.Name, path)
			}
		}
	}
}
