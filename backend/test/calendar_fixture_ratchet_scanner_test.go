package test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

type calendarClockFinding struct {
	file     string
	function string
	line     int
	risk     string
}

func scanCalendarFixtureClockRisks(root string, exceptions map[string]string) ([]calendarClockFinding, error) {
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
	return applyCalendarClockExceptions(findings, exceptions)
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
	var findings []calendarClockFinding
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Body == nil || !strings.HasPrefix(fn.Name.Name, "Test") {
			continue
		}
		instantVars := currentInstantVariables(fn.Body, timePackages, timezonePackages)
		dateVars := currentCalendarDateVariables(fn.Body, timezonePackages)
		findings = append(findings, findFunctionCalendarClockRisks(
			fn, filepath.ToSlash(rel), fset, instantVars, dateVars, timePackages, timezonePackages,
		)...)
	}
	return findings, nil
}

func findFunctionCalendarClockRisks(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	findings := findInstantDateConversions(fn, rel, fset, instantVars, timePackages, timezonePackages)
	findings = append(findings, findDirectCalendarBoundaryCalls(fn, rel, fset, instantVars, dateVars, timePackages, timezonePackages)...)
	findings = append(findings, findLiveCalendarRangeCalls(fn, rel, fset, dateVars, timezonePackages)...)
	if hasWeeklySummaryExpectation(fn.Body) {
		findings = append(findings, findLiveDateShifts(fn, rel, fset, dateVars, timezonePackages)...)
		findings = append(findings, findLiveInstants(fn, rel, fset, timePackages, timezonePackages)...)
	}
	return findings
}

func findLiveCalendarRangeCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || !calendarRangeCallName(expressionName(call.Fun)) {
			return true
		}
		liveDateArgs := 0
		for _, arg := range call.Args {
			if expressionUsesTodayDate(arg, dateVars, timezonePackages) {
				liveDateArgs++
			}
		}
		if liveDateArgs >= 2 {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock defines a calendar range"))
		}
		return true
	})
	return findings
}

func calendarRangeCallName(name string) bool {
	return name == "GetHistory"
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

func currentCalendarDateVariables(body *ast.BlockStmt, timezonePackages map[string]bool) map[string]bool {
	dateVars := map[string]bool{}
	for changed := true; changed; {
		changed = false
		ast.Inspect(body, func(node ast.Node) bool {
			switch declaration := node.(type) {
			case *ast.AssignStmt:
				for i, lhs := range declaration.Lhs {
					id, isID := lhs.(*ast.Ident)
					if isID && i < len(declaration.Rhs) && markCurrentDate(id.Name, declaration.Rhs[i], dateVars, timezonePackages) {
						changed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range declaration.Names {
					if i < len(declaration.Values) && markCurrentDate(name.Name, declaration.Values[i], dateVars, timezonePackages) {
						changed = true
					}
				}
			}
			return true
		})
	}
	return dateVars
}

func markCurrentDate(name string, value ast.Expr, dateVars, timezonePackages map[string]bool) bool {
	if dateVars[name] || !expressionUsesTodayDate(value, dateVars, timezonePackages) {
		return false
	}
	dateVars[name] = true
	return true
}

func expressionUsesTodayDate(expr ast.Expr, dateVars, timezonePackages map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		return dateVars[value.Name]
	case *ast.ParenExpr:
		return expressionUsesTodayDate(value.X, dateVars, timezonePackages)
	case *ast.CallExpr:
		if isImportedCall(value.Fun, timezonePackages, "TodayDate") {
			return true
		}
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && expressionUsesTodayDate(selector.X, dateVars, timezonePackages)
	case *ast.SelectorExpr:
		return expressionUsesTodayDate(value.X, dateVars, timezonePackages)
	default:
		return false
	}
}

func findLiveDateShifts(fn *ast.FuncDecl, rel string, fset *token.FileSet, dateVars, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if ok && selector.Sel.Name == "AddDays" && expressionUsesTodayDate(selector.X, dateVars, timezonePackages) {
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

func findDirectCalendarBoundaryCalls(fn *ast.FuncDecl, rel string, fset *token.FileSet, instantVars, dateVars, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
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
		usesDate := expressionUsesTodayDate(selector.X, dateVars, timezonePackages)
		if usesInstant || usesDate {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live clock feeds a day or ISO-week expectation"))
		}
		return true
	})
	return findings
}

func findLiveInstants(fn *ast.FuncDecl, rel string, fset *token.FileSet, timePackages, timezonePackages map[string]bool) []calendarClockFinding {
	var findings []calendarClockFinding
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if ok && (isImportedCall(call.Fun, timePackages, "Now") || isImportedCall(call.Fun, timezonePackages, "Now")) {
			findings = append(findings, newCalendarClockFinding(rel, fn.Name.Name, fset.Position(call.Pos()).Line, "live instant feeds an ISO-week expectation"))
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
