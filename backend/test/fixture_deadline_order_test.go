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

// TestFixtureDeadlinesStartAfterNestedFixtures is the source gate for #5191.
// Invariant: no fixture deadline exists before every call that does work
// under a budget of its own has returned; creating the test's tenant is such
// a call. A call that receives the deadline's context shares the budget and
// is allowed.
//
// Nothing is listed by name; everything follows from the call graph of this
// package, closed transitively:
//   - a deadline starts at an assignment from context.WithTimeout/WithDeadline
//     or from a deadline constructor, a package function that returns such a
//     call's result (directly or through another constructor);
//   - a call works under its own budget when the callee starts a deadline or
//     runs a statement under a fresh context (context.Background/TODO or a
//     .Context() method passed straight to Scan, Exec, ExecContext,
//     QueryContext, QueryRowContext, BeginTx or PingContext), directly or
//     through other package functions and methods;
//   - a tenant resolver reaches (*tenantEntry).ensure. When ensure is its only
//     own budget and the function resolved the tenant earlier, the call is a
//     cached lookup and allowed.
//
// Calls in a defer statement, or in a block that ends with a return not using
// the deadline (a failure exit), run after the fixture's own statements and
// are allowed. Known limits: calls through function values, interfaces or
// goroutines and methods of other packages' types are not followed; methods
// are matched by name; fixtures outside this package are not scanned; a
// deadline created inline in an expression, or returned through a variable
// from another function, is not seen. The register of exceptions is empty.
func TestFixtureDeadlinesStartAfterNestedFixtures(t *testing.T) {
	t.Parallel()

	findings := scanFixtureDeadlines(t, ".")
	require.Empty(t, findings, "start the fixture's deadline after its nested fixtures, right before its first own statement (#5191)")
}

type fixtureFunc struct {
	decl    *ast.FuncDecl
	callees map[string]bool
}

// scanFixtureDeadlines returns every call that runs inside a fixture's own
// deadline although it creates a context of its own.
func scanFixtureDeadlines(t *testing.T, dir string) []string {
	t.Helper()
	fset := token.NewFileSet()
	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	require.NoError(t, err)
	funcs := map[string]*fixtureFunc{}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		require.NoError(t, err)
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				funcs[funcKey(fn)] = &fixtureFunc{decl: fn, callees: map[string]bool{}}
			}
		}
	}
	methods := map[string][]string{} // method name -> keys
	for key, fn := range funcs {
		if fn.decl.Recv != nil {
			methods[fn.decl.Name.Name] = append(methods[fn.decl.Name.Name], key)
		}
	}
	resolve := func(call *ast.CallExpr) []string {
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if _, ok := funcs[fun.Name]; ok {
				return []string{fun.Name}
			}
		case *ast.SelectorExpr:
			return methods[fun.Sel.Name]
		}
		return nil
	}

	ownBudget, viaEnsureOnly, constructor := map[string]bool{}, map[string]bool{}, map[string]bool{}
	tenantResolver := map[string]bool{"tenantEntry.ensure": true}
	for key, fn := range funcs {
		ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				for _, callee := range resolve(call) {
					fn.callees[callee] = true
				}
				if runsUnderFreshContext(call) {
					ownBudget[key] = true
				}
			}
			return true
		})
	}
	// Deadline constructors: functions returning a new deadline.
	for changed := true; changed; {
		changed = false
		for key, fn := range funcs {
			if !constructor[key] && returnsDeadline(fn.decl, funcs, constructor) {
				constructor[key], changed = true, true
			}
		}
	}
	for key, fn := range funcs {
		ast.Inspect(fn.decl.Body, func(n ast.Node) bool {
			if _, ok := deadlineStart(n, funcs, constructor); ok {
				ownBudget[key] = true
			}
			return true
		})
	}
	// Fixed points over the call graph. direct excludes the path through
	// ensure, so a pure tenant lookup can be told apart from a fixture.
	direct := map[string]bool{}
	for key := range ownBudget {
		if key != "tenantEntry.ensure" {
			direct[key] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for key, fn := range funcs {
			for callee := range fn.callees {
				if propagate(ownBudget, key, callee) || propagate(tenantResolver, key, callee) {
					changed = true
				}
				if callee != "tenantEntry.ensure" && propagate(direct, key, callee) {
					changed = true
				}
			}
		}
	}
	for key := range tenantResolver {
		viaEnsureOnly[key] = !direct[key]
	}

	var findings []string
	for key, fn := range funcs {
		body := fn.decl.Body
		ast.Inspect(body, func(n ast.Node) bool {
			ctxName, ok := deadlineStart(n, funcs, constructor)
			if !ok {
				return true
			}
			start, last := n.End(), token.NoPos
			ast.Inspect(body, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && id.Name == ctxName && id.Pos() > start {
					last = id.Pos()
				}
				return true
			})
			ast.Inspect(body, func(m ast.Node) bool {
				if deferred, ok := m.(*ast.DeferStmt); ok && deferred.Pos() > start {
					return false
				}
				if block, ok := m.(*ast.BlockStmt); ok && block != body && failureExit(block, ctxName) {
					return false
				}
				call, ok := m.(*ast.CallExpr)
				if !ok || call.Pos() < start || call.Pos() > last || passesIdent(call, ctxName) {
					return true
				}
				for _, callee := range resolve(call) {
					if callee == key || !ownBudget[callee] {
						continue
					}
					if viaEnsureOnly[callee] && resolvesTenantBefore(body, call.Pos(), resolve, tenantResolver) {
						continue
					}
					findings = append(findings, fset.Position(call.Pos()).String()+": "+key+" calls "+callee+" inside its own deadline")
				}
				return true
			})
			return true
		})
	}
	return findings
}

