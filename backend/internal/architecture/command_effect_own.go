package architecture

import (
	"go/ast"
	"go/types"
	"sort"

	"golang.org/x/tools/go/packages"
)

// A concrete receiver of the source owner is the owner's own implementation,
// not a binding. Its body is not inventoried as foreign effects, but callables
// it invokes can carry root closures supplied through its constructor (#4836
// M3, ADR 0094). Follow only those callables and the owner-internal interfaces
// (see ownInternalSources), reached through the owner's own statically called
// methods and functions.
func (s *effectScanner) resolveOwnMethod(port, binding string, receiver types.Type, name string, seen map[string]bool, trail []string) {
	obj, indexes, _ := types.LookupFieldOrMethod(receiver, true, effectTypePackage(receiver), name)
	if len(indexes) > 1 {
		s.resolve(port, binding, effectEmbeddedReceiver(receiver, indexes), name, seen, trail)
		return
	}
	if fn, ok := obj.(*types.Func); ok {
		s.resolveOwnReach(port, binding, s.ownReachable(fn.Origin()), seen, trail)
	}
}

func (s *effectScanner) resolveOwnReach(port, binding string, reach ownReach, seen map[string]bool, trail []string) {
	for _, callable := range reach.callables {
		s.resolveOrDiagnose(port, binding, callable, "unresolved:"+effectCallableIdentity(callable), trail)
	}
	for _, call := range reach.calls {
		for _, source := range s.ownInternalSources(call.receiver) {
			s.resolve(port, binding, source, call.name, seen, trail)
		}
	}
}

// An own interface is a separate port when any binding comes from outside
// the owner. When every binding is a concrete type of the owner itself (its
// composition's runtime adapter, a unit of work holding composed callables),
// no other port inventories what the call enters: the call resolves into
// those bindings (#6125). An interface without bindings is nil or unset.
func (s *effectScanner) ownInternalSources(receiver types.Type) []types.Type {
	var sources []types.Type
	for destination, candidates := range s.bindings {
		if !types.Identical(destination, receiver) {
			continue
		}
		for _, source := range distinctEffectSources(receiver, candidates) {
			if effectInterface(source) != nil || s.receiverOwner(source) != s.sourceOwner {
				return nil
			}
			sources = append(sources, source)
		}
	}
	sort.Slice(sources, func(i, j int) bool { return effectTypeID(sources[i]) < effectTypeID(sources[j]) })
	return sources
}

// The owner's own bodies are not binding evidence: editing them changes no
// supplied closure. Their summary is therefore independent of the port and
// computed once per function.
type ownEffectSummary struct {
	callables []types.Object
	callees   []*types.Func
	calls     []ownInterfaceCall
}

// ownInterfaceCall is a method call on one of the owner's own interfaces.
type ownInterfaceCall struct {
	receiver types.Type
	name     string
}

func (c ownInterfaceCall) identity() string { return effectTypeID(c.receiver) + "." + c.name }

// ownReach is everything an own function reaches through the owner's
// statically called functions: callables it reads or forwards and calls on
// own interfaces.
type ownReach struct {
	callables []types.Object
	calls     []ownInterfaceCall
}

func (s *effectScanner) ownReachable(start *types.Func) ownReach {
	if cached, ok := s.ownCallables[start]; ok {
		return cached
	}
	result := s.ownReachFrom(s.ownFunctionSummary(start), map[*types.Func]bool{start: true})
	s.ownCallables[start] = result
	return result
}

func (s *effectScanner) ownReachFrom(first ownEffectSummary, visited map[*types.Func]bool) ownReach {
	found := map[types.Object]bool{}
	calls := map[string]ownInterfaceCall{}
	queue := []ownEffectSummary{first}
	for len(queue) > 0 {
		summary := queue[0]
		queue = queue[1:]
		for _, callable := range summary.callables {
			found[callable] = true
		}
		for _, call := range summary.calls {
			calls[call.identity()] = call
		}
		for _, callee := range summary.callees {
			if !visited[callee] {
				visited[callee] = true
				queue = append(queue, s.ownFunctionSummary(callee))
			}
		}
	}
	result := ownReach{callables: sortedEffectObjects(found)}
	for _, call := range calls {
		result.calls = append(result.calls, call)
	}
	sortOwnInterfaceCalls(result.calls)
	return result
}

func sortOwnInterfaceCalls(calls []ownInterfaceCall) {
	sort.Slice(calls, func(i, j int) bool { return calls[i].identity() < calls[j].identity() })
}

func (s *effectScanner) ownFunctionSummary(fn *types.Func) ownEffectSummary {
	if cached, ok := s.ownSummaries[fn]; ok {
		return cached
	}
	var summary ownEffectSummary
	if body, ok := s.bodies[fn]; ok && body.declaration.Body != nil {
		summary = s.ownNodeSummary(body.pkg, body.declaration.Body)
	}
	s.ownSummaries[fn] = summary
	return summary
}

