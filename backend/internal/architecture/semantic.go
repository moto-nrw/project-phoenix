package architecture

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path"
	"reflect"
	"regexp"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

var (
	dataObjectPattern          = regexp.MustCompile(`(?i)^\s*["` + "`" + `]?([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)`)
	qualifiedDataObjectPattern = regexp.MustCompile(`(?i)\b([a-z][a-z0-9_]*\.[a-z][a-z0-9_]*)\b`)
	writeTablePattern          = regexp.MustCompile(`(?i)\b(?:INSERT\s+INTO|UPDATE|DELETE\s+FROM|MERGE\s+INTO)\s+(?:ONLY\s+)?([a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)?)`)
	truncatePattern            = regexp.MustCompile(`(?i)\bTRUNCATE(?:\s+TABLE)?\s+([^;]+)`)
	truncateModifier           = regexp.MustCompile(`(?i)\s+(?:RESTART|CONTINUE)\s+IDENTITY\b.*$|\s+(?:CASCADE|RESTRICT)\b.*$`)
	sqlCTEPattern              = regexp.MustCompile(`(?i)(?:\bWITH\s+(?:RECURSIVE\s+)?|,)\s*["` + "`" + `]?([a-z][a-z0-9_]*)["` + "`" + `]?(?:\s*\([^)]*\))?\s+AS\s*\(`)
	sqlTokenPattern            = regexp.MustCompile(`(?i)[a-z][a-z0-9_]*(?:\.[a-z][a-z0-9_]*)?|[(),;]`)
	sqlSourceStops             = map[string]bool{
		"CROSS": true, "EXCEPT": true, "FOR": true, "FULL": true, "GROUP": true,
		"INNER": true, "INTERSECT": true, "LEFT": true, "LIMIT": true, "OFFSET": true,
		"ON": true, "ORDER": true, "RETURNING": true, "RIGHT": true, "SET": true,
		"UNION": true, "VALUES": true, "WHEN": true, "WHERE": true,
	}
	crudMethodNames = map[string]bool{
		"Add": true, "Count": true, "Create": true, "Delete": true, "DeleteByID": true,
		"Exists": true, "Find": true, "FindAll": true, "FindByID": true, "Get": true,
		"GetAll": true, "GetByID": true, "Insert": true, "List": true, "ListAll": true,
		"Patch": true, "Query": true, "Read": true, "Remove": true, "Save": true,
		"Search": true, "Update": true, "UpdateByID": true, "UpdateColumns": true, "Upsert": true,
	}
	sqlFragmentMethods = map[string]bool{
		"ColumnExpr": true, "DistinctOn": true, "GroupExpr": true, "Having": true,
		"Join": true, "JoinOn": true, "JoinOnOr": true, "On": true,
		"OrderExpr": true, "Returning": true, "Set": true, "Where": true, "WhereOr": true,
	}
)

func analyzeSemantics(project string, policy *Policy) ([]Violation, error) {
	loaded, err := loadTypedPackages(project, policy.Build)
	if err != nil {
		return nil, err
	}
	analyzer := newSemanticAnalyzer(policy, project)
	if err := validateLoadedLegacySymbols(loaded, analyzer.legacySymbols); err != nil {
		return nil, err
	}

	var violations []Violation
	for _, pkg := range loaded {
		if isOwnPackage(policy.ModulePath, pkg.PkgPath) {
			packageViolations, err := analyzer.analyzePackage(pkg)
			if err != nil {
				return nil, err
			}
			violations = append(violations, packageViolations...)
		}
	}
	return uniqueSortedViolations(violations), nil
}

func loadTypedPackages(project string, build Build) ([]*packages.Package, error) {
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedDeps | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir: project,
		Env: fixedBuildEnvironment(build),
	}
	loaded, err := packages.Load(config, "./...")
	if err != nil {
		return nil, fmt.Errorf("load Go types: %w", err)
	}
	for _, pkg := range loaded {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load Go types for %s: %s", pkg.PkgPath, pkg.Errors[0])
		}
	}
	return loaded, nil
}

func newSemanticAnalyzer(policy *Policy, project string) semanticAnalyzer {
	analyzer := semanticAnalyzer{
		project:       project,
		packages:      policy.packageMap(),
		dataObjects:   make(map[string]DataObject, len(policy.DataObjects)),
		dataSchemas:   make(map[string]struct{}),
		projections:   make(map[string]map[string]struct{}, len(policy.ReadProjections)),
		legacySymbols: make(map[string]map[string]struct{}, len(policy.LegacyComposition)),
	}
	populateSemanticPolicy(&analyzer, policy)
	return analyzer
}

func populateSemanticPolicy(analyzer *semanticAnalyzer, policy *Policy) {
	for _, object := range policy.DataObjects {
		analyzer.dataObjects[object.Name] = object
		analyzer.dataSchemas[strings.SplitN(object.Name, ".", 2)[0]] = struct{}{}
	}
	for _, projection := range policy.ReadProjections {
		packagePath := policy.absolutePackage(projection.Package)
		objects := analyzer.projections[packagePath]
		if objects == nil {
			objects = make(map[string]struct{}, len(projection.DataObjects))
		}
		for _, object := range projection.DataObjects {
			objects[object] = struct{}{}
		}
		analyzer.projections[packagePath] = objects
	}
	for _, legacy := range policy.LegacyComposition {
		packagePath := policy.absolutePackage(legacy.Package)
		symbols := analyzer.legacySymbols[packagePath]
		if symbols == nil {
			symbols = make(map[string]struct{}, len(legacy.Symbols))
		}
		for _, symbol := range legacy.Symbols {
			symbols[symbol] = struct{}{}
		}
		analyzer.legacySymbols[packagePath] = symbols
	}
}

type semanticAnalyzer struct {
	project       string
	packages      map[string]Package
	dataObjects   map[string]DataObject
	dataSchemas   map[string]struct{}
	projections   map[string]map[string]struct{}
	legacySymbols map[string]map[string]struct{}
}

