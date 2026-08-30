package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"hash/fnv"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type calendarClockFinding struct {
	file        string
	function    string
	line        int
	risk        string
	fingerprint string
}

type calendarFunctionScan struct {
	fn                *ast.FuncDecl
	file              string
	fset              *token.FileSet
	instantVars       calendarVariables
	dateVars          calendarVariables
	rangeVars         calendarVariables
	instantHelpers    map[string]bool
	weeklyHelpers     map[string]bool
	dateHelpers       map[string]bool
	timePackages      map[string]bool
	timezonePackages  map[string]bool
	assertionPackages map[string]bool
}

type calendarPackageHelpers struct {
	dates      map[string]map[string]bool
	instants   map[string]map[string]bool
	weekly     map[string]map[string]bool
	candidates map[string]map[string][]calendarHelperCandidate
}

type calendarVariables map[*ast.Object]bool

type calendarHelperCandidate struct {
	name             string
	receiver         string
	resultType       string
	fn               *ast.FuncDecl
	fingerprint      string
	timePackages     map[string]bool
	timezonePackages map[string]bool
}

func scanCalendarFixtureClockRisks(root string) ([]calendarClockFinding, error) {
	helpers, err := discoverCalendarPackageHelpers(root)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var findings []calendarClockFinding
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if skipCalendarFixtureDir(entry) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		found, err := scanCalendarFixtureFile(root, path, fset, helpers)
		findings = append(findings, found...)
		return err
	})
	sort.Slice(findings, func(i, j int) bool {
		return findings[i].file < findings[j].file ||
			(findings[i].file == findings[j].file && findings[i].line < findings[j].line)
	})
	if err != nil {
		return nil, err
	}
	return findings, nil
}

func applyCalendarClockLegacyBaseline(findings []calendarClockFinding, baseline map[string]string) ([]calendarClockFinding, error) {
	matched := make(map[string]bool, len(baseline))
	changed := map[string]calendarClockFinding{}
	kept := findings[:0]
	for _, finding := range findings {
		key := finding.file + ":" + finding.function
		if fingerprint, ok := baseline[key]; ok {
			matched[key] = true
			if fingerprint == finding.fingerprint {
				continue
			}
			changed[key] = finding
		}
		kept = append(kept, finding)
	}
	var stale []string
	for key := range baseline {
		if !matched[key] {
			stale = append(stale, key)
		}
	}
	if len(stale) != 0 {
		sort.Strings(stale)
		return nil, fmt.Errorf("legacy calendar fixture clock baseline has no matching finding:\n\t%s", strings.Join(stale, "\n\t"))
	}
	if len(changed) != 0 {
		return nil, fmt.Errorf("legacy calendar fixture functions changed:\n%s", formatChangedCalendarFunctions(changed))
	}
	return kept, nil
}

func applyCalendarClockExceptions(findings []calendarClockFinding, exceptions map[string]string) ([]calendarClockFinding, error) {
	matched := make(map[string]bool, len(exceptions))
	for key, reason := range exceptions {
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("calendar fixture clock exception %q requires a non-empty reason", key)
		}
	}
	kept := findings[:0]
	for _, finding := range findings {
		key := finding.file + ":" + finding.function
		if _, ok := exceptions[key]; ok {
			matched[key] = true
			continue
		}
		kept = append(kept, finding)
	}
	for key := range exceptions {
		if !matched[key] {
			return nil, fmt.Errorf("calendar fixture clock exception %q has no matching finding", key)
		}
	}
	return kept, nil
}

func skipCalendarFixtureDir(entry fs.DirEntry) bool {
	name := entry.Name()
	return name == "vendor" || name == "node_modules" || strings.HasPrefix(name, ".")
}

func scanCalendarFixtureFile(root, path string, fset *token.FileSet, helpers calendarPackageHelpers) ([]calendarClockFinding, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		rel = path
	}
	key := calendarPackageKey(path, file.Name.Name)
	return scanCalendarFixtureFunctions(filepath.ToSlash(rel), fset, file, source, helpers, key)
}

