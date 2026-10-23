package architecture

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Preserve type-level may-bind semantics through containers. Only actual
// assignments contribute sources, never mere structural assignability.
func (b effectBindings) addContainer(destination, source types.Type) bool {
	// Identical containers add no implementation binding. This also terminates
	// legal recursive named slices/maps (type Tree map[string]Tree).
	if types.Identical(destination, source) {
		return false
	}
	switch target := destination.Underlying().(type) {
	case *types.Slice:
		if origin, ok := source.Underlying().(*types.Slice); ok {
			b.add(target.Elem(), origin.Elem())
		}
	case *types.Array:
		if origin, ok := source.Underlying().(*types.Array); ok {
			b.add(target.Elem(), origin.Elem())
		}
	case *types.Map:
		if origin, ok := source.Underlying().(*types.Map); ok {
			b.add(target.Key(), origin.Key())
			b.add(target.Elem(), origin.Elem())
		}
	default:
		return false
	}
	return true
}

func (b effectBindings) collectContainerLiteral(pkg *packages.Package, literal *ast.CompositeLit) {
	switch shape := pkg.TypesInfo.TypeOf(literal).Underlying().(type) {
	case *types.Slice:
		b.collectElements(pkg, shape.Elem(), literal.Elts)
	case *types.Array:
		b.collectElements(pkg, shape.Elem(), literal.Elts)
	case *types.Map:
		for _, element := range literal.Elts {
			if keyed, ok := element.(*ast.KeyValueExpr); ok {
				b.add(shape.Key(), pkg.TypesInfo.TypeOf(keyed.Key))
				b.add(shape.Elem(), pkg.TypesInfo.TypeOf(keyed.Value))
			}
		}
	}
}

func (b effectBindings) collectElements(pkg *packages.Package, destination types.Type, elements []ast.Expr) {
	for _, element := range elements {
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			element = keyed.Value
		}
		b.add(destination, pkg.TypesInfo.TypeOf(element))
	}
}

func effectArgumentTypes(pkg *packages.Package, expressions []ast.Expr) []types.Type {
	if len(expressions) == 1 {
		if tuple, ok := pkg.TypesInfo.TypeOf(expressions[0]).(*types.Tuple); ok {
			result := make([]types.Type, tuple.Len())
			for i := range result {
				result[i] = tuple.At(i).Type()
			}
			return result
		}
	}
	result := make([]types.Type, len(expressions))
	for i, expression := range expressions {
		result[i] = pkg.TypesInfo.TypeOf(expression)
	}
	return result
}

func (b effectBindings) collectAssignments(pkg *packages.Package, left, right []ast.Expr) {
	sources := effectArgumentTypes(pkg, right)
	if len(left) != len(sources) {
		return
	}
	for i, destination := range left {
		b.add(pkg.TypesInfo.TypeOf(destination), sources[i])
	}
}

func (b effectBindings) collectAppend(pkg *packages.Package, call *ast.CallExpr) bool {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	builtin, ok := pkg.TypesInfo.Uses[id].(*types.Builtin)
	if !ok || builtin.Name() != "append" || len(call.Args) == 0 {
		return false
	}
	slice, ok := pkg.TypesInfo.TypeOf(call).Underlying().(*types.Slice)
	if !ok {
		return false
	}
	for _, arg := range call.Args[1:] {
		destination := slice.Elem()
		if call.Ellipsis.IsValid() {
			destination = pkg.TypesInfo.TypeOf(call)
		}
		b.add(destination, pkg.TypesInfo.TypeOf(arg))
	}
	return true
}
