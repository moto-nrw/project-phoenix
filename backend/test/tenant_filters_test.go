package test

import (
	"go/ast"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// A numerical school ID must never decide whether a tenant predicate exists.
// Cross-school queries use an explicit scope or a separately named row source.
func TestTenantFiltersRequireExplicitScope(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "tmp" || entry.Name() == "testdata" || entry.Name() == "migrations" || entry.Name() == "test") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		_, file, err := parseGoSourceCached(path, nil)
		if err != nil {
			return err
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text == "//go:build testhooks" {
					return nil
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			branch, ok := node.(*ast.IfStmt)
			if !ok || !numericTenantCondition(branch) {
				return true
			}
			// A positive-ID guard conditionally adding a school WHERE is forbidden.
			// Required-ID validation that returns an error before an unconditional
			// WHERE is allowed. An early return of an unfiltered query is not.
			if tenantWhere(branch.Body) || returnsUnfilteredQuery(branch.Body) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s: tenant predicate depends on a numeric tenant condition", rel)
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func numericTenantCondition(branch *ast.IfStmt) bool {
	tenant := false
	if branch.Init != nil {
		ast.Inspect(branch.Init, func(n ast.Node) bool {
			if name, ok := n.(*ast.Ident); ok && tenantName(name.Name) {
				tenant = true
			}
			return true
		})
	}
	numeric := false
	ast.Inspect(branch.Cond, func(n ast.Node) bool {
		if name, ok := n.(*ast.Ident); ok && tenantName(name.Name) {
			tenant = true
		}
		if binary, ok := n.(*ast.BinaryExpr); ok {
			switch binary.Op {
			case token.GTR, token.GEQ, token.LSS, token.LEQ, token.EQL, token.NEQ:
				if literal, ok := binary.Y.(*ast.BasicLit); ok && literal.Kind == token.INT && literal.Value == "0" {
					numeric = true
				}
			}
		}
		return true
	})
	return tenant && numeric
}
func tenantName(name string) bool {
	name = strings.ToLower(name)
	return name == "tenantid" || name == "schoolid" || name == "ctxtenant" || name == "currenttenantid" || name == "expectedtenantid" || name == "fromcontext"
}
func tenantWhere(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Where" {
			return true
		}
		for _, arg := range call.Args {
			if text, ok := arg.(*ast.BasicLit); ok && text.Kind == token.STRING && strings.Contains(text.Value, "tenant_id") {
				found = true
			}
		}
		return true
	})
	return found
}
func returnsUnfilteredQuery(body ast.Node) bool {
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		result, ok := n.(*ast.ReturnStmt)
		if !ok || len(result.Results) != 1 {
			return true
		}
		if name, ok := result.Results[0].(*ast.Ident); ok && (name.Name == "q" || name.Name == "query") {
			found = true
		}
		return true
	})
	return found
}
