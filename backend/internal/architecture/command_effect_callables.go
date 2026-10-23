package architecture

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// A local method/function value already contributes its typed selector/body.
// Only suppress a dynamic diagnostic when every observed assignment is static.
func knownEffectCallables(pkg *packages.Package, body *ast.BlockStmt) map[*types.Var]bool {
	known := map[*types.Var]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != len(assignment.Rhs) {
			return true
		}
		for index, left := range assignment.Lhs {
			recordKnownEffectCallable(pkg, known, left, assignment.Rhs[index])
		}
		return true
	})
	return known
}

func recordKnownEffectCallable(pkg *packages.Package, known map[*types.Var]bool, left, right ast.Expr) {
	id, ok := left.(*ast.Ident)
	if !ok {
		return
	}
	variable, _ := pkg.TypesInfo.ObjectOf(id).(*types.Var)
	if variable == nil {
		return
	}
	static := staticEffectCallable(pkg, known, right)
	if previous, seen := known[variable]; seen {
		static = previous && static
	}
	known[variable] = static
}

func staticEffectCallable(pkg *packages.Package, known map[*types.Var]bool, expression ast.Expr) bool {
	switch value := expression.(type) {
	case *ast.FuncLit:
		return true
	case *ast.SelectorExpr:
		if selection := pkg.TypesInfo.Selections[value]; selection != nil {
			return selection.Kind() != types.FieldVal
		}
		_, function := pkg.TypesInfo.Uses[value.Sel].(*types.Func)
		return function
	case *ast.Ident:
		switch object := pkg.TypesInfo.ObjectOf(value).(type) {
		case *types.Func:
			return true
		case *types.Var:
			return known[object]
		}
	}
	return false
}