func (a semanticAnalyzer) analyzePackage(pkg *packages.Package) ([]Violation, error) {
	classification, classified := a.packages[pkg.PkgPath]
	if !classified {
		return nil, nil
	}

	var violations []Violation
	for _, file := range pkg.Syntax {
		fileViolations, err := a.analyzeFile(pkg, classification, file)
		if err != nil {
			return nil, err
		}
		violations = append(violations, fileViolations...)
	}
	legacyViolations, err := a.legacyReferenceViolations(pkg)
	if err != nil {
		return nil, err
	}
	violations = append(violations, legacyViolations...)
	if classification.Role == "public" || classification.Role == "contract" {
		contractFindings, err := a.locateContractViolations(pkg, contractViolations(pkg))
		if err != nil {
			return nil, err
		}
		violations = append(violations, contractFindings...)
	}
	return violations, nil
}

func (a semanticAnalyzer) analyzeFile(pkg *packages.Package, classification Package, file *ast.File) ([]Violation, error) {
	parents := astParents(file)
	var violations []Violation
	for _, decl := range file.Decls {
		declaration := declarationName(pkg.PkgPath, decl)
		var locationErr error
		ast.Inspect(decl, func(node ast.Node) bool {
			if locationErr != nil {
				return false
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callViolations := a.callViolations(pkg, classification, declaration, call, parents)
			if len(callViolations) == 0 {
				return true
			}
			location, err := a.nodeLocation(pkg, call.Pos(), declaration)
			if err != nil {
				locationErr = err
				return false
			}
			violations = append(violations, attachLocation(callViolations, location)...)
			return true
		})
		if locationErr != nil {
			return nil, fmt.Errorf("resolve semantic location in %s: %w", pkg.PkgPath, locationErr)
		}
	}
	return violations, nil
}

func declarationName(packagePath string, declaration ast.Decl) string {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil || len(typed.Recv.List) == 0 {
			return typed.Name.Name
		}
		return receiverTypeName(typed.Recv.List[0].Type) + "." + typed.Name.Name
	case *ast.GenDecl:
		if names := declaredNames(typed); len(names) > 0 {
			return typed.Tok.String() + " " + strings.Join(names, ", ")
		}
	}
	return path.Base(packagePath)
}

func declaredNames(declaration *ast.GenDecl) []string {
	var names []string
	for _, specification := range declaration.Specs {
		switch typed := specification.(type) {
		case *ast.TypeSpec:
			names = append(names, typed.Name.Name)
		case *ast.ValueSpec:
			for _, name := range typed.Names {
				names = append(names, name.Name)
			}
		}
	}
	return names
}

func receiverTypeName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexExpr:
		return receiverTypeName(typed.X)
	case *ast.IndexListExpr:
		return receiverTypeName(typed.X)
	default:
		return "method"
	}
}

func attachLocation(violations []Violation, location Location) []Violation {
	for index := range violations {
		violations[index].Locations = append(violations[index].Locations, location)
	}
	return violations
}

func (a semanticAnalyzer) nodeLocation(pkg *packages.Package, position token.Pos, declaration string) (Location, error) {
	if pkg.Fset == nil {
		return Location{}, fmt.Errorf("go file set is unavailable")
	}
	return sourceLocation(a.project, pkg.Fset.PositionFor(position, false), declaration)
}

func astParents(root ast.Node) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[node] = stack[len(stack)-1]
		}
		stack = append(stack, node)
		return true
	})
	return parents
}

func (a semanticAnalyzer) callViolations(pkg *packages.Package, classification Package, functionName string, call *ast.CallExpr, parents map[ast.Node]ast.Node) []Violation {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	receiver := pkg.TypesInfo.TypeOf(selector.X)
	method := selector.Sel.Name
	violations := a.directDBCallViolations(pkg, classification, functionName, receiver, pkg.TypesInfo.Selections[selector], method, call.Args)
	if !isRuntimeDataPackage(classification) {
		return violations
	}

	operation, tableMethod := bunTableOperation(receiver, method)
	if tableMethod {
		return append(violations, a.tableMethodViolations(pkg, classification, functionName, method, operation, call.Args)...)
	}
	if method == "Model" {
		return append(violations, a.modelMethodViolations(pkg, classification, functionName, receiver, call, parents)...)
	}
	if _, queryMethod := bunQueryOperation(receiver); queryMethod && sqlFragmentMethods[method] {
		return append(violations, a.queryFragmentViolations(pkg, classification, functionName, method, call.Args)...)
	}
	return violations
}

func (a semanticAnalyzer) directDBCallViolations(pkg *packages.Package, classification Package, functionName string, receiver types.Type, selection *types.Selection, method string, args []ast.Expr) []Violation {
	if !isBunDBCall(receiver, selection, method) {
		return nil
	}
	var violations []Violation
	if !a.directDBAllowed(pkg.PkgPath, classification) {
		violations = append(violations, Violation{
			Scope: ScopeProduction, Rule: "database.direct-access", Source: pkg.PkgPath,
			Target: bunCallTarget(receiver, method), Detail: "direct database access is only allowed in Postgres, migration, test, or named projection adapters",
		})
	}
	if isRuntimeDataPackage(classification) && isRawSQLMethod(method) {
		violations = append(violations, a.rawSQLViolations(pkg, classification, functionName, receiver, selection, method, args)...)
	}
	return violations
}

func isRawSQLMethod(method string) bool {
	return method == "NewRaw" || method == "Exec" || method == "ExecContext" || method == "Query" || method == "QueryContext" || method == "QueryRow" || method == "QueryRowContext"
}