func scanCalendarFixtureFunctions(rel string, fset *token.FileSet, file *ast.File, source []byte, helpers calendarPackageHelpers, key string) ([]calendarClockFinding, error) {
	timePackages, timezonePackages := timeImportNames(file)
	assertionPackages := assertionImportNames(file)
	var findings []calendarClockFinding
	for _, fn := range declaredFunctions(file) {
		if !isTestFunction(fn) {
			continue
		}
		instantVars := currentCalendarInstantVariables(fn.Body, helpers.instants[key], timePackages, timezonePackages, nil)
		dateVars := currentCalendarDateVariables(fn.Body, helpers.dates[key], timezonePackages)
		rangeVars := currentLiveCalendarRangeVariables(fn.Body, dateVars, helpers.dates[key], timezonePackages)
		scan := calendarFunctionScan{
			fn: fn, file: rel, fset: fset,
			instantVars: instantVars, dateVars: dateVars, rangeVars: rangeVars,
			instantHelpers: helpers.instants[key], weeklyHelpers: helpers.weekly[key], dateHelpers: helpers.dates[key],
			timePackages: timePackages, timezonePackages: timezonePackages,
			assertionPackages: assertionPackages,
		}
		functionFindings := scan.findings()
		helperFingerprint := calendarHelperClosureFingerprint(fn, helpers.candidates[key])
		if err := stampCalendarFunctionFingerprint(fset, fn, source, helperFingerprint, functionFindings); err != nil {
			return nil, err
		}
		findings = append(findings, functionFindings...)
	}
	return findings, nil
}

func stampCalendarFunctionFingerprint(fset *token.FileSet, fn *ast.FuncDecl, source []byte, helperFingerprint string, findings []calendarClockFinding) error {
	fingerprint, err := calendarFunctionFingerprint(fset, fn, source)
	if err != nil {
		return err
	}
	for i := range findings {
		findings[i].fingerprint = calendarFingerprint(fingerprint + ":" + helperFingerprint)
	}
	return nil
}

func calendarFunctionFingerprint(fset *token.FileSet, fn *ast.FuncDecl, source []byte) (string, error) {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.End()).Offset
	if start < 0 || end < start || end > len(source) {
		return "", fmt.Errorf("invalid source range for calendar ratchet fingerprint: %s", fn.Name.Name)
	}
	return calendarFingerprint(string(source[start:end])), nil
}

func calendarFingerprint(source string) string {
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(source))
	return fmt.Sprintf("%016x", hash.Sum64())
}

func calendarHelperClosureFingerprint(fn *ast.FuncDecl, candidates map[string][]calendarHelperCandidate) string {
	seen := map[*ast.FuncDecl]bool{}
	var fingerprints []string
	collectCalendarHelperFingerprints(fn, candidates, seen, &fingerprints)
	sort.Strings(fingerprints)
	return calendarFingerprint(strings.Join(fingerprints, "\x00"))
}

func collectCalendarHelperFingerprints(fn *ast.FuncDecl, candidates map[string][]calendarHelperCandidate, seen map[*ast.FuncDecl]bool, fingerprints *[]string) {
	receiverTypes := calendarReceiverTypes(fn)
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		for _, candidate := range candidates[calendarCalledHelper(call, receiverTypes)] {
			if seen[candidate.fn] {
				continue
			}
			seen[candidate.fn] = true
			identity := calendarHelperIdentity(candidate.receiver, candidate.name)
			*fingerprints = append(*fingerprints, identity+":"+candidate.fingerprint)
			collectCalendarHelperFingerprints(candidate.fn, candidates, seen, fingerprints)
		}
		return true
	})
}

func formatChangedCalendarFunctions(findings map[string]calendarClockFinding) string {
	keys := make([]string, 0, len(findings))
	for key := range findings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		finding := findings[key]
		lines = append(lines, fmt.Sprintf("\t%s:%d: %s: %s; remediation: replace the live clock with a fixed calendar fixture (new fingerprint %q)",
			finding.file, finding.line, finding.function, finding.risk, finding.fingerprint))
	}
	return strings.Join(lines, "\n")
}

func declaredFunctions(file *ast.File) []*ast.FuncDecl {
	var functions []*ast.FuncDecl
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Body != nil {
			functions = append(functions, fn)
		}
	}
	return functions
}

func calendarHelperIdentity(receiver, name string) string {
	if receiver == "" {
		return name
	}
	return receiver + "." + name
}

func calendarReceiverName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return ""
	}
	return expressionName(fn.Recv.List[0].Type)
}

func calendarReceiverTypes(fn *ast.FuncDecl) map[*ast.Object]string {
	types := map[*ast.Object]string{}
	recordCalendarFieldTypes(fn.Recv, types)
	recordCalendarFieldTypes(fn.Type.Params, types)
	for changed := true; changed; {
		changed = recordCalendarAssignedTypes(fn.Body, types)
	}
	return types
}

func recordCalendarFieldTypes(fields *ast.FieldList, types map[*ast.Object]string) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.Obj != nil {
				types[name.Obj] = expressionName(field.Type)
			}
		}
	}
}

