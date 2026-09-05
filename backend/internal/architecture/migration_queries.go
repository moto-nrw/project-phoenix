package architecture

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
)

type migrationQueries struct {
	info   *types.Info
	writes map[types.Object]int
	values map[types.Object]ast.Expr
}

func analyzeMigrationQueries(file *ast.File, fset *token.FileSet) *migrationQueries {
	q := &migrationQueries{
		info:   &types.Info{Types: make(map[ast.Expr]types.TypeAndValue), Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)},
		writes: make(map[types.Object]int), values: make(map[types.Object]ast.Expr),
	}
	// Resolve lexical identities and local constants without importing or running
	// migration dependencies. Missing external types are expected here: any SQL
	// expression whose value cannot be established below fails closed.
	config := types.Config{Error: func(error) {}}
	_, _ = config.Check("migration", fset, []*ast.File{file}, q.info)
	ast.Inspect(file, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					object := q.info.ObjectOf(id)
					q.writes[object]++
					if len(node.Lhs) == len(node.Rhs) {
						q.values[object] = node.Rhs[i]
					}
				}
			}
		case *ast.RangeStmt:
			for _, lhs := range []ast.Expr{node.Key, node.Value} {
				if id, ok := ast.Unparen(lhs).(*ast.Ident); ok {
					q.writes[q.info.ObjectOf(id)]++
				}
			}
		case *ast.ValueSpec:
			for i, id := range node.Names {
				object := q.info.ObjectOf(id)
				q.writes[object]++
				if len(node.Names) == len(node.Values) {
					q.values[object] = node.Values[i]
				}
			}
		case *ast.UnaryExpr:
			if id, ok := ast.Unparen(node.X).(*ast.Ident); ok && node.Op == token.AND {
				q.writes[q.info.ObjectOf(id)] += 2
			}
		}
		return true
	})
	// Package variables can be mutated in other files; only local variables with
	// one assignment (and no escaping address) are safe to resolve.
	for _, decl := range file.Decls {
		if decl, ok := decl.(*ast.GenDecl); ok && decl.Tok == token.VAR {
			for _, spec := range decl.Specs {
				if spec, ok := spec.(*ast.ValueSpec); ok {
					for _, id := range spec.Names {
						q.writes[q.info.ObjectOf(id)] += 2
					}
				}
			}
		}
	}
	return q
}

func (q *migrationQueries) expression(expr ast.Expr, depth int) (string, bool) {
	if depth > 32 {
		return "", false
	}
	if value := q.info.Types[expr].Value; value != nil && value.Kind() == constant.String {
		return constant.StringVal(value), true
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.BinaryExpr:
		if expr.Op != token.ADD {
			return "", false
		}
		left, leftOK := q.expression(expr.X, depth+1)
		right, rightOK := q.expression(expr.Y, depth+1)
		return left + right, leftOK && rightOK
	case *ast.Ident:
		object := q.info.ObjectOf(expr)
		if object != nil && q.writes[object] == 1 {
			return q.expression(q.values[object], depth+1)
		}
	}
	return "", false
}

func (q *migrationQueries) selector(expr ast.Expr, depth int) *ast.SelectorExpr {
	if depth > 32 {
		return nil
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.SelectorExpr:
		return expr
	case *ast.Ident:
		object := q.info.ObjectOf(expr)
		if object != nil && q.writes[object] == 1 {
			return q.selector(q.values[object], depth+1)
		}
	}
	return nil
}

// BUN builders execute with a context argument, not another SQL string.
func (q *migrationQueries) builder(expr ast.Expr, depth int) bool {
	if depth > 32 {
		return false
	}
	switch expr := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		selector, ok := expr.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		switch selector.Sel.Name {
		case "NewRaw", "NewSelect", "NewInsert", "NewUpdate", "NewDelete", "NewDropTable", "NewCreateIndex", "NewDropIndex", "NewAddColumn", "NewDropColumn", "NewTruncateTable":
			return true
		}
		return q.builder(selector.X, depth+1)
	case *ast.Ident:
		object := q.info.ObjectOf(expr)
		if object != nil && q.writes[object] == 1 {
			return q.builder(q.values[object], depth+1)
		}
	}
	return false
}