// Nested closures belong to the owner and are summarized inline.
func (s *effectScanner) ownNodeSummary(pkg *packages.Package, body *ast.BlockStmt) ownEffectSummary {
	owner := s.policy[pkg.PkgPath].Owner
	known := knownEffectCallables(pkg, body)
	callables := map[types.Object]bool{}
	callees := map[*types.Func]bool{}
	calls := map[string]ownInterfaceCall{}
	ast.Inspect(body, func(node ast.Node) bool {
		var callable types.Object
		var callee *types.Func
		switch n := node.(type) {
		case *ast.SelectorExpr:
			if call, ok := s.ownInterfaceSelector(pkg, owner, n); ok {
				calls[call.identity()] = call
			}
			callable, callee = s.ownSelectorSummary(pkg, owner, n)
		case *ast.Ident:
			callable, callee = s.ownIdentSummary(pkg, owner, n, known)
		}
		if callable != nil {
			callables[effectOriginObject(callable)] = true
		}
		if callee != nil {
			callees[callee] = true
		}
		return true
	})
	summary := ownEffectSummary{callables: sortedEffectObjects(callables)}
	for _, call := range calls {
		summary.calls = append(summary.calls, call)
	}
	sortOwnInterfaceCalls(summary.calls)
	for callee := range callees {
		summary.callees = append(summary.callees, callee)
	}
	sort.Slice(summary.callees, func(i, j int) bool { return summary.callees[i].FullName() < summary.callees[j].FullName() })
	return summary
}

// ownIdentSummary returns a forwarded callable variable (forwarding a callable
// parameter counts like invoking it) or a statically called own function.
func (s *effectScanner) ownIdentSummary(pkg *packages.Package, owner string, id *ast.Ident, known map[*types.Var]bool) (types.Object, *types.Func) {
	switch object := pkg.TypesInfo.Uses[id].(type) {
	case *types.Func:
		if object.Signature().Recv() == nil && s.functionOwner(object) == owner {
			return nil, object.Origin()
		}
	case *types.Var:
		if !object.IsField() && !known[object] && effectCallableCarrier(object.Type(), map[types.Type]bool{}) {
			return object, nil
		}
	}
	return nil, nil
}

// ownSelectorSummary returns a function-valued field the body reads, or an
// own concrete method/package function it statically calls.
func (s *effectScanner) ownSelectorSummary(pkg *packages.Package, owner string, selector *ast.SelectorExpr) (types.Object, *types.Func) {
	selection := pkg.TypesInfo.Selections[selector]
	if selection == nil {
		if fn, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func); ok && s.functionOwner(fn) == owner {
			return nil, fn.Origin()
		}
		return nil, nil
	}
	if fn, method := selection.Obj().(*types.Func); method {
		receiver := pkg.TypesInfo.TypeOf(selector.X)
		if s.receiverOwner(receiver) == owner && effectInterface(receiver) == nil {
			return nil, fn.Origin()
		}
		return nil, nil
	}
	if selection.Kind() == types.FieldVal && effectCallableCarrier(selection.Obj().Type(), map[types.Type]bool{}) {
		return selection.Obj(), nil
	}
	return nil, nil
}

// ownInterfaceSelector reports a method selected on a named interface of the
// owner, such as s.runtime.WithAdminTx or s.tx.RunPlatform.
func (s *effectScanner) ownInterfaceSelector(pkg *packages.Package, owner string, selector *ast.SelectorExpr) (ownInterfaceCall, bool) {
	selection := pkg.TypesInfo.Selections[selector]
	if selection == nil || selection.Kind() != types.MethodVal {
		return ownInterfaceCall{}, false
	}
	receiver := pkg.TypesInfo.TypeOf(selector.X)
	if namedEffectType(receiver) == nil || effectInterface(receiver) == nil || s.receiverOwner(receiver) != owner {
		return ownInterfaceCall{}, false
	}
	return ownInterfaceCall{receiver: receiver, name: selector.Sel.Name}, true
}

func (s *effectScanner) functionOwner(fn *types.Func) string {
	if fn.Pkg() == nil {
		return ""
	}
	return s.policy[fn.Pkg().Path()].Owner
}

// Diagnostics of the owner's own callables keep the field/callable vocabulary
// used for root adapters.
func effectCallableIdentity(object types.Object) string {
	if variable, ok := object.(*types.Var); ok && variable.IsField() {
		return "function:" + object.Pkg().Path() + "." + effectFieldOwnerName(variable) + object.Name()
	}
	if object.Pkg() == nil {
		return "callable:" + object.Name()
	}
	return "callable:" + object.Pkg().Path() + "." + object.Name()
}

func effectFieldOwnerName(field *types.Var) string {
	scope := field.Pkg().Scope()
	for _, name := range scope.Names() {
		named, ok := scope.Lookup(name).Type().(*types.Named)
		if !ok {
			continue
		}
		if shape, ok := named.Underlying().(*types.Struct); ok {
			for i := 0; i < shape.NumFields(); i++ {
				if shape.Field(i) == field {
					return name + "."
				}
			}
		}
	}
	return ""
}

func sortedEffectObjects(set map[types.Object]bool) []types.Object {
	result := make([]types.Object, 0, len(set))
	for object := range set {
		result = append(result, object)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Pkg() != nil && result[j].Pkg() != nil && result[i].Pkg() != result[j].Pkg() {
			return result[i].Pkg().Path() < result[j].Pkg().Path()
		}
		return result[i].Pos() < result[j].Pos()
	})
	return result
}
