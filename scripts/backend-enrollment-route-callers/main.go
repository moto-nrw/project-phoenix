// Command backend-enrollment-route-callers writes the #2734 caller inventory:
// the retired api/enrollment package and the route helpers that were
// exported only for the parents portal must have no importer, source file,
// declaration or reference left.
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
	retired = module + "api/enrollment"
	routes  = module + "modules/enrollment/http"
)

type symbol struct {
	Package      string   `json:"package"`
	Name         string   `json:"symbol"`
	Declarations []string `json:"declarations"`
	References   []string `json:"references"`
}

type inventory struct {
	SchemaVersion int    `json:"schema_version"`
	Generator     string `json:"generator"`
	Retired       struct {
		Package           string   `json:"package"`
		RemainingImporter []string `json:"remaining_importers"`
		RemainingFiles    []string `json:"remaining_source_files"`
	} `json:"retired_provider"`
	ReplacementProvider string   `json:"replacement_provider"`
	RetiredSymbols      []symbol `json:"retired_symbols"`
	ScannedGoFiles      int      `json:"scanned_go_files"`
}

func main() {
	inv := inventory{SchemaVersion: 1, ReplacementProvider: routes,
		Generator: "scripts/run-go-toolchain.sh go run scripts/backend-enrollment-route-callers/main.go; Go AST inventory over backend/**/*.go including tests, excluding testdata"}
	inv.Retired.Package = retired
	inv.Retired.RemainingImporter, inv.Retired.RemainingFiles = []string{}, []string{}
	for _, name := range []string{"MapSubmitError", "BuildServiceRequest", "RenderPublicEnrollmentBootstrapError", "BuildPublicEnrollmentFormBootstrapResponse"} {
		inv.RetiredSymbols = append(inv.RetiredSymbols, symbol{Package: routes, Name: name, Declarations: []string{}, References: []string{}})
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

func scan(inv *inventory, fset *token.FileSet, path string, f *ast.File) {
	if filepath.Dir(path) == "backend/api/enrollment" {
		inv.Retired.RemainingFiles = append(inv.Retired.RemainingFiles, path)
	}
	inRoutes := filepath.Dir(path) == "backend/modules/enrollment/http"
	aliases := map[string]bool{}
	for _, i := range f.Imports {
		p, _ := strconv.Unquote(i.Path.Value)
		if p == retired {
			inv.Retired.RemainingImporter = append(inv.Retired.RemainingImporter, path)
		}
		if p == routes {
			alias := "enrollmenthttp"
			if i.Name != nil {
				alias = i.Name.Name
			}
			aliases[alias] = true
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			for i := range inv.RetiredSymbols {
				if inRoutes && node.Name.Name == inv.RetiredSymbols[i].Name {
					inv.RetiredSymbols[i].Declarations = append(inv.RetiredSymbols[i].Declarations, path)
				}
			}
		case *ast.SelectorExpr:
			id, ok := node.X.(*ast.Ident)
			if !ok || !aliases[id.Name] {
				return true
			}
			for i := range inv.RetiredSymbols {
				if node.Sel.Name == inv.RetiredSymbols[i].Name {
					inv.RetiredSymbols[i].References = append(inv.RetiredSymbols[i].References, fset.Position(node.Pos()).String())
				}
			}
		case *ast.Ident:
			for i := range inv.RetiredSymbols {
				if inRoutes && node.Name == inv.RetiredSymbols[i].Name {
					inv.RetiredSymbols[i].References = append(inv.RetiredSymbols[i].References, fset.Position(node.Pos()).String())
				}
			}
		}
		return true
	})
}
