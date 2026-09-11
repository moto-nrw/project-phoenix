package architecture

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// CompareCompositionSurface derives both inventories from source, never from a
// candidate-editable manifest. Locations are evidence, not declaration identity.
func CompareCompositionSurface(project, ref string) error {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return err
	}
	prefix, err := repositoryRelativePath(root, project, project)
	if err != nil {
		return err
	}
	listing, err := gitOutput(root, "ls-tree", "-r", "--name-only", "-z", sha, "--", filepath.ToSlash(prefix))
	if err != nil {
		return err
	}
	var paths []string
	for _, name := range strings.Split(listing, "\x00") {
		if name == "" {
			continue
		}
		relative, err := filepath.Rel(prefix, name)
		if err != nil {
			return err
		}
		if !compositionSource(relative) {
			continue
		}
		paths = append(paths, filepath.ToSlash(relative))
	}
	base, err := readCompositionBase(root, prefix, sha, paths)
	if err != nil {
		return err
	}
	candidate := newCompositionSnapshot()
	err = filepath.WalkDir(project, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "vendor" || entry.Name() == "testdata") {
			return filepath.SkipDir
		}
		relative, err := filepath.Rel(project, path)
		if err != nil {
			return err
		}
		if entry.IsDir() || !compositionSource(relative) {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return candidate.add(filepath.ToSlash(relative), source)
	})
	if err != nil {
		return err
	}
	external, err := loadCompositionExternalTypes(project, candidate)
	if err != nil {
		return err
	}
	base.external, candidate.external = external, external
	accepted := base.inventory()
	var added []string
	current := candidate.inventory()
	for key, location := range current {
		if _, exists := accepted[key]; !exists {
			added = append(added, key+" at "+location)
		}
	}
	sort.Strings(added)
	if len(added) != 0 {
		return fmt.Errorf("composition surface grew against base %s:\n  %s", sha, strings.Join(added, "\n  "))
	}
	fmt.Printf("composition surface passed: %d -> %d field/setter targets across production and test scopes\n", len(accepted), len(current))
	return nil
}

