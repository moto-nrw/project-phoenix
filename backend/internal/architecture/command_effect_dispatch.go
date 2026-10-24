package architecture

import (
	"go/ast"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// effectCapture collects the targets and evidence of one callable resolution
// so that it runs once per source owner instead of once per port and path.
type effectCapture struct {
	resolved, partial bool
	targets           map[string]map[string]bool
}

func (c *effectCapture) add(target, owner string, trail []string) {
	key := target + "\x00" + owner
	if c.targets[key] == nil {
		c.targets[key] = map[string]bool{}
	}
	for _, value := range trail {
		c.targets[key][value] = true
	}
}

// resolveOrDiagnose resolves a callable for the port and records its exact
// unresolved diagnostic when any of its values cannot be followed. Both carry
// the callable's assignment evidence.
func (s *effectScanner) resolveOrDiagnose(port, binding string, callable types.Object, diagnostic string, trail []string) {
	evidence := s.callableTrail(callable, trail)
	if !s.resolveCallableOnce(port, binding, callable, evidence) {
		s.record(port, binding, diagnostic, "", evidence)
	}
}

func (s *effectScanner) resolveCallableOnce(port, binding string, object types.Object, trail []string) bool {
	object = effectOriginObject(object)
	key := s.sourceOwner + "\x00" + object.Pkg().Path() + "\x00" + effectCallableIdentity(object) + "\x00" + effectObjectPosition(object)
	capture, cached := s.callableSites[key]
	if !cached {
		if s.callableActive[key] {
			// Recursive callable flow: the enclosing resolution covers it,
			// but a result depending on it must not be cached on its own.
			s.capture.partial = true
			return true
		}
		capture = &effectCapture{targets: map[string]map[string]bool{}}
		outer := s.capture
		s.capture, s.callableActive[key] = capture, true
		capture.resolved = s.resolveCallable("", "", object, map[string]bool{}, map[types.Object]bool{}, nil)
		s.capture = outer
		delete(s.callableActive, key)
		if !capture.partial {
			s.callableSites[key] = capture
		} else if outer != nil {
			outer.partial = true
		}
	}
	for target, evidence := range capture.targets {
		separator := strings.IndexByte(target, 0)
		combined := append([]string{}, trail...)
		for value := range evidence {
			combined = append(combined, value)
		}
		s.record(port, binding, target[:separator], target[separator+1:], combined)
	}
	return capture.resolved
}

// Callable identities name only the package and identifier; the declaring
// position keeps same-named parameters of different functions apart.
func effectObjectPosition(object types.Object) string {
	return strconv.Itoa(int(object.Pos()))
}

// resolveCallable follows the typed assignments of a function-valued field,
// parameter or variable to the closures, functions and method values that
// populate it (#4836). Root closures are walked like root adapters; foreign
// functions and method values stay first-hop terminals. It reports whether
// every observed value was resolved: a callable with any untraceable source,
// or none at all, keeps its exact unresolved diagnostic.
func (s *effectScanner) resolveCallable(port, binding string, object types.Object, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	object = effectOriginObject(object)
	if visited[object] {
		return true // The enclosing traversal already resolves this value set.
	}
	visited[object] = true
	defer delete(visited, object)
	values := s.values[object]
	if len(values) == 0 {
		// An unassigned callable of the source owner is nil or set only by
		// the owner itself; no binding can supply it.
		return object.Pkg() != nil && s.policy[object.Pkg().Path()].Owner == s.sourceOwner
	}
	ownVariable := false
	if variable, ok := object.(*types.Var); ok && !variable.IsField() && variable.Pkg() != nil {
		ownVariable = s.policy[variable.Pkg().Path()].Owner == s.sourceOwner
	}
	resolved := true
	for _, value := range values {
		// The owner's own arguments to its own parameters and locals are
		// summarized where they occur, in the caller reached from the port.
		// Unioning every call site would give each port all other ports' work.
		if ownVariable && s.policy[value.pkg.PkgPath].Owner == s.sourceOwner {
			continue
		}
		if !s.resolveCallableValue(port, binding, value.pkg, value.expression, seen, visited, trail) {
			resolved = false
		}
	}
	return resolved
}

func (s *effectScanner) resolveCallableValue(port, binding string, pkg *packages.Package, expression ast.Expr, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	switch value := expression.(type) {
	case *ast.ParenExpr:
		return s.resolveCallableValue(port, binding, pkg, value.X, seen, visited, trail)
	case *ast.FuncLit:
		return s.resolveCallableLiteral(port, binding, pkg, value, seen, trail)
	case *ast.CompositeLit:
		return s.resolveCallableElements(port, binding, pkg, value, seen, visited, trail)
	case *ast.Ident:
		return s.resolveCallableObjectValue(port, binding, pkg.TypesInfo.ObjectOf(value), seen, visited, trail)
	case *ast.SelectorExpr:
		return s.resolveCallableSelector(port, binding, pkg, value, seen, visited, trail)
	case *ast.IndexExpr:
		return s.resolveCallableValue(port, binding, pkg, value.X, seen, visited, trail)
	case *ast.IndexListExpr:
		return s.resolveCallableValue(port, binding, pkg, value.X, seen, visited, trail)
	case *ast.CallExpr:
		if s.resolveCallableFactory(port, binding, pkg, value, seen, visited, trail) {
			return true
		}
	}
	// An untraceable value the source owner computes itself (a factory call,
	// a conversion) is its own implementation, not a supplied binding.
	return s.policy[pkg.PkgPath].Owner == s.sourceOwner
}

// Root closures are part of the binding and are walked like root adapters.
// The source owner's own closures only forward callables. A closure declared
// by another owner is that owner's implementation, which the scan does not walk.
func (s *effectScanner) resolveCallableLiteral(port, binding string, pkg *packages.Package, literal *ast.FuncLit, seen map[string]bool, trail []string) bool {
	owner := s.policy[pkg.PkgPath].Owner
	if owner != "root-composition" && owner != s.sourceOwner {
		return false
	}
	normalized, err := normalizeEffectNode(literal)
	if err != nil {
		return false
	}
	trail = append(trail, pkg.PkgPath+":"+normalized)
	if owner == s.sourceOwner {
		summary := s.ownNodeSummary(pkg, literal.Body)
		callables := map[types.Object]bool{}
		for _, callable := range summary.callables {
			callables[callable] = true
		}
		for _, callee := range summary.callees {
			for _, callable := range s.ownReachable(callee) {
				callables[callable] = true
			}
		}
		s.resolveOwnCallables(port, binding, sortedEffectObjects(callables), seen, trail)
	} else {
		s.walkBody(port, binding, pkg, literal.Body, seen, trail)
	}
	return true
}

func (s *effectScanner) resolveCallableElements(port, binding string, pkg *packages.Package, literal *ast.CompositeLit, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	resolved := true
	for _, element := range literal.Elts {
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			if effectCallableCarrier(pkg.TypesInfo.TypeOf(keyed.Key), map[types.Type]bool{}) && !s.resolveCallableValue(port, binding, pkg, keyed.Key, seen, visited, trail) {
				resolved = false
			}
			element = keyed.Value
		}
		if !effectCallableCarrier(pkg.TypesInfo.TypeOf(element), map[types.Type]bool{}) {
			continue
		}
		if !s.resolveCallableValue(port, binding, pkg, element, seen, visited, trail) {
			resolved = false
		}
	}
	return resolved
}

