package architecture

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// preparedStatementSQL follows only a single local binding to sql.Prepare.
// Reassignment, address escape, parameters and dynamic SQL remain unresolved.
// A sql.Stmt's query is immutable, so value aliases cannot replace its SQL.
func preparedStatementSQL(pkg *packages.Package, execution *ast.CallExpr) (string, bool) {
	selector, ok := execution.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	id, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	object := pkg.TypesInfo.ObjectOf(id)
	if object == nil || object.Parent() == pkg.Types.Scope() {
		return "", false
	}
	preparation := singleStatementPreparation(pkg, object)
	if preparation == nil {
		return "", false
	}
	return preparedSQLLiteral(pkg, preparation)
}

func singleStatementPreparation(pkg *packages.Package, object types.Object) *ast.CallExpr {
	var preparation *ast.CallExpr
	writes, escaped := 0, false
	for _, file := range pkg.Syntax {
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.UnaryExpr:
				id, ok := node.X.(*ast.Ident)
				if ok && node.Op == token.AND && pkg.TypesInfo.ObjectOf(id) == object {
					escaped = true
				}
			case *ast.RangeStmt:
				writes += statementRangeWrites(pkg, object, node)
			case *ast.AssignStmt:
				count, declaration := statementAssignment(pkg, object, node)
				writes += count
				if declaration != nil {
					preparation = declaration
				}
			}
			return true
		})
	}
	if writes != 1 || escaped {
		return nil
	}
	return preparation
}

func statementAssignment(pkg *packages.Package, object types.Object, assignment *ast.AssignStmt) (int, *ast.CallExpr) {
	writes := 0
	var preparation *ast.CallExpr
	for index, lhs := range assignment.Lhs {
		id, ok := lhs.(*ast.Ident)
		if !ok || pkg.TypesInfo.ObjectOf(id) != object {
			continue
		}
		writes++
		if index == 0 && len(assignment.Rhs) == 1 && assignment.Tok == token.DEFINE && pkg.TypesInfo.Defs[id] == object {
			preparation, _ = assignment.Rhs[0].(*ast.CallExpr)
		}
	}
	return writes, preparation
}

func preparedSQLLiteral(pkg *packages.Package, preparation *ast.CallExpr) (string, bool) {
	prepare, ok := preparation.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	// Only the standard library's immutable statement contract qualifies.
	switch types.TypeString(pkg.TypesInfo.TypeOf(prepare.X), packagePathQualifier) {
	case "*database/sql.DB", "*database/sql.Tx", "*database/sql.Conn":
	default:
		return "", false
	}
	index := 0
	switch prepare.Sel.Name {
	case "Prepare":
	case "PrepareContext":
		index = 1
	default:
		return "", false
	}
	if len(preparation.Args) <= index {
		return "", false
	}
	return constantString(pkg.TypesInfo, preparation.Args[index])
}

func statementRangeWrites(pkg *packages.Package, object types.Object, loop *ast.RangeStmt) int {
	writes := 0
	for _, target := range []ast.Expr{loop.Key, loop.Value} {
		if id, ok := target.(*ast.Ident); ok && pkg.TypesInfo.ObjectOf(id) == object {
			writes++
		}
	}
	return writes
}
