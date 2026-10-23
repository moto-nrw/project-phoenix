package migrations

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEveryUploadKindHasDataLifecycleClassification(t *testing.T) {
	t.Parallel()
	observed := make(map[string]bool)
	err := filepath.WalkDir("../..", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".." {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, kind := range lifecycleUploadKinds(t, string(data)) {
			observed[kind] = true
		}
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, observed, "upload discovery must not pass vacuously")
	catalog := make(map[string]bool)
	for _, file := range readLifecycleCatalog(t).Files {
		require.False(t, catalog[file.Kind], "duplicate file kind %s", file.Kind)
		catalog[file.Kind] = true
		require.Contains(t, []string{"tenantDir", "tenantPrefix", "account"}, file.Layout)
		require.Contains(t, []string{"include", "excludeSecret", "excludeInternal"}, file.Export)
		require.NotEmpty(t, strings.TrimSpace(file.Reason))
		require.True(t, observed[file.Kind], "stale file kind %s", file.Kind)
	}
	for kind := range observed {
		require.True(t, catalog[kind], "unclassified upload kind %s", kind)
	}
}

// This is a source-coverage tripwire, not whole-program dataflow analysis.
// Scan literals in every production Go tree (comments/tests excluded), upload
// path joins, namespace constants and literal/constant TenantKey/private-object
// call sites. Variable forwarding is checked where its kind is constructed.
func lifecycleUploadKinds(t *testing.T, source string) []string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "uploads.go", source, 0)
	require.NoError(t, err)
	constants := make(map[string]string)
	ast.Inspect(file, func(node ast.Node) bool {
		decl, ok := node.(*ast.GenDecl)
		if !ok || decl.Tok != token.CONST {
			return true
		}
		for _, spec := range decl.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if i < len(value.Values) {
					if literal, ok := value.Values[i].(*ast.BasicLit); ok && literal.Kind == token.STRING {
						constants[name.Name], err = strconv.Unquote(literal.Value)
						require.NoError(t, err)
					}
				}
			}
		}
		return true
	})
	kinds := make(map[string]bool)
	for name, value := range constants {
		if strings.HasSuffix(name, "ObjectKind") || strings.HasSuffix(name, "StorageKind") {
			kinds[value] = true
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch value := node.(type) {
		case *ast.BasicLit:
			if value.Kind != token.STRING {
				return true
			}
			literal, unquoteErr := strconv.Unquote(value.Value)
			require.NoError(t, unquoteErr)
			for _, prefix := range []string{"public/uploads/", "/uploads/"} {
				if strings.HasPrefix(literal, prefix) {
					kind := strings.Split(strings.TrimPrefix(literal, prefix), "/")[0]
					if kind != "" {
						kinds[kind] = true
					}
				}
			}
		case *ast.CallExpr:
			name := lifecycleCallName(value.Fun)
			arg := -1
			switch name {
			case "TenantKey":
				arg = 0
			case "SavePrivate", "OpenPrivate", "RemovePrivate":
				arg = 1
			}
			if arg >= 0 && len(value.Args) > arg {
				if kind := lifecycleString(value.Args[arg], constants); kind != "" {
					kinds[kind] = true
				}
			}
			if name == "Join" {
				for i := 0; i+1 < len(value.Args); i++ {
					if lifecycleString(value.Args[i], constants) == "uploads" {
						if kind := lifecycleString(value.Args[i+1], constants); kind != "" {
							kinds[kind] = true
						}
					}
				}
			}
		case *ast.CompositeLit:
			if lifecycleCallName(value.Type) == "ObjectStore" {
				for _, element := range value.Elts {
					pair, ok := element.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					if key, ok := pair.Key.(*ast.Ident); ok && key.Name == "Kind" {
						if kind := lifecycleString(pair.Value, constants); kind != "" {
							kinds[kind] = true
						}
					}
				}
			}
		}
		return true
	})
	result := make([]string, 0, len(kinds))
	for kind := range kinds {
		result = append(result, kind)
	}
	sort.Strings(result)
	return result
}

func lifecycleCallName(expr ast.Expr) string {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Name
	case *ast.SelectorExpr:
		return value.Sel.Name
	}
	return ""
}

func lifecycleString(expr ast.Expr, constants map[string]string) string {
	switch value := expr.(type) {
	case *ast.BasicLit:
		if value.Kind == token.STRING {
			result, _ := strconv.Unquote(value.Value)
			return result
		}
	case *ast.Ident:
		return constants[value.Name]
	}
	return ""
}

func TestLifecycleUploadDiscovery(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"attachment-fixture", "constant-fixture", "join-fixture", "literal-fixture", "object-fixture", "private-fixture", "storage-fixture"}, lifecycleUploadKinds(t, `package sample
const fixture = "constant-fixture"
const StudentDocumentStorageKind = "storage-fixture"
const attachmentObjectKind = "attachment-fixture"
func sample() {
 _ = "public/uploads/literal-fixture"
 _ = filepath.Join(root, "uploads", "join-fixture")
 _ = objects.TenantKey(fixture, tenant, name)
 _ = backend.SavePrivate(ctx, "private-fixture", tenant, name, source)
 _ = application.ObjectStore{Kind: "object-fixture"}
 // public/uploads/not-a-real-kind
 _ = "an unrelated string"
}`))
}
