package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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
	instantVars       map[string]bool
	dateVars          map[string]bool
	rangeVars         map[string]bool
	instantHelpers    map[string]bool
	dateHelpers       map[string]bool
	timePackages      map[string]bool
	timezonePackages  map[string]bool
	assertionPackages map[string]bool
}

type calendarPackageHelpers struct {
	dates    map[string]map[string]bool
	instants map[string]map[string]bool
}

type calendarHelperCandidate struct {
	name             string
	fn               *ast.FuncDecl
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
	changed := map[string]string{}
	kept := findings[:0]
	for _, finding := range findings {
		key := finding.file + ":" + finding.function
		if fingerprint, ok := baseline[key]; ok {
			matched[key] = true
			if fingerprint == finding.fingerprint {
				continue
			}
			changed[key] = finding.fingerprint
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
		return nil, fmt.Errorf("legacy calendar fixture functions changed; remove the live-clock dependency instead of refreshing these fingerprints:\n%s", formatCalendarClockFingerprints(changed))
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
	timePackages, timezonePackages := timeImportNames(file)
	assertionPackages := assertionImportNames(file)
	functions := declaredFunctions(file)
	key := calendarPackageKey(path, file.Name.Name)
	dateHelpers := helpers.dates[key]
	instantHelpers := helpers.instants[key]
	var findings []calendarClockFinding
	for _, fn := range functions {
		if !isTestFunction(fn) {
			continue
		}
		instantVars := currentCalendarInstantVariables(fn.Body, instantHelpers, timePackages, timezonePackages)
		dateVars := currentCalendarDateVariables(fn.Body, dateHelpers, timezonePackages)
		rangeVars := currentLiveCalendarRangeVariables(fn.Body, dateVars, dateHelpers, timezonePackages)
		scan := calendarFunctionScan{
			fn: fn, file: filepath.ToSlash(rel), fset: fset,
			instantVars: instantVars, dateVars: dateVars, rangeVars: rangeVars,
			instantHelpers: instantHelpers, dateHelpers: dateHelpers,
			timePackages: timePackages, timezonePackages: timezonePackages,
			assertionPackages: assertionPackages,
		}
		functionFindings := scan.findings()
		fingerprint, err := calendarFunctionFingerprint(fset, fn, source)
		if err != nil {
			return nil, err
		}
		for i := range functionFindings {
			functionFindings[i].fingerprint = fingerprint
		}
		findings = append(findings, functionFindings...)
	}
	return findings, nil
}

func calendarFunctionFingerprint(fset *token.FileSet, fn *ast.FuncDecl, source []byte) (string, error) {
	start := fset.Position(fn.Pos()).Offset
	end := fset.Position(fn.End()).Offset
	if start < 0 || end < start || end > len(source) {
		return "", fmt.Errorf("invalid source range for calendar ratchet fingerprint: %s", fn.Name.Name)
	}
	const fnvOffset64 = uint64(14695981039346656037)
	const fnvPrime64 = uint64(1099511628211)
	hash := fnvOffset64
	for _, value := range source[start:end] {
		hash ^= uint64(value)
		hash *= fnvPrime64
	}
	return fmt.Sprintf("%016x", hash), nil
}

func formatCalendarClockFingerprints(fingerprints map[string]string) string {
	keys := make([]string, 0, len(fingerprints))
	for key := range fingerprints {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var lines []string
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("\t%q: %q,", key, fingerprints[key]))
	}
	return strings.Join(lines, "\n")
}

func declaredFunctions(file *ast.File) map[string]*ast.FuncDecl {
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if ok && fn.Body != nil {
			functions[fn.Name.Name] = fn
		}
	}
	return functions
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
	result := calendarPackageHelpers{dates: map[string]map[string]bool{}, instants: map[string]map[string]bool{}}
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
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return fmt.Errorf("parse %s while finding calendar helpers: %w", path, err)
		}
		key := calendarPackageKey(path, file.Name.Name)
		timePackages, timezonePackages := timeImportNames(file)
		for name, fn := range declaredFunctions(file) {
			if !isTestFunction(fn) {
				candidates[key] = append(candidates[key], calendarHelperCandidate{
					name: name, fn: fn, timePackages: timePackages, timezonePackages: timezonePackages,
				})
			}
		}
		result.dates[key] = map[string]bool{}
		result.instants[key] = map[string]bool{}
		return nil
	})
	if err != nil {
		return calendarPackageHelpers{}, err
	}
	propagateCalendarHelpers(candidates, result)
	return result, nil
}

func calendarPackageKey(path, packageName string) string {
	return filepath.Clean(filepath.Dir(path)) + ":" + packageName
}