func recordCalendarAssignedTypes(body *ast.BlockStmt, types map[*ast.Object]string) bool {
	changed := false
	ast.Inspect(body, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range declaration.Lhs {
				id, ok := lhs.(*ast.Ident)
				if ok && id.Obj != nil && i < len(declaration.Rhs) {
					changed = recordCalendarType(id.Obj, calendarExpressionType(declaration.Rhs[i], types), types) || changed
				}
			}
		case *ast.ValueSpec:
			for i, name := range declaration.Names {
				typeName := expressionName(declaration.Type)
				if typeName == "" && i < len(declaration.Values) {
					typeName = calendarExpressionType(declaration.Values[i], types)
				}
				changed = recordCalendarType(name.Obj, typeName, types) || changed
			}
		}
		return true
	})
	return changed
}

func recordCalendarType(object *ast.Object, typeName string, types map[*ast.Object]string) bool {
	if object == nil || typeName == "" || types[object] != "" {
		return false
	}
	types[object] = typeName
	return true
}

func calendarExpressionType(expr ast.Expr, types map[*ast.Object]string) string {
	switch value := expr.(type) {
	case *ast.CompositeLit:
		return expressionName(value.Type)
	case *ast.UnaryExpr:
		return calendarExpressionType(value.X, types)
	case *ast.ParenExpr:
		return calendarExpressionType(value.X, types)
	case *ast.Ident:
		if types != nil && types[value.Obj] != "" {
			return types[value.Obj]
		}
		return calendarObjectType(value.Obj, types, map[*ast.Object]bool{})
	case *ast.CallExpr:
		if resultType := calendarFunctionResultType(value.Fun); resultType != "" {
			return resultType
		}
		return expressionName(value.Fun) + "()"
	default:
		return ""
	}
}

func calendarFunctionResultType(expr ast.Expr) string {
	id, ok := expr.(*ast.Ident)
	if !ok || id.Obj == nil {
		return ""
	}
	fn, ok := id.Obj.Decl.(*ast.FuncDecl)
	if !ok {
		return ""
	}
	return calendarResultType(fn)
}

func calendarResultType(fn *ast.FuncDecl) string {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return ""
	}
	return expressionName(fn.Type.Results.List[0].Type)
}

func calendarObjectType(object *ast.Object, types map[*ast.Object]string, seen map[*ast.Object]bool) string {
	if object == nil || seen[object] {
		return ""
	}
	seen[object] = true
	switch declaration := object.Decl.(type) {
	case *ast.Field:
		return expressionName(declaration.Type)
	case *ast.ValueSpec:
		if typeName := expressionName(declaration.Type); typeName != "" {
			return typeName
		}
		for i, name := range declaration.Names {
			if name.Obj == object && i < len(declaration.Values) {
				return calendarExpressionType(declaration.Values[i], types)
			}
		}
	case *ast.AssignStmt:
		for i, lhs := range declaration.Lhs {
			id, ok := lhs.(*ast.Ident)
			if ok && id.Obj == object && i < len(declaration.Rhs) {
				return calendarExpressionType(declaration.Rhs[i], types)
			}
		}
	}
	return ""
}

func calendarCalledHelper(call *ast.CallExpr, receiverTypes map[*ast.Object]string) string {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		receiver := calendarExpressionType(function.X, receiverTypes)
		return calendarHelperIdentity(receiver, function.Sel.Name)
	default:
		return ""
	}
}

func isTestFunction(fn *ast.FuncDecl) bool {
	if fn.Recv != nil {
		return false
	}
	name := fn.Name.Name
	if strings.HasPrefix(name, "Example") {
		return fieldListArity(fn.Type.Params) == 0 && fieldListArity(fn.Type.Results) == 0
	}
	return (goTestName(name, "Test") || goTestName(name, "Benchmark") || goTestName(name, "Fuzz")) &&
		fieldListArity(fn.Type.Params) == 1 && fieldListArity(fn.Type.Results) == 0
}

func goTestName(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) || len(name) == len(prefix) {
		return false
	}
	first, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(first)
}

func fieldListArity(fields *ast.FieldList) int {
	if fields == nil {
		return 0
	}
	count := 0
	for _, field := range fields.List {
		if len(field.Names) == 0 {
			count++
		} else {
			count += len(field.Names)
		}
	}
	return count
}

func discoverCalendarPackageHelpers(root string) (calendarPackageHelpers, error) {
	result := calendarPackageHelpers{
		dates: map[string]map[string]bool{}, instants: map[string]map[string]bool{}, weekly: map[string]map[string]bool{},
		candidates: map[string]map[string][]calendarHelperCandidate{},
	}
	candidates := map[string][]calendarHelperCandidate{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if skipCalendarFixtureDir(entry) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		return addCalendarHelperCandidates(path, fset, candidates, result)
	})
	if err != nil {
		return calendarPackageHelpers{}, err
	}
	propagateCalendarHelpers(candidates, result)
	for key, packageCandidates := range candidates {
		result.candidates[key] = map[string][]calendarHelperCandidate{}
		for _, candidate := range packageCandidates {
			identity := calendarHelperIdentity(candidate.receiver, candidate.name)
			result.candidates[key][identity] = append(result.candidates[key][identity], candidate)
		}
		addCalendarMethodAliases(packageCandidates, result, key)
	}
	return result, nil
}