func (a semanticAnalyzer) tableMethodViolations(pkg *packages.Package, classification Package, functionName, method string, operation tableOperation, args []ast.Expr) []Violation {
	if len(args) == 0 {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	table, ok := constantString(pkg.TypesInfo, args[0])
	if !ok {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	names := sqlSourceDataObjects(stripIdentifierQuotes(stripSQLLiteralsAndComments("FROM " + table)))
	if len(names) == 0 {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	var violations []Violation
	for _, name := range names {
		violations = append(violations, a.tableAccessViolations(pkg.PkgPath, classification.Owner, operation, name)...)
	}
	return violations
}

func (a semanticAnalyzer) modelMethodViolations(pkg *packages.Package, classification Package, functionName string, receiver types.Type, call *ast.CallExpr, parents map[ast.Node]ast.Node) []Violation {
	operation, ok := bunQueryOperation(receiver)
	if !ok {
		return nil
	}
	if len(call.Args) == 0 {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, "Model")}
	}
	name, ok := bunModelDataObject(pkg.TypesInfo.TypeOf(call.Args[0]))
	if !ok {
		if queryChainHasExplicitTable(call, parents) {
			return nil
		}
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, "Model")}
	}
	return a.tableAccessViolations(pkg.PkgPath, classification.Owner, operation, name)
}

func isRuntimeDataPackage(classification Package) bool {
	return classification.Role != "migration" && classification.Role != "test-support"
}

func queryChainHasExplicitTable(call *ast.CallExpr, parents map[ast.Node]ast.Node) bool {
	if selector, ok := call.Fun.(*ast.SelectorExpr); ok && queryReceiverHasExplicitTable(selector.X) {
		return true
	}
	for node := ast.Node(call); parents[node] != nil; node = parents[node] {
		selector, ok := parents[node].(*ast.SelectorExpr)
		if !ok || selector.X != node {
			continue
		}
		if selector.Sel.Name == "Table" || selector.Sel.Name == "TableExpr" || selector.Sel.Name == "ModelTableExpr" {
			return true
		}
	}
	return false
}

func queryReceiverHasExplicitTable(expr ast.Expr) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	if selector.Sel.Name == "Table" || selector.Sel.Name == "TableExpr" || selector.Sel.Name == "ModelTableExpr" {
		return true
	}
	return queryReceiverHasExplicitTable(selector.X)
}