func readCompositionBase(root, prefix, sha string, paths []string) (*compositionSnapshot, error) {
	var input strings.Builder
	for _, path := range paths {
		if strings.ContainsAny(path, "\r\n") {
			return nil, fmt.Errorf("composition source path contains a newline: %q", path)
		}
		fmt.Fprintf(&input, "%s:%s\n", sha, filepath.ToSlash(filepath.Join(prefix, path)))
	}
	command := exec.Command("git", "-C", root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(input.String())
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("read base composition sources: %w", err)
	}
	reader := bufio.NewReader(bytes.NewReader(output))
	snapshot := newCompositionSnapshot()
	for _, path := range paths {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("read base source %s header: %w", path, err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || fields[1] != "blob" {
			return nil, fmt.Errorf("invalid base source %s header: %q", path, header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("invalid base source %s size: %q", path, fields[2])
		}
		blob := make([]byte, size)
		if _, err := io.ReadFull(reader, blob); err != nil {
			return nil, err
		}
		if separator, err := reader.ReadByte(); err != nil || separator != '\n' {
			return nil, fmt.Errorf("invalid base source %s separator", path)
		}
		if err := snapshot.add(path, blob); err != nil {
			return nil, err
		}
	}
	return snapshot, nil
}

func compositionSource(path string) bool {
	path = filepath.ToSlash(path)
	if !strings.HasSuffix(path, ".go") && path != "go.mod" {
		return false
	}
	for _, segment := range strings.Split(path, "/") {
		if segment == "testdata" || segment == "vendor" || strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return true
}

type compositionFile struct {
	pkg, scope string
	syntax     *ast.File
	imports    map[string]string
}

type compositionType struct {
	file *compositionFile
	expr ast.Expr
}

//nolint:staticcheck // Only parser-resolved local bindings are used, never field/type resolution.
type compositionObject = ast.Object

type compositionSnapshot struct {
	module   string
	fset     *token.FileSet
	files    []*compositionFile
	types    map[string]compositionType
	external map[string]*types.Package
}

func loadCompositionExternalTypes(project string, snapshot *compositionSnapshot) (map[string]*types.Package, error) {
	imports := map[string]bool{}
	for _, file := range snapshot.files {
		if !compositionPackage(file.pkg) {
			continue
		}
		for _, path := range file.imports {
			if path != "C" && !strings.HasPrefix(path, snapshot.module+"/") {
				imports[path] = true
			}
		}
	}
	paths := make([]string, 0, len(imports))
	for path := range imports {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	result := map[string]*types.Package{}
	if len(paths) == 0 {
		return result, nil
	}
	cgoEnabled := false
	loaded, err := packages.Load(&packages.Config{
		Dir: project, Mode: packages.NeedName | packages.NeedTypes | packages.NeedImports | packages.NeedDeps,
		Env: fixedBuildEnvironment(Build{GOOS: "linux", GOARCH: "amd64", CGOEnabled: &cgoEnabled}),
	}, paths...)
	if err != nil {
		return nil, fmt.Errorf("load composition dependency types: %w", err)
	}
	for _, pkg := range loaded {
		if len(pkg.Errors) == 0 && pkg.Types != nil {
			result[pkg.PkgPath] = pkg.Types
		}
	}
	return result, nil
}

func newCompositionSnapshot() *compositionSnapshot {
	return &compositionSnapshot{fset: token.NewFileSet(), types: map[string]compositionType{}}
}

func (s *compositionSnapshot) add(path string, source []byte) error {
	if path == "go.mod" {
		for _, line := range strings.Split(string(source), "\n") {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[0] == "module" {
				s.module = strings.Trim(fields[1], "\"")
				return nil
			}
		}
		return fmt.Errorf("composition source has no module declaration")
	}
	syntax, err := parser.ParseFile(s.fset, path, source, 0)
	if err != nil {
		return fmt.Errorf("parse composition source: %w", err)
	}
	file := &compositionFile{pkg: filepath.ToSlash(filepath.Dir(path)), scope: "production", syntax: syntax, imports: map[string]string{}}
	if strings.HasSuffix(path, "_test.go") {
		file.scope = "internal_test"
		if strings.HasSuffix(syntax.Name.Name, "_test") {
			file.scope = "external_test"
		}
	}
	for _, spec := range syntax.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return err
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		file.imports[name] = path
	}
	s.files = append(s.files, file)
	for _, declaration := range syntax.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			if typ, ok := spec.(*ast.TypeSpec); ok {
				s.types[file.scope+"|"+file.pkg+"|"+typ.Name.Name] = compositionType{file, typ.Type}
			}
		}
	}
	return nil
}

func (s *compositionSnapshot) inventory() map[string]string {
	result := map[string]string{}
	for _, file := range s.files {
		if !compositionPackage(file.pkg) {
			continue
		}
		add := func(kind, symbol string, pos token.Pos) {
			result[file.scope+"|"+kind+"|"+file.pkg+"|"+symbol] = s.fset.Position(pos).String()
		}
		for _, declaration := range file.syntax.Decls {
			switch declaration := declaration.(type) {
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					typ, ok := spec.(*ast.TypeSpec)
					if !ok || !centralAggregate(file.pkg, typ.Name.Name) {
						continue
					}
					structure, _ := s.structure(file, typ.Type, map[string]bool{})
					if structure == nil {
						continue
					}
					for _, field := range structure.Fields.List {
						if len(field.Names) == 0 {
							add("field", typ.Name.Name+"."+compositionTypeName(field.Type), typ.Pos())
						}
						for _, name := range field.Names {
							add("field", typ.Name.Name+"."+name.Name, typ.Pos())
						}
					}
				}
			case *ast.FuncDecl:
				s.collectSetters(file, declaration, add)
			}
		}
	}
	return result
}

func (s *compositionSnapshot) collectSetters(file *compositionFile, declaration *ast.FuncDecl, add func(string, string, token.Pos)) {
	if declaration.Body == nil {
		return
	}
	targets := map[*compositionObject]compositionType{}
	if declaration.Recv != nil {
		for _, receiver := range declaration.Recv.List {
			for _, name := range receiver.Names {
				targets[name.Obj] = compositionType{file, receiver.Type}
			}
		}
	}
	for _, parameter := range declaration.Type.Params.List {
		if _, pointer := parameter.Type.(*ast.StarExpr); !pointer {
			continue
		}
		for _, name := range parameter.Names {
			targets[name.Obj] = compositionType{file, parameter.Type}
		}
	}
	ast.Inspect(declaration.Body, func(node ast.Node) bool {
		assignment, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, lhs := range assignment.Lhs {
			target, field, ok := compositionDestination(lhs, targets, map[*compositionObject]bool{})
			if !ok || !s.guardedField(target, field) {
				continue
			}
			symbol := declaration.Name.Name
			if declaration.Recv != nil {
				symbol = compositionTypeName(declaration.Recv.List[0].Type) + "." + symbol
			}
			add("setter", symbol+"->"+compositionTypeName(target.expr)+"."+field, assignment.Pos())
		}
		return true
	})
}

func compositionPackage(pkg string) bool {
	if strings.Contains(pkg+"/", "/domain/") || strings.HasPrefix(pkg, "models/") {
		return false
	}
	for _, root := range []string{"api", "services", "database/repositories", "modules", "app", "application", "workflows", "cmd"} {
		if pkg == root || strings.HasPrefix(pkg, root+"/") {
			return true
		}
	}
	return false
}