func addCalendarHelperCandidates(path string, fset *token.FileSet, candidates map[string][]calendarHelperCandidate, helpers calendarPackageHelpers) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s while finding calendar helpers: %w", path, err)
	}
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return fmt.Errorf("parse %s while finding calendar helpers: %w", path, err)
	}
	key := calendarPackageKey(path, file.Name.Name)
	timePackages, timezonePackages := timeImportNames(file)
	for _, fn := range declaredFunctions(file) {
		if !isTestFunction(fn) {
			fingerprint, err := calendarFunctionFingerprint(fset, fn, source)
			if err != nil {
				return err
			}
			candidates[key] = append(candidates[key], calendarHelperCandidate{
				name: fn.Name.Name, receiver: calendarReceiverName(fn), resultType: calendarResultType(fn),
				fn: fn, fingerprint: fingerprint,
				timePackages: timePackages, timezonePackages: timezonePackages,
			})
		}
	}
	helpers.dates[key] = map[string]bool{}
	helpers.instants[key] = map[string]bool{}
	helpers.weekly[key] = map[string]bool{}
	return nil
}

func addCalendarMethodAliases(candidates []calendarHelperCandidate, helpers calendarPackageHelpers, key string) {
	for _, method := range candidates {
		if method.receiver == "" {
			continue
		}
		identity := calendarHelperIdentity(method.receiver, method.name)
		for _, factory := range candidates {
			if factory.receiver != "" || factory.resultType != method.receiver {
				continue
			}
			alias := calendarHelperIdentity(factory.name+"()", method.name)
			helpers.candidates[key][alias] = append(helpers.candidates[key][alias], method)
			helpers.dates[key][alias] = helpers.dates[key][identity]
			helpers.instants[key][alias] = helpers.instants[key][identity]
			helpers.weekly[key][alias] = helpers.weekly[key][identity]
		}
	}
}

func calendarPackageKey(path, packageName string) string {
	return filepath.Clean(filepath.Dir(path)) + ":" + packageName
}

func propagateCalendarHelpers(candidates map[string][]calendarHelperCandidate, helpers calendarPackageHelpers) {
	for changed := true; changed; {
		changed = false
		for key, packageCandidates := range candidates {
			for _, candidate := range packageCandidates {
				identity := calendarHelperIdentity(candidate.receiver, candidate.name)
				dateVars := currentCalendarDateVariables(candidate.fn.Body, helpers.dates[key], candidate.timezonePackages)
				instantVars := currentCalendarInstantVariables(candidate.fn.Body, helpers.instants[key], candidate.timePackages, candidate.timezonePackages, nil)
				weeklyVars := currentCalendarInstantVariables(candidate.fn.Body, helpers.weekly[key], candidate.timePackages, candidate.timezonePackages, weeklyFixtureInstantField)
				if !helpers.dates[key][identity] && functionReturnsLiveDate(candidate.fn, dateVars, helpers.dates[key], candidate.timezonePackages) {
					helpers.dates[key][identity], changed = true, true
				}
				if !helpers.instants[key][identity] && functionReturnsLiveInstant(candidate.fn, instantVars, helpers.instants[key], candidate.timePackages, candidate.timezonePackages) {
					helpers.instants[key][identity], changed = true, true
				}
				if !helpers.weekly[key][identity] && functionReturnsLiveWeeklyInstant(candidate.fn, weeklyVars, helpers.weekly[key], candidate.timePackages, candidate.timezonePackages) {
					helpers.weekly[key][identity], changed = true, true
				}
			}
		}
	}
}

func functionReturnsLiveDate(fn *ast.FuncDecl, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	return functionReturns(fn, func(expr ast.Expr) bool {
		return expressionContainsTodayDate(expr, dateVars, dateHelpers, timezonePackages)
	})
}

func functionReturnsLiveInstant(fn *ast.FuncDecl, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	return functionReturns(fn, func(expr ast.Expr) bool {
		return expressionUsesCalendarInstant(expr, instantVars, instantHelpers, timePackages, timezonePackages)
	})
}

func functionReturnsLiveWeeklyInstant(fn *ast.FuncDecl, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	return functionReturns(fn, func(expr ast.Expr) bool {
		return expressionUsesWeeklyFixtureInstant(expr, instantVars, instantHelpers, timePackages, timezonePackages)
	})
}

