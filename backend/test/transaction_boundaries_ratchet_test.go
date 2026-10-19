package test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type transactionSite struct {
	Key, Owner, Kind, Role, Operation string
	Admin                             bool
}

// Administrative work is always enumerated. Outside roots and workflows,
// only exact command/runtime boundaries may start a unit of work. No path
// wildcard or package-wide allowance can conceal a new transaction start.
func TestTransactionBoundaryRatchet(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, transactionBoundaryRegister))
	if err != nil {
		t.Fatal(err)
	}
	var register transactionBoundaryRegisterData
	if err = json.Unmarshal(raw, &register); err != nil {
		t.Fatal(err)
	}
	sites, err := transactionSites(root)
	if err != nil {
		t.Fatal(err)
	}
	admin, commands := map[string]bool{}, map[string]bool{}
	for _, site := range sites {
		if site.Admin {
			admin[site.Key] = true
		}
		if !site.Admin && site.Role != "migration" && (site.Kind != "workflow" || site.Role == "adapter") && site.Owner != "root-composition" {
			commands[site.Key] = true
		}
	}
	for _, entry := range []struct {
		name    string
		actual  map[string]bool
		allowed map[string]string
	}{{"admin", admin, register.Admin}, {"command", commands, register.Commands}} {
		var failures []string
		for key := range entry.actual {
			if strings.TrimSpace(entry.allowed[key]) == "" {
				failures = append(failures, "unlisted "+key)
			}
		}
		for key := range entry.allowed {
			if !entry.actual[key] {
				failures = append(failures, "stale "+key)
			}
		}
		sort.Strings(failures)
		if len(failures) > 0 {
			t.Errorf("%s transaction boundaries:\n%s", entry.name, strings.Join(failures, "\n"))
		}
	}
	if err := transactionRegisterOnlyShrinks(root, register); err != nil {
		t.Error(err)
	}
}

func transactionSites(root string) ([]transactionSite, error) {
	raw, err := os.ReadFile(filepath.Join(root, "architecture/policy.json"))
	if err != nil {
		return nil, err
	}
	var policy struct {
		Packages []struct{ Path, Owner, Role string }
		Owners   []struct{ ID, Kind string }
	}
	if err = json.Unmarshal(raw, &policy); err != nil {
		return nil, err
	}
	sort.Slice(policy.Packages, func(i, j int) bool { return len(policy.Packages[i].Path) > len(policy.Packages[j].Path) })
	kinds := map[string]string{}
	for _, owner := range policy.Owners {
		kinds[owner.ID] = owner.Kind
	}
	var sites []transactionSite
	scalarCache := map[string]map[string]map[string]bool{}
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "tmp" || entry.Name() == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		dir := filepath.ToSlash(filepath.Dir(rel))
		var owner, role string
		for _, pkg := range policy.Packages {
			if dir == pkg.Path || strings.HasPrefix(dir, pkg.Path+"/") {
				owner, role = pkg.Owner, pkg.Role
				break
			}
		}
		if role == "test-support" {
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
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Body == nil {
					continue
				}
				name := decl.Name.Name
				if decl.Recv != nil {
					name = transactionExpression(decl.Recv.List[0].Type) + "." + name
				}
				sites = append(sites, transactionNodes(rel, name, owner, kinds[owner], role, decl.Body, scalarTransactionFields(filepath.Dir(path), decl, scalarCache))...)
			case *ast.GenDecl:
				if decl.Tok != token.VAR {
					continue
				}
				for _, spec := range decl.Specs {
					value := spec.(*ast.ValueSpec)
					for i, expr := range value.Values {
						sites = append(sites, transactionNodes(rel, "var:"+value.Names[min(i, len(value.Names)-1)].Name, owner, kinds[owner], role, expr)...)
					}
				}
			}
		}
		return nil
	})
	return sites, err
}

func transactionOperation(name string) (start, admin bool) {
	switch name {
	case "WithinSystem", "WithinAdmin", "WithinAdminRetry", "WithAdminTx", "WithAdminTxOrDirect", "AdminTx", "RunAdmin", "RunAdminRead", "AdminScope", "withinAdmin", "withinAdminRetry", "withAdminTx", "adminTx", "inAdmin":
		return true, true
	case "WithinTenant", "WithinTenantRetry", "WithinCurrentTenant", "WithTenantTx", "TenantTx", "RunTenant", "RunInTx", "RunInTxWithRetry", "RunInTransaction", "Begin", "BeginTx", "RunWrite", "RunRead", "RunPlatform", "withinTenant", "withinTenantRetry", "runInTx", "InTenantTx", "inTenantTx", "inTenant", "runInTenantTx", "runInSessionTx":
		return true, false
	}
	return false, false
}

