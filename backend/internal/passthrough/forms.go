package passthrough

import (
	"go/ast"
	"go/token"
	"strings"
)

// Form is deliberately conservative: only unchanged forwards, leading nil
// guards and ADR 0075's four exact exceptions have automatic classifications.
// Everything else needs semantic review, not a default "translation" label.
func siteForm(dir string, method *ast.FuncDecl) string {
	if namedException(dir, method) {
		return "adr0075-named-exception"
	}
	statements := method.Body.List
	guarded := false
	for len(statements) > 1 && nilGuard(statements[0]) {
		statements = statements[1:]
		guarded = true
	}
	if unchangedForward(method, statements) {
		roots := dir == "." || strings.HasPrefix(dir, "roots/") || dir == "api" || strings.HasPrefix(dir, "api/") || dir == "cmd" || strings.HasPrefix(dir, "cmd/")
		if roots {
			if guarded {
				return "guard-debt"
			}
			return "forward-debt"
		}
		if guarded {
			return "guarded-forward-evidence"
		}
		return "unchanged-forward-evidence"
	}
	return "unclassified-evidence"
}

func nilGuard(statement ast.Stmt) bool {
	guard, ok := statement.(*ast.IfStmt)
	if !ok || guard.Init != nil || guard.Else != nil || len(guard.Body.List) != 1 {
		return false
	}
	condition, ok := guard.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.EQL {
		return false
	}
	nilValue := func(expr ast.Expr) bool { value, ok := expr.(*ast.Ident); return ok && value.Name == "nil" }
	if !nilValue(condition.X) && !nilValue(condition.Y) {
		return false
	}
	_, returns := guard.Body.List[0].(*ast.ReturnStmt)
	return returns
}

func unchangedForward(method *ast.FuncDecl, statements []ast.Stmt) bool {
	call := forwardedCall(statements)
	if call == nil {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	field, ok := selector.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	root := modulePassthroughSelectorRoot(field)
	return root != nil && root.Name == modulePassthroughReceiverName(method) && unchangedArguments(method, call)
}

func forwardedCall(statements []ast.Stmt) *ast.CallExpr {
	if len(statements) == 2 {
		assignment, ok := statements[0].(*ast.AssignStmt)
		if !ok || len(assignment.Rhs) != 1 {
			return nil
		}
		result, ok := statements[1].(*ast.ReturnStmt)
		if !ok || !unchangedResults(assignment.Lhs, result.Results) {
			return nil
		}
		call, _ := assignment.Rhs[0].(*ast.CallExpr)
		return call
	}
	if len(statements) != 1 {
		return nil
	}
	switch statement := statements[0].(type) {
	case *ast.ReturnStmt:
		if len(statement.Results) == 1 {
			call, _ := statement.Results[0].(*ast.CallExpr)
			return call
		}
	case *ast.ExprStmt:
		call, _ := statement.X.(*ast.CallExpr)
		return call
	}
	return nil
}

func unchangedResults(left, right []ast.Expr) bool {
	if len(left) != len(right) {
		return false
	}
	for i, expr := range right {
		lhs, ok := left[i].(*ast.Ident)
		rhs, matches := expr.(*ast.Ident)
		if !ok || !matches || lhs.Name != rhs.Name {
			return false
		}
	}
	return true
}

func unchangedArguments(method *ast.FuncDecl, call *ast.CallExpr) bool {
	var params []string
	for _, field := range method.Type.Params.List {
		for _, name := range field.Names {
			params = append(params, name.Name)
		}
	}
	if len(params) != len(call.Args) {
		return false
	}
	for i, arg := range call.Args {
		name, ok := arg.(*ast.Ident)
		if !ok || name.Name != params[i] {
			return false
		}
	}
	return true
}

func namedException(dir string, method *ast.FuncDecl) bool {
	typ := method.Recv.List[0].Type
	if pointer, ok := typ.(*ast.StarExpr); ok {
		typ = pointer.X
	}
	name, ok := typ.(*ast.Ident)
	if !ok {
		return false
	}
	switch dir + "/" + name.Name + "." + method.Name.Name {
	case "modules/careplan/internal/application/DocumentFileRecords.MarkFileDeleted", "modules/careplan/internal/application/DocumentFileRecords.MarkQueuedCleanupCompleteByFilename", "modules/careplan/internal/application/DocumentFileRecords.ActivateQueuedCleanup", "modules/settings/internal/application/TenantOperations.Schema":
		return true
	}
	return false
}
