package common_test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	backendModulePath = "github.com/moto-nrw/project-phoenix"
	commonPackagePath = backendModulePath + "/api/common"
	// Observability functions take a code as a metric label, never as the
	// code of a response, so their parameters are not checked.
	observabilityPackagePath = backendModulePath + "/observability"
)

var errorCodeShape = regexp.MustCompile(`^[a-z][a-z0-9_]*\.[a-z][a-z0-9_]*$`)

type errorCodeRegistry struct {
	codes  map[string]bool
	areas  map[string]bool
	legacy map[string]string
}

type sourceFile struct {
	syntax  *ast.File
	imports map[string]string // local name -> import path
}

type sourcePackage struct {
	path   string
	name   string
	files  []*sourceFile
	consts map[string]constDefinition
}

type constDefinition struct {
	value ast.Expr
	file  *sourceFile
}

type sourceTree struct {
	fset     *token.FileSet
	packages map[string]*sourcePackage
	// codeParameters holds, per package-level function ("path.Name"), the
	// positions of its parameters named code or fallbackCode.
	codeParameters map[string][]int
	// functions holds every package-level function by "path.Name".
	functions map[string]functionDefinition
}

type functionDefinition struct {
	decl *ast.FuncDecl
	pkg  *sourcePackage
	file *sourceFile
}

// TestErrorPathsAnswerWithRegisteredCodes fails when an error path answers
// with a code that is not in error-registry.json, or names its code as a
// string at the call site instead of using the generated constant (ADR 0006,
// #2506).
//
// A code reaches the wire through a function parameter named code or
// fallbackCode (a common helper or a package's own wrapper around one), the Code field of common.ErrResponse, the ErrorCode method of
// a module's business rejection, or a package's own code constant. Packages
// that may not import api/common declare their code once as a named constant
// holding the registered value; call sites name that constant. The check is
// syntactic: it follows named constants across packages, and a code that is
// only known at run time is outside it.
func TestErrorPathsAnswerWithRegisteredCodes(t *testing.T) {
	t.Parallel()

	violations := scanErrorCodes(t, backendRoot(t), loadErrorCodeRegistry(t))
	if len(violations) > 0 {
		t.Errorf("%d error code violation(s). Register the code in error-registry.json, run "+
			"node scripts/generate-error-codes.mjs and use the generated common.Code* constant:\n%s",
			len(violations), strings.Join(violations, "\n"))
	}
}

// TestErrorCodeScanFindsEachViolation runs the scan over a fixture tree with
// one violation of each kind, so a scan that silently finds nothing fails.
func TestErrorCodeScanFindsEachViolation(t *testing.T) {
	t.Parallel()

	root := filepath.Join("testdata", "errorcodes")
	violations := scanErrorCodes(t, root, loadErrorCodeRegistry(t))
	for index, violation := range violations {
		violations[index] = filepath.ToSlash(strings.TrimPrefix(violation, root+string(filepath.Separator)))
	}
	want := []string{
		`api/bad/bad.go:10:19: constant localCode repeats the registered code "care.announcement_stale"; use the generated constant`,
		`api/bad/bad.go:14:49: code argument is the string "announcement_stale"; use the generated constant`,
		`api/bad/bad.go:16:29: ErrResponse.Code is the string "care.announcement_stale"; use the generated constant`,
		`api/bad/bad.go:17:49: code argument is "care.not_registered", which is not in error-registry.json`,
		`api/bad/bad.go:18:26: the registered code "care.announcement_stale" is written as a string; use the generated constant`,
		`api/bad/bad.go:27:28: code argument is the string "announcement_stale"; use the generated constant`,
		`api/bad/bad.go:29:64: code argument can be "conflict", which is not in error-registry.json`,
		`modules/sample/sample.go:12:46: ErrorCode result is "care.not_registered", which is not in error-registry.json`,
		`modules/sample/sample.go:15:21: "care.not_registered" looks like an error code but is not in error-registry.json`,
		`modules/sample/sample.go:4:17: "announcement_stale" is a code from before the rename; use "care.announcement_stale"`,
	}
	sort.Strings(want)
	if strings.Join(violations, "\n") != strings.Join(want, "\n") {
		t.Errorf("scan found:\n%s\n\nwant:\n%s", strings.Join(violations, "\n"), strings.Join(want, "\n"))
	}
}

