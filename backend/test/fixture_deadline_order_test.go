package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestFixtureDeadlinesStartAfterNestedFixtures is the source gate for #5191:
// no fixture in this package may create its own deadline before a nested
// fixture that does not share it. A nested fixture is a package function
// that takes a testing.TB or creates its own deadline. fixtureTenantID and
// Tenant count only where they first resolve the test's tenant in the
// function, because only then can they create it. A call that receives the
// deadline's context shares the budget and is allowed. The register of
// exceptions is empty.
func TestFixtureDeadlinesStartAfterNestedFixtures(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	require.NoError(t, err)
	funcs := map[string]*ast.FuncDecl{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				funcs[fn.Name.Name] = fn
			}
		}
	}
	nested := map[string]bool{}
	for name, fn := range funcs {
		nested[name] = name != "parseTimeHHMM" && (takesTestingTB(fn) || createsDeadline(fn))
	}

	var findings []string
	for name, fn := range funcs {
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			ctxName, ok := deadlineAssignment(n)
			if !ok {
				return true
			}
			start, first, last := n.End(), token.NoPos, token.NoPos
			ast.Inspect(fn.Body, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && id.Name == ctxName && id.Pos() > start {
					if first == token.NoPos {
						first = id.Pos()
					}
					last = id.Pos()
				}
				return true
			})
			ast.Inspect(fn.Body, func(m ast.Node) bool {
				call, ok := m.(*ast.CallExpr)
				if !ok || call.Pos() < start || call.Pos() > last {
					return true
				}
				callee, ok := call.Fun.(*ast.Ident)
				if !ok || !nested[callee.Name] || callee.Name == name || passesIdent(call, ctxName) {
					return true
				}
				if (callee.Name == "fixtureTenantID" || callee.Name == "Tenant") && resolvesTenantBefore(fn, call.Pos(), nested, name) {
					return true
				}
				findings = append(findings, fset.Position(call.Pos()).String()+": "+name+" calls "+callee.Name+" inside its own deadline")
				return true
			})
			return true
		})
	}
	require.Empty(t, findings, "start the fixture's deadline after its nested fixtures, right before its first own statement (#5191)")
}

func takesTestingTB(fn *ast.FuncDecl) bool {
	for _, param := range fn.Type.Params.List {
		typ := param.Type
		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
		}
		if sel, ok := typ.(*ast.SelectorExpr); ok && (sel.Sel.Name == "TB" || sel.Sel.Name == "T") {
			return true
		}
	}
	return false
}

func createsDeadline(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		_, ok := deadlineAssignment(n)
		found = found || ok
		return !found
	})
	return found
}

// deadlineAssignment matches `ctx, cancel := context.WithTimeout/WithDeadline(...)`.
func deadlineAssignment(n ast.Node) (string, bool) {
	assign, ok := n.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return "", false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "WithTimeout" && sel.Sel.Name != "WithDeadline") {
		return "", false
	}
	id, ok := assign.Lhs[0].(*ast.Ident)
	return id.Name, ok
}

func passesIdent(call *ast.CallExpr, name string) bool {
	found := false
	for _, arg := range call.Args {
		ast.Inspect(arg, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		})
	}
	return found
}

// resolvesTenantBefore reports whether fn already resolved the test's tenant
// before pos, through a nested fixture or an earlier tenant lookup.
func resolvesTenantBefore(fn *ast.FuncDecl, pos token.Pos, nested map[string]bool, self string) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && call.Pos() < pos {
			if id, ok := call.Fun.(*ast.Ident); ok && nested[id.Name] && id.Name != self && id.Name != "TokenHash" {
				found = true
			}
		}
		return !found
	})
	return found
}