func funcKey(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	typ := fn.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	if id, ok := typ.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}
	return fn.Name.Name
}

func propagate(set map[string]bool, key, callee string) bool {
	if set[callee] && !set[key] {
		set[key] = true
		return true
	}
	return false
}

func isDeadlineCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "context" && (sel.Sel.Name == "WithTimeout" || sel.Sel.Name == "WithDeadline")
}

func isFreshContext(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "context" {
		return sel.Sel.Name == "Background" || sel.Sel.Name == "TODO"
	}
	return sel.Sel.Name == "Context" && len(call.Args) == 0
}

// runsUnderFreshContext matches a statement executed under a context created
// on the spot, which therefore has a budget of its own.
func runsUnderFreshContext(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 || !isFreshContext(call.Args[0]) {
		return false
	}
	switch sel.Sel.Name {
	case "Scan", "Exec", "ExecContext", "QueryContext", "QueryRowContext", "BeginTx", "PingContext":
		return true
	}
	return false
}

func isDeadlineSource(call *ast.CallExpr, funcs map[string]*fixtureFunc, constructor map[string]bool) bool {
	if isDeadlineCall(call) {
		return true
	}
	id, ok := call.Fun.(*ast.Ident)
	return ok && funcs[id.Name] != nil && constructor[id.Name]
}

// returnsDeadline reports whether fn returns the result of a deadline call.
func returnsDeadline(fn *ast.FuncDecl, funcs map[string]*fixtureFunc, constructor map[string]bool) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
			if call, ok := ret.Results[0].(*ast.CallExpr); ok && isDeadlineSource(call, funcs, constructor) {
				found = true
			}
		}
		return !found
	})
	return found
}

// deadlineStart matches an assignment whose first value is a new deadline.
func deadlineStart(n ast.Node, funcs map[string]*fixtureFunc, constructor map[string]bool) (string, bool) {
	assign, ok := n.(*ast.AssignStmt)
	if !ok || len(assign.Rhs) != 1 {
		return "", false
	}
	call, ok := assign.Rhs[0].(*ast.CallExpr)
	if !ok || !isDeadlineSource(call, funcs, constructor) {
		return "", false
	}
	id, ok := assign.Lhs[0].(*ast.Ident)
	return id.Name, ok && id.Name != "_"
}

// failureExit reports whether block ends the function with a return that no
// longer uses the deadline.
func failureExit(block *ast.BlockStmt, ctxName string) bool {
	if len(block.List) == 0 {
		return false
	}
	ret, ok := block.List[len(block.List)-1].(*ast.ReturnStmt)
	if !ok {
		return false
	}
	for _, result := range ret.Results {
		if call, ok := result.(*ast.CallExpr); ok && passesIdent(call, ctxName) {
			return false
		}
		if id, ok := result.(*ast.Ident); ok && id.Name == ctxName {
			return false
		}
	}
	return true
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

// resolvesTenantBefore reports whether the body resolved the test's tenant
// before pos.
func resolvesTenantBefore(body *ast.BlockStmt, pos token.Pos, resolve func(*ast.CallExpr) []string, tenantResolver map[string]bool) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && call.Pos() < pos {
			for _, callee := range resolve(call) {
				found = found || tenantResolver[callee]
			}
		}
		return !found
	})
	return found
}
