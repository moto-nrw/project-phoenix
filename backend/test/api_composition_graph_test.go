package test

import (
	"go/ast"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// Function references include aliases and callbacks. Receiver dispatch is
// conservative within the receiver's composition package and its imports.
// Package initialization is an implicit edge even if no symbol calls init.
type compositionNode struct {
	source routeBuilderFile
	name   string
	body   ast.Node
}

type compositionGraph struct {
	nodes          map[string]compositionNode
	imported       map[string]string
	methods        map[string][]string
	initializers   map[string][]string
	packageImports map[string]map[string]bool
	methodPackages map[string]map[string]bool
	roots          []string
}

func compositionPackage(source routeBuilderFile) string {
	return compositionImportPath(source) + "|" + source.file.Name.Name
}

func compositionImportPath(source routeBuilderFile) string {
	return projectImportRoot + "/" + filepath.ToSlash(filepath.Dir(source.relPath))
}

func compositionGraphViolations(files []routeBuilderFile) []string {
	graph := newCompositionGraph(files)
	reverse := map[string][]string{}
	witnesses := map[string]string{}
	for key, node := range graph.nodes {
		edges, broad := graph.references(node)
		for _, target := range edges {
			reverse[target] = append(reverse[target], key)
		}
		if len(broad) > 0 {
			sort.Strings(broad)
			witnesses[key] = node.source.relPath + "#" + node.name + " references broad composition " + broad[0]
		}
	}
	queue := make([]string, 0, len(witnesses))
	for key := range witnesses {
		queue = append(queue, key)
	}
	sort.Strings(queue)
	for i := 0; i < len(queue); i++ {
		target := queue[i]
		sort.Strings(reverse[target])
		for _, caller := range reverse[target] {
			if _, exists := witnesses[caller]; exists {
				continue
			}
			node := graph.nodes[caller]
			witnesses[caller] = node.source.relPath + "#" + node.name + " -> " + witnesses[target]
			queue = append(queue, caller)
		}
	}
	var result []string
	for _, root := range graph.roots {
		if witness, ok := witnesses[root]; ok {
			result = append(result, witness)
		}
	}
	sort.Strings(result)
	return result
}

func newCompositionGraph(files []routeBuilderFile) *compositionGraph {
	graph := &compositionGraph{
		nodes: map[string]compositionNode{}, imported: map[string]string{},
		methods: map[string][]string{}, initializers: map[string][]string{},
		packageImports: map[string]map[string]bool{},
	}
	for _, source := range files {
		isTest := strings.HasSuffix(source.relPath, "_test.go")
		if isTest && !routeBuilderScope(source.relPath) {
			continue
		}
		pkg := compositionPackage(source)
		if graph.packageImports[pkg] == nil {
			graph.packageImports[pkg] = map[string]bool{}
		}
		for _, spec := range source.file.Imports {
			path, err := strconv.Unquote(spec.Path.Value)
			if err == nil {
				graph.packageImports[pkg][path] = true
			}
		}
		for _, declaration := range source.file.Decls {
			switch decl := declaration.(type) {
			case *ast.FuncDecl:
				if decl.Body == nil {
					continue
				}
				method := decl.Recv != nil
				graph.add(source, decl.Name.Name, decl.Body, method, decl.Name.Name == "init")
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok {
						for _, name := range value.Names {
							graph.add(source, name.Name, value, false, true)
						}
					}
				}
			}
		}
	}
	// Even a package with no init function initializes its imported packages.
	seen := map[string]bool{}
	for _, source := range files {
		pkg := compositionPackage(source)
		if seen[pkg] || strings.HasSuffix(source.relPath, "_test.go") {
			continue
		}
		seen[pkg] = true
		graph.add(source, "init", &ast.BlockStmt{}, false, true)
	}
	graph.indexMethodPackages()
	return graph
}

// Constructors can return receivers owned by a dependency rather than their
// own package. Include the dependency closure when resolving method dispatch.
func (graph *compositionGraph) indexMethodPackages() {
	keys := map[string][]string{}
	for pkg := range graph.packageImports {
		path := strings.SplitN(pkg, "|", 2)[0]
		keys[path] = append(keys[path], pkg)
	}
	graph.methodPackages = map[string]map[string]bool{}
	for root := range graph.packageImports {
		paths := map[string]bool{}
		seen := map[string]bool{}
		queue := []string{root}
		for len(queue) > 0 {
			pkg := queue[0]
			queue = queue[1:]
			if seen[pkg] {
				continue
			}
			seen[pkg] = true
			paths[strings.SplitN(pkg, "|", 2)[0]] = true
			for path := range graph.packageImports[pkg] {
				paths[path] = true
				queue = append(queue, keys[path]...)
			}
		}
		graph.methodPackages[root] = paths
	}
}

