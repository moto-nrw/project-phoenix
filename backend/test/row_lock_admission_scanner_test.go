package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
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

	// A plain function of the same file that admits through a checked
	// tenant.OrderLock admits for its callers too, when they check its error.
	admitters := map[string]bool{}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil && len(checkedAdmissions(fn.Body, tenantName, nil)) > 0 {
			admitters[fn.Name.Name] = true
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
		admissions := checkedAdmissions(fn.Body, tenantName, admitters)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			what := ""
			switch n := node.(type) {
			case *ast.CallExpr:
				if sel, ok := n.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "For" && len(n.Args) == 1 {
					what = "For(" + exprText(n.Args[0]) + ")"
				}

			case *ast.SelectorExpr:
				// The runtime's advisory lock, called or handed on as a value.
				if pkg, ok := n.X.(*ast.Ident); ok && tenantName != "" && pkg.Name == tenantName && n.Sel.Name == "AcquireLock" {
					what = "tenant.AcquireLock"
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

// checkedAdmissions lists the positions of the admitting calls whose error
// the code uses: tenant.OrderLock, or a same-file admitter. A bare call
// statement or an assignment to blanks only discards the refusal.
func checkedAdmissions(body *ast.BlockStmt, tenantName string, admitters map[string]bool) []token.Pos {
	if tenantName == "" {
		return nil
	}
	admits := func(call *ast.CallExpr) bool {
		if ident, ok := call.Fun.(*ast.Ident); ok && admitters[ident.Name] {
			return true
		}
		return isTenantOrderLockCall(call, tenantName)
	}
	var positions []token.Pos
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.ExprStmt:
			if call, ok := n.X.(*ast.CallExpr); ok && admits(call) {
				return false
			}
		case *ast.AssignStmt:
			if len(n.Rhs) == 1 {
				if call, ok := n.Rhs[0].(*ast.CallExpr); ok && admits(call) {
					if !allBlank(n.Lhs) {
						positions = append(positions, call.Pos())
					}
					return false
				}
			}
		case *ast.CallExpr:
			if admits(n) {
				positions = append(positions, n.Pos())
			}
		}
		return true
	})
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	return positions
}

func isTenantOrderLockCall(call *ast.CallExpr, tenantName string) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (sel.Sel.Name != "OrderLock" && sel.Sel.Name != "AdmitBefore") {
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

// admitsBeforeCall reports whether the named function of the file (a method
// of any receiver or a plain function) calls a function or method named
// called, and its last such call, the one that takes the lock, comes after a
// checked admission.
func admitsBeforeCall(file string, source []byte, function, called string) (bool, error) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, source, parser.SkipObjectResolution)
	if err != nil {
		return false, err
	}
	tenantName := ""
	for _, spec := range parsed.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); path == tenantImportPath {
			tenantName = "tenant"
			if spec.Name != nil {
				tenantName = spec.Name.Name
			}
		}
	}
	admitters := map[string]bool{}
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil && len(checkedAdmissions(fn.Body, tenantName, nil)) > 0 {
			admitters[fn.Name.Name] = true
		}
	}
	found := false
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil || fn.Name.Name != function {
			continue
		}
		admissions := checkedAdmissions(fn.Body, tenantName, admitters)
		last := token.NoPos
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && calledName(call) == called && call.Pos() > last {
				last = call.Pos()
			}
			return true
		})
		if last == token.NoPos {
			continue
		}
		found = true
		if len(admissions) == 0 || admissions[0] > last {
			return false, nil
		}
	}
	return found, nil
}

func calledName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

// everyLockingCallAdmitted checks every function of the file, or of the
// directory's non-test files, that calls called with a lock: each such call needs a checked
// admission before it in the same function. A call that passes "" or false
// takes no lock. found reports whether any locking call exists.
func everyLockingCallAdmitted(dir, called string) (unadmitted []string, found bool, err error) {
	paths := []string{dir}
	if !strings.HasSuffix(dir, ".go") {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, false, err
		}
		paths = paths[:0]
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") && !strings.HasSuffix(entry.Name(), "_test.go") {
				paths = append(paths, filepath.Join(dir, entry.Name()))
			}
		}
	}
	fset := token.NewFileSet()
	var files []*ast.File
	for _, path := range paths {
		parsed, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, false, err
		}
		files = append(files, parsed)
	}
	admitters := map[string]bool{}
	for _, file := range files {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil && len(checkedAdmissions(fn.Body, tenantNameOf(file), nil)) > 0 {
				admitters[fn.Name.Name] = true
			}
		}
	}
	for _, file := range files {
		tenantName := tenantNameOf(file)
		if tenantName == "" {
			tenantName = "tenant"
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			admissions := checkedAdmissions(fn.Body, tenantName, admitters)
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || calledName(call) != called || !takesLock(call) {
					return true
				}
				found = true
				if len(admissions) == 0 || admissions[0] > call.Pos() {
					unadmitted = append(unadmitted, fset.Position(call.Pos()).String())
				}
				return true
			})
		}
	}
	return unadmitted, found, nil
}

func tenantNameOf(file *ast.File) string {
	for _, spec := range file.Imports {
		if path, _ := strconv.Unquote(spec.Path.Value); path == tenantImportPath {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return "tenant"
		}
	}
	return ""
}

// takesLock is false for a call that passes an empty lock strength or a
// false lock flag.
func takesLock(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if lit, ok := arg.(*ast.BasicLit); ok && lit.Value == `""` {
			return false
		}
		if ident, ok := arg.(*ast.Ident); ok && ident.Name == "false" {
			return false
		}
	}
	return true
}
