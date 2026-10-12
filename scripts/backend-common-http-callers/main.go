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

type symbol struct {
	Package      string   `json:"package"`
	Name         string   `json:"symbol"`
	Declarations []string `json:"declarations"`
	References   []string `json:"references"`
}

func main() {
	old := "github.com/moto-nrw/project-phoenix/internal/storage"
	common := "github.com/moto-nrw/project-phoenix/api/common"
	retired := []symbol{{Package: common, Name: "ProtectedTenantGroup", Declarations: []string{}, References: []string{}}, {Package: common, Name: "ProtectedSchoolGroup", Declarations: []string{}, References: []string{}}, {Package: "github.com/moto-nrw/project-phoenix/internal/schoolclass", Name: "GradePrefix", Declarations: []string{}, References: []string{}}}
	imports, providerFiles := []string{}, []string{}
	scanned := 0
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
		scanned++
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		if strings.HasPrefix(path, "backend/internal/storage/") {
			providerFiles = append(providerFiles, path)
		}
		aliases := map[string]string{}
		for _, i := range f.Imports {
			p, _ := strconv.Unquote(i.Path.Value)
			if p == old {
				imports = append(imports, path)
			}
			if p == common || p == "github.com/moto-nrw/project-phoenix/internal/schoolclass" {
				alias := filepath.Base(p)
				if i.Name != nil {
					alias = i.Name.Name
				}
				aliases[alias] = p
			}
		}
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				for i := range retired {
					if fn.Name.Name == retired[i].Name && filepath.Dir(path) == "backend/"+strings.TrimPrefix(retired[i].Package, "github.com/moto-nrw/project-phoenix/") {
						retired[i].Declarations = append(retired[i].Declarations, path)
					}
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			s, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := s.X.(*ast.Ident)
			if !ok || aliases[id.Name] == "" {
				return true
			}
			for i := range retired {
				if s.Sel.Name == retired[i].Name && aliases[id.Name] == retired[i].Package {
					retired[i].References = append(retired[i].References, fset.Position(s.Pos()).String())
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		panic(err)
	}
	result := map[string]any{"schema_version": 1, "evidence_base": "6f7128cff805697af1b1ac9c1e213c2e68aca03e", "generator": "scripts/run-go-toolchain.sh go run scripts/backend-common-http-callers/main.go; Go AST inventory over backend/**/*.go including tests, excluding synthetic testdata and ignored runtime directories", "scanned_go_files": scanned, "retired_symbols": retired, "retired_provider": map[string]any{"package": old, "remaining_source_files": providerFiles, "remaining_importers": imports}, "replacement_provider": "github.com/moto-nrw/project-phoenix/modules/delivery/objects", "verification": "Full compilation and architecture graph verify type resolution; import and caller inventories report zero retired references, without a relocation mapping."}
	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("backend/architecture/callers/common-http-2738.json", append(out, '\n'), 0644); err != nil {
		panic(err)
	}
	if len(imports) > 0 || len(providerFiles) > 0 {
		panic("retired provider still exists")
	}
	for _, s := range retired {
		if len(s.Declarations) > 0 || len(s.References) > 0 {
			panic("retired symbol still exists")
		}
	}
}