func functionReturns(fn *ast.FuncDecl, predicate func(ast.Expr) bool) bool {
	found := false
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		returned, ok := node.(*ast.ReturnStmt)
		for _, expression := range returnedResults(returned, ok) {
			found = found || predicate(expression)
		}
		return !found
	})
	return found
}

func returnedResults(statement *ast.ReturnStmt, ok bool) []ast.Expr {
	if !ok {
		return nil
	}
	return statement.Results
}

func (scan calendarFunctionScan) findings() []calendarClockFinding {
	findings := findInstantDateConversions(scan.fn, scan.file, scan.fset, scan.instantVars, scan.instantHelpers, scan.timePackages, scan.timezonePackages)
	findings = append(findings, findDirectCalendarBoundaryCalls(scan.fn, scan.file, scan.fset, scan.instantVars, scan.dateVars, scan.instantHelpers, scan.dateHelpers, scan.timePackages, scan.timezonePackages)...)
	findings = append(findings, findLiveCalendarRangeCalls(scan.fn, scan.file, scan.fset, scan.dateVars, scan.rangeVars, scan.dateHelpers, scan.timezonePackages)...)
	findings = append(findings, findLiveCalendarAssertions(scan.fn, scan.file, scan.fset, scan.dateVars, scan.dateHelpers, scan.timezonePackages, scan.assertionPackages)...)
	findings = append(findings, findLiveCalendarComparisons(scan.fn, scan.file, scan.fset, scan.dateVars, scan.dateHelpers, scan.timezonePackages)...)
	if hasWeeklySummaryExpectation(scan.fn.Body) {
		findings = append(findings, findLiveDateShifts(scan.fn, scan.file, scan.fset, scan.dateVars, scan.dateHelpers, scan.timezonePackages)...)
		findings = append(findings, findLiveWeeklyFixtureInstants(scan.fn, scan.file, scan.fset, scan.weeklyHelpers, scan.timePackages, scan.timezonePackages)...)
	}
	return findings
}

func findLiveCalendarRangeCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, rangeVars calendarVariables, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := expressionName(call.Fun)
		liveDateArgs := 0
		liveRangeArg := false
		for _, arg := range call.Args {
			if calendarRangeArgumentUsesLiveDate(arg, dateVars, dateHelpers, timezonePackages) {
				liveDateArgs++
			}
			liveRangeArg = liveRangeArg || expressionUsesCalendarRange(arg, rangeVars) ||
				liveCalendarRangeLiteral(arg, dateVars, dateHelpers, timezonePackages)
		}
		if (calendarRangeCallName(name) && liveDateArgs >= 1) ||
			(calendarRangeConsumerName(name) && (liveRangeArg || liveDateArgs >= 2)) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock defines a calendar range"))
		}
		return true
	})
	return findings
}

func currentLiveCalendarRangeVariables(body *ast.BlockStmt, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) calendarVariables {
	rangeVars := calendarVariables{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					if i >= len(declaration.Rhs) {
						continue
					}
					if id, ok := lhs.(*ast.Ident); ok && id.Obj != nil && !rangeVars[id.Obj] &&
						(liveCalendarRangeLiteral(declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) || expressionUsesCalendarRange(declaration.Rhs[i], rangeVars)) {
						rangeVars[id.Obj], changed = true, true
					}
					selector, selected := lhs.(*ast.SelectorExpr)
					root, named := selectorRootObject(selector, selected)
					if named && !rangeVars[root] && calendarRangeFieldName(selector.Sel.Name) && expressionContainsTodayDate(declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
						rangeVars[root], changed = true, true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && name.Obj != nil && !rangeVars[name.Obj] &&
						(liveCalendarRangeLiteral(declaration.Values[i], dateVars, dateHelpers, timezonePackages) || expressionUsesCalendarRange(declaration.Values[i], rangeVars)) {
						rangeVars[name.Obj], changed = true, true
					}
				}
			}
			return true
		})
	}
	return rangeVars
}

func selectorRootObject(selector *ast.SelectorExpr, ok bool) (*ast.Object, bool) {
	if !ok {
		return nil, false
	}
	root, named := selector.X.(*ast.Ident)
	if !named {
		return nil, false
	}
	return root.Obj, root.Obj != nil
}