func (graph *compositionGraph) add(source routeBuilderFile, name string, body ast.Node, method, initializer bool) {
	pkg := compositionPackage(source)
	isTest := strings.HasSuffix(source.relPath, "_test.go")
	key := pkg + "." + name
	if method || name == "init" {
		key += "@" + source.relPath + ":" + strconv.Itoa(int(body.Pos()))
	}
	graph.nodes[key] = compositionNode{source, name, body}
	if method {
		methodKey := compositionImportPath(source) + "." + name
		graph.methods[methodKey] = append(graph.methods[methodKey], key)
	} else if !isTest && name != "init" {
		graph.imported[compositionImportPath(source)+"."+name] = key
	}
	if initializer {
		graph.initializers[pkg] = append(graph.initializers[pkg], key)
		if !isTest {
			graph.initializers[compositionImportPath(source)] = append(graph.initializers[compositionImportPath(source)], key)
		}
	}
	if isTest && !method && (initializer || strings.HasPrefix(name, "Test") || strings.HasPrefix(name, "Benchmark") || strings.HasPrefix(name, "Example")) && !fullRouterGoldenNode(source, name) {
		graph.roots = append(graph.roots, key)
	}
}

func (graph *compositionGraph) references(node compositionNode) ([]string, []string) {
	var edges, broad []string
	pkgKey := compositionPackage(node.source)
	edges = append(edges, graph.initializers[pkgKey]...)
	for imported := range graph.packageImports[pkgKey] {
		edges = append(edges, graph.initializers[imported]...)
	}
	selectorNames := map[*ast.Ident]bool{}
	ast.Inspect(node.body, func(value ast.Node) bool {
		if selector, ok := value.(*ast.SelectorExpr); ok {
			selectorNames[selector.Sel] = true
		}
		return true
	})
	ast.Inspect(node.body, func(value ast.Node) bool {
		var pkg, symbol, target string
		switch ref := value.(type) {
		case *ast.SelectorExpr:
			symbol = ref.Sel.Name
			if id, ok := ref.X.(*ast.Ident); ok {
				pkg = node.source.imports[id.Name]
			}
			if pkg == "" {
				for path := range graph.methodPackages[pkgKey] {
					edges = append(edges, graph.methods[path+"."+symbol]...)
				}
				return true
			}
			target = graph.imported[pkg+"."+symbol]
		case *ast.Ident:
			if selectorNames[ref] {
				return true
			}
			if ref.Obj != nil {
				candidate, exists := graph.nodes[pkgKey+"."+ref.Name]
				if _, function := ref.Obj.Decl.(*ast.FuncDecl); !function && (!exists || candidate.body != ref.Obj.Decl) {
					return true
				}
			}
			symbol = ref.Name
			pkg = compositionImportPath(node.source)
			target = pkgKey + "." + symbol
		default:
			return true
		}
		if broadCompositionSymbol(pkg, symbol) {
			broad = append(broad, pkg+"."+symbol)
		} else if _, exists := graph.nodes[target]; exists {
			edges = append(edges, target)
		}
		return true
	})
	return edges, broad
}

func broadCompositionSymbol(pkg, name string) bool {
	if (pkg == rootServicesImport || pkg == projectImportRoot+"/database/repositories") && strings.HasPrefix(name, "NewFactory") {
		return true
	}
	if pkg == testutilImport && (name == "SetupAPITest" || name == "setupAPITest" || name == "SetupFeedbackAPITest") {
		return true
	}
	return pkg == projectImportRoot+"/api" && (name == "New" || name == "newRuntime" || name == "initializeAPIResources" || name == "buildFullProductionRouterGolden" || name == "TestFullProductionRouterGolden")
}

func fullRouterGoldenNode(source routeBuilderFile, name string) bool {
	return source.relPath == "api/route_table_golden_test.go" && name == "TestFullProductionRouterGolden"
}
