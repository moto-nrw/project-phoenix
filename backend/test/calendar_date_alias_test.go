package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// isCanonicalDateAlias accepts Go aliases of timezone.Date, not new named
// types or timestamp aliases. The import path and declaration establish type
// identity; the caller's choice of package qualifier does not.
func isCanonicalDateAlias(root, source, spelling string) bool {
	parts := strings.Split(strings.TrimPrefix(spelling, "*"), ".")
	if len(parts) != 2 {
		return false
	}
	consumer, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, source), nil, parser.ImportsOnly)
	if err != nil {
		return false
	}
	const modulePrefix = "github.com/moto-nrw/project-phoenix/"
	importPath := dateAliasImportPath(consumer, parts[0])
	if !strings.HasPrefix(importPath, modulePrefix) {
		return false
	}
	files, err := filepath.Glob(filepath.Join(root, strings.TrimPrefix(importPath, modulePrefix), "*.go"))
	if err != nil {
		return false
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		decls, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			return false
		}
		for _, decl := range decls.Decls {
			group, ok := decl.(*ast.GenDecl)
			if !ok || group.Tok != token.TYPE {
				continue
			}
			for _, spec := range group.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != parts[1] || !typeSpec.Assign.IsValid() {
					continue
				}
				selector, ok := typeSpec.Type.(*ast.SelectorExpr)
				if !ok || selector.Sel.Name != "Date" {
					return false
				}
				qualifier, ok := selector.X.(*ast.Ident)
				return ok && dateAliasImportPath(decls, qualifier.Name) == modulePrefix+"internal/timezone"
			}
		}
	}
	return false
}

func dateAliasImportPath(file *ast.File, qualifier string) string {
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == qualifier {
			return path
		}
	}
	return ""
}

func TestCalendarDateAliasTypeIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, imported, declaration string
		want                        bool
	}{
		{"canonical alias", "github.com/moto-nrw/project-phoenix/internal/timezone", "type Date = clock.Date", true},
		{"new named type", "github.com/moto-nrw/project-phoenix/internal/timezone", "type Date clock.Date", false},
		{"timestamp alias", "time", "type Date = clock.Time", false},
		{"lookalike package", "example.com/timezone", "type Date = clock.Date", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			dir := filepath.Join(root, "modules/example/ports")
			if err := os.MkdirAll(dir, 0o700); err != nil {
				t.Fatal(err)
			}
			write := func(path, contents string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(dir, "date.go"), "package ports\nimport clock "+strconv.Quote(tc.imported)+"\n"+tc.declaration)
			consumer := "modules/example/row.go"
			write(filepath.Join(root, consumer), "package example\nimport calendar \"github.com/moto-nrw/project-phoenix/modules/example/ports\"\n")
			for _, spelling := range []string{"calendar.Date", "*calendar.Date"} {
				if got := isCanonicalDateAlias(root, consumer, spelling); got != tc.want {
					t.Errorf("%s: canonical date alias = %v, want %v", spelling, got, tc.want)
				}
			}
		})
	}
}