func centralAggregate(pkg, name string) bool {
	return pkg == "services" && name == "Factory" || pkg == "database/repositories" && name == "Factory" || pkg == "api" && name == "API"
}

func compositionTypeName(expression ast.Expr) string {
	switch expression := expression.(type) {
	case *ast.Ident:
		return expression.Name
	case *ast.StarExpr:
		return compositionTypeName(expression.X)
	case *ast.SelectorExpr:
		return expression.Sel.Name
	case *ast.IndexExpr:
		return compositionTypeName(expression.X)
	case *ast.IndexListExpr:
		return compositionTypeName(expression.X)
	case *ast.ParenExpr:
		return compositionTypeName(expression.X)
	}
	return ""
}

func (s *compositionSnapshot) resolve(file *compositionFile, expression ast.Expr) (compositionType, string, bool) {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		return s.resolve(file, pointer.X)
	}
	pkg, name, scope := file.pkg, compositionTypeName(expression), file.scope
	if selector, ok := expression.(*ast.SelectorExpr); ok {
		alias, ok := selector.X.(*ast.Ident)
		if !ok {
			return compositionType{}, "", false
		}
		path := file.imports[alias.Name]
		if s.module == "" || !strings.HasPrefix(path, s.module+"/") {
			return compositionType{}, "", false
		}
		pkg, scope = strings.TrimPrefix(path, s.module+"/"), "production"
	}
	key := scope + "|" + pkg + "|" + name
	typ, ok := s.types[key]
	if !ok && scope == "internal_test" {
		key = "production|" + pkg + "|" + name
		typ, ok = s.types[key]
	}
	return typ, key, ok
}

func (s *compositionSnapshot) structure(file *compositionFile, expression ast.Expr, seen map[string]bool) (*ast.StructType, *compositionFile) {
	if structure, ok := expression.(*ast.StructType); ok {
		return structure, file
	}
	typ, key, ok := s.resolve(file, expression)
	if !ok || seen[key] {
		return nil, file
	}
	seen[key] = true
	return s.structure(typ.file, typ.expr, seen)
}

func (s *compositionSnapshot) guardedField(target compositionType, name string) bool {
	structure, file := s.structure(target.file, target.expr, map[string]bool{})
	if structure == nil || !compositionPackage(file.pkg) {
		return false
	}
	if centralAggregate(file.pkg, compositionTypeName(target.expr)) {
		return true
	}
	if name == "*" {
		return s.dependencyType(file, structure, map[string]bool{}) || compositionTypeName(target.expr) == "Worker" || compositionTypeName(target.expr) == "Scheduler"
	}
	return s.guardedStructureField(file, structure, name, compositionTypeName(target.expr), map[*ast.StructType]bool{})
}

func (s *compositionSnapshot) guardedStructureField(file *compositionFile, structure *ast.StructType, name, receiver string, seen map[*ast.StructType]bool) bool {
	if structure == nil || seen[structure] || !compositionPackage(file.pkg) {
		return false
	}
	seen[structure] = true
	for _, field := range structure.Fields.List {
		names := field.Names
		if len(names) == 0 {
			names = []*ast.Ident{{Name: compositionTypeName(field.Type)}}
		}
		for _, declared := range names {
			if declared.Name != name {
				continue
			}
			if s.dependencyType(file, field.Type, map[string]bool{}) {
				return true
			}
			// Worker and Scheduler configuration also includes scalar limits and
			// durations. Domain records such as ScheduledTask are not receivers.
			return (receiver == "Worker" || receiver == "Scheduler") && s.scalarType(file, field.Type, map[string]bool{})
		}
	}
	for _, field := range structure.Fields.List {
		if len(field.Names) != 0 {
			continue
		}
		embedded, owner := s.structure(file, field.Type, map[string]bool{})
		if s.guardedStructureField(owner, embedded, name, receiver, seen) {
			return true
		}
	}
	return false
}

func (s *compositionSnapshot) scalarType(file *compositionFile, expression ast.Expr, seen map[string]bool) bool {
	if mapping, ok := expression.(*ast.MapType); ok {
		return s.scalarType(file, mapping.Value, seen)
	}
	if array, ok := expression.(*ast.ArrayType); ok {
		return s.scalarType(file, array.Elt, seen)
	}
	if id, ok := expression.(*ast.Ident); ok {
		switch id.Name {
		case "bool", "string", "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "float32", "float64":
			return true
		}
	}
	if external := s.externalType(file, expression); external != nil {
		_, scalar := external.Underlying().(*types.Basic)
		return scalar
	}
	typ, key, ok := s.resolve(file, expression)
	if ok && !seen[key] {
		seen[key] = true
		return s.scalarType(typ.file, typ.expr, seen)
	}
	return false
}

