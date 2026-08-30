package test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	rootServicesImport = "github.com/moto-nrw/project-phoenix/services"
	testutilImport     = "github.com/moto-nrw/project-phoenix/api/testutil"
	chiImport          = "github.com/go-chi/chi/v5"
	httpImport         = "net/http"
)

type routeBuilderKind uint8

const (
	notRouteBuilder routeBuilderKind = iota
	routeBuilder
	moduleBuilder
)

type routeBuilderFile struct {
	relPath string
	file    *ast.File
	imports map[string]string
	types   map[string]ast.Expr
}

type routeBuilderResult struct {
	hasFactory bool
	hasRoute   bool
	hasUntyped bool
}

type parsedRouteBuilderFile struct {
	relPath string
	file    *ast.File
	imports map[string]string
	pkgKey  string
}

type routeBuilderParseState struct {
	backendRoot    string
	parsed         []parsedRouteBuilderFile
	typesByPackage map[string]map[string]ast.Expr
}

func TestAPITestsUseNarrowRouteBuilders(t *testing.T) {
	t.Parallel()

	violations, err := routeSizedBuilderViolations(filepath.Clean(".."))
	require.NoError(t, err)
	require.Empty(t, violations, "API tests must hide broad composition behind truthful route/module builders")
}

func routeSizedBuilderViolations(backendRoot string) ([]string, error) {
	files, err := parseRouteBuilderFiles(backendRoot)
	if err != nil {
		return nil, err
	}

	var violations []string
	for _, source := range files {
		violations = append(violations, inspectRouteBuilderFile(source)...)
	}

	sort.Strings(violations)
	return violations, nil
}

func inspectRouteBuilderFile(source routeBuilderFile) []string {
	var violations []string
	for _, declaration := range source.file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Body != nil {
			violations = append(violations, inspectRouteBuilderFunction(source, function)...)
		}
	}
	return violations
}

func inspectRouteBuilderFunction(source routeBuilderFile, function *ast.FuncDecl) []string {
	name := function.Name.Name
	kind := classifyRouteBuilder(name)
	result := inspectRouteBuilderResults(source, function.Type.Results)
	var violations []string
	if kind != notRouteBuilder && result.hasFactory {
		violations = append(violations, source.relPath+"#"+name+" returns services.Factory")
	}
	if kind != notRouteBuilder && result.hasUntyped {
		violations = append(violations, source.relPath+"#"+name+" returns an untyped capability")
	}
	if kind == routeBuilder && !result.hasRoute {
		violations = append(violations, source.relPath+"#"+name+" does not return a router, handler, or API resource")
	}
	return append(violations, broadCompositionCalls(source, function, kind)...)
}

func broadCompositionCalls(source routeBuilderFile, function *ast.FuncDecl, kind routeBuilderKind) []string {
	var violations []string
	name := function.Name.Name
	ast.Inspect(function.Body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		identifier, identifierOK := selectorIdentifier(selector)
		if !ok || !identifierOK {
			return true
		}
		importPath := source.imports[identifier.Name]
		smokeTest := source.relPath == "api/testutil/helpers_test.go" && name == "TestSetupAPITest"
		if importPath == testutilImport && selector.Sel.Name == "SetupAPITest" && kind == notRouteBuilder && !smokeTest {
			violations = append(violations, source.relPath+"#"+name+" calls SetupAPITest outside a route/module builder")
		}
		if importPath == rootServicesImport && selector.Sel.Name == "NewFactory" && kind == notRouteBuilder {
			violations = append(violations, source.relPath+"#"+name+" calls services.NewFactory outside a route/module builder")
		}
		return true
	})
	return violations
}