func scanErrorCodes(t *testing.T, root string, registry errorCodeRegistry) []string {
	t.Helper()
	tree := loadSourceTree(t, root)
	violationAt := map[string]string{}
	report := func(node ast.Node, format string, args ...any) {
		position := tree.fset.Position(node.Pos()).String()
		if _, seen := violationAt[position]; !seen {
			violationAt[position] = position + ": " + fmt.Sprintf(format, args...)
		}
	}
	for _, pkg := range tree.packages {
		for _, file := range pkg.files {
			if strings.HasSuffix(tree.fset.Position(file.syntax.Pos()).Filename, "error_codes.generated.go") {
				continue
			}
			tree.checkFile(pkg, file, registry, report)
		}
	}
	violations := make([]string, 0, len(violationAt))
	for _, violation := range violationAt {
		violations = append(violations, violation)
	}
	sort.Strings(violations)
	return violations
}

func (tree *sourceTree) checkFile(pkg *sourcePackage, file *sourceFile, registry errorCodeRegistry, report func(ast.Node, string, ...any)) {
	inCommon := pkg.path == commonPackagePath
	importsCommon := inCommon
	for _, importPath := range file.imports {
		importsCommon = importsCommon || importPath == commonPackagePath
	}
	checkCodeValue := func(expr ast.Expr, where string) {
		expr = ast.Unparen(expr)
		if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			report(expr, "%s is the string %s; use the generated constant", where, lit.Value)
			return
		}
		if value, ok := tree.resolve(pkg, file, expr, 0); ok && !registry.codes[value] {
			report(expr, "%s is %q, which is not in error-registry.json", where, value)
		}
		for _, value := range tree.returnedStrings(pkg, file, expr) {
			if !registry.codes[value] {
				report(expr, "%s can be %q, which is not in error-registry.json", where, value)
			}
		}
	}

	ast.Inspect(file.syntax, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if function, ok := tree.functionKey(pkg, file, n.Fun); ok {
				for _, index := range tree.codeParameters[function] {
					if index < len(n.Args) {
						checkCodeValue(n.Args[index], "code argument")
					}
				}
			}
		case *ast.CompositeLit:
			name, ok := tree.commonName(file, inCommon, n.Type)
			isResponse := ok && name == "ErrResponse"
			for _, element := range n.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok || !strings.EqualFold(identName(field.Key), "code") {
					continue
				}
				if isResponse {
					checkCodeValue(field.Value, "ErrResponse.Code")
				} else if value, ok := tree.resolve(pkg, file, field.Value, 0); ok {
					checkDeclaredCode(field.Value, value, registry, false, report)
				}
			}
		case *ast.FuncDecl:
			if n.Recv != nil && n.Name.Name == "ErrorCode" && n.Body != nil {
				ast.Inspect(n.Body, func(inner ast.Node) bool {
					if ret, ok := inner.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
						checkCodeValue(ret.Results[0], "ErrorCode result")
					}
					return true
				})
			}
		case *ast.GenDecl:
			if n.Tok != token.CONST {
				return true
			}
			for _, spec := range n.Specs {
				valueSpec := spec.(*ast.ValueSpec)
				for index, value := range valueSpec.Values {
					literal, ok := stringLiteral(value)
					if !ok || index >= len(valueSpec.Names) {
						continue
					}
					name := valueSpec.Names[index].Name
					if registry.codes[literal] && importsCommon {
						report(value, "constant %s repeats the registered code %q; use the generated constant", name, literal)
					} else if strings.Contains(strings.ToLower(name), "code") {
						checkDeclaredCode(value, literal, registry, true, report)
					}
				}
			}
			return false
		case *ast.BasicLit:
			if value, ok := stringLiteral(n); ok && registry.codes[value] {
				report(n, "the registered code %q is written as a string; use the generated constant", value)
			}
		}
		return true
	})
}

