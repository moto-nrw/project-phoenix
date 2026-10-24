package architecture

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"
	"testing"
)

const semanticLoaderModule = "example.test/semantic-loader"
const semanticLoaderDependency = "external.test/semantic-dependency"

func semanticLoaderFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	project := filepath.Join(root, "project")
	writeFile(t, filepath.Join(project, "go.mod"), "module "+semanticLoaderModule+"\n\ngo 1.25\n\nrequire "+semanticLoaderDependency+" v0.0.0\nreplace "+semanticLoaderDependency+" => ../dependency\n")
	writeFile(t, filepath.Join(root, "dependency", "go.mod"), "module "+semanticLoaderDependency+"\n\ngo 1.25\n")
	writeFile(t, filepath.Join(root, "dependency", "value.go"), `package dependency
type Value[T any] struct { value T }
func (v Value[T]) Get() T { return v.value }
type Embedded[T any] struct { Value[T] }
func Wrap[T any](value T) Embedded[T] { return Embedded[T]{Value[T]{value}} }
`)
	writeFile(t, filepath.Join(project, "domain", "value.go"), `package domain
type Value struct { Label string }
func New(label string) Value { return Value{Label: label} }
`)
	writeFile(t, filepath.Join(project, "value.go"), `package project
import (
  dependency "`+semanticLoaderDependency+`"
  domain "`+semanticLoaderModule+`/domain"
)
func Resolve() string {
  value := domain.New("value")
  wrapped := dependency.Wrap(value)
  return wrapped.Get().Label
}
`)
	return project
}

func TestSemanticLoaderKeepsOwnSyntaxAndImportedGenericTypes(t *testing.T) {
	t.Parallel()
	project := semanticLoaderFixture(t)
	loaded, err := loadTypedPackages(project, Build{GOOS: "linux", GOARCH: "amd64"})
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d initial packages, want both own packages", len(loaded))
	}
	var generic, embedded, ownReference bool
	for _, pkg := range loaded {
		if len(pkg.Syntax) == 0 || pkg.TypesInfo == nil {
			t.Fatalf("own package %s lost typed syntax", pkg.PkgPath)
		}
		for _, file := range pkg.Syntax {
			for _, declaration := range file.Decls {
				if function, ok := declaration.(*ast.FuncDecl); ok && function.Body == nil {
					t.Fatalf("own function %s.%s lost its body", pkg.PkgPath, function.Name.Name)
				}
			}
		}
		if pkg.PkgPath != semanticLoaderModule {
			continue
		}
		dependency := pkg.Imports[semanticLoaderDependency]
		if dependency == nil || dependency.Types == nil || !dependency.Types.Complete() {
			t.Fatal("imported dependency types are incomplete")
		}
		if len(dependency.Syntax) != 0 {
			t.Fatal("dependency syntax was loaded although only its exported types are used")
		}
		for identifier, instance := range pkg.TypesInfo.Instances {
			if identifier.Name == "Wrap" && instance.TypeArgs.Len() == 1 {
				argument, ok := instance.TypeArgs.At(0).(*types.Named)
				generic = ok && argument.Obj().Pkg().Path() == semanticLoaderModule+"/domain"
			}
		}
		for selector, selection := range pkg.TypesInfo.Selections {
			if selector.Sel.Name == "Get" {
				embedded = selection.Obj().Pkg().Path() == semanticLoaderDependency && len(selection.Index()) == 2
			}
		}
		for identifier, object := range pkg.TypesInfo.Uses {
			if identifier.Name == "New" && object.Pkg() != nil {
				ownReference = object.Pkg().Path() == semanticLoaderModule+"/domain"
			}
		}
	}
	if !generic || !embedded || !ownReference {
		t.Fatalf("typed syntax lost: external generic=%t, promoted method=%t, own cross-package reference=%t", generic, embedded, ownReference)
	}
}

func TestSemanticLoaderRejectsOwnFunctionBodyErrors(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"value.go", "domain/value.go"} {
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			project := semanticLoaderFixture(t)
			filename := filepath.Join(project, filepath.FromSlash(file))
			writeFile(t, filename, readFile(t, filename)+"\nfunc Broken() int { return missingValue }\n")
			_, err := loadTypedPackages(project, Build{GOOS: "linux", GOARCH: "amd64"})
			if err == nil || !strings.Contains(err.Error(), "missingValue") {
				t.Fatalf("own function-body error was not rejected: %v", err)
			}
		})
	}
}

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
