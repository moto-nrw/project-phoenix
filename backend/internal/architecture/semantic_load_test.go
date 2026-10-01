package architecture

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"testing"
)

func TestDependencyParsingKeepsOnlyTypeRelevantSource(t *testing.T) {
	t.Parallel()
	dependencyRoot := filepath.Join(t.TempDir(), "mod") + string(filepath.Separator)
	parse := parseWithoutDependencyBodies([]string{dependencyRoot})
	source := []byte(`// Package sample documents itself.
package sample

// Plain is documented.
func Plain() int { return 1 }

func Generic[T any](value T) T { return value }

func (Receiver) Method() {}

type Receiver struct{}
`)

	bodies := func(file *ast.File) map[string]bool {
		result := map[string]bool{}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				result[function.Name.Name] = function.Body != nil
			}
		}
		return result
	}

	dependency, err := parse(token.NewFileSet(), filepath.Join(dependencyRoot, "sample.go"), source)
	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(dependency); got["Plain"] || got["Method"] || !got["Generic"] {
		t.Fatalf("dependency bodies = %v, want only the generic body kept", got)
	}
	if len(dependency.Comments) != 0 {
		t.Fatalf("dependency comments were kept: %d groups", len(dependency.Comments))
	}

	project, err := parse(token.NewFileSet(), filepath.Join(t.TempDir(), "project", "sample.go"), source)
	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(project); !got["Plain"] || !got["Method"] || !got["Generic"] {
		t.Fatalf("project bodies = %v, want every body kept", got)
	}
	if len(project.Comments) == 0 {
		t.Fatal("project comments were dropped")
	}
}