func propagateCalendarHelpers(candidates map[string][]calendarHelperCandidate, helpers calendarPackageHelpers) {
	for changed := true; changed; {
		changed = false
		for key, packageCandidates := range candidates {
			for _, candidate := range packageCandidates {
				dateVars := currentCalendarDateVariables(candidate.fn.Body, helpers.dates[key], candidate.timezonePackages)
				instantVars := currentCalendarInstantVariables(candidate.fn.Body, helpers.instants[key], candidate.timePackages, candidate.timezonePackages)
				if !helpers.dates[key][candidate.name] && functionReturnsLiveDate(candidate.fn, dateVars, helpers.dates[key], candidate.timezonePackages) {
					helpers.dates[key][candidate.name], changed = true, true
				}
				if !helpers.instants[key][candidate.name] && functionReturnsLiveInstant(candidate.fn, instantVars, helpers.instants[key], candidate.timePackages, candidate.timezonePackages) {
					helpers.instants[key][candidate.name], changed = true, true
				}
			}
		}
	}
}

func functionReturnsLiveDate(fn *ast.FuncDecl, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
	return functionReturns(fn, func(expr ast.Expr) bool {
		return expressionContainsTodayDate(expr, dateVars, dateHelpers, timezonePackages)
	})
}

func functionReturnsLiveInstant(fn *ast.FuncDecl, instantVars, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	return functionReturns(fn, func(expr ast.Expr) bool {
		return expressionUsesCalendarInstant(expr, instantVars, instantHelpers, timePackages, timezonePackages)
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
		findings = append(findings, findLiveWeeklyFixtureInstants(scan.fn, scan.file, scan.fset, scan.instantHelpers, scan.timePackages, scan.timezonePackages)...)
	}
	return findings
}

func findLiveCalendarRangeCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, rangeVars, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
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
			liveRangeArg = liveRangeArg || expressionUsesCalendarRange(arg, rangeVars)
		}
		if (calendarRangeCallName(name) && liveDateArgs >= 1) ||
			(calendarRangeConsumerName(name) && (liveRangeArg || liveDateArgs >= 2)) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock defines a calendar range"))
		}
		return true
	})
	return findings
}

func currentLiveCalendarRangeVariables(body *ast.BlockStmt, dateVars, dateHelpers, timezonePackages map[string]bool) map[string]bool {
	rangeVars := map[string]bool{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range declaration.Lhs {
				if i >= len(declaration.Rhs) {
					continue
				}
				if id, ok := lhs.(*ast.Ident); ok && liveCalendarRangeLiteral(declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
					rangeVars[id.Name] = true
				}
				selector, selected := lhs.(*ast.SelectorExpr)
				root, named := selectorRootName(selector, selected)
				if named && calendarRangeFieldName(selector.Sel.Name) && expressionContainsTodayDate(declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
					rangeVars[root] = true
				}
			}
		case *ast.ValueSpec:
			for i, name := range declaration.Names {
				if i < len(declaration.Values) && liveCalendarRangeLiteral(declaration.Values[i], dateVars, dateHelpers, timezonePackages) {
					rangeVars[name.Name] = true
				}
			}
		}
		return true
	})
	return rangeVars
}

func selectorRootName(selector *ast.SelectorExpr, ok bool) (string, bool) {
	if !ok {
		return "", false
	}
	root, named := selector.X.(*ast.Ident)
	if !named {
		return "", false
	}
	return root.Name, true
}

func liveCalendarRangeLiteral(expr ast.Expr, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
	literal, ok := expr.(*ast.CompositeLit)
	return ok && calendarRangeTypeName(expressionName(literal.Type)) &&
		expressionContainsTodayDate(literal, dateVars, dateHelpers, timezonePackages)
}

func expressionUsesCalendarRange(expr ast.Expr, rangeVars map[string]bool) bool {
	usesRange := false
	ast.Inspect(expr, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		usesRange = usesRange || (ok && rangeVars[id.Name])
		return !usesRange
	})
	return usesRange
}

func calendarRangeArgumentUsesLiveDate(expr ast.Expr, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
	return expressionContainsTodayDate(expr, dateVars, dateHelpers, timezonePackages)
}

func findLiveCalendarAssertions(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages, assertionPackages map[string]bool) []calendarClockFinding {
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

func findLiveCalendarComparisons(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
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

func findInstantDateConversions(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, instantHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
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

func currentCalendarInstantVariables(body *ast.BlockStmt, instantHelpers, timePackages, timezonePackages map[string]bool) map[string]bool {
	instantVars := map[string]bool{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					id, isID := lhs.(*ast.Ident)
					if isID && i < len(declaration.Rhs) && markCurrentInstant(id.Name, declaration.Rhs[i], instantVars, instantHelpers, timePackages, timezonePackages) {
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && markCurrentInstant(name.Name, declaration.Values[i], instantVars, instantHelpers, timePackages, timezonePackages) {
						changed = true
					}
				}
			}
			return true
		})
	}
	return instantVars
}