func (s *compositionSnapshot) externalType(file *compositionFile, expression ast.Expr) types.Type {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	alias, ok := selector.X.(*ast.Ident)
	if !ok {
		return nil
	}
	pkg := s.external[file.imports[alias.Name]]
	if pkg == nil {
		return nil
	}
	if object := pkg.Scope().Lookup(selector.Sel.Name); object != nil {
		return object.Type()
	}
	return nil
}

func externalCompositionDependency(typ types.Type, seen map[types.Type]bool) bool {
	if seen[typ] {
		return false
	}
	seen[typ] = true
	switch underlying := typ.Underlying().(type) {
	case *types.Interface:
		return underlying.NumMethods() != 0
	case *types.Signature:
		return true
	case *types.Pointer:
		return externalCompositionDependency(underlying.Elem(), seen)
	case *types.Slice:
		return externalCompositionDependency(underlying.Elem(), seen)
	case *types.Array:
		return externalCompositionDependency(underlying.Elem(), seen)
	case *types.Map:
		return externalCompositionDependency(underlying.Elem(), seen)
	case *types.Struct:
		for index := 0; index < underlying.NumFields(); index++ {
			if externalCompositionDependency(underlying.Field(index).Type(), seen) {
				return true
			}
		}
	}
	return false
}

func (s *compositionSnapshot) dependencyType(file *compositionFile, expression ast.Expr, seen map[string]bool) bool {
	if external := s.externalType(file, expression); external != nil {
		return externalCompositionDependency(external, map[types.Type]bool{})
	}
	switch expression := expression.(type) {
	case *ast.InterfaceType:
		return len(expression.Methods.List) != 0
	case *ast.FuncType:
		return true
	case *ast.MapType:
		return s.dependencyType(file, expression.Value, seen)
	case *ast.ArrayType:
		return s.dependencyType(file, expression.Elt, seen)
	case *ast.StarExpr:
		return s.dependencyType(file, expression.X, seen)
	case *ast.StructType:
		for _, field := range expression.Fields.List {
			if s.dependencyType(file, field.Type, seen) {
				return true
			}
		}
		return false
	}
	typ, key, ok := s.resolve(file, expression)
	if !ok || seen[key] {
		// Missing external export data must not silently approve mutable wiring.
		// A removed base-only dependency needs no download to compare its debt.
		_, external := expression.(*ast.SelectorExpr)
		if !ok && external {
			return true
		}
		return false
	}
	if _, record := typ.expr.(*ast.StructType); record && (strings.HasPrefix(typ.file.pkg, "models/") || strings.Contains(typ.file.pkg+"/", "/domain/")) {
		return false
	}
	seen[key] = true
	return s.dependencyType(typ.file, typ.expr, seen)
}

// Follow aliases of existing receiver/parameter objects, never newly constructed
// local values. Those belong to construction, not a mutable setter API.
func compositionTarget(expression ast.Expr, targets map[*compositionObject]compositionType, seen map[*compositionObject]bool) (compositionType, bool) {
	if paren, ok := expression.(*ast.ParenExpr); ok {
		return compositionTarget(paren.X, targets, seen)
	}
	id, ok := expression.(*ast.Ident)
	if !ok || id.Obj == nil || seen[id.Obj] {
		return compositionType{}, false
	}
	if target, ok := targets[id.Obj]; ok {
		return target, true
	}
	seen[id.Obj] = true
	if assignment, ok := id.Obj.Decl.(*ast.AssignStmt); ok {
		for index, lhs := range assignment.Lhs {
			if name, ok := lhs.(*ast.Ident); ok && name.Obj == id.Obj && index < len(assignment.Rhs) {
				return compositionTarget(assignment.Rhs[index], targets, seen)
			}
		}
	}
	if declaration, ok := id.Obj.Decl.(*ast.ValueSpec); ok {
		for index, name := range declaration.Names {
			if name.Obj == id.Obj && index < len(declaration.Values) {
				return compositionTarget(declaration.Values[index], targets, seen)
			}
		}
	}
	return compositionType{}, false
}

func compositionDestination(expression ast.Expr, targets map[*compositionObject]compositionType, seen map[*compositionObject]bool) (compositionType, string, bool) {
	switch expression := expression.(type) {
	case *ast.StarExpr:
		target, ok := compositionTarget(expression.X, targets, seen)
		return target, "*", ok
	case *ast.ParenExpr:
		return compositionDestination(expression.X, targets, seen)
	case *ast.IndexExpr:
		return compositionDestination(expression.X, targets, seen)
	case *ast.SelectorExpr:
		if target, ok := compositionTarget(expression.X, targets, seen); ok {
			return target, expression.Sel.Name, true
		}
		return compositionDestination(expression.X, targets, seen)
	}
	return compositionType{}, "", false
}