func liveCalendarRangeLiteral(expr ast.Expr, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	if pointer, ok := expr.(*ast.UnaryExpr); ok && pointer.Op == token.AND {
		return liveCalendarRangeLiteral(pointer.X, dateVars, dateHelpers, timezonePackages)
	}
	if paren, ok := expr.(*ast.ParenExpr); ok {
		return liveCalendarRangeLiteral(paren.X, dateVars, dateHelpers, timezonePackages)
	}
	literal, ok := expr.(*ast.CompositeLit)
	return ok && calendarRangeTypeName(expressionName(literal.Type)) &&
		expressionContainsTodayDate(literal, dateVars, dateHelpers, timezonePackages)
}

func expressionUsesCalendarRange(expr ast.Expr, rangeVars calendarVariables) bool {
	usesRange := false
	ast.Inspect(expr, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		usesRange = usesRange || (ok && id.Obj != nil && rangeVars[id.Obj])
		return !usesRange
	})
	return usesRange
}

func calendarRangeArgumentUsesLiveDate(expr ast.Expr, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	return expressionContainsTodayDate(expr, dateVars, dateHelpers, timezonePackages)
}

func findLiveCalendarAssertions(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars calendarVariables, dateHelpers, timezonePackages, assertionPackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isAssertionCall(call.Fun, assertionPackages) {
			return true
		}
		for _, arg := range call.Args[1:] {
			if expressionContainsTodayDate(arg, dateVars, dateHelpers, timezonePackages) {
				findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(arg.Pos()).Line, "live calendar date used as an expectation"))
				break
			}
		}
		return true
	})
	return findings
}

func isAssertionCall(expr ast.Expr, assertionPackages map[string]bool) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok || !calendarAssertionMethods[selector.Sel.Name] {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Obj == nil && assertionPackages[pkg.Name]
}

var calendarAssertionMethods = map[string]bool{
	"Contains":       true,
	"ElementsMatch":  true,
	"Equal":          true,
	"EqualValues":    true,
	"Exactly":        true,
	"False":          true,
	"Greater":        true,
	"GreaterOrEqual": true,
	"Less":           true,
	"LessOrEqual":    true,
	"NotContains":    true,
	"NotEqual":       true,
	"True":           true,
}

func findLiveCalendarComparisons(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		comparison, ok := node.(*ast.BinaryExpr)
		if !ok || !isComparisonOperator(comparison.Op) {
			return true
		}
		if expressionContainsTodayDate(comparison, dateVars, dateHelpers, timezonePackages) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(comparison.Pos()).Line, "live calendar date used in a comparison"))
		}
		return true
	})
	return findings
}

func isComparisonOperator(operator token.Token) bool {
	return operator == token.EQL || operator == token.NEQ || operator == token.LSS ||
		operator == token.LEQ || operator == token.GTR || operator == token.GEQ
}

func assertionImportNames(file *ast.File) map[string]bool {
	packages := map[string]bool{}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil || (!strings.HasSuffix(importPath, "/assert") && !strings.HasSuffix(importPath, "/require")) {
			continue
		}
		name := pathpkg.Base(importPath)
		if spec.Name != nil && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		packages[name] = true
	}
	return packages
}

func calendarRangeCallName(name string) bool {
	return strings.Contains(name, "History") || strings.Contains(name, "Range") ||
		strings.Contains(name, "Period") || strings.Contains(name, "Between")
}

func calendarRangeConsumerName(name string) bool {
	return strings.Contains(name, "List") || strings.Contains(name, "Find") || strings.Contains(name, "Get") ||
		strings.Contains(name, "Count") || strings.Contains(name, "Query") || strings.Contains(name, "Search") ||
		calendarRangeCallName(name)
}

func calendarRangeTypeName(name string) bool {
	return strings.Contains(name, "Range") || strings.Contains(name, "Period") || strings.Contains(name, "Window")
}

func calendarRangeFieldName(name string) bool {
	switch name {
	case "From", "To", "Start", "End", "DateFrom", "DateTo", "StartDate", "EndDate":
		return true
	default:
		return false
	}
}

func findInstantDateConversions(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isImportedCall(call.Fun, timezonePackages, "DateFromTime") || len(call.Args) == 0 {
			return true
		}
		if expressionUsesCalendarInstant(call.Args[0], instantVars, instantHelpers, timePackages, timezonePackages) {
			findings = append(findings, calendarClockFinding{
				file: rel, function: fn.Name.Name, line: fset.Position(call.Pos()).Line,
				risk: "live instant converted to a calendar date",
			})
		}
		return true
	})
	return findings
}