func (a semanticAnalyzer) rawSQLViolations(pkg *packages.Package, classification Package, functionName string, receiver types.Type, selection *types.Selection, method string, args []ast.Expr) []Violation {
	if isSQLStmtCall(receiver, selection) {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	queryIndex := 0
	if strings.HasSuffix(method, "Context") {
		queryIndex = 1
	}
	if len(args) <= queryIndex {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	query, ok := constantString(pkg.TypesInfo, args[queryIndex])
	if !ok {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	return a.sqlStringViolations(pkg.PkgPath, classification.Owner, functionName, method, query, true)
}

func (a semanticAnalyzer) queryFragmentViolations(pkg *packages.Package, classification Package, functionName, method string, args []ast.Expr) []Violation {
	if len(args) == 0 {
		return nil
	}
	query, ok := constantString(pkg.TypesInfo, args[0])
	if !ok {
		return []Violation{unresolvedTableViolation(pkg.PkgPath, functionName, method)}
	}
	return a.sqlStringViolations(pkg.PkgPath, classification.Owner, functionName, method, query, false)
}

func (a semanticAnalyzer) sqlStringViolations(source, sourceOwner, functionName, method, query string, includeWrites bool) []Violation {
	query = stripIdentifierQuotes(stripSQLLiteralsAndComments(query))
	aliases := sqlCTEAliases(query)
	matches := []tableMatchSummary{
		a.matchDataObjects(source, sourceOwner, sqlSourceDataObjects(query), aliases, tableRead),
		a.matchDataObjects(source, sourceOwner, a.qualifiedDataObjects(query), aliases, tableRead),
	}
	if includeWrites {
		matches = append(matches, a.matchSQLTables(source, sourceOwner, query, aliases, writeTablePattern, tableWrite))
		matches = append(matches, a.matchDataObjects(source, sourceOwner, truncateDataObjects(query), nil, tableWrite))
	}
	var violations []Violation
	unresolved := false
	for _, match := range matches {
		violations = append(violations, match.violations...)
		unresolved = unresolved || match.unresolved
	}
	if unresolved {
		violations = append(violations, unresolvedTableViolation(source, functionName, method))
	}
	return violations
}

func (a semanticAnalyzer) qualifiedDataObjects(query string) []string {
	matches := qualifiedDataObjectPattern.FindAllStringSubmatch(query, -1)
	objects := make([]string, 0, len(matches))
	for _, match := range matches {
		name := strings.ToLower(match[1])
		if _, ok := a.dataObjects[name]; ok {
			objects = append(objects, name)
		}
	}
	return objects
}

func stripSQLLiteralsAndComments(query string) string {
	var result strings.Builder
	for index := 0; index < len(query); {
		if end := sqlDollarQuotedLiteralEnd(query, index); end > index {
			result.WriteString(strings.Repeat(" ", end-index))
			index = end
			continue
		}
		var end int
		switch {
		case query[index] == '\'':
			end = sqlStringLiteralEnd(query, index+1)
		case index+1 < len(query) && query[index:index+2] == "--":
			end = strings.IndexByte(query[index:], '\n')
			if end < 0 {
				end = len(query)
			} else {
				end += index
			}
		case index+1 < len(query) && query[index:index+2] == "/*":
			end = strings.Index(query[index+2:], "*/")
			if end < 0 {
				end = len(query)
			} else {
				end += index + 4
			}
		default:
			result.WriteByte(query[index])
			index++
			continue
		}
		result.WriteString(strings.Repeat(" ", end-index))
		index = end
	}
	return result.String()
}

func sqlStringLiteralEnd(query string, index int) int {
	for index < len(query) {
		if query[index] != '\'' {
			index++
			continue
		}
		index++
		if index == len(query) || query[index] != '\'' {
			return index
		}
		index++
	}
	return len(query)
}

func sqlDollarQuotedLiteralEnd(query string, index int) int {
	if query[index] != '$' || index+1 >= len(query) {
		return index
	}
	end := index + 1
	if query[end] != '$' && !isSQLIdentifierStart(query[end]) {
		return index
	}
	for end < len(query) && isSQLIdentifierPart(query[end]) {
		end++
	}
	if end >= len(query) || query[end] != '$' {
		return index
	}
	delimiter := query[index : end+1]
	closing := strings.Index(query[end+1:], delimiter)
	if closing < 0 {
		return index
	}
	return end + 1 + closing + len(delimiter)
}

func isSQLIdentifierStart(char byte) bool {
	return char == '_' || char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z'
}

func isSQLIdentifierPart(char byte) bool {
	return isSQLIdentifierStart(char) || char >= '0' && char <= '9'
}

type tableMatchSummary struct {
	violations []Violation
	unresolved bool
}

func (a semanticAnalyzer) matchSQLTables(source, sourceOwner, query string, aliases map[string]struct{}, pattern *regexp.Regexp, operation tableOperation) tableMatchSummary {
	var objects []string
	for _, match := range pattern.FindAllStringSubmatch(query, -1) {
		objects = append(objects, match[1])
	}
	return a.matchDataObjects(source, sourceOwner, objects, aliases, operation)
}

func (a semanticAnalyzer) matchDataObjects(source, sourceOwner string, objects []string, aliases map[string]struct{}, operation tableOperation) tableMatchSummary {
	var result tableMatchSummary
	for _, name := range objects {
		if isCTEReference(name, aliases) {
			continue
		}
		if !a.knownDataSchema(name) {
			result.unresolved = true
			continue
		}
		result.violations = append(result.violations, a.tableAccessViolations(source, sourceOwner, operation, strings.ToLower(name))...)
	}
	return result
}

func truncateDataObjects(query string) []string {
	var objects []string
	for _, match := range truncatePattern.FindAllStringSubmatch(query, -1) {
		clause := truncateModifier.ReplaceAllString(match[1], "")
		for _, item := range strings.Split(clause, ",") {
			fields := strings.Fields(strings.TrimSpace(item))
			if len(fields) > 0 && strings.EqualFold(fields[0], "ONLY") {
				fields = fields[1:]
			}
			if len(fields) > 0 {
				objects = append(objects, strings.TrimSuffix(fields[0], "*"))
			}
		}
	}
	return objects
}

func sqlSourceDataObjects(query string) []string {
	states := make(map[int]bool)
	depth := 0
	var objects []string
	tokens := sqlTokenPattern.FindAllString(query, -1)
	for index, token := range tokens {
		if nextDepth, handled := updateSQLSourceState(token, states, depth); handled {
			depth = nextDepth
			continue
		}
		keyword := strings.ToUpper(token)
		if keyword == "FROM" || keyword == "JOIN" || keyword == "USING" {
			states[depth] = true
			continue
		}
		if sqlSourceStops[keyword] {
			delete(states, depth)
			continue
		}
		if expect, active := states[depth]; active && expect && keyword != "ONLY" && keyword != "LATERAL" {
			if index+1 < len(tokens) && tokens[index+1] == "(" {
				states[depth] = false
				continue
			}
			objects = append(objects, token)
			states[depth] = false
		}
	}
	return objects
}

func updateSQLSourceState(token string, states map[int]bool, depth int) (int, bool) {
	switch token {
	case "(":
		if _, active := states[depth]; active {
			states[depth] = false
		}
		return depth + 1, true
	case ")":
		delete(states, depth)
		if depth > 0 {
			depth--
		}
		return depth, true
	case ",":
		if _, active := states[depth]; active {
			states[depth] = true
		}
		return depth, true
	case ";":
		delete(states, depth)
		return depth, true
	default:
		return depth, false
	}
}

func sqlCTEAliases(query string) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, match := range sqlCTEPattern.FindAllStringSubmatch(query, -1) {
		aliases[strings.ToLower(match[1])] = struct{}{}
	}
	return aliases
}

func isCTEReference(name string, aliases map[string]struct{}) bool {
	if strings.Contains(name, ".") {
		return false
	}
	_, ok := aliases[strings.ToLower(name)]
	return ok
}

type tableOperation string

const (
	tableRead  tableOperation = "read"
	tableWrite tableOperation = "write"
)

func (a semanticAnalyzer) tableAccessViolations(source, sourceOwner string, operation tableOperation, name string) []Violation {
	classification := a.packages[source]
	if classification.Role == "migration" || classification.Role == "test-support" {
		return nil
	}
	object, ok := a.dataObjects[name]
	if !ok {
		return []Violation{{Scope: ScopeProduction, Rule: "tables.unclassified", Source: source, Target: name, Detail: "database table has no runtime write owner"}}
	}
	if operation == tableWrite && object.WriteOwner != sourceOwner {
		return []Violation{{Scope: ScopeProduction, Rule: "tables.foreign-write", Source: source, Target: name, Detail: fmt.Sprintf("owner %s writes data owned by %s", sourceOwner, object.WriteOwner)}}
	}
	if operation == tableRead && object.WriteOwner != sourceOwner && !a.projectionAllows(source, name) {
		return []Violation{{Scope: ScopeProduction, Rule: "tables.foreign-read", Source: source, Target: name, Detail: fmt.Sprintf("owner %s reads data owned by %s without an owner query or named tenant-safe projection", sourceOwner, object.WriteOwner)}}
	}
	return nil
}

func (a semanticAnalyzer) knownDataSchema(name string) bool {
	schema, _, ok := strings.Cut(strings.ToLower(name), ".")
	if !ok {
		return false
	}
	_, ok = a.dataSchemas[schema]
	return ok
}

func (a semanticAnalyzer) projectionAllows(source, object string) bool {
	objects, ok := a.projections[source]
	if !ok {
		return false
	}
	_, ok = objects[object]
	return ok
}

func (a semanticAnalyzer) directDBAllowed(source string, classification Package) bool {
	if classification.Role == "postgres" || classification.Role == "migration" || classification.Role == "test-support" {
		return true
	}
	_, projection := a.projections[source]
	return projection
}

func unresolvedTableViolation(source, functionName, method string) Violation {
	return Violation{Scope: ScopeProduction, Rule: "tables.unresolved", Source: source, Target: path.Base(source) + "." + method, Detail: fmt.Sprintf("%s uses a table expression that cannot be resolved statically", functionName)}
}

func bunTableOperation(receiver types.Type, method string) (tableOperation, bool) {
	if method != "Table" && method != "TableExpr" && method != "ModelTableExpr" {
		return "", false
	}
	return bunQueryOperation(receiver)
}

func bunQueryOperation(receiver types.Type) (tableOperation, bool) {
	typeName := types.TypeString(receiver, packagePathQualifier)
	switch {
	case strings.Contains(typeName, "github.com/uptrace/bun.SelectQuery"):
		return tableRead, true
	case strings.Contains(typeName, "github.com/uptrace/bun.InsertQuery"), strings.Contains(typeName, "github.com/uptrace/bun.UpdateQuery"), strings.Contains(typeName, "github.com/uptrace/bun.DeleteQuery"), strings.Contains(typeName, "github.com/uptrace/bun.MergeQuery"), strings.Contains(typeName, "github.com/uptrace/bun.TruncateTableQuery"):
		return tableWrite, true
	default:
		return "", false
	}
}

func bunModelDataObject(model types.Type) (string, bool) {
	for {
		switch typed := model.(type) {
		case *types.Pointer:
			model = typed.Elem()
		case *types.Slice:
			model = typed.Elem()
		case *types.Array:
			model = typed.Elem()
		case *types.Named:
			model = typed.Underlying()
		default:
			structure, ok := model.(*types.Struct)
			if !ok {
				return "", false
			}
			for i := 0; i < structure.NumFields(); i++ {
				bunTag := reflect.StructTag(structure.Tag(i)).Get("bun")
				var schema, table string
				for _, option := range strings.Split(bunTag, ",") {
					if name, found := strings.CutPrefix(option, "schema:"); found {
						schema = name
					}
					if name, found := strings.CutPrefix(option, "table:"); found {
						table = name
					}
				}
				if strings.Contains(table, ".") {
					return normalizeDataObject(table)
				}
				if schema != "" && table != "" {
					return normalizeDataObject(schema + "." + table)
				}
			}
			return "", false
		}
	}
}

func isBunDBCall(receiver types.Type, selection *types.Selection, method string) bool {
	if receiver == nil {
		return false
	}
	if isBunDBType(receiver) || isBunDatabaseInterface(receiver, method) || isSQLDatabaseInterface(receiver, method) {
		return true
	}
	if selection == nil {
		return false
	}
	function, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := function.Type().(*types.Signature)
	return ok && signature.Recv() != nil && isBunDBType(signature.Recv().Type())
}

func isBunDatabaseInterface(receiver types.Type, method string) bool {
	if receiver == nil {
		return false
	}
	interfaceType, ok := receiver.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	interfaceType.Complete()
	for index := 0; index < interfaceType.NumMethods(); index++ {
		interfaceMethod := interfaceType.Method(index)
		if interfaceMethod.Name() == method && isBunQueryMethod(interfaceMethod) {
			return true
		}
	}
	return false
}

func isBunQueryMethod(function *types.Func) bool {
	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return false
	}
	for result := 0; result < signature.Results().Len(); result++ {
		typeName := types.TypeString(signature.Results().At(result).Type(), packagePathQualifier)
		if strings.Contains(typeName, "github.com/uptrace/bun.SelectQuery") || strings.Contains(typeName, "github.com/uptrace/bun.InsertQuery") || strings.Contains(typeName, "github.com/uptrace/bun.UpdateQuery") || strings.Contains(typeName, "github.com/uptrace/bun.DeleteQuery") || strings.Contains(typeName, "github.com/uptrace/bun.MergeQuery") || strings.Contains(typeName, "github.com/uptrace/bun.RawQuery") {
			return true
		}
	}
	return false
}

var sqlDatabaseInterfaceMethods = map[string]struct{}{
	"Begin": {}, "BeginTx": {}, "Conn": {}, "Exec": {}, "ExecContext": {},
	"Ping": {}, "PingContext": {}, "Prepare": {}, "PrepareContext": {},
	"Query": {}, "QueryContext": {}, "QueryRow": {}, "QueryRowContext": {},
}

func isSQLDatabaseInterface(receiver types.Type, method string) bool {
	if receiver == nil {
		return false
	}
	if _, ok := sqlDatabaseInterfaceMethods[method]; !ok {
		return false
	}
	interfaceType, ok := receiver.Underlying().(*types.Interface)
	if !ok {
		return false
	}
	interfaceType.Complete()
	for index := 0; index < interfaceType.NumMethods(); index++ {
		if interfaceType.Method(index).Name() == method && isSQLDatabaseInterfaceMethod(method, interfaceType.Method(index)) {
			return true
		}
	}
	return false
}

func isSQLDatabaseInterfaceMethod(method string, function *types.Func) bool {
	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return false
	}
	queryArgument := 0
	if strings.HasSuffix(method, "Context") {
		queryArgument = 1
	}
	switch method {
	case "Exec", "ExecContext":
		return hasSQLQueryArgument(signature, queryArgument) && hasSQLResult(signature, "database/sql.Result")
	case "Query", "QueryContext":
		return hasSQLQueryArgument(signature, queryArgument) && hasSQLResult(signature, "database/sql.Rows")
	case "QueryRow", "QueryRowContext":
		return hasSQLQueryArgument(signature, queryArgument) && hasSQLResult(signature, "database/sql.Row")
	case "Begin", "BeginTx":
		return hasSQLResult(signature, "database/sql.Tx")
	case "Conn":
		return hasSQLResult(signature, "database/sql.Conn")
	case "Prepare", "PrepareContext":
		return hasSQLQueryArgument(signature, queryArgument) && hasSQLResult(signature, "database/sql.Stmt")
	case "Ping":
		return signature.Params().Len() == 0 && hasErrorResult(signature)
	case "PingContext":
		return signature.Params().Len() == 1 && hasContextArgument(signature, 0) && hasErrorResult(signature)
	default:
		return false
	}
}