func selectorIdentifier(selector *ast.SelectorExpr) (*ast.Ident, bool) {
	if selector == nil {
		return nil, false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return identifier, ok
}

func parseRouteBuilderFiles(backendRoot string) ([]routeBuilderFile, error) {
	state := &routeBuilderParseState{
		backendRoot:    backendRoot,
		typesByPackage: make(map[string]map[string]ast.Expr),
	}
	if err := filepath.WalkDir(backendRoot, state.parse); err != nil {
		return nil, err
	}
	return state.files(), nil
}

func (state *routeBuilderParseState) parse(path string, entry fs.DirEntry, walkErr error) error {
	if walkErr != nil {
		return walkErr
	}
	if entry.IsDir() || !strings.HasSuffix(path, "_test.go") {
		return nil
	}
	relPath, err := filepath.Rel(state.backendRoot, path)
	if err != nil {
		return err
	}
	relPath = filepath.ToSlash(relPath)
	if !routeBuilderScope(relPath) {
		return nil
	}

	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	pkgKey := filepath.Dir(relPath) + "|" + file.Name.Name
	if state.typesByPackage[pkgKey] == nil {
		state.typesByPackage[pkgKey] = make(map[string]ast.Expr)
	}
	collectRouteBuilderTypes(file, state.typesByPackage[pkgKey])
	state.parsed = append(state.parsed, parsedRouteBuilderFile{
		relPath: relPath,
		file:    file,
		imports: routeBuilderImports(file),
		pkgKey:  pkgKey,
	})
	return nil
}

func collectRouteBuilderTypes(file *ast.File, types map[string]ast.Expr) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec := specification.(*ast.TypeSpec)
			types[typeSpec.Name.Name] = typeSpec.Type
		}
	}
}

func (state *routeBuilderParseState) files() []routeBuilderFile {
	files := make([]routeBuilderFile, 0, len(state.parsed))
	for _, source := range state.parsed {
		files = append(files, routeBuilderFile{
			relPath: source.relPath,
			file:    source.file,
			imports: source.imports,
			types:   state.typesByPackage[source.pkgKey],
		})
	}
	return files
}

func routeBuilderScope(relPath string) bool {
	return strings.HasPrefix(relPath, "api/") ||
		strings.HasPrefix(relPath, "test/e2e/calendar/") ||
		strings.HasPrefix(relPath, "test/e2e/timetable/")
}

func routeBuilderImports(file *ast.File) map[string]string {
	imports := make(map[string]string, len(file.Imports))
	for _, specification := range file.Imports {
		path := strings.Trim(specification.Path.Value, `"`)
		name := ""
		if specification.Name != nil {
			name = specification.Name.Name
		} else {
			name = defaultRouteBuilderImportName(path)
		}
		if name != "_" && name != "." {
			imports[name] = path
		}
	}
	return imports
}

func defaultRouteBuilderImportName(path string) string {
	switch path {
	case chiImport:
		return "chi"
	case httpImport:
		return "http"
	default:
		parts := strings.Split(path, "/")
		return parts[len(parts)-1]
	}
}

func classifyRouteBuilder(name string) routeBuilderKind {
	if !strings.HasPrefix(name, "setup") && !strings.HasPrefix(name, "build") {
		return notRouteBuilder
	}
	if strings.HasSuffix(name, "Module") {
		return moduleBuilder
	}
	if strings.HasSuffix(name, "Route") || strings.HasSuffix(name, "Router") {
		return routeBuilder
	}
	return notRouteBuilder
}

func inspectRouteBuilderResults(source routeBuilderFile, results *ast.FieldList) routeBuilderResult {
	if results == nil {
		return routeBuilderResult{}
	}
	result := routeBuilderResult{}
	visited := make(map[string]bool)
	for _, field := range results.List {
		mergeRouteBuilderResult(&result, inspectRouteBuilderType(source, field.Type, visited))
	}
	return result
}

func inspectRouteBuilderType(source routeBuilderFile, expression ast.Expr, visited map[string]bool) routeBuilderResult {
	switch typed := expression.(type) {
	case *ast.Ident:
		return inspectRouteBuilderIdentifier(source, typed, visited)
	case *ast.SelectorExpr:
		return inspectRouteBuilderSelector(source, typed)
	case *ast.StarExpr:
		return inspectRouteBuilderType(source, typed.X, visited)
	case *ast.ParenExpr:
		return inspectRouteBuilderType(source, typed.X, visited)
	case *ast.ArrayType:
		return inspectRouteBuilderType(source, typed.Elt, visited)
	case *ast.Ellipsis:
		return inspectRouteBuilderType(source, typed.Elt, visited)
	}
	return inspectCompositeRouteBuilderType(source, expression, visited)
}

