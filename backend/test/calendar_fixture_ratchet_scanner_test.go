package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type calendarClockFinding struct {
	file     string
	function string
	line     int
	risk     string
}

func scanCalendarFixtureClockRisks(root string) ([]calendarClockFinding, error) {
	fset := token.NewFileSet()
	var findings []calendarClockFinding
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
		found, err := scanCalendarFixtureFile(root, path, fset)
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
	kept := findings[:0]
	for _, finding := range findings {
		key := finding.file + ":" + finding.function
		if _, ok := baseline[key]; ok {
			matched[key] = true
			continue
		}
		kept = append(kept, finding)
	}
	for key := range baseline {
		if !matched[key] {
			return nil, fmt.Errorf("legacy calendar fixture clock baseline %q has no matching finding", key)
		}
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

func scanCalendarFixtureFile(root, path string, fset *token.FileSet) ([]calendarClockFinding, error) {
	file, err := parser.ParseFile(fset, path, nil, 0)
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
	dateHelpers := liveDateHelpers(functions, timezonePackages)
	var findings []calendarClockFinding
	for _, fn := range functions {
		if !isTestFunction(fn) {
			continue
		}
		instantVars := currentInstantVariables(fn.Body, timePackages, timezonePackages)
		dateVars := currentCalendarDateVariables(fn.Body, dateHelpers, timezonePackages)
		findings = append(findings, findFunctionCalendarClockRisks(
			fn, filepath.ToSlash(rel), fset, instantVars, dateVars, dateHelpers, timePackages, timezonePackages, assertionPackages,
		)...)
	}
	return findings, nil
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
	return strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") ||
		strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Example")
}

func liveDateHelpers(functions map[string]*ast.FuncDecl, timezonePackages map[string]bool) map[string]bool {
	helpers := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, fn := range functions {
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				returned, ok := node.(*ast.ReturnStmt)
				for _, value := range returnedResults(returned, ok) {
					if !helpers[name] && expressionUsesTodayDate(value, nil, helpers, timezonePackages) {
						helpers[name], changed = true, true
					}
				}
				return true
			})
		}
	}
	return helpers
}

func returnedResults(statement *ast.ReturnStmt, ok bool) []ast.Expr {
	if !ok {
		return nil
	}
	return statement.Results
}

func findFunctionCalendarClockRisks(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars, dateHelpers, timePackages, timezonePackages, assertionPackages map[string]bool) []calendarClockFinding {
	findings := findInstantDateConversions(fn, rel, fset, instantVars, timePackages, timezonePackages)
	findings = append(findings, findDirectCalendarBoundaryCalls(fn, rel, fset, instantVars, dateVars, dateHelpers, timePackages, timezonePackages)...)
	findings = append(findings, findLiveCalendarRangeCalls(fn, rel, fset, dateVars, dateHelpers, timezonePackages)...)
	findings = append(findings, findLiveCalendarRangeLiterals(fn, rel, fset, dateVars, dateHelpers, timezonePackages)...)
	findings = append(findings, findLiveCalendarAssertions(fn, rel, fset, dateVars, dateHelpers, timezonePackages, assertionPackages)...)
	if hasWeeklySummaryExpectation(fn.Body) {
		findings = append(findings, findLiveDateShifts(fn, rel, fset, dateVars, dateHelpers, timezonePackages)...)
		findings = append(findings, findLiveWeeklyFixtureInstants(fn, rel, fset, instantVars, timePackages, timezonePackages)...)
	}
	return findings
}

func findLiveCalendarRangeCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !calendarRangeCallName(expressionName(call.Fun)) {
			return true
		}
		liveDateArgs := 0
		for _, arg := range call.Args {
			if expressionUsesTodayDate(arg, dateVars, dateHelpers, timezonePackages) {
				liveDateArgs++
			}
		}
		if liveDateArgs >= 1 {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock defines a calendar range"))
		}
		return true
	})
	return findings
}

func findLiveCalendarRangeLiterals(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok || !calendarRangeTypeName(expressionName(literal.Type)) {
			return true
		}
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if isCalendarRangeField(field, ok) && expressionUsesTodayDate(field.Value, dateVars, dateHelpers, timezonePackages) {
				findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(field.Value.Pos()).Line, "live clock defines a calendar range"))
				break
			}
		}
		return true
	})
	return findings
}

func isCalendarRangeField(field *ast.KeyValueExpr, ok bool) bool {
	if !ok {
		return false
	}
	name, named := field.Key.(*ast.Ident)
	return named && calendarRangeFieldName(name.Name)
}

func findLiveCalendarAssertions(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, dateHelpers, timezonePackages, assertionPackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isAssertionCall(call.Fun, assertionPackages) {
			return true
		}
		for _, arg := range call.Args[1:] {
			if expressionUsesTodayDate(arg, dateVars, dateHelpers, timezonePackages) {
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
	if !ok || (selector.Sel.Name != "Equal" && selector.Sel.Name != "EqualValues" && selector.Sel.Name != "NotEqual") {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Obj == nil && assertionPackages[pkg.Name]
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

func findInstantDateConversions(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !isImportedCall(call.Fun, timezonePackages, "DateFromTime") || len(call.Args) == 0 {
			return true
		}
		if expressionUsesCurrentInstant(call.Args[0], instantVars, timePackages, timezonePackages) {
			findings = append(findings, calendarClockFinding{
				file: rel, function: fn.Name.Name, line: fset.Position(call.Pos()).Line,
				risk: "live instant converted to a calendar date",
			})
		}
		return true
	})
	return findings
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
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages)
	case *ast.SelectorExpr:
		return expressionUsesTodayDate(value.X, dateVars, dateHelpers, timezonePackages)
	default:
		return false
	}
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

func findDirectCalendarBoundaryCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars, dateHelpers, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
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
		usesInstant := expressionUsesCurrentInstant(selector.X, instantVars, timePackages, timezonePackages)
		usesDate := expressionUsesTodayDate(selector.X, dateVars, dateHelpers, timezonePackages)
		if usesInstant || usesDate {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock feeds a day or ISO-week expectation"))
		}
		return true
	})
	return findings
}

var weeklyInstantFixtureFields = map[string]bool{
	"CheckInTime":  true,
	"CheckOutTime": true,
	"StartTime":    true,
	"EndTime":      true,
}

func findLiveWeeklyFixtureInstants(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		field, ok := node.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		name, ok := field.Key.(*ast.Ident)
		if ok && weeklyInstantFixtureFields[name.Name] && expressionUsesCurrentInstant(field.Value, instantVars, timePackages, timezonePackages) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(field.Value.Pos()).Line, "live instant feeds an ISO-week expectation"))
		}
		return true
	})
	return findings
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
