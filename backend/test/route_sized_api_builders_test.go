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
	projectImportRoot  = "github.com/moto-nrw/project-phoenix"
)

type routeBuilderKind uint8

const (
	notRouteBuilder routeBuilderKind = iota
	routeBuilder
	moduleBuilder
)

type routeBuilderFile struct {
	relPath           string
	file              *ast.File
	imports           map[string]string
	types             map[string]routeBuilderTypeDecl
	typesByPackage    map[string]map[string]routeBuilderTypeDecl
	typesByImportPath map[string]map[string]routeBuilderTypeDecl
}

type routeBuilderTypeDecl struct {
	expression ast.Expr
	imports    map[string]string
	pkgKey     string
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
	backendRoot       string
	parsed            []parsedRouteBuilderFile
	typesByPackage    map[string]map[string]routeBuilderTypeDecl
	typesByImportPath map[string]map[string]routeBuilderTypeDecl
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
		if !strings.HasSuffix(source.relPath, "_test.go") || !routeBuilderScope(source.relPath) {
			continue
		}
		violations = append(violations, inspectRouteBuilderFile(source)...)
	}
	violations = append(violations, compositionGraphViolations(files)...)

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
	if result.hasFactory {
		violations = append(violations, source.relPath+"#"+name+" returns services.Factory")
	}
	if kind != notRouteBuilder && result.hasUntyped {
		violations = append(violations, source.relPath+"#"+name+" returns an untyped capability")
	}
	if kind == routeBuilder && !result.hasRoute {
		violations = append(violations, source.relPath+"#"+name+" does not return a router, handler, or API resource")
	}
	return append(violations, broadCompositionCalls(source, function)...)
}

func broadCompositionCalls(source routeBuilderFile, function *ast.FuncDecl) []string {
	var violations []string
	name := function.Name.Name
	ast.Inspect(function.Body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		identifier, identifierOK := selectorIdentifier(selector)
		if !ok || !identifierOK {
			return true
		}
		importPath := source.imports[identifier.Name]
		if importPath == testutilImport && selector.Sel.Name == "SetupAPITest" {
			violations = append(violations, source.relPath+"#"+name+" calls SetupAPITest outside a route/module builder")
		}
		if importPath == rootServicesImport && selector.Sel.Name == "NewFactory" {
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
		backendRoot:       backendRoot,
		typesByPackage:    make(map[string]map[string]routeBuilderTypeDecl),
		typesByImportPath: make(map[string]map[string]routeBuilderTypeDecl),
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
	if entry.IsDir() || !strings.HasSuffix(path, ".go") {
		return nil
	}
	relPath, err := filepath.Rel(state.backendRoot, path)
	if err != nil {
		return err
	}
	relPath = filepath.ToSlash(relPath)
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
	if err != nil {
		return err
	}
	pkgKey := filepath.Dir(relPath) + "|" + file.Name.Name
	if state.typesByPackage[pkgKey] == nil {
		state.typesByPackage[pkgKey] = make(map[string]routeBuilderTypeDecl)
	}
	imports := routeBuilderImports(file)
	collectRouteBuilderTypes(file, imports, pkgKey, state.typesByPackage[pkgKey])
	if !strings.HasSuffix(relPath, "_test.go") {
		importPath := projectImportRoot
		if dir := filepath.ToSlash(filepath.Dir(relPath)); dir != "." {
			importPath += "/" + dir
		}
		if state.typesByImportPath[importPath] == nil {
			state.typesByImportPath[importPath] = make(map[string]routeBuilderTypeDecl)
		}
		collectRouteBuilderTypes(file, imports, pkgKey, state.typesByImportPath[importPath])
	}
	state.parsed = append(state.parsed, parsedRouteBuilderFile{
		relPath: relPath,
		file:    file,
		imports: imports,
		pkgKey:  pkgKey,
	})
	return nil
}

func collectRouteBuilderTypes(file *ast.File, imports map[string]string, pkgKey string, types map[string]routeBuilderTypeDecl) {
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok || general.Tok != token.TYPE {
			continue
		}
		for _, specification := range general.Specs {
			typeSpec := specification.(*ast.TypeSpec)
			types[typeSpec.Name.Name] = routeBuilderTypeDecl{
				expression: typeSpec.Type,
				imports:    imports,
				pkgKey:     pkgKey,
			}
		}
	}
}

func (state *routeBuilderParseState) files() []routeBuilderFile {
	files := make([]routeBuilderFile, 0, len(state.parsed))
	for _, source := range state.parsed {
		files = append(files, routeBuilderFile{
			relPath:           source.relPath,
			file:              source.file,
			imports:           source.imports,
			types:             state.typesByPackage[source.pkgKey],
			typesByPackage:    state.typesByPackage,
			typesByImportPath: state.typesByImportPath,
		})
	}
	return files
}

func routeBuilderScope(relPath string) bool {
	return strings.HasPrefix(relPath, "api/") ||
		(strings.HasPrefix(relPath, "modules/") && (strings.Contains(relPath, "/http/") || strings.Contains(relPath, "/compose/httpadapter/"))) ||
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
		return inspectRouteBuilderSelector(source, typed, visited)
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
	if (identifier.Name == "Resource" && routeBuilderScope(source.relPath)) ||
		(identifier.Name == "API" && filepath.Dir(source.relPath) == "api") {
		return routeBuilderResult{hasRoute: true}
	}
	declaration, ok := source.types[identifier.Name]
	if !ok {
		return routeBuilderResult{}
	}
	key := declaration.pkgKey + "." + identifier.Name
	if visited[key] {
		return routeBuilderResult{}
	}
	visited[key] = true
	nested := source
	nested.imports = declaration.imports
	nested.types = source.typesByPackage[declaration.pkgKey]
	result := inspectRouteBuilderType(nested, declaration.expression, visited)
	delete(visited, key)
	return result
}

func inspectRouteBuilderSelector(source routeBuilderFile, selector *ast.SelectorExpr, visited map[string]bool) routeBuilderResult {
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return routeBuilderResult{}
	}
	importPath := source.imports[identifier.Name]
	result := routeBuilderResult{
		hasFactory: importPath == rootServicesImport && selector.Sel.Name == "Factory",
		hasRoute: (importPath == chiImport && selector.Sel.Name == "Router") ||
			(importPath == httpImport && selector.Sel.Name == "Handler") ||
			(isHTTPResourceImport(importPath) && strings.HasSuffix(selector.Sel.Name, "Resource")),
	}
	if result.hasFactory || result.hasRoute {
		return result
	}
	declaration, ok := source.typesByImportPath[importPath][selector.Sel.Name]
	if !ok {
		return result
	}
	key := importPath + "." + selector.Sel.Name
	if visited[key] {
		return result
	}
	visited[key] = true
	nested := source
	nested.imports = declaration.imports
	nested.types = source.typesByPackage[declaration.pkgKey]
	result = inspectRouteBuilderType(nested, declaration.expression, visited)
	delete(visited, key)
	result.hasUntyped = inspectImportedCarrierUntyped(nested, declaration.expression, make(map[string]bool))
	if typed, isInterface := declaration.expression.(*ast.InterfaceType); isInterface &&
		typed.Methods != nil && len(typed.Methods.List) > 0 {
		return routeBuilderResult{hasFactory: result.hasFactory, hasUntyped: result.hasUntyped}
	}
	return result
}