// checkDeclaredCode checks a code declared outside the generated constants: a
// new-looking name that was never registered, and for a named code constant
// also an old name left behind. A Code field is not checked for old names:
// words like not_found also serve as import-row and failure-kind codes.
func checkDeclaredCode(node ast.Node, value string, registry errorCodeRegistry, checkLegacy bool, report func(ast.Node, string, ...any)) {
	if renamed, ok := registry.legacy[value]; ok && checkLegacy {
		report(node, "%q is a code from before the rename; use %q", value, renamed)
		return
	}
	area, _, _ := strings.Cut(value, ".")
	if errorCodeShape.MatchString(value) && registry.areas[area] && !registry.codes[value] {
		report(node, "%q looks like an error code but is not in error-registry.json", value)
	}
}

// functionKey names the package-level function a call targets as
// "path.Name". Method calls are not resolved.
func (tree *sourceTree) functionKey(pkg *sourcePackage, file *sourceFile, fun ast.Expr) (string, bool) {
	switch e := ast.Unparen(fun).(type) {
	case *ast.Ident:
		return pkg.path + "." + e.Name, true
	case *ast.SelectorExpr:
		if alias, ok := e.X.(*ast.Ident); ok {
			if importPath, ok := file.imports[alias.Name]; ok {
				return importPath + "." + e.Sel.Name, true
			}
		}
	}
	return "", false
}

// commonName returns the name of an api/common identifier referenced by
// expr: common.Name from outside the package, Name from inside it.
func (tree *sourceTree) commonName(file *sourceFile, inCommon bool, expr ast.Expr) (string, bool) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return e.Name, inCommon
	case *ast.SelectorExpr:
		if alias, ok := e.X.(*ast.Ident); ok && file.imports[alias.Name] == commonPackagePath {
			return e.Sel.Name, true
		}
	}
	return "", false
}

// returnedStrings lists the string values a call to a package-level function
// can return, when expr is such a call. Results the scan cannot resolve are
// left out.
func (tree *sourceTree) returnedStrings(pkg *sourcePackage, file *sourceFile, expr ast.Expr) []string {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil
	}
	key, ok := tree.functionKey(pkg, file, call.Fun)
	if !ok {
		return nil
	}
	function, ok := tree.functions[key]
	if !ok || function.decl.Body == nil {
		return nil
	}
	var values []string
	ast.Inspect(function.decl.Body, func(node ast.Node) bool {
		if _, nested := node.(*ast.FuncLit); nested {
			return false
		}
		if ret, ok := node.(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
			if value, ok := tree.resolve(function.pkg, function.file, ret.Results[0], 0); ok {
				values = append(values, value)
			}
		}
		return true
	})
	return values
}

// resolve returns the string value of a literal or of a named constant,
// following constants across packages.
func (tree *sourceTree) resolve(pkg *sourcePackage, file *sourceFile, expr ast.Expr, depth int) (string, bool) {
	if depth > 10 {
		return "", false
	}
	switch e := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		return stringLiteral(e)
	case *ast.Ident:
		if definition, ok := pkg.consts[e.Name]; ok {
			return tree.resolve(pkg, definition.file, definition.value, depth+1)
		}
	case *ast.SelectorExpr:
		alias, ok := e.X.(*ast.Ident)
		if !ok {
			return "", false
		}
		target, ok := tree.packages[file.imports[alias.Name]]
		if !ok {
			return "", false
		}
		if definition, ok := target.consts[e.Sel.Name]; ok {
			return tree.resolve(target, definition.file, definition.value, depth+1)
		}
	}
	return "", false
}