func (s *effectScanner) resolveCallableObjectValue(port, binding string, object types.Object, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	switch value := object.(type) {
	case *types.Func:
		if value.Signature().Recv() != nil {
			return false
		}
		s.resolveFunction(port, binding, value.Origin(), seen, trail)
		return true
	case *types.Var:
		return s.resolveCallable(port, binding, value, seen, visited, trail)
	case *types.Nil:
		return true
	}
	return false
}

func (s *effectScanner) resolveCallableSelector(port, binding string, pkg *packages.Package, selector *ast.SelectorExpr, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	selection := pkg.TypesInfo.Selections[selector]
	if selection == nil {
		return s.resolveCallableObjectValue(port, binding, pkg.TypesInfo.Uses[selector.Sel], seen, visited, trail)
	}
	switch selection.Kind() {
	case types.MethodVal, types.MethodExpr:
		s.resolve(port, binding, selection.Recv(), selector.Sel.Name, seen, trail)
		return true
	case types.FieldVal:
		return s.resolveCallable(port, binding, selection.Obj(), seen, visited, trail)
	}
	return false
}

// A callable produced by a factory call is whatever the factory returns. Root
// and source-owner factories are followed to their returned closures; any
// other owner's factory is that owner's implementation and a first-hop
// terminal of its own (or no binding at all for kernels and libraries).
func (s *effectScanner) resolveCallableFactory(port, binding string, pkg *packages.Package, call *ast.CallExpr, seen map[string]bool, visited map[types.Object]bool, trail []string) bool {
	if selector, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
		if selection := pkg.TypesInfo.Selections[selector]; selection != nil && selection.Kind() == types.MethodVal {
			s.resolve(port, binding, selection.Recv(), selector.Sel.Name, seen, trail)
			return true
		}
	}
	fn, ok := effectObject(pkg, call.Fun).(*types.Func)
	if !ok {
		return false
	}
	fn = fn.Origin()
	if owner := s.functionOwner(fn); owner != "root-composition" && owner != s.sourceOwner {
		s.resolveFunction(port, binding, fn, seen, trail)
		return true
	}
	body, ok := s.bodies[fn]
	if !ok || body.declaration.Body == nil {
		return false
	}
	normalized, err := normalizeEffectBody(body)
	if err != nil {
		return false
	}
	trail = append(trail, fn.FullName()+":"+normalized)
	resolved := true
	for _, result := range effectReturnedCallables(body, fn.Signature().Results()) {
		if !s.resolveCallableValue(port, binding, body.pkg, result, seen, visited, trail) {
			resolved = false
		}
	}
	return resolved
}

// effectReturnedCallables lists the factory's own return expressions in
// callable result slots; nested closures return for themselves.
func effectReturnedCallables(body effectBody, results *types.Tuple) []ast.Expr {
	var returned []ast.Expr
	ast.Inspect(body.declaration.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		if ret, ok := node.(*ast.ReturnStmt); ok && len(ret.Results) == results.Len() {
			for i, result := range ret.Results {
				if effectCallableCarrier(results.At(i).Type(), map[types.Type]bool{}) {
					returned = append(returned, result)
				}
			}
		}
		return true
	})
	return returned
}

// Instantiated generic objects are distinct go/types objects. Their typed
// assignments, bodies and parameters are recorded on the generic origin.
func effectOriginObject(object types.Object) types.Object {
	switch value := object.(type) {
	case *types.Var:
		return value.Origin()
	case *types.Func:
		return value.Origin()
	}
	return object
}