func inspectImportedCarrierUntyped(source routeBuilderFile, expression ast.Expr, visited map[string]bool) bool {
	switch typed := expression.(type) {
	case *ast.Ident:
		if typed.Name == "any" {
			return true
		}
		declaration, ok := source.types[typed.Name]
		if !ok {
			return false
		}
		key := declaration.pkgKey + "." + typed.Name
		if visited[key] {
			return false
		}
		visited[key] = true
		nested := source
		nested.imports = declaration.imports
		nested.types = source.typesByPackage[declaration.pkgKey]
		result := inspectImportedCarrierUntyped(nested, declaration.expression, visited)
		delete(visited, key)
		return result
	case *ast.StarExpr:
		return inspectImportedCarrierUntyped(source, typed.X, visited)
	case *ast.ParenExpr:
		return inspectImportedCarrierUntyped(source, typed.X, visited)
	case *ast.ArrayType:
		return inspectImportedCarrierUntyped(source, typed.Elt, visited)
	case *ast.Ellipsis:
		return inspectImportedCarrierUntyped(source, typed.Elt, visited)
	}
	return inspectImportedCompositeUntyped(source, expression, visited)
}

func inspectImportedCompositeUntyped(source routeBuilderFile, expression ast.Expr, visited map[string]bool) bool {
	switch typed := expression.(type) {
	case *ast.MapType:
		return inspectImportedCarrierUntyped(source, typed.Key, visited) ||
			inspectImportedCarrierUntyped(source, typed.Value, visited)
	case *ast.ChanType:
		return inspectImportedCarrierUntyped(source, typed.Value, visited)
	case *ast.FuncType:
		return importedFieldsHaveUntyped(source, typed.Params, visited) ||
			importedFieldsHaveUntyped(source, typed.Results, visited)
	case *ast.StructType:
		return importedFieldsHaveUntyped(source, typed.Fields, visited)
	case *ast.InterfaceType:
		return importedInterfaceIsUntyped(source, typed, visited)
	case *ast.SelectorExpr:
		return inspectImportedSelectorUntyped(source, typed, visited)
	case *ast.IndexExpr:
		return inspectImportedCarrierUntyped(source, typed.X, visited) ||
			inspectImportedCarrierUntyped(source, typed.Index, visited)
	case *ast.IndexListExpr:
		if inspectImportedCarrierUntyped(source, typed.X, visited) {
			return true
		}
		for _, index := range typed.Indices {
			if inspectImportedCarrierUntyped(source, index, visited) {
				return true
			}
		}
	}
	return false
}

func inspectImportedSelectorUntyped(source routeBuilderFile, selector *ast.SelectorExpr, visited map[string]bool) bool {
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	importPath := source.imports[identifier.Name]
	if isHTTPResourceImport(importPath) && strings.HasSuffix(selector.Sel.Name, "Resource") {
		return false
	}
	declaration, ok := source.typesByImportPath[importPath][selector.Sel.Name]
	if !ok {
		return false
	}
	key := importPath + "." + selector.Sel.Name
	if visited[key] {
		return false
	}
	visited[key] = true
	nested := source
	nested.imports = declaration.imports
	nested.types = source.typesByPackage[declaration.pkgKey]
	result := inspectImportedCarrierUntyped(nested, declaration.expression, visited)
	delete(visited, key)
	return result
}

func isHTTPResourceImport(path string) bool {
	return strings.Contains(path, "/api/") || strings.Contains(path, "/http/") || strings.Contains(path, "/compose/httpadapter")
}

func importedInterfaceIsUntyped(source routeBuilderFile, typed *ast.InterfaceType, visited map[string]bool) bool {
	if typed.Methods == nil || len(typed.Methods.List) == 0 {
		return true
	}
	for _, method := range typed.Methods.List {
		if len(method.Names) == 0 && inspectImportedCarrierUntyped(source, method.Type, visited) {
			return true
		}
	}
	return false
}

func importedFieldsHaveUntyped(source routeBuilderFile, fields *ast.FieldList, visited map[string]bool) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if inspectImportedCarrierUntyped(source, field.Type, visited) {
			return true
		}
	}
	return false
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
