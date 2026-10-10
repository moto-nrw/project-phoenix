// Command backend-api-aggregate-callers writes the #2745 caller inventory:
// the retired API aggregate and its constructor in package api must have no
// declaration or reference left, in production code or tests.
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	module  = "github.com/moto-nrw/project-phoenix/"
	retired = module + "api"
	rootDir = "backend/api"
)

type symbol struct {
	Package      string   `json:"package"`
	Name         string   `json:"symbol"`
	Declarations []string `json:"declarations"`
	References   []string `json:"references"`
}

type inventory struct {
	SchemaVersion       int      `json:"schema_version"`
	Generator           string   `json:"generator"`
	ReplacementProvider string   `json:"replacement_provider"`
	RetiredSymbols      []symbol `json:"retired_symbols"`
	ScannedGoFiles      int      `json:"scanned_go_files"`
}

func main() {
	inv := inventory{SchemaVersion: 1,
		ReplacementProvider: "package-private serveGraph and the mount functions in backend/api/base.go",
		Generator:           "scripts/run-go-toolchain.sh go run scripts/backend-api-aggregate-callers/main.go; Go AST inventory over backend/**/*.go including tests, excluding testdata"}
	for _, name := range []string{"API", "New"} {
		inv.RetiredSymbols = append(inv.RetiredSymbols, symbol{Package: retired, Name: name, Declarations: []string{}, References: []string{}})
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

// scan records declarations and references of the retired symbols: inside
// package api as top-level declarations and bare identifiers, elsewhere as
// selectors on an import of package api.
func scan(inv *inventory, fset *token.FileSet, path string, f *ast.File) {
	inPackage := filepath.Dir(path) == rootDir && f.Name.Name == "api"
	aliases := map[string]bool{}
	for _, i := range f.Imports {
		if p, _ := strconv.Unquote(i.Path.Value); p == retired {
			alias := "api"
			if i.Name != nil {
				alias = i.Name.Name
			}
			aliases[alias] = true
		}
	}
	excluded := map[*ast.Ident]bool{}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			excluded[decl.Name] = true
			if inPackage && decl.Recv == nil {
				record(inv, decl.Name.Name, func(s *symbol) { s.Declarations = append(s.Declarations, fset.Position(decl.Pos()).String()) })
			}
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					excluded[typ.Name] = true
					if inPackage {
						record(inv, typ.Name.Name, func(s *symbol) { s.Declarations = append(s.Declarations, fset.Position(typ.Pos()).String()) })
					}
				}
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			excluded[node.Sel] = true
			if id, ok := node.X.(*ast.Ident); ok && aliases[id.Name] {
				record(inv, node.Sel.Name, func(s *symbol) { s.References = append(s.References, fset.Position(node.Pos()).String()) })
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
			if inPackage && !excluded[node] {
				record(inv, node.Name, func(s *symbol) { s.References = append(s.References, fset.Position(node.Pos()).String()) })
			}
		}
		return true
	})
}

func record(inv *inventory, name string, add func(*symbol)) {
	for i := range inv.RetiredSymbols {
		if inv.RetiredSymbols[i].Name == name {
			add(&inv.RetiredSymbols[i])
		}
	}
}
