// Command backend-scheduler-setter-callers writes the #2749 caller inventory:
// the Worker's post-construction wiring functions, its scalar configuration
// fields written while scheduling, any Set* method on the Scheduler and the
// broad SetupAPITest helper must have no declaration or reference left, in
// production code or tests.
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
	"unicode"
)

const module = "github.com/moto-nrw/project-phoenix/"

type symbol struct {
	Package      string   `json:"package"`
	Name         string   `json:"symbol"`
	Kind         string   `json:"kind"`
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

// retired lists each symbol by package directory under backend/, name and
// kind: func (package-level function), field (Scheduler field), setter (any
// SetX method on Scheduler; without a declaration it has no reference) or
// exported (exported function, referenced in its package or through an
// import).
var retired = []symbol{
	{Package: "services/scheduler", Name: "addCleanupDependencies", Kind: "func"},
	{Package: "services/scheduler", Name: "addScheduleDependencies", Kind: "func"},
	{Package: "services/scheduler", Name: "addRuntimeDependencies", Kind: "func"},
	{Package: "services/scheduler", Name: "sessionCleanupIntervalMinutes", Kind: "field"},
	{Package: "services/scheduler", Name: "sessionAbandonedThresholdMinutes", Kind: "field"},
	{Package: "services/scheduler", Name: "breakAutoEndIntervalSeconds", Kind: "field"},
	{Package: "services/scheduler", Name: "Set", Kind: "setter"},
	{Package: "api", Name: "addWorkerServiceDependencies", Kind: "func"},
	{Package: "api", Name: "addWorkerRepositoryDependencies", Kind: "func"},
	{Package: "api", Name: "workerRuntimeDependencies", Kind: "func"},
	{Package: "api/testutil", Name: "SetupAPITest", Kind: "exported"},
}

func main() {
	inv := inventory{SchemaVersion: 1,
		ReplacementProvider: "scheduler.newScheduler builds the Worker in one literal from WorkerDependencies; api.workerDependencies builds WorkerDependencies in one literal; route-sized builders in api/testutil and the owner test support",
		Generator:           "scripts/run-go-toolchain.sh go run scripts/backend-scheduler-setter-callers/main.go; Go AST inventory over backend/**/*.go including tests, excluding testdata"}
	for _, s := range retired {
		s.Package = module + s.Package
		s.Declarations, s.References = []string{}, []string{}
		inv.RetiredSymbols = append(inv.RetiredSymbols, s)
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
	pkg := module + strings.TrimPrefix(filepath.ToSlash(filepath.Dir(path)), "backend/")
	pos := func(node ast.Node) string { return fset.Position(node.Pos()).String() }
	aliases := importAliases(f)
	for i := range inv.RetiredSymbols {
		s := &inv.RetiredSymbols[i]
		same := s.Package == pkg
		declared := map[*ast.Ident]bool{}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.FuncDecl:
				if same && declares(s, node) {
					declared[node.Name] = true
					s.Declarations = append(s.Declarations, pos(node))
				}
			case *ast.TypeSpec:
				for _, name := range schedulerFields(node) {
					if same && s.Kind == "field" && name.Name == s.Name {
						declared[name] = true
						s.Declarations = append(s.Declarations, pos(name))
					}
				}
			}
			return true
		})
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.SelectorExpr:
				// A package-qualified use from another package.
				if id, ok := node.X.(*ast.Ident); ok && !same && s.Kind == "exported" && aliases[id.Name] == s.Package && node.Sel.Name == s.Name {
					s.References = append(s.References, pos(node))
				}
			case *ast.Ident:
				// Any other use inside the package: calls, function values,
				// selectors and composite-literal keys.
				if same && s.Kind != "setter" && node.Name == s.Name && !declared[node] {
					s.References = append(s.References, pos(node))
				}
			}
			return true
		})
	}
}

func declares(s *symbol, decl *ast.FuncDecl) bool {
	switch s.Kind {
	case "func", "exported":
		return decl.Recv == nil && decl.Name.Name == s.Name
	case "setter":
		return decl.Recv != nil && receiverIsScheduler(decl.Recv) && isSetter(decl.Name.Name)
	}
	return false
}

// isSetter matches SetX names, not Settings or Setup.
func isSetter(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "Set") && unicode.IsUpper(rune(name[3]))
}

func schedulerFields(spec *ast.TypeSpec) []*ast.Ident {
	structure, ok := spec.Type.(*ast.StructType)
	if !ok || spec.Name.Name != "Scheduler" {
		return nil
	}
	var names []*ast.Ident
	for _, field := range structure.Fields.List {
		names = append(names, field.Names...)
	}
	return names
}

func receiverIsScheduler(recv *ast.FieldList) bool {
	for _, field := range recv.List {
		typ := field.Type
		if star, ok := typ.(*ast.StarExpr); ok {
			typ = star.X
		}
		if id, ok := typ.(*ast.Ident); ok && id.Name == "Scheduler" {
			return true
		}
	}
	return false
}

func importAliases(f *ast.File) map[string]string {
	aliases := map[string]string{}
	for _, i := range f.Imports {
		path, _ := strconv.Unquote(i.Path.Value)
		alias := path[strings.LastIndex(path, "/")+1:]
		if i.Name != nil {
			alias = i.Name.Name
		}
		aliases[alias] = path
	}
	return aliases
}
