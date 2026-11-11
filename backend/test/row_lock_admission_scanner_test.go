package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// rowLockStatement matches a lock clause in SQL text: a row lock (FOR UPDATE,
// FOR NO KEY UPDATE, FOR SHARE, FOR KEY SHARE, with or without SKIP LOCKED or
// NOWAIT), a table lock or an advisory lock. Upper-case SQL is matched as is;
// lower-case SQL only inside a statement that also selects, so error texts
// such as "find person for update" are not lock sites.
var (
	rowLockClause      = regexp.MustCompile(`\bFOR\s+(NO\s+KEY\s+UPDATE|UPDATE|KEY\s+SHARE|SHARE)\b|\bLOCK\s+TABLE\b|pg_(try_)?advisory(_xact)?_lock`)
	rowLockClauseLower = regexp.MustCompile(`(?i)\bfor\s+(no\s+key\s+update|update|key\s+share|share)\b|(?i)\block\s+table\b|(?i)pg_(try_)?advisory(_xact)?_lock`)
	rowLockSelect      = regexp.MustCompile(`(?i)\bselect\b|(?i)\block\s+table\b`)
)

func isLockSQL(text string) bool {
	if rowLockClause.MatchString(text) {
		return true
	}
	return rowLockClauseLower.MatchString(text) && rowLockSelect.MatchString(text)
}

// rowLockSite is one lock acquisition in source: a BUN For(...) call or a
// lock clause in a string literal or package-level string constant.
type rowLockSite struct {
	file     string
	function string
	line     int
	what     string
	admitted bool
}

func (s rowLockSite) String() string {
	return fmt.Sprintf("%s:%d %s (%s)", s.file, s.line, s.function, s.what)
}

const tenantImportPath = "github.com/moto-nrw/project-phoenix/tenant"

// scanRowLockSites parses one Go source file and reports every lock site
// with whether a checked tenant.OrderLock call admits it first: the call
// sits in the same or an enclosing function, before the site, and its error
// is not discarded. Only a call counts; a comment or an unused reference to
// OrderLock does not.
func scanRowLockSites(file string, source []byte) ([]rowLockSite, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	tenantName := ""
	for _, spec := range parsed.Imports {
		path, _ := strconv.Unquote(spec.Path.Value)
		if path != tenantImportPath {
			continue
		}
		tenantName = "tenant"
		if spec.Name != nil {
			tenantName = spec.Name.Name
		}
	}
	lockConsts := map[string]bool{}
	for _, decl := range parsed.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if i < len(value.Values) && containsLockLiteral(value.Values[i]) {
					lockConsts[name.Name] = true
				}
			}
		}
	}

	var sites []rowLockSite
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			name = receiverName(fn.Recv.List[0].Type) + "." + name
		}
		admissions := checkedAdmissions(fn.Body, tenantName)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			what := ""
			switch n := node.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "For" && len(n.Args) == 1 {
					what = "For(" + exprText(n.Args[0]) + ")"
				}
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "AcquireLock" && tenantName != "" {
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == tenantName {
						what = "tenant.AcquireLock"
					}
				}
			case *ast.BasicLit:
				if n.Kind == token.STRING {
					if text, err := strconv.Unquote(n.Value); err == nil && isLockSQL(text) {
						what = "SQL " + lockClauseOf(text)
					}
				}
			case *ast.Ident:
				if lockConsts[n.Name] {
					what = "SQL constant " + n.Name
				}
			}
			if what == "" {
				return true
			}
			site := rowLockSite{file: file, function: name, line: fset.Position(node.Pos()).Line, what: what}
			for _, admission := range admissions {
				if admission < node.Pos() {
					site.admitted = true
					break
				}
			}
			sites = append(sites, site)
			return true
		})
	}
	return sites, nil
}

// checkedAdmissions lists the positions of tenant.OrderLock calls whose
// error the code uses.
func checkedAdmissions(body *ast.BlockStmt, tenantName string) []token.Pos {
	if tenantName == "" {
		return nil
	}
	var positions []token.Pos
	var visit func(node ast.Node, discarded bool)
	visit = func(node ast.Node, discarded bool) {
		ast.Inspect(node, func(child ast.Node) bool {
			switch n := child.(type) {
			case *ast.ExprStmt:
				if call, ok := n.X.(*ast.CallExpr); ok && isOrderLockCall(call, tenantName) {
					return false
				}
			case *ast.AssignStmt:
				if len(n.Rhs) == 1 {
					if call, ok := n.Rhs[0].(*ast.CallExpr); ok && isOrderLockCall(call, tenantName) {
						if allBlank(n.Lhs) {
							return false
						}
						positions = append(positions, call.Pos())
						return false
					}
				}
			case *ast.CallExpr:
				if isOrderLockCall(n, tenantName) && !discarded {
					positions = append(positions, n.Pos())
				}
			}
			return true
		})
	}
	visit(body, false)
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	return positions
}

func isOrderLockCall(call *ast.CallExpr, tenantName string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "OrderLock" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == tenantName
}

func containsLockLiteral(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if lit, ok := node.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if text, err := strconv.Unquote(lit.Value); err == nil && isLockSQL(text) {
				found = true
			}
		}
		return !found
	})
	return found
}

func exprText(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Value
	case *ast.Ident:
		return e.Name
	}
	return "…"
}

func lockClauseOf(text string) string {
	if match := rowLockClause.FindString(text); match != "" {
		return strings.Join(strings.Fields(match), " ")
	}
	return strings.Join(strings.Fields(rowLockClauseLower.FindString(text)), " ")
}