func hasContextArgument(signature *types.Signature, index int) bool {
	return signature.Params().Len() > index && types.TypeString(signature.Params().At(index).Type(), packagePathQualifier) == "context.Context"
}

func hasErrorResult(signature *types.Signature) bool {
	return signature.Results().Len() == 1 && types.Identical(signature.Results().At(0).Type(), types.Universe.Lookup("error").Type())
}

func hasSQLQueryArgument(signature *types.Signature, index int) bool {
	return signature.Params().Len() > index && types.Identical(signature.Params().At(index).Type(), types.Typ[types.String])
}

func hasSQLResult(signature *types.Signature, typeName string) bool {
	for index := 0; index < signature.Results().Len(); index++ {
		if strings.TrimPrefix(types.TypeString(signature.Results().At(index).Type(), packagePathQualifier), "*") == typeName {
			return true
		}
	}
	return false
}

func isBunDBType(receiver types.Type) bool {
	typeName := types.TypeString(receiver, packagePathQualifier)
	return strings.Contains(typeName, "github.com/uptrace/bun.DB") || strings.Contains(typeName, "github.com/uptrace/bun.IDB") || strings.Contains(typeName, "github.com/uptrace/bun.Tx") || strings.Contains(typeName, "database/sql.DB") || strings.Contains(typeName, "database/sql.Tx") || strings.Contains(typeName, "database/sql.Conn") || strings.Contains(typeName, "database/sql.Stmt")
}