func inspectRouteBuilderIdentifier(source routeBuilderFile, identifier *ast.Ident, visited map[string]bool) routeBuilderResult {
	if identifier.Name == "any" {
		return routeBuilderResult{hasUntyped: true}
	}
	if (identifier.Name == "Resource" && strings.HasPrefix(source.relPath, "api/")) ||
		(identifier.Name == "API" && filepath.Dir(source.relPath) == "api") {
		return routeBuilderResult{hasRoute: true}
	}
	if visited[identifier.Name] {
		return routeBuilderResult{}
	}
	declaration, ok := source.types[identifier.Name]
	if !ok {
		return routeBuilderResult{}
	}
	visited[identifier.Name] = true
	result := inspectRouteBuilderType(source, declaration, visited)
	delete(visited, identifier.Name)
	return result
}

func inspectRouteBuilderSelector(source routeBuilderFile, selector *ast.SelectorExpr) routeBuilderResult {
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return routeBuilderResult{}
	}
	importPath := source.imports[identifier.Name]
	return routeBuilderResult{
		hasFactory: importPath == rootServicesImport && selector.Sel.Name == "Factory",
		hasRoute: (importPath == chiImport && selector.Sel.Name == "Router") ||
			(importPath == httpImport && selector.Sel.Name == "Handler") ||
			(strings.Contains(importPath, "/api/") && selector.Sel.Name == "Resource"),
	}
}

func inspectCompositeRouteBuilderType(source routeBuilderFile, expression ast.Expr, visited map[string]bool) routeBuilderResult {
	switch typed := expression.(type) {
	case *ast.MapType:
		result := inspectRouteBuilderType(source, typed.Key, visited)
		mergeRouteBuilderResult(&result, inspectRouteBuilderType(source, typed.Value, visited))
		return result
	case *ast.ChanType:
		return inspectRouteBuilderType(source, typed.Value, visited)
	case *ast.FuncType:
		result := inspectRouteBuilderFieldList(source, typed.Params, visited)
		mergeRouteBuilderResult(&result, inspectRouteBuilderFieldList(source, typed.Results, visited))
		return result
	case *ast.StructType:
		return inspectRouteBuilderFieldList(source, typed.Fields, visited)
	case *ast.InterfaceType:
		if typed.Methods == nil || len(typed.Methods.List) == 0 {
			return routeBuilderResult{hasUntyped: true}
		}
		return inspectRouteBuilderFieldList(source, typed.Methods, visited)
	case *ast.IndexExpr:
		result := inspectRouteBuilderType(source, typed.X, visited)
		mergeRouteBuilderResult(&result, inspectRouteBuilderType(source, typed.Index, visited))
		return result
	case *ast.IndexListExpr:
		result := inspectRouteBuilderType(source, typed.X, visited)
		for _, index := range typed.Indices {
			mergeRouteBuilderResult(&result, inspectRouteBuilderType(source, index, visited))
		}
		return result
	}
	return routeBuilderResult{}
}

func inspectRouteBuilderFieldList(source routeBuilderFile, fields *ast.FieldList, visited map[string]bool) routeBuilderResult {
	if fields == nil {
		return routeBuilderResult{}
	}
	result := routeBuilderResult{}
	for _, field := range fields.List {
		mergeRouteBuilderResult(&result, inspectRouteBuilderType(source, field.Type, visited))
	}
	return result
}

func mergeRouteBuilderResult(target *routeBuilderResult, source routeBuilderResult) {
	target.hasFactory = target.hasFactory || source.hasFactory
	target.hasRoute = target.hasRoute || source.hasRoute
	target.hasUntyped = target.hasUntyped || source.hasUntyped
}
