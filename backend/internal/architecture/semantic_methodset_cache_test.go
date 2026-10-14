package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"reflect"
	"testing"
)

func contractMethodSetFixture(t *testing.T) *types.Package {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "contracts.go", `package contracts
type Empty struct{}
type Value struct{}
func (Value) Inspect() {}
type PointerOnly struct{}
func (*PointerOnly) Read(query map[string]any) {}
type Promoted struct { *PointerOnly }
type Generic[T any] struct{}
func (*Generic[T]) Inspect(value T) {}
type Port interface { Read(map[string]any) }
type CycleA struct{}
type CycleB struct{}
func (*CycleA) Advance() *CycleB { return nil }
func (*CycleB) Find(query map[string]any) *CycleA { return nil }
func RootPointer() *PointerOnly { return nil }
func RootPromoted() Promoted { return Promoted{} }
func RootGenericInt() *Generic[int] { return nil }
func RootGenericFilters() *Generic[map[string]any] { return nil }
func RootCycle() *CycleA { return nil }
`, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	config := &types.Config{GoVersion: "go1.25"}
	typed, err := config.Check("example.test/contracts", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return typed
}

func contractNamedType(t *testing.T, pkg *types.Package, name string) *types.Named {
	t.Helper()
	object := pkg.Scope().Lookup(name)
	if object == nil {
		t.Fatalf("fixture has no type %s", name)
	}
	named, ok := object.Type().(*types.Named)
	if !ok {
		t.Fatalf("fixture %s has type %T, want named", name, object.Type())
	}
	return named
}

func TestContractMethodSetCacheKeepsEmptyPointerAndGenericSets(t *testing.T) {
	t.Parallel()
	pkg := contractMethodSetFixture(t)
	cache := make(contractMethodSetCache)
	for _, test := range []struct {
		name           string
		value, pointer int
	}{
		{name: "Empty", value: 0, pointer: 0},
		{name: "Value", value: 1, pointer: 1},
		{name: "PointerOnly", value: 0, pointer: 1},
		{name: "Promoted", value: 1, pointer: 1},
		{name: "Port", value: 1, pointer: 0},
	} {
		named := contractNamedType(t, pkg, test.name)
		sets := cache.sets(named)
		if sets[0].Len() != test.value || sets[1].Len() != test.pointer {
			t.Fatalf("%s method counts = %d/%d, want %d/%d", test.name, sets[0].Len(), sets[1].Len(), test.value, test.pointer)
		}
		if _, cached := cache[named]; !cached {
			t.Fatalf("%s method sets were not retained, including empty sets", test.name)
		}
		if repeated := cache.sets(named); sets != repeated {
			t.Fatalf("%s method sets were rebuilt, including empty sets", test.name)
		}
	}

	generic := contractNamedType(t, pkg, "Generic")
	for _, argument := range []types.Type{types.Typ[types.Int], types.Typ[types.String]} {
		instance, err := types.Instantiate(nil, generic, []types.Type{argument}, true)
		if err != nil {
			t.Fatal(err)
		}
		named, ok := instance.(*types.Named)
		if !ok {
			t.Fatalf("generic instance has type %T, want named", instance)
		}
		sets := cache.sets(named)
		if sets[0].Len() != 0 || sets[1].Len() != 1 {
			t.Fatalf("generic %s lost its pointer-only method", argument)
		}
		signature, ok := sets[1].At(0).Type().(*types.Signature)
		if !ok || !types.Identical(signature.Params().At(0).Type(), argument) {
			t.Fatalf("generic %s method signature reused another instance: %v", argument, sets[1].At(0).Type())
		}
		if repeated := cache.sets(named); sets != repeated {
			t.Fatalf("generic %s method sets were rebuilt", argument)
		}
	}
}

func TestContractMethodSetCachePreservesFindingsAndRecursivePaths(t *testing.T) {
	t.Parallel()
	pkg := contractMethodSetFixture(t)
	cache := make(contractMethodSetCache)
	var findings int
	compare := func(target string, analyze func(contractMethodSetCache) []Violation) {
		t.Helper()
		want := uniqueSortedViolations(analyze(nil))
		findings += len(want)
		for range 2 {
			got := uniqueSortedViolations(analyze(cache))
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("cached %s findings differ from uncached findings:\ngot: %#v\nwant: %#v", target, got, want)
			}
		}
	}
	for _, name := range []string{"RootPointer", "RootPromoted", "RootGenericInt", "RootGenericFilters", "RootCycle"} {
		function, ok := pkg.Scope().Lookup(name).(*types.Func)
		if !ok {
			t.Fatalf("fixture %s is not a function", name)
		}
		compare(name, func(methodSets contractMethodSetCache) []Violation {
			return contractFunctionViolations(pkg.Path(), "contracts."+name, function, methodSets)
		})
	}
	for _, name := range []string{"Empty", "Value", "PointerOnly", "Promoted", "Generic", "Port", "CycleA", "CycleB"} {
		named := contractNamedType(t, pkg, name)
		compare(name, func(methodSets contractMethodSetCache) []Violation {
			return contractMethodViolations(pkg.Path(), "contracts."+name, named, methodSets)
		})
	}
	if findings == 0 {
		t.Fatal("fixture must exercise contract violations")
	}

	// An active recursion guard must not cache suppressed violations or affect
	// another declaration's later traversal and target.
	cycle := contractNamedType(t, pkg, "CycleA")
	cycleCache := make(contractMethodSetCache)
	stack := map[types.Type]struct{}{cycle: {}}
	if got := contractMethodViolationsSeen(pkg.Path(), "contracts.Suppressed", cycle, stack, cycleCache); len(got) != 0 {
		t.Fatalf("recursive named type produced findings despite its active guard: %v", got)
	}
	got := uniqueSortedViolations(contractMethodViolations(pkg.Path(), "contracts.Later", cycle, cycleCache))
	want := uniqueSortedViolations(contractMethodViolations(pkg.Path(), "contracts.Later", cycle, nil))
	if len(want) == 0 || !reflect.DeepEqual(got, want) {
		t.Fatalf("recursive context leaked into completed method sets:\ngot: %#v\nwant: %#v", got, want)
	}
}
