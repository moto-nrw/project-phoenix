package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

// moduleFacadeFixture builds a package whose Owner embeds `halves` facades
// of `perHalf` methods each and declares one method of its own, the shape of
// identityaccess.Owner.
func moduleFacadeFixture(halves, perHalf int) string {
	var b strings.Builder
	b.WriteString("package fixture\n\n")
	for h := 0; h < halves; h++ {
		fmt.Fprintf(&b, "type Facade%d interface {\n", h)
		for m := 0; m < perHalf; m++ {
			fmt.Fprintf(&b, "\tF%dM%d()\n", h, m)
		}
		b.WriteString("}\n\n")
	}
	b.WriteString("type Owner interface {\n")
	for h := 0; h < halves; h++ {
		fmt.Fprintf(&b, "\tFacade%d\n", h)
	}
	b.WriteString("\tOwn()\n}\n\n")
	// A type-set constraint is no facade, however many methods it lists.
	b.WriteString("type Constraint interface {\n\t~int\n\tFacade0\n\tFacade1\n}\n")
	return b.String()
}

// moduleFacadeFixtureWidths runs both measurements over one fixture source.
func moduleFacadeFixtureWidths(t *testing.T, src string) (declared, methodSet map[string]int) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "/x/modules/fixture/fixture.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	pkg, err := (&types.Config{}).Check("fixture", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("type-check fixture: %v", err)
	}
	methodSet, err = moduleFacadeMethodSetWidths(pkg, fset, "modules/fixture")
	if err != nil {
		t.Fatalf("measure fixture: %v", err)
	}
	declared = make(map[string]int)
	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.TypeSpec); ok {
			if iface, ok := spec.Type.(*ast.InterfaceType); ok {
				if width := moduleFacadeMethodCount(iface); width > moduleFacadeThreshold {
					declared["modules/fixture/fixture.go:"+spec.Name.Name] = width
				}
			}
		}
		return true
	})
	return declared, methodSet
}

// TestModuleFacadeMethodSetCatchesCompositeGrowth is the #6178 point 3
// regression: an Owner that grows by embedding one more small facade stays
// invisible to the declared count and must fail the method-set register.
func TestModuleFacadeMethodSetCatchesCompositeGrowth(t *testing.T) {
	t.Parallel()

	const key = "modules/fixture/fixture.go:Owner"
	// Seed: 3 facades of 6 methods plus one own method = 19.
	declared, methodSet := moduleFacadeFixtureWidths(t, moduleFacadeFixture(3, 6))
	if len(declared) != 0 {
		t.Fatalf("declared register should see no facade above %d, got %v", moduleFacadeThreshold, declared)
	}
	if got := methodSet[key]; got != 19 {
		t.Fatalf("method set of Owner = %d, want 19 (widths %v)", got, methodSet)
	}
	if len(methodSet) != 1 {
		t.Fatalf("only the composite may be measured; embedded facades and constraints stay out: %v", methodSet)
	}
	allowlist := map[string]int{key: 19}
	if v := moduleFacadeViolations(methodSet, allowlist, moduleFacadeMethodSetMeasure); len(v) != 0 {
		t.Fatalf("seeded composite must pass: %v", v)
	}

	// Growth by embedding a fourth facade: still 0 declared violations.
	declared, methodSet = moduleFacadeFixtureWidths(t, moduleFacadeFixture(4, 6))
	if v := moduleFacadeViolations(declared, nil, moduleFacadeDeclaredMeasure); len(v) != 0 {
		t.Fatalf("declared register unexpectedly sees the growth: %v", v)
	}
	v := moduleFacadeViolations(methodSet, allowlist, moduleFacadeMethodSetMeasure)
	if len(v) != 1 || !strings.Contains(v[0], "has a method set of 25 methods, allowed 19") {
		t.Fatalf("composite growth 19 -> 25 must fail the method-set register, got %v", v)
	}

	// A new composite above the threshold that is not registered fails too.
	if v := moduleFacadeViolations(methodSet, nil, moduleFacadeMethodSetMeasure); len(v) != 1 || !strings.Contains(v[0], "not in the allowlist") {
		t.Fatalf("unregistered composite must fail, got %v", v)
	}
}

// TestModuleFacadeMethodSetRejectsUnresolvedEmbeddings: the scan resolves
// imports outside the module to empty packages. A composite that embeds such
// an interface must fail loudly rather than lose those methods.
func TestModuleFacadeMethodSetRejectsUnresolvedEmbeddings(t *testing.T) {
	t.Parallel()

	src := "package fixture\n\nimport \"example.com/external\"\n\ntype Inner interface{ external.Reader }\n\ntype Owner interface {\n\tInner\n\tOwn()\n}\n"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "/x/modules/fixture/fixture.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	imp, err := newModuleFacadeImporter(t.TempDir())
	if err != nil {
		t.Fatalf("importer: %v", err)
	}
	pkg, _ := (&types.Config{Importer: imp, Error: func(error) {}}).Check("fixture", fset, []*ast.File{file}, nil)
	_, err = moduleFacadeMethodSetWidths(pkg, fset, "modules/fixture")
	if err == nil || !strings.Contains(err.Error(), "did not resolve to an interface") {
		t.Fatalf("unresolved embedding must fail the scan, got %v", err)
	}
}