func currentCalendarInstantVariables(body *ast.BlockStmt, instantHelpers, timePackages, timezonePackages map[string]bool, fieldFilter func(ast.Expr) bool) calendarVariables {
	instantVars := calendarVariables{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					if i >= len(declaration.Rhs) {
						continue
					}
					id, isID := lhs.(*ast.Ident)
					if isID && markCurrentInstant(id.Obj, declaration.Rhs[i], instantVars, instantHelpers, timePackages, timezonePackages, fieldFilter) {
						changed = true
					}
					selector, selected := lhs.(*ast.SelectorExpr)
					root, rooted := selectorRootObject(selector, selected)
					if rooted && (fieldFilter == nil || fieldFilter(selector.Sel)) && markCurrentInstant(root, declaration.Rhs[i], instantVars, instantHelpers, timePackages, timezonePackages, fieldFilter) {
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && markCurrentInstant(name.Obj, declaration.Values[i], instantVars, instantHelpers, timePackages, timezonePackages, fieldFilter) {
						changed = true
					}
				}
			}
			return true
		})
	}
	return instantVars
}

func markCurrentInstant(object *ast.Object, value ast.Expr, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool, fieldFilter func(ast.Expr) bool) bool {
	usesInstant := expressionUsesCalendarInstant(value, instantVars, instantHelpers, timePackages, timezonePackages)
	if fieldFilter != nil {
		usesInstant = expressionUsesWeeklyFixtureInstant(value, instantVars, instantHelpers, timePackages, timezonePackages)
	}
	if object == nil || instantVars[object] || !usesInstant {
		return false
	}
	instantVars[object] = true
	return true
}

func expressionUsesCalendarInstant(expr ast.Expr, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		id, named := node.(*ast.Ident)
		if named && id.Obj != nil && instantVars[id.Obj] {
			found = true
			return false
		}
		call, called := node.(*ast.CallExpr)
		if called && isLiveInstantSource(call, instantHelpers, timePackages, timezonePackages) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func expressionUsesWeeklyFixtureInstant(expr ast.Expr, instantVars calendarVariables, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		if field, ok := node.(*ast.KeyValueExpr); ok && !weeklyFixtureInstantField(field.Key) {
			return false
		}
		id, named := node.(*ast.Ident)
		if named && id.Obj != nil && instantVars[id.Obj] {
			found = true
		}
		call, called := node.(*ast.CallExpr)
		if called && isLiveInstantSource(call, instantHelpers, timePackages, timezonePackages) {
			found = true
		}
		return !found
	})
	return found
}

func currentCalendarDateVariables(body *ast.BlockStmt, dateHelpers, timezonePackages map[string]bool) calendarVariables {
	dateVars := calendarVariables{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					if i >= len(declaration.Rhs) {
						continue
					}
					id, isID := lhs.(*ast.Ident)
					if isID && markCurrentDate(id.Obj, declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
						changed = true
					}
					selector, selected := lhs.(*ast.SelectorExpr)
					root, rooted := selectorRootObject(selector, selected)
					if rooted && markCurrentDate(root, declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && markCurrentDate(name.Obj, declaration.Values[i], dateVars, dateHelpers, timezonePackages) {
						changed = true
					}
				}
			}
			return true
		})
	}
	return dateVars
}

func markCurrentDate(object *ast.Object, value ast.Expr, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	if object == nil || dateVars[object] || !expressionUsesTodayDate(value, dateVars, dateHelpers, timezonePackages) {
		return false
	}
	dateVars[object] = true
	return true
}

func expressionUsesTodayDate(expr ast.Expr, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		return value.Obj != nil && dateVars[value.Obj]
	case *ast.ParenExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	case *ast.CallExpr:
		if dateHelpers[calendarCalledHelper(value, nil)] {
			return true
		}
		if isImportedCall(value.Fun, timezonePackages, "TodayDate") {
			return true
		}
		if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
			if expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages) {
				return true
			}
		}
		return false
	case *ast.SelectorExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	case *ast.BinaryExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages) ||
			expressionUsesTodayDate(value.Y, dateVars, dateHelpers, timezonePackages)
	case *ast.UnaryExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	case *ast.StarExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	case *ast.IndexExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages) ||
			expressionUsesTodayDate(value.Index, dateVars, dateHelpers, timezonePackages)
	default:
		return false
	}
}

func expressionContainsTodayDate(expr ast.Expr, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) bool {
	found := false
	ast.Inspect(expr, func(node ast.Node) bool {
		expression, ok := node.(ast.Expr)
		if ok && expressionUsesTodayDate(expression, dateVars, dateHelpers, timezonePackages) {
			found = true
			return false
		}
		return !found
	})
	return found
}

func findLiveDateShifts(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars calendarVariables, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "AddDays" && expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live calendar date shifted into a range"))
		}
		return true
	})
	return findings
}

var weeklySummarySelectors = map[string]bool{
	"WeeklySummary":   true,
	"WeeklySummaries": true,
}

