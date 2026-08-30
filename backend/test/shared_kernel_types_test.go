package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

func TestSharedKernelTypesAreNamedDefinitions(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	contracts := []struct {
		file string
		name string
	}{
		{"internal/timezone/date.go", "Date"},
		{"internal/timezone/wall_clock.go", "WallClock"},
		{"tenant/id.go", "TenantID"},
		{"observability/correlation_id.go", "CorrelationID"},
	}

	for _, contract := range contracts {
		contract := contract
		t.Run(contract.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(backendRoot, filepath.FromSlash(contract.file))
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			for _, declaration := range file.Decls {
				group, ok := declaration.(*ast.GenDecl)
				if !ok || group.Tok != token.TYPE {
					continue
				}
				for _, specification := range group.Specs {
					typeSpec, ok := specification.(*ast.TypeSpec)
					if !ok || typeSpec.Name.Name != contract.name {
						continue
					}
					if typeSpec.Assign.IsValid() {
						t.Fatalf("%s must be a named type, not an alias", contract.name)
					}
					return
				}
			}
			t.Fatalf("%s type declaration not found in %s", contract.name, contract.file)
		})
	}
}
