package architecture

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/packages"
)

// Type-level flow deliberately unions all assignments to a named interface.
// It never guesses an implementation from structural assignability alone.
type effectBindings map[types.Type][]types.Type

func namedEffectType(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, _ := t.(*types.Named)
	return n
}

func effectInterface(t types.Type) *types.Interface {
	if t == nil {
		return nil
	}
	i, _ := types.Unalias(t).Underlying().(*types.Interface)
	return i
}

func (b effectBindings) add(destination, source types.Type) {
	if destination != nil {
		destination = types.Unalias(destination)
	}
	if source != nil {
		source = types.Unalias(source)
	}
	if source == types.Typ[types.UntypedNil] {
		return
	}
	if effectInterface(destination) == nil || source == nil {
		return
	}
	for _, previous := range b[destination] {
		if types.Identical(previous, source) {
			return
		}
	}
	b[destination] = append(b[destination], source)
}

func collectEffectBindings(loaded []*packages.Package) effectBindings {
	bindings := effectBindings{}
	for _, pkg := range loaded {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool { bindings.collectNode(pkg, node); return true })
		}
	}
	return bindings
}

func (b effectBindings) collectNode(pkg *packages.Package, node ast.Node) {
	switch n := node.(type) {
	case *ast.CompositeLit:
		b.collectLiteral(pkg, n)
	case *ast.AssignStmt:
		if len(n.Lhs) == len(n.Rhs) {
			for i, lhs := range n.Lhs {
				b.add(pkg.TypesInfo.TypeOf(lhs), pkg.TypesInfo.TypeOf(n.Rhs[i]))
			}
		}
	case *ast.ValueSpec:
		b.collectValueSpec(pkg, n)
	case *ast.CallExpr:
		b.collectCall(pkg, n)
	case *ast.FuncDecl:
		if obj, ok := pkg.TypesInfo.Defs[n.Name].(*types.Func); ok {
			collectEffectReturns(b, pkg, n.Body, obj.Type().(*types.Signature))
		}
	case *ast.FuncLit:
		if signature, ok := pkg.TypesInfo.TypeOf(n).(*types.Signature); ok {
			collectEffectReturns(b, pkg, n.Body, signature)
		}
	}
}

func (b effectBindings) collectLiteral(pkg *packages.Package, literal *ast.CompositeLit) {
	shape, ok := pkg.TypesInfo.TypeOf(literal).Underlying().(*types.Struct)
	if !ok {
		return
	}
	for index, element := range literal.Elts {
		value := element
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			value = keyed.Value
			index = effectLiteralFieldIndex(shape, keyed.Key)
		}
		if index >= 0 && index < shape.NumFields() {
			b.add(shape.Field(index).Type(), pkg.TypesInfo.TypeOf(value))
		}
	}
}

func effectLiteralFieldIndex(shape *types.Struct, key ast.Expr) int {
	id, ok := key.(*ast.Ident)
	if !ok {
		return -1
	}
	for index := 0; index < shape.NumFields(); index++ {
		if shape.Field(index).Name() == id.Name {
			return index
		}
	}
	return -1
}

func (b effectBindings) collectCall(pkg *packages.Package, call *ast.CallExpr) {
	if tv, ok := pkg.TypesInfo.Types[call.Fun]; ok && tv.IsType() && len(call.Args) == 1 {
		b.add(tv.Type, pkg.TypesInfo.TypeOf(call.Args[0]))
		return
	}
	signature, ok := pkg.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok {
		return
	}
	for i, arg := range call.Args {
		if i < signature.Params().Len() {
			b.add(signature.Params().At(i).Type(), pkg.TypesInfo.TypeOf(arg))
		}
	}
}

func collectEffectReturns(bindings effectBindings, pkg *packages.Package, body *ast.BlockStmt, signature *types.Signature) {
	if body == nil {
		return
	}
	ast.Inspect(body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		if ret, ok := node.(*ast.ReturnStmt); ok {
			bindings.collectReturn(pkg, ret, signature.Results())
		}
		return true
	})
}

func (b effectBindings) collectReturn(pkg *packages.Package, ret *ast.ReturnStmt, results *types.Tuple) {
	if len(ret.Results) == 1 && b.collectTupleReturn(pkg, ret.Results[0], results) {
		return
	}
	if len(ret.Results) != results.Len() {
		return
	}
	for i, value := range ret.Results {
		b.add(results.At(i).Type(), pkg.TypesInfo.TypeOf(value))
	}
}

func (b effectBindings) collectTupleReturn(pkg *packages.Package, value ast.Expr, results *types.Tuple) bool {
	tuple, multi := pkg.TypesInfo.TypeOf(value).(*types.Tuple)
	if !multi || tuple.Len() != results.Len() {
		return false
	}
	for i := 0; i < tuple.Len(); i++ {
		b.add(results.At(i).Type(), tuple.At(i).Type())
	}
	return true
}

func (b effectBindings) collectValueSpec(pkg *packages.Package, declaration *ast.ValueSpec) {
	if declaration.Type == nil || len(declaration.Names) != len(declaration.Values) {
		return
	}
	for _, value := range declaration.Values {
		b.add(pkg.TypesInfo.TypeOf(declaration.Type), pkg.TypesInfo.TypeOf(value))
	}
}