func hasWeeklySummaryExpectation(body *ast.BlockStmt) bool {
	found := false
	ast.Inspect(body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && weeklySummarySelectors[selector.Sel.Name] {
			found = true
			return false
		}
		return !found
	})
	return found
}

func findDirectCalendarBoundaryCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars calendarVariables, instantHelpers, dateHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (selector.Sel.Name != "ISOWeek" && selector.Sel.Name != "Weekday") {
			return true
		}
		usesInstant := expressionUsesCalendarInstant(selector.X, instantVars, instantHelpers, timePackages, timezonePackages)
		usesDate := expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages)
		if usesInstant || usesDate {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock feeds a day or ISO-week expectation"))
		}
		return true
	})
	return findings
}

func findLiveWeeklyFixtureInstants(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	assignments := assignedCalendarExpressions(fn.Body)
	sources := map[token.Pos]ast.Expr{}
	for _, root := range weeklySummaryRoots(fn.Body) {
		collectLiveInstantSources(root, assignments, instantHelpers, timePackages, timezonePackages, map[*ast.Object]bool{}, sources)
	}
	positions := make([]token.Pos, 0, len(sources))
	for position := range sources {
		positions = append(positions, position)
	}
	sort.Slice(positions, func(i, j int) bool { return positions[i] < positions[j] })
	var findings []calendarClockFinding
	for _, position := range positions {
		findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(position).Line, "live instant feeds an ISO-week expectation"))
	}
	return findings
}

func assignedCalendarExpressions(body *ast.BlockStmt) map[*ast.Object][]ast.Expr {
	assignments := map[*ast.Object][]ast.Expr{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range declaration.Lhs {
				id, isID := lhs.(*ast.Ident)
				if isID && id.Obj != nil && i < len(declaration.Rhs) {
					assignments[id.Obj] = append(assignments[id.Obj], declaration.Rhs[i])
				}
				selector, selected := lhs.(*ast.SelectorExpr)
				root, rooted := selectorRootObject(selector, selected)
				if rooted && weeklyFixtureInstantField(selector.Sel) && i < len(declaration.Rhs) {
					assignments[root] = append(assignments[root], declaration.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, name := range declaration.Names {
				if name.Obj != nil && i < len(declaration.Values) {
					assignments[name.Obj] = append(assignments[name.Obj], declaration.Values[i])
				}
			}
		}
		return true
	})
	return assignments
}

func weeklySummaryRoots(body *ast.BlockStmt) []ast.Expr {
	var roots []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if ok && weeklySummarySelectors[selector.Sel.Name] {
			roots = append(roots, selector.X)
		}
		return true
	})
	return roots
}

func collectLiveInstantSources(expr ast.Expr, assignments map[*ast.Object][]ast.Expr, instantHelpers, timePackages, timezonePackages map[string]bool, seen map[*ast.Object]bool, sources map[token.Pos]ast.Expr) {
	ast.Inspect(expr, func(node ast.Node) bool {
		if field, ok := node.(*ast.KeyValueExpr); ok && !weeklyFixtureInstantField(field.Key) {
			return false
		}
		call, called := node.(*ast.CallExpr)
		if called && isLiveInstantSource(call, instantHelpers, timePackages, timezonePackages) {
			sources[call.Pos()] = call
			return false
		}
		id, named := node.(*ast.Ident)
		if !named || id.Obj == nil || seen[id.Obj] {
			return true
		}
		values := assignments[id.Obj]
		if len(values) == 0 {
			return true
		}
		seen[id.Obj] = true
		for _, value := range values {
			collectLiveInstantSources(value, assignments, instantHelpers, timePackages, timezonePackages, seen, sources)
		}
		return false
	})
}

func weeklyFixtureInstantField(expr ast.Expr) bool {
	field, ok := expr.(*ast.Ident)
	return ok && (field.Name == "CheckInTime" || field.Name == "CheckOutTime")
}

func isLiveInstantSource(call *ast.CallExpr, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	if isImportedCall(call.Fun, timePackages, "Now") || isImportedCall(call.Fun, timezonePackages, "Now") {
		return true
	}
	return instantHelpers[calendarCalledHelper(call, nil)]
}

func newCalendarClockFinding(file, function string, line int, risk string) calendarClockFinding {
	return calendarClockFinding{file: file, function: function, line: line, risk: risk}
}

func formatCalendarClockFindings(findings []calendarClockFinding) []string {
	formatted := make([]string, 0, len(findings))
	for _, finding := range findings {
		formatted = append(formatted, fmt.Sprintf("%s:%d: %s: %s",
			finding.file, finding.line, finding.function, finding.risk))
	}
	return formatted
}
