// Command backend-service-factory-callers writes the #2747 caller inventory:
// the top-level services package is gone, so it has no Go file and no
// importer left, and the retired service Factory, its constructors, its
// tenant runtime binding and the test graph builders named after it have no
// declaration or reference left, in production code or tests.
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	module          = "github.com/moto-nrw/project-phoenix/"
	retiredPackage  = module + "services"
	retiredDir      = "backend/services"
	retiredPkgName  = "services"
	rootPackage     = module + "api"
	rootDir         = "backend/api"
	rootPackageName = "api"
)

type symbol struct {
	Package      string   `json:"package"`
	Name         string   `json:"symbol"`
	Declarations []string `json:"declarations"`
	References   []string `json:"references"`
}

type retiredPackageInventory struct {
	Path      string   `json:"path"`
	GoFiles   []string `json:"go_files"`
	Importers []string `json:"importers"`
}

type inventory struct {
	SchemaVersion       int                     `json:"schema_version"`
	Generator           string                  `json:"generator"`
	ReplacementProvider string                  `json:"replacement_provider"`
	RetiredPackage      retiredPackageInventory `json:"retired_package"`
	RetiredSymbols      []symbol                `json:"retired_symbols"`
	ScannedGoFiles      int                     `json:"scanned_go_files"`
}

// The retired symbols per package. Factory.SetTenantRuntime is recorded under
// its method name; NewFactoryForTests and its variants are the root's former
// test graph builders, which the service graph builders replace.
var retiredSymbols = map[string][]string{
	retiredPackage: {"Factory", "NewFactoryWithModules", "NewFactoryWithConfig", "SetTenantRuntime"},
	rootPackage:    {"Factory", "NewFactoryForTests", "NewFactoryForTestsWithConfig", "TestFactory"},
}

func main() {
	inv := inventory{SchemaVersion: 1,
		Generator:           "scripts/run-go-toolchain.sh go run scripts/backend-service-factory-callers/main.go; Go AST inventory over backend/**/*.go including tests, excluding testdata",
		ReplacementProvider: "the root's service graph (backend/api/service_graph*.go), composed by newServeServiceGraph in the serving root and by NewServiceGraphForTests in the test graphs, over the owner compositions",
		RetiredPackage:      retiredPackageInventory{Path: retiredPackage, GoFiles: []string{}, Importers: []string{}},
	}
	packages := make([]string, 0, len(retiredSymbols))
	for pkg := range retiredSymbols {
		packages = append(packages, pkg)
	}
	sort.Strings(packages)
	for _, pkg := range packages {
		for _, name := range retiredSymbols[pkg] {
			inv.RetiredSymbols = append(inv.RetiredSymbols, symbol{Package: pkg, Name: name, Declarations: []string{}, References: []string{}})
		}
	}
	fset := token.NewFileSet()
	err := filepath.WalkDir("backend", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "testdata" || d.Name() == "tmp" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		inv.ScannedGoFiles++
		if filepath.Dir(path) == retiredDir {
			inv.RetiredPackage.GoFiles = append(inv.RetiredPackage.GoFiles, path)
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		scan(&inv, fset, path, f)
		return nil
	})
	if err != nil {
		panic(err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(inv); err != nil {
		panic(err)
	}
}

// scan records the importers of the retired package and the declarations and
// references of the retired symbols: inside their package as top-level
// declarations, methods and bare identifiers, elsewhere as selectors on an
// import of that package.
func scan(inv *inventory, fset *token.FileSet, path string, f *ast.File) {
	home := ""
	switch {
	case filepath.Dir(path) == retiredDir && f.Name.Name == retiredPkgName:
		home = retiredPackage
	case filepath.Dir(path) == rootDir && f.Name.Name == rootPackageName:
		home = rootPackage
	}
	aliases := map[string]string{}
	for _, i := range f.Imports {
		p, _ := strconv.Unquote(i.Path.Value)
		if p == retiredPackage {
			inv.RetiredPackage.Importers = append(inv.RetiredPackage.Importers, fset.Position(i.Pos()).String())
		}
		if _, retired := retiredSymbols[p]; retired {
			alias := filepath.Base(p)
			if i.Name != nil {
				alias = i.Name.Name
			}
			aliases[alias] = p
		}
	}
	excluded := map[*ast.Ident]bool{}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			excluded[decl.Name] = true
			if home != "" && (decl.Recv == nil || receiverName(decl.Recv) == "Factory") {
				record(inv, home, decl.Name.Name, func(s *symbol) { s.Declarations = append(s.Declarations, fset.Position(decl.Pos()).String()) })
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					excluded[typ.Name] = true
					if home != "" {
						record(inv, home, typ.Name.Name, func(s *symbol) { s.Declarations = append(s.Declarations, fset.Position(typ.Pos()).String()) })
					}
				}
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			excluded[node.Sel] = true
			if id, ok := node.X.(*ast.Ident); ok && aliases[id.Name] != "" {
				record(inv, aliases[id.Name], node.Sel.Name, func(s *symbol) { s.References = append(s.References, fset.Position(node.Pos()).String()) })
			}
		case *ast.Field:
			for _, name := range node.Names {
				excluded[name] = true
			}
		case *ast.KeyValueExpr:
			if key, ok := node.Key.(*ast.Ident); ok {
				excluded[key] = true
			}
		case *ast.Ident:
			if home != "" && !excluded[node] {
				record(inv, home, node.Name, func(s *symbol) { s.References = append(s.References, fset.Position(node.Pos()).String()) })
			}
		}
		return true
	})
}

// receiverName is the type a method is declared on, without its pointer.
func receiverName(receivers *ast.FieldList) string {
	if receivers == nil || len(receivers.List) == 0 {
		return ""
	}
	expression := receivers.List[0].Type
	if star, ok := expression.(*ast.StarExpr); ok {
		expression = star.X
	}
	if id, ok := expression.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func record(inv *inventory, pkg, name string, add func(*symbol)) {
	for i := range inv.RetiredSymbols {
		if inv.RetiredSymbols[i].Package == pkg && inv.RetiredSymbols[i].Name == name {
			add(&inv.RetiredSymbols[i])
		}
	}
}