func isSQLStmtCall(receiver types.Type, selection *types.Selection) bool {
	if strings.Contains(types.TypeString(receiver, packagePathQualifier), "database/sql.Stmt") {
		return true
	}
	if selection == nil {
		return false
	}
	function, ok := selection.Obj().(*types.Func)
	if !ok {
		return false
	}
	signature, ok := function.Type().(*types.Signature)
	return ok && signature.Recv() != nil && strings.Contains(types.TypeString(signature.Recv().Type(), packagePathQualifier), "database/sql.Stmt")
}

func bunCallTarget(receiver types.Type, method string) string {
	typeName := strings.TrimPrefix(types.TypeString(receiver, packagePathQualifier), "*")
	return typeName + "." + method
}

func packagePathQualifier(pkg *types.Package) string { return pkg.Path() }

func normalizeDataObject(expression string) (string, bool) {
	expression = stripIdentifierQuotes(expression)
	match := dataObjectPattern.FindStringSubmatch(expression)
	if len(match) != 2 {
		return "", false
	}
	return strings.ToLower(match[1]), true
}

func stripIdentifierQuotes(value string) string {
	return strings.NewReplacer(`"`, "", "`", "").Replace(value)
}

func constantString(info *types.Info, expression ast.Expr) (string, bool) {
	typed, ok := info.Types[expression]
	if !ok || typed.Value == nil || typed.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(typed.Value), true
}

func (a semanticAnalyzer) legacyReferenceViolations(pkg *packages.Package) ([]Violation, error) {
	var violations []Violation
	for _, file := range pkg.Syntax {
		for _, declaration := range file.Decls {
			found, err := a.legacyReferencesInDeclaration(pkg, declaration)
			if err != nil {
				return nil, err
			}
			violations = append(violations, found...)
		}
	}
	return violations, nil
}

func (a semanticAnalyzer) legacyReferencesInDeclaration(pkg *packages.Package, declaration ast.Decl) ([]Violation, error) {
	var violations []Violation
	var locationErr error
	name := declarationName(pkg.PkgPath, declaration)
	ast.Inspect(declaration, func(node ast.Node) bool {
		identifier, ok := node.(*ast.Ident)
		if !ok {
			return true
		}
		violation, found, err := a.legacyReferenceViolation(pkg, identifier, name)
		if err != nil {
			locationErr = err
			return false
		}
		if found {
			violations = append(violations, violation)
		}
		return true
	})
	if locationErr != nil {
		return nil, fmt.Errorf("resolve semantic location in %s: %w", pkg.PkgPath, locationErr)
	}
	return violations, nil
}

func (a semanticAnalyzer) legacyReferenceViolation(pkg *packages.Package, identifier *ast.Ident, declaration string) (Violation, bool, error) {
	object := pkg.TypesInfo.Uses[identifier]
	if object == nil || object.Pkg() == nil || object.Pkg().Path() == pkg.PkgPath {
		return Violation{}, false, nil
	}
	symbols, configured := a.legacySymbols[object.Pkg().Path()]
	_, included := symbols[object.Name()]
	if !configured || !included {
		return Violation{}, false, nil
	}
	location, err := a.nodeLocation(pkg, identifier.Pos(), declaration)
	if err != nil {
		return Violation{}, false, err
	}
	return Violation{
		Scope: ScopeProduction, Rule: "composition.legacy-reference", Source: pkg.PkgPath,
		Target: object.Pkg().Path() + "." + object.Name(), Detail: "typed reference couples the package to legacy composition",
		Locations: []Location{location},
	}, true, nil
}

func (a semanticAnalyzer) locateContractViolations(pkg *packages.Package, violations []Violation) ([]Violation, error) {
	for index := range violations {
		position, ok := contractDeclarationPosition(pkg, violations[index].Target)
		if !ok {
			return nil, fmt.Errorf("resolve semantic location for %s: declaration %q is unavailable", violations[index].Key(), violations[index].Target)
		}
		location, err := a.nodeLocation(pkg, position, violations[index].Target)
		if err != nil {
			return nil, fmt.Errorf("resolve semantic location for %s: %w", violations[index].Key(), err)
		}
		violations[index].Locations = append(violations[index].Locations, location)
	}
	return violations, nil
}