func transactionNodes(path, name, owner, kind, role string, body ast.Node, scalarFields ...map[string]bool) []transactionSite {
	if body == nil {
		return nil
	}
	counts := map[string]int{}
	var result []transactionSite
	add := func(expr ast.Expr) {
		var operation, callee string
		switch node := expr.(type) {
		case *ast.SelectorExpr:
			operation = node.Sel.Name
		case *ast.Ident:
			operation = node.Name
		default:
			return
		}
		start, admin := transactionOperation(operation)
		if !start {
			return
		}
		callee = transactionExpression(expr)
		counts[callee]++
		result = append(result, transactionSite{Key: fmt.Sprintf("%s:%s:%s#%d", path, name, callee, counts[callee]), Owner: owner, Kind: kind, Role: role, Operation: operation, Admin: admin})
	}
	addBinding := func(expr ast.Expr) {
		if selector, ok := expr.(*ast.SelectorExpr); ok {
			if len(scalarFields) > 0 && scalarFields[0][transactionExpression(selector)] {
				return
			}
			add(selector)
		}
	}
	ast.Inspect(body, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.CallExpr:
			add(node.Fun)
			for _, arg := range node.Args {
				addBinding(arg)
			}
		case *ast.AssignStmt:
			for _, expr := range node.Rhs {
				addBinding(expr)
			}
		case *ast.ValueSpec:
			for _, expr := range node.Values {
				addBinding(expr)
			}
		case *ast.KeyValueExpr:
			addBinding(node.Value)
		case *ast.ReturnStmt:
			for _, expr := range node.Results {
				addBinding(expr)
			}
		}
		return true
	})
	return result
}

func TestTransactionScannerIncludesAliasesAndNestedStarts(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", `package fixture
func command(){ open := runtime.WithinAdmin; open(ctx,func(ctx Context)error{return tenant.WithinTenant(ctx,id,fn)}); db.BeginTx(ctx,nil); forward(runtime.WithinAdmin); use(caller.AdminScope) }`, 0)
	if err != nil {
		t.Fatal(err)
	}
	sites := transactionNodes("fixture.go", "command", "owner", "domain", "application", file.Decls[0].(*ast.FuncDecl).Body, map[string]bool{"caller.AdminScope": true})
	if len(sites) != 4 {
		t.Fatalf("want admin alias and callback binding, nested tenant and direct SQL transaction, but no boolean flag: %+v", sites)
	}
	if !sites[0].Admin || sites[1].Admin || sites[2].Operation != "BeginTx" {
		t.Fatalf("incorrect transaction classifications: %+v", sites)
	}
}

func transactionExpression(node ast.Expr) string {
	switch node := node.(type) {
	case *ast.Ident:
		return node.Name
	case *ast.SelectorExpr:
		return transactionExpression(node.X) + "." + node.Sel.Name
	case *ast.StarExpr:
		return "*" + transactionExpression(node.X)
	case *ast.CallExpr:
		return transactionExpression(node.Fun) + "()"
	case *ast.IndexExpr:
		return transactionExpression(node.X) + "[]"
	case *ast.IndexListExpr:
		return transactionExpression(node.X) + "[]"
	case *ast.CompositeLit:
		return transactionExpression(node.Type) + "{}"
	case *ast.UnaryExpr:
		return node.Op.String() + transactionExpression(node.X)
	case *ast.ParenExpr:
		return "(" + transactionExpression(node.X) + ")"
	default:
		panic(fmt.Sprintf("unsupported transaction target %T", node))
	}
}

// Permission flags can share API names (Caller.AdminScope is a bool). Resolve
// local parameter/struct fields so passing such a value is not an admin start.
func scalarTransactionFields(dir string, decl *ast.FuncDecl, cache map[string]map[string]map[string]bool) map[string]bool {
	structs, loaded := cache[dir]
	if !loaded {
		structs = map[string]map[string]bool{}
		entries, err := os.ReadDir(dir)
		if err != nil {
			panic(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			_, file, err := parseGoSourceCached(filepath.Join(dir, entry.Name()), nil)
			if err != nil {
				panic(err)
			}
			for _, declaration := range file.Decls {
				group, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range group.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					structure, ok := typ.Type.(*ast.StructType)
					if !ok {
						continue
					}
					fields := map[string]bool{}
					for _, field := range structure.Fields.List {
						basic, ok := field.Type.(*ast.Ident)
						if !ok || basic.Name != "bool" {
							continue
						}
						for _, name := range field.Names {
							fields[name.Name] = true
						}
					}
					structs[typ.Name.Name] = fields
				}
			}
		}
		cache[dir] = structs
	}
	result := map[string]bool{}
	if decl.Type.Params == nil {
		return result
	}
	for _, param := range decl.Type.Params.List {
		typ, ok := param.Type.(*ast.Ident)
		if !ok {
			continue
		}
		for _, name := range param.Names {
			for field := range structs[typ.Name] {
				result[name.Name+"."+field] = true
			}
		}
	}
	return result
}

// Migrations can own their schema transaction, but that role must not hide
// administrative API calls from the exact administrative register.
func TestTransactionScannerIncludesAdministrativeMigrationCalls(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "architecture"), 0700); err != nil {
		t.Fatal(err)
	}
	policy := `{"owners":[{"ID":"schema","Kind":"infrastructure"}],"packages":[{"Path":"database/migrations","Owner":"schema","Role":"migration"}]}`
	if err := os.WriteFile(filepath.Join(root, "architecture/policy.json"), []byte(policy), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "database/migrations")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	source := `package migrations
 func upgrade(){ tenant.WithinAdmin(ctx, callback); db.BeginTx(ctx,nil) }
 `
	if err := os.WriteFile(filepath.Join(dir, "upgrade.go"), []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	sites, err := transactionSites(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || !sites[0].Admin || sites[0].Role != "migration" {
		t.Fatalf("migration administrative API escaped the register: %+v", sites)
	}
}
