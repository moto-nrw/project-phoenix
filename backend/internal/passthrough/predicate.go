package passthrough

import (
	"go/ast"
	"go/token"
)

// MaxBodyLines preserves the legacy predicate threshold.
const MaxBodyLines = 14

// FlowHelperLines is what condition 3 counts back for a method that hands its
// work to a flow helper (ADR 0177): the helper takes over the nil guard (three
// lines), the runtime binding and the deferred error translation (one line
// each), and its function literal adds two framing lines. A collapse into the
// helper therefore neither hides nor reveals a delegation.
const FlowHelperLines = 3

// flowHelpers are the free functions ADR 0177 names. They are Identity &
// Access's unit-of-work helpers; no other free function of that name exists
// in the scanned trees.
var flowHelpers = map[string]bool{"runFlow": true, "runFlowErr": true, "runFlow2": true}

// IsDelegation applies the four conditions documented by the legacy gate.
func IsDelegation(fn *ast.FuncDecl, fset *token.FileSet) bool {
	// (1) a method, not a free function.
	if fn.Recv == nil || len(fn.Recv.List) == 0 || fn.Body == nil {
		return false
	}
	// (2) exported.
	if fn.Name == nil || !fn.Name.IsExported() {
		return false
	}
	// The receiver must be named; an unnamed or blank receiver cannot be the
	// root of the forwarded call.
	receiver := modulePassthroughReceiverName(fn)
	if receiver == "" {
		return false
	}
	// (3) body line span.
	span := fset.Position(fn.Body.Rbrace).Line - fset.Position(fn.Body.Lbrace).Line + 1
	if callsFlowHelper(fn.Body) {
		span += FlowHelperLines
	}
	if span > MaxBodyLines {
		return false
	}
	// (4) exactly one call through a field of the receiver. Sister-method
	// calls (s.helper(...)) and free functions (fmt.Errorf, domain.X, len)
	// are ignored: neither leaves the module. A second field call means the
	// method combines two collaborators, which is orchestration.
	return modulePassthroughFieldCalls(fn.Body, receiver) == 1
}

// callsFlowHelper reports whether body calls one of the flow helpers as a
// free function; a method or a selector of that name is no flow helper.
func callsFlowHelper(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if name, ok := call.Fun.(*ast.Ident); ok && flowHelpers[name.Name] {
				found = true
			}
		}
		return !found
	})
	return found
}

// modulePassthroughFieldCalls counts the calls in body whose Fun is a selector
// chain of at least two steps bottoming out in the receiver identifier —
// s.store.Find(...), s.ports.Y(...), s.deps.Clock.Now(...). A one-step chain
// (s.Helper(...)) is a sister method and is not counted; so is every call that
// does not start at the receiver at all. Function literals are inspected as
// part of the body, so a store call wrapped in s.run("…", func(){…}) counts.
func modulePassthroughFieldCalls(body *ast.BlockStmt, receiver string) int {
	count := 0
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		// The method name is one step; at least one field must sit between it
		// and the receiver.
		field, ok := selector.X.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if root := modulePassthroughSelectorRoot(field); root != nil && root.Name == receiver {
			count++
		}
		return true
	})
	return count
}

// modulePassthroughReceiverName returns the receiver's identifier, or "" when
// the method has none (`func (*Service) X()`) or discards it (`func (_ *S) X()`).
func modulePassthroughReceiverName(fn *ast.FuncDecl) string {
	names := fn.Recv.List[0].Names
	if len(names) == 0 || names[0] == nil || names[0].Name == "_" {
		return ""
	}
	return names[0].Name
}

// modulePassthroughSelectorRoot peels a selector chain down to its innermost
// expression: r.store.Find → r, s.Helper → s. It returns nil when the chain
// does not bottom out in a plain identifier (a call, an index or a literal).
func modulePassthroughSelectorRoot(expr ast.Expr) *ast.Ident {
	for {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		expr = selector.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}
	return ident
}