func contractDeclarationPosition(pkg *packages.Package, target string) (token.Pos, bool) {
	qualifiedPrefix := path.Base(pkg.PkgPath) + "."
	pathParts := strings.Split(strings.TrimPrefix(target, qualifiedPrefix), ".")
	if len(pathParts) == 0 || pathParts[0] == target {
		return token.NoPos, false
	}
	object := pkg.Types.Scope().Lookup(pathParts[0])
	if object == nil || object.Pos() == token.NoPos {
		return token.NoPos, false
	}
	position := object.Pos()
	current := []types.Type{object.Type()}
	for _, name := range pathParts[1:] {
		var members []types.Object
		var next []types.Type
		for _, candidate := range current {
			candidates := []types.Type{candidate}
			if signature, ok := candidate.(*types.Signature); ok {
				candidates = contractResultTypes(signature.Results())
			}
			for _, candidate := range candidates {
				member, _, _ := types.LookupFieldOrMethod(candidate, true, pkg.Types, name)
				if member != nil {
					members = append(members, member)
					next = append(next, member.Type())
				}
			}
		}
		if len(members) == 0 {
			return position, true
		}
		sort.Slice(members, func(i, j int) bool { return members[i].Pos() < members[j].Pos() })
		member := members[0]
		if member.Pkg() != nil && member.Pkg().Path() == pkg.PkgPath && member.Pos() != token.NoPos {
			position = member.Pos()
		}
		current = next
	}
	return position, true
}

func contractResultTypes(results *types.Tuple) []types.Type {
	resultTypes := make(map[types.Type]struct{})
	walkTuple(results, make(map[types.Type]struct{}), func(current types.Type) {
		switch current.(type) {
		case *types.Named, *types.Interface:
			resultTypes[current] = struct{}{}
		}
	})
	types := make([]types.Type, 0, len(resultTypes))
	for result := range resultTypes {
		types = append(types, result)
	}
	sort.Slice(types, func(i, j int) bool { return types[i].String() < types[j].String() })
	return types
}

func validateLoadedLegacySymbols(loaded []*packages.Package, legacy map[string]map[string]struct{}) error {
	packagesByPath := make(map[string]*packages.Package, len(loaded))
	for _, pkg := range loaded {
		packagesByPath[pkg.PkgPath] = pkg
	}
	for packagePath, symbols := range legacy {
		pkg, ok := packagesByPath[packagePath]
		if !ok {
			return fmt.Errorf("legacy composition package %q is not loaded", packagePath)
		}
		for symbol := range symbols {
			if pkg.Types.Scope().Lookup(symbol) == nil {
				return fmt.Errorf("legacy composition symbol %q does not exist", packagePath+"."+symbol)
			}
		}
	}
	return nil
}

func contractViolations(pkg *packages.Package) []Violation {
	var violations []Violation
	names := pkg.Types.Scope().Names()
	sort.Strings(names)
	for _, name := range names {
		if !token.IsExported(name) {
			continue
		}
		object := pkg.Types.Scope().Lookup(name)
		target := path.Base(pkg.PkgPath) + "." + name
		if function, ok := object.(*types.Func); ok {
			violations = append(violations, contractFunctionViolations(pkg.PkgPath, target, function)...)
		} else {
			violations = append(violations, forbiddenTypeViolations(pkg.PkgPath, target, object.Type())...)
		}
		typeName, ok := object.(*types.TypeName)
		if !ok {
			continue
		}
		named, ok := typeName.Type().(*types.Named)
		if !ok {
			continue
		}
		violations = append(violations, contractMethodViolations(pkg.PkgPath, target, named)...)
	}
	violations = append(violations, ormTagViolations(pkg)...)
	return uniqueSortedViolations(violations)
}

func contractMethodViolations(source, target string, named *types.Named) []Violation {
	return contractMethodViolationsSeen(source, target, named, make(map[types.Type]struct{}))
}

func contractMethodViolationsSeen(source, target string, named *types.Named, contractStack map[types.Type]struct{}) []Violation {
	if _, exists := contractStack[named]; exists {
		return nil
	}
	contractStack[named] = struct{}{}
	defer delete(contractStack, named)
	var violations []Violation
	seen := make(map[string]struct{})
	sets := []*types.MethodSet{types.NewMethodSet(named), types.NewMethodSet(types.NewPointer(named))}
	if iface, ok := named.Underlying().(*types.Interface); ok {
		for _, method := range contractInterfaceMethods(iface) {
			seen[method.Name()] = struct{}{}
		}
		violations = append(violations, contractInterfaceMethodViolationsSeen(source, target, iface, contractStack)...)
	}
	for _, set := range sets {
		for i := 0; i < set.Len(); i++ {
			method, ok := set.At(i).Obj().(*types.Func)
			if !ok {
				continue
			}
			if method.Pkg() == nil || method.Pkg().Path() != source {
				continue
			}
			if _, exists := seen[method.Name()]; exists {
				continue
			}
			seen[method.Name()] = struct{}{}
			violations = append(violations, contractFunctionViolationsSeen(source, target+"."+method.Name(), method, contractStack)...)
		}
	}
	return violations
}

func contractFunctionViolations(source, target string, function *types.Func) []Violation {
	return contractFunctionViolationsSeen(source, target, function, make(map[types.Type]struct{}))
}

func contractFunctionViolationsSeen(source, target string, function *types.Func, contractStack map[types.Type]struct{}) []Violation {
	violations := forbiddenTypeViolations(source, target, function.Type())
	violations = append(violations, contractResultMethodViolations(source, target, function, contractStack)...)
	if crudMethodNames[function.Name()] {
		violations = append(violations, contractViolation(source, "contracts.generic-crud", target, "public contracts use capability-specific operations, not generic CRUD"))
	}
	return violations
}

func contractResultMethodViolations(source, target string, function *types.Func, contractStack map[types.Type]struct{}) []Violation {
	signature, ok := function.Type().(*types.Signature)
	if !ok {
		return nil
	}
	var violations []Violation
	for _, resultType := range contractResultTypes(signature.Results()) {
		switch typed := resultType.(type) {
		case *types.Named:
			violations = append(violations, contractMethodViolationsSeen(source, target, typed, contractStack)...)
		case *types.Interface:
			violations = append(violations, contractInterfaceMethodViolationsSeen(source, target, typed, contractStack)...)
		}
	}
	return violations
}

func contractInterfaceMethodViolationsSeen(source, target string, iface *types.Interface, contractStack map[types.Type]struct{}) []Violation {
	if _, exists := contractStack[iface]; exists {
		return nil
	}
	contractStack[iface] = struct{}{}
	defer delete(contractStack, iface)
	var violations []Violation
	for _, method := range contractInterfaceMethods(iface) {
		violations = append(violations, contractFunctionViolationsSeen(source, target+"."+method.Name(), method, contractStack)...)
	}
	return violations
}

