package architecture

import (
	"bytes"
	"go/ast"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"
)

// Callable values are unioned by their typed destination object, not by field
// name or function signature. Keep diagnostics unresolved, but fingerprint the
// assignments behind them so an old diagnostic cannot approve a new closure.
type effectValue struct {
	pkg        *packages.Package
	expression ast.Expr
	argument   bool
}
type effectValues map[types.Object][]effectValue

func collectEffectValues(loaded []*packages.Package) effectValues {
	values := effectValues{}
	for _, pkg := range loaded {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(node ast.Node) bool { values.collect(pkg, node); return true })
		}
	}
	return values
}

func effectObject(pkg *packages.Package, expression ast.Expr) types.Object {
	switch value := expression.(type) {
	case *ast.Ident:
		return pkg.TypesInfo.ObjectOf(value)
	case *ast.SelectorExpr:
		if selection := pkg.TypesInfo.Selections[value]; selection != nil {
			return selection.Obj()
		}
		return pkg.TypesInfo.ObjectOf(value.Sel)
	case *ast.ParenExpr:
		return effectObject(pkg, value.X)
	case *ast.IndexExpr:
		return effectObject(pkg, value.X)
	case *ast.IndexListExpr:
		return effectObject(pkg, value.X)
	}
	return nil
}

func (values effectValues) add(pkg *packages.Package, object types.Object, expression ast.Expr, argument bool) {
	if object == nil {
		return
	}
	if !effectCallableCarrier(object.Type(), map[types.Type]bool{}) {
		return
	}
	object = effectOriginObject(object)
	values[object] = append(values[object], effectValue{pkg, expression, argument})
}

func (values effectValues) collect(pkg *packages.Package, node ast.Node) {
	switch n := node.(type) {
	case *ast.AssignStmt:
		values.collectAssignments(pkg, n.Lhs, n.Rhs)
	case *ast.ValueSpec:
		left := make([]ast.Expr, len(n.Names))
		for i, name := range n.Names {
			left[i] = name
		}
		values.collectAssignments(pkg, left, n.Values)
	case *ast.CompositeLit:
		values.collectLiteral(pkg, n)
	case *ast.CallExpr:
		values.collectCall(pkg, n)
	}
}

// Follow value aliases and root helpers only. Foreign command bodies remain
// contract terminals. Normalization excludes comments and source positions.
func (s *effectScanner) callableTrail(object types.Object, trail []string) []string {
	evidence, cached := s.callables[object]
	if !cached {
		found := map[string]bool{}
		s.valueEvidence(object, map[types.Object]bool{}, found)
		for item := range found {
			evidence = append(evidence, item)
		}
		s.callables[object] = evidence
	}
	if len(evidence) == 0 {
		return trail
	}
	result := append(append([]string{}, trail...), evidence...)
	sort.Strings(result)
	return result
}

func (s *effectScanner) valueEvidence(object types.Object, seen map[types.Object]bool, evidence map[string]bool) {
	if object == nil {
		return
	}
	object = effectOriginObject(object)
	if seen[object] {
		return
	}
	seen[object] = true
	for _, value := range s.values[object] {
		s.nodeEvidence(value.pkg, value.expression, seen, evidence)
	}
	if fn, ok := object.(*types.Func); ok && fn.Pkg() != nil {
		evidence[fn.FullName()+":"+effectTypeID(fn.Type())] = true
		if s.policy[fn.Pkg().Path()].Owner == "root-composition" {
			if body, ok := s.bodies[fn.Origin()]; ok {
				s.nodeEvidence(body.pkg, body.declaration.Body, seen, evidence)
			}
		}
	}
}

func (s *effectScanner) nodeEvidence(pkg *packages.Package, node ast.Node, seen map[types.Object]bool, evidence map[string]bool) {
	if node == nil {
		return
	}
	var normalized bytes.Buffer
	if err := ast.Fprint(&normalized, nil, node, effectASTField); err != nil {
		return
	}
	evidence[pkg.PkgPath+":"+normalized.String()] = true
	ast.Inspect(node, func(child ast.Node) bool {
		if expression, ok := child.(ast.Expr); ok {
			object := effectObject(pkg, expression)
			if object != nil {
				if effectCallableCarrier(object.Type(), map[types.Type]bool{}) {
					s.valueEvidence(object, seen, evidence)
				}
			}
		}
		return true
	})
}

func (values effectValues) collectLiteral(pkg *packages.Package, n *ast.CompositeLit) {
	shape, ok := pkg.TypesInfo.TypeOf(n).Underlying().(*types.Struct)
	if !ok {
		return
	}
	for i, element := range n.Elts {
		expression := element
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			expression, i = keyed.Value, effectLiteralFieldIndex(shape, keyed.Key)
		}
		if i >= 0 && i < shape.NumFields() {
			values.add(pkg, shape.Field(i), expression, false)
		}
	}
}

func effectCallableCarrier(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch shape := t.Underlying().(type) {
	case *types.Signature:
		return true
	case *types.Slice:
		return effectCallableCarrier(shape.Elem(), seen)
	case *types.Array:
		return effectCallableCarrier(shape.Elem(), seen)
	case *types.Map:
		return effectCallableCarrier(shape.Key(), seen) || effectCallableCarrier(shape.Elem(), seen)
	}
	return false
}

func (values effectValues) collectCall(pkg *packages.Package, n *ast.CallExpr) {
	signature, ok := effectSignature(pkg.TypesInfo.TypeOf(n.Fun))
	if !ok {
		return
	}
	// A sole multi-result expression supplies multiple parameters. Retain that
	// expression for every callable slot so its factory body joins the evidence.
	for i := range effectArgumentTypes(pkg, n.Args) {
		arg := n.Args[0]
		if len(n.Args) > 1 {
			arg = n.Args[i]
		}
		if signature.Variadic() && i >= signature.Params().Len()-1 {
			i = signature.Params().Len() - 1
		}
		if i >= 0 && i < signature.Params().Len() {
			values.add(pkg, signature.Params().At(i), arg, true)
		}
	}
}

func (values effectValues) collectAssignments(pkg *packages.Package, left, right []ast.Expr) {
	if len(right) == 1 && len(left) > 1 {
		for _, destination := range left {
			values.add(pkg, effectObject(pkg, destination), right[0], false)
		}
	} else if len(left) == len(right) {
		for i, destination := range left {
			values.add(pkg, effectObject(pkg, destination), right[i], false)
		}
	}
}