func markCurrentInstant(name string, value ast.Expr, instantVars, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	if instantVars[name] || !expressionUsesCalendarInstant(value, instantVars, instantHelpers, timePackages, timezonePackages) {
		return false
	}
	instantVars[name] = true
	return true
}

func expressionUsesCalendarInstant(expr ast.Expr, instantVars, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	call, ok := expr.(*ast.CallExpr)
	if ok {
		if id, named := call.Fun.(*ast.Ident); named && instantHelpers[id.Name] {
			return true
		}
		if selector, selected := call.Fun.(*ast.SelectorExpr); selected && instantHelpers[selector.Sel.Name] {
			return true
		}
	}
	return expressionUsesCurrentInstant(expr, instantVars, timePackages, timezonePackages)
}

func currentCalendarDateVariables(body *ast.BlockStmt, dateHelpers, timezonePackages map[string]bool) map[string]bool {
	dateVars := map[string]bool{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					id, isID := lhs.(*ast.Ident)
					if isID && i < len(declaration.Rhs) && markCurrentDate(id.Name, declaration.Rhs[i], dateVars, dateHelpers, timezonePackages) {
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && markCurrentDate(name.Name, declaration.Values[i], dateVars, dateHelpers, timezonePackages) {
						changed = true
					}
				}
			}
			return true
		})
	}
	return dateVars
}

func markCurrentDate(name string, value ast.Expr, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
	if dateVars[name] || !expressionUsesTodayDate(value, dateVars, dateHelpers, timezonePackages) {
		return false
	}
	dateVars[name] = true
	return true
}

func expressionUsesTodayDate(expr ast.Expr, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		return dateVars[value.Name]
	case *ast.ParenExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	case *ast.CallExpr:
		if id, ok := value.Fun.(*ast.Ident); ok && dateHelpers[id.Name] {
			return true
		}
		if isImportedCall(value.Fun, timezonePackages, "TodayDate") {
			return true
		}
		if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
			if dateHelpers[selector.Sel.Name] || expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages) {
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

func expressionContainsTodayDate(expr ast.Expr, dateVars, dateHelpers, timezonePackages map[string]bool) bool {
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

func findLiveDateShifts(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
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

func findDirectCalendarBoundaryCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars, instantHelpers, dateHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
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
		collectLiveInstantSources(root, assignments, instantHelpers, timePackages, timezonePackages, map[string]bool{}, sources)
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

func assignedCalendarExpressions(body *ast.BlockStmt) map[string][]ast.Expr {
	assignments := map[string][]ast.Expr{}
	ast.Inspect(body, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.AssignStmt:
			for i, lhs := range declaration.Lhs {
				id, isID := lhs.(*ast.Ident)
				if isID && i < len(declaration.Rhs) {
					assignments[id.Name] = append(assignments[id.Name], declaration.Rhs[i])
				}
			}
		case *ast.ValueSpec:
			for i, name := range declaration.Names {
				if i < len(declaration.Values) {
					assignments[name.Name] = append(assignments[name.Name], declaration.Values[i])
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

func collectLiveInstantSources(expr ast.Expr, assignments map[string][]ast.Expr, instantHelpers, timePackages, timezonePackages map[string]bool, seen map[string]bool, sources map[token.Pos]ast.Expr) {
	ast.Inspect(expr, func(node ast.Node) bool {
		call, called := node.(*ast.CallExpr)
		if called && isLiveInstantSource(call, instantHelpers, timePackages, timezonePackages) {
			sources[call.Pos()] = call
			return false
		}
		id, named := node.(*ast.Ident)
		if !named || seen[id.Name] {
			return true
		}
		values := assignments[id.Name]
		if len(values) == 0 {
			return true
		}
		seen[id.Name] = true
		for _, value := range values {
			collectLiveInstantSources(value, assignments, instantHelpers, timePackages, timezonePackages, seen, sources)
		}
		return false
	})
}

func isLiveInstantSource(call *ast.CallExpr, instantHelpers, timePackages, timezonePackages map[string]bool) bool {
	if isImportedCall(call.Fun, timePackages, "Now") || isImportedCall(call.Fun, timezonePackages, "Now") {
		return true
	}
	if id, ok := call.Fun.(*ast.Ident); ok {
		return instantHelpers[id.Name]
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && instantHelpers[selector.Sel.Name]
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