func contractInterfaceMethods(iface *types.Interface) []*types.Func {
	iface.Complete()
	methods := make([]*types.Func, 0, iface.NumMethods())
	for i := 0; i < iface.NumMethods(); i++ {
		methods = append(methods, iface.Method(i))
	}
	return methods
}

func forbiddenTypeViolations(source, target string, contractType types.Type) []Violation {
	rules := make(map[string]string)
	walkType(contractType, make(map[types.Type]struct{}), func(current types.Type) {
		switch typed := current.(type) {
		case *types.Named:
			object := typed.Obj()
			if object.Pkg() == nil {
				return
			}
			packagePath := object.Pkg().Path()
			switch {
			case packagePath == "database/sql" || strings.HasPrefix(packagePath, "github.com/uptrace/bun"):
				rules["contracts.orm-type"] = "public contract exposes an ORM or SQL type"
			case strings.Contains(packagePath, "/database/repositories"):
				rules["contracts.repository-type"] = "public contract exposes a repository type"
			case strings.Contains(packagePath, "/models/") || strings.HasSuffix(packagePath, "/models") || strings.Contains(packagePath, "/internal/"):
				rules["contracts.internal-model"] = "public contract exposes an internal model type"
			}
		case *types.Map:
			if basic, ok := typed.Key().(*types.Basic); ok && basic.Kind() == types.String && isAny(typed.Elem()) {
				rules["contracts.filter-map"] = "public contract exposes map[string]any instead of a typed query"
			}
		}
	})
	violations := make([]Violation, 0, len(rules))
	for rule, detail := range rules {
		violations = append(violations, contractViolation(source, rule, target, detail))
	}
	return violations
}

func contractViolation(source, rule, target, detail string) Violation {
	return Violation{Scope: ScopeProduction, Rule: rule, Source: source, Target: target, Detail: detail}
}

func walkType(current types.Type, seen map[types.Type]struct{}, visit func(types.Type)) {
	if current == nil {
		return
	}
	if _, exists := seen[current]; exists {
		return
	}
	seen[current] = struct{}{}
	visit(current)
	walkTypeChildren(current, seen, visit)
}

func walkTypeChildren(current types.Type, seen map[types.Type]struct{}, visit func(types.Type)) {
	switch typed := current.(type) {
	case *types.Alias:
		walkType(types.Unalias(typed), seen, visit)
	case *types.Array:
		walkType(typed.Elem(), seen, visit)
	case *types.Chan:
		walkType(typed.Elem(), seen, visit)
	case *types.Map:
		walkType(typed.Key(), seen, visit)
		walkType(typed.Elem(), seen, visit)
	case *types.Named:
		walkNamedType(typed, seen, visit)
	case *types.Pointer:
		walkType(typed.Elem(), seen, visit)
	case *types.Signature:
		walkSignatureType(typed, seen, visit)
	case *types.Slice:
		walkType(typed.Elem(), seen, visit)
	case *types.Struct:
		walkStructType(typed, seen, visit)
	case *types.Interface:
		walkInterfaceType(typed, seen, visit)
	case *types.TypeParam:
		walkType(typed.Constraint(), seen, visit)
	}
}

func walkNamedType(named *types.Named, seen map[types.Type]struct{}, visit func(types.Type)) {
	for i := 0; i < named.TypeParams().Len(); i++ {
		walkType(named.TypeParams().At(i).Constraint(), seen, visit)
	}
	for i := 0; i < named.TypeArgs().Len(); i++ {
		walkType(named.TypeArgs().At(i), seen, visit)
	}
	walkType(named.Underlying(), seen, visit)
}

func walkSignatureType(signature *types.Signature, seen map[types.Type]struct{}, visit func(types.Type)) {
	for i := 0; i < signature.TypeParams().Len(); i++ {
		walkType(signature.TypeParams().At(i).Constraint(), seen, visit)
	}
	for i := 0; i < signature.RecvTypeParams().Len(); i++ {
		walkType(signature.RecvTypeParams().At(i).Constraint(), seen, visit)
	}
	walkTuple(signature.Params(), seen, visit)
	walkTuple(signature.Results(), seen, visit)
}

func walkStructType(structure *types.Struct, seen map[types.Type]struct{}, visit func(types.Type)) {
	for i := 0; i < structure.NumFields(); i++ {
		if structure.Field(i).Exported() || structure.Field(i).Embedded() {
			walkType(structure.Field(i).Type(), seen, visit)
		}
	}
}

func walkInterfaceType(iface *types.Interface, seen map[types.Type]struct{}, visit func(types.Type)) {
	iface.Complete()
	for i := 0; i < iface.NumMethods(); i++ {
		walkType(iface.Method(i).Type(), seen, visit)
	}
}

func walkTuple(tuple *types.Tuple, seen map[types.Type]struct{}, visit func(types.Type)) {
	if tuple == nil {
		return
	}
	for i := 0; i < tuple.Len(); i++ {
		walkType(tuple.At(i).Type(), seen, visit)
	}
}

func isAny(value types.Type) bool {
	iface, ok := value.Underlying().(*types.Interface)
	return ok && iface.NumMethods() == 0
}

func ormTagViolations(pkg *packages.Package) []Violation {
	var violations []Violation
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			general, ok := decl.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, spec := range general.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				if !typeSpec.Name.IsExported() {
					continue
				}
				structure, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				if hasBunTag(structure) {
					violations = append(violations, contractViolation(pkg.PkgPath, "contracts.orm-tag", path.Base(pkg.PkgPath)+"."+typeSpec.Name.Name, "public contract exposes an ORM struct tag"))
				}
			}
		}
	}
	return violations
}

func hasBunTag(structure *ast.StructType) bool {
	for _, field := range structure.Fields.List {
		if field.Tag != nil && strings.Contains(field.Tag.Value, "bun:") {
			return true
		}
	}
	return false
}