func stringLiteral(expr ast.Expr) (string, bool) {
	lit, ok := ast.Unparen(expr).(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(lit.Value)
	return value, err == nil
}

func identName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// loadSourceTree parses every production Go file under root.
func loadSourceTree(t *testing.T, root string) *sourceTree {
	t.Helper()
	tree := &sourceTree{
		fset: token.NewFileSet(), packages: map[string]*sourcePackage{},
		codeParameters: map[string][]int{}, functions: map[string]functionDefinition{},
	}
	err := filepath.WalkDir(root, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if file != root && (name == "testdata" || name == "vendor" || name == "tmp" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
			return nil
		}
		syntax, err := parser.ParseFile(tree.fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, filepath.Dir(file))
		if err != nil {
			return err
		}
		importPath := backendModulePath
		if relative != "." {
			importPath += "/" + filepath.ToSlash(relative)
		}
		pkg := tree.packages[importPath]
		if pkg == nil {
			pkg = &sourcePackage{path: importPath, name: syntax.Name.Name, consts: map[string]constDefinition{}}
			tree.packages[importPath] = pkg
		}
		pkg.files = append(pkg.files, &sourceFile{syntax: syntax})
		return nil
	})
	if err != nil {
		t.Fatalf("parse backend sources: %v", err)
	}
	for _, pkg := range tree.packages {
		for _, file := range pkg.files {
			file.imports = tree.importNames(file.syntax)
			for name, definition := range constDefinitions(file) {
				pkg.consts[name] = definition
			}
			for function, positions := range codeParameterPositions(pkg.path, file.syntax) {
				tree.codeParameters[function] = positions
			}
			for _, decl := range file.syntax.Decls {
				if function, ok := decl.(*ast.FuncDecl); ok && function.Recv == nil {
					tree.functions[pkg.path+"."+function.Name.Name] = functionDefinition{decl: function, pkg: pkg, file: file}
				}
			}
		}
	}
	return tree
}

func (tree *sourceTree) importNames(syntax *ast.File) map[string]string {
	names := map[string]string{}
	for _, spec := range syntax.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(importPath)
		if target, ok := tree.packages[importPath]; ok {
			name = target.name
		}
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = importPath
	}
	return names
}

func constDefinitions(file *sourceFile) map[string]constDefinition {
	definitions := map[string]constDefinition{}
	for _, decl := range file.syntax.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			valueSpec := spec.(*ast.ValueSpec)
			for index, name := range valueSpec.Names {
				if index < len(valueSpec.Values) {
					definitions[name.Name] = constDefinition{value: valueSpec.Values[index], file: file}
				}
			}
		}
	}
	return definitions
}

// codeParameterPositions returns, per package-level function of the file,
// the positions of its parameters named code or fallbackCode.
func codeParameterPositions(packagePath string, syntax *ast.File) map[string][]int {
	positions := map[string][]int{}
	for _, decl := range syntax.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Recv != nil || packagePath == observabilityPackagePath {
			continue
		}
		key := packagePath + "." + function.Name.Name
		position := 0
		for _, field := range function.Type.Params.List {
			if len(field.Names) == 0 {
				position++
				continue
			}
			for _, name := range field.Names {
				if name.Name == "code" || name.Name == "fallbackCode" {
					positions[key] = append(positions[key], position)
				}
				position++
			}
		}
	}
	return positions
}

func loadErrorCodeRegistry(t *testing.T) errorCodeRegistry {
	t.Helper()
	encoded, err := os.ReadFile(filepath.Join(backendRoot(t), "..", "error-registry.json"))
	if err != nil {
		t.Fatalf("read error registry: %v", err)
	}
	var registry struct {
		Codes []struct {
			Code   string `json:"code"`
			Legacy string `json:"legacy"`
		} `json:"codes"`
	}
	if err := json.Unmarshal(encoded, &registry); err != nil {
		t.Fatalf("decode error registry: %v", err)
	}
	result := errorCodeRegistry{codes: map[string]bool{}, areas: map[string]bool{}, legacy: map[string]string{}}
	for _, entry := range registry.Codes {
		result.codes[entry.Code] = true
		area, _, _ := strings.Cut(entry.Code, ".")
		result.areas[area] = true
		if entry.Legacy != "" && entry.Legacy != entry.Code {
			result.legacy[entry.Legacy] = entry.Code
		}
	}
	return result
}

func backendRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate backend root")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}
