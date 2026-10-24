package architecture

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// compositionOwnership classifies the structs a root-composition package
// holds (ADR 0059, #4071). The analysis is semantic: the root packages are
// loaded with full type information (go/types), production and test variants
// alike, and every struct type they write down is a carrier:
//
//   - a struct type literal anywhere in a root file: a package-level or local
//     type declaration, an anonymous struct of a variable, literal, parameter
//     or field, nested structs included;
//   - a type expression that instantiates a generic struct of a root package
//     (Graph[alpha.Port, beta.Port] as an alias, a variable or a literal);
//   - a variable whose type a call infers as such an instance.
//
// A carrier counts when its struct holds values of at least two owners other
// than the root. A type holds a value of owner O when it is, after resolving
// aliases and unwrapping pointers, slices, arrays, maps and channels, a named
// type declared in a package the policy assigns to O; the type arguments of a
// foreign generic type are values it holds as well. Named types of a root
// package (local ones included) are followed into their underlying type, so
// wrapper structs, local aliases and instantiated generics resolve to what
// they hold. A function type and the own methods of an interface are ports of
// the root and hold no owner value; an interface embedding a named interface
// holds what that one holds. Owners of the kernel kind are shared vocabulary
// and do not count; an internal package without classification counts as an
// owner of its own.
//
// Each counted carrier contributes one key per field:
// scope|field|package|<carrier>.<field>. The carrier name is lexical: the
// declared type or variable name, prefixed with the enclosing function or
// method when it is local, the parent carrier and field for a nested struct,
// or "struct<n>" for the n-th counted unnamed struct of a declaration. A name
// that repeats within a package scope gets "#2", "#3" in source order. Only
// counted carriers take part in the numbering.
//
// Base and candidate are classified by every policy the comparison knows, and
// a struct counts when any of them sees several owners. A candidate policy
// that moves packages between owners therefore cannot remove a struct from
// the inventory it still holds in the base.
type compositionOwnership struct {
	roots     map[string]bool
	policies  []compositionOwnerPolicy
	buildTags []string
}

type compositionOwnerPolicy struct {
	owners map[string]string
	ignore map[string]bool
}

func newCompositionOwnership(policies ...*Policy) *compositionOwnership {
	ownership := &compositionOwnership{roots: map[string]bool{}}
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		ownership.buildTags = append(ownership.buildTags, policy.Build.Tags...)
		classification := compositionOwnerPolicy{owners: map[string]string{}, ignore: map[string]bool{}}
		for _, owner := range policy.Owners {
			if owner.Kind == "kernel" {
				classification.ignore[owner.ID] = true
			}
		}
		for _, pkg := range policy.Packages {
			classification.owners[pkg.Path] = pkg.Owner
			if pkg.Owner == rootCompositionOwner {
				ownership.roots[pkg.Path] = true
				classification.ignore[pkg.Owner] = true
			}
		}
		ownership.policies = append(ownership.policies, classification)
	}
	return ownership
}

func (o *compositionOwnership) root(pkg string) bool {
	return o != nil && o.roots[pkg]
}

// severalOwners reports whether typ holds values of at least two owners
// under any known policy.
func (o *compositionOwnership) severalOwners(module string, typ types.Type) bool {
	held := map[string]bool{}
	o.held(module, typ, held, map[types.Type]bool{})
	for _, policy := range o.policies {
		owners := map[string]bool{}
		for pkg := range held {
			owner, classified := policy.owners[pkg]
			if !classified {
				owner = "unclassified:" + pkg
			}
			if !policy.ignore[owner] {
				owners[owner] = true
			}
		}
		if len(owners) >= 2 {
			return true
		}
	}
	return false
}

// held collects the module-relative packages whose values typ holds.
func (o *compositionOwnership) held(module string, typ types.Type, held map[string]bool, seen map[types.Type]bool) {
	switch typ := types.Unalias(typ).(type) {
	case *types.Pointer:
		o.held(module, typ.Elem(), held, seen)
	case *types.Slice:
		o.held(module, typ.Elem(), held, seen)
	case *types.Array:
		o.held(module, typ.Elem(), held, seen)
	case *types.Map:
		o.held(module, typ.Key(), held, seen)
		o.held(module, typ.Elem(), held, seen)
	case *types.Chan:
		o.held(module, typ.Elem(), held, seen)
	case *types.Struct:
		for index := range typ.NumFields() {
			o.held(module, typ.Field(index).Type(), held, seen)
		}
	case *types.Interface:
		for index := range typ.NumEmbeddeds() {
			o.held(module, typ.EmbeddedType(index), held, seen)
		}
	case *types.Named:
		if seen[typ] {
			return
		}
		seen[typ] = true
		pkg := typ.Obj().Pkg()
		if pkg == nil {
			return
		}
		relative, internal := modulePackage(module, pkg.Path())
		if internal && o.root(relative) {
			// The root's own types, local ones included, hold what their
			// underlying type holds; an instance has its arguments substituted.
			o.held(module, typ.Underlying(), held, seen)
			return
		}
		if internal {
			held[relative] = true
		}
		for index := range typ.TypeArgs().Len() {
			o.held(module, typ.TypeArgs().At(index), held, seen)
		}
	}
}

// modulePackage returns the module-relative path of a package of module,
// with an external test package mapped to the package it tests.
func modulePackage(module, path string) (string, bool) {
	path = strings.TrimSuffix(path, "_test")
	if path == module {
		return ".", true
	}
	if strings.HasPrefix(path, module+"/") {
		return strings.TrimPrefix(path, module+"/"), true
	}
	return "", false
}

// baseRootStructInventory extracts the module tree of the base commit and
// inventories its root structs like the candidate's.
func baseRootStructInventory(root, prefix, sha string, ownership *compositionOwnership) (map[string]string, error) {
	directory, err := os.MkdirTemp("", "composition-base-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	if err := extractGitTree(root, sha, prefix, directory); err != nil {
		return nil, err
	}
	return rootStructInventory(filepath.Join(directory, filepath.FromSlash(prefix)), ownership)
}

// extractGitTree writes the files below prefix at sha into destination:
// git archive piped into tar, without touching the index or a worktree.
func extractGitTree(root, sha, prefix, destination string) error {
	archive := exec.Command("git", "-C", root, "archive", "--format=tar", sha, "--", prefix)
	extract := exec.Command("tar", "-x", "-f", "-", "-C", destination)
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	extract.Stdin = pipe
	var archiveErr, extractErr strings.Builder
	archive.Stderr, extract.Stderr = &archiveErr, &extractErr
	if err := extract.Start(); err != nil {
		return err
	}
	if err := archive.Run(); err != nil {
		_ = extract.Wait()
		return fmt.Errorf("git archive %s: %w: %s", sha, err, strings.TrimSpace(archiveErr.String()))
	}
	if err := extract.Wait(); err != nil {
		return fmt.Errorf("extract %s: %w: %s", sha, err, strings.TrimSpace(extractErr.String()))
	}
	return nil
}

// rootStructInventory type-checks the root-composition packages of the
// module in directory, with their tests, and returns the field keys of every
// counted carrier.
func rootStructInventory(directory string, ownership *compositionOwnership) (map[string]string, error) {
	return rootStructInventoryWith(directory, ownership, nil)
}

// rootStructInventoryWith loads with inherited added to the process
// environment, so tests can pass variables such as GOWORK in parallel.
func rootStructInventoryWith(directory string, ownership *compositionOwnership, inherited []string) (map[string]string, error) {
	result := map[string]string{}
	module, err := moduleName(directory)
	if err != nil {
		return nil, err
	}
	var patterns []string
	for _, pkg := range sortedKeys(ownership.roots) {
		entries, err := filepath.Glob(filepath.Join(directory, filepath.FromSlash(pkg), "*.go"))
		if err != nil {
			return nil, err
		}
		if len(entries) == 0 {
			continue
		}
		if pkg == "." {
			patterns = append(patterns, ".")
		} else {
			patterns = append(patterns, "./"+pkg)
		}
	}
	if len(patterns) == 0 {
		return result, nil
	}
	if workspace, ok := enclosingWorkspace(directory); ok {
		return nil, fmt.Errorf("root structs are measured for a standalone module, but %s declares a Go workspace", workspace)
	}
	cgoEnabled := false
	loaded, err := packages.Load(&packages.Config{
		Dir: directory, Tests: true,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo,
		Env: standaloneModuleEnvironment(append(fixedBuildEnvironment(Build{GOOS: "linux", GOARCH: "amd64", CGOEnabled: &cgoEnabled, Tags: ownership.buildTags}), inherited...)),
	}, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load root packages: %w", err)
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].ID < loaded[j].ID })
	// Every loaded package must type-check and every requested root must be
	// among them; a load failure is never an empty inventory.
	found := map[string]bool{}
	for _, pkg := range loaded {
		if len(pkg.Errors) != 0 {
			return nil, fmt.Errorf("type-check %s: %s", pkg.ID, pkg.Errors[0])
		}
		if relative, internal := modulePackage(module, pkg.PkgPath); internal && pkg.ID == pkg.PkgPath {
			found[relative] = true
		}
	}
	for _, pattern := range patterns {
		if relative := strings.TrimPrefix(pattern, "./"); !found[relative] {
			return nil, fmt.Errorf("root package %s of module %s was not loaded", relative, module)
		}
	}
	for _, pkg := range loaded {
		if strings.HasSuffix(pkg.PkgPath, ".test") {
			continue
		}
		relative, internal := modulePackage(module, pkg.PkgPath)
		if !internal || !ownership.root(relative) {
			continue
		}
		scope := "production"
		switch {
		case strings.HasSuffix(pkg.PkgPath, "_test"):
			scope = "external_test"
		case pkg.ID != pkg.PkgPath:
			scope = "internal_test"
		}
		walk := &rootStructWalk{
			ownership: ownership, module: module, info: pkg.TypesInfo, fset: pkg.Fset,
			prefix: scope + "|field|" + relative + "|", result: result,
			counters: map[string]int{}, keys: map[ast.Node]string{},
		}
		files := make([]int, 0, len(pkg.Syntax))
		for index := range pkg.Syntax {
			files = append(files, index)
		}
		sort.Slice(files, func(i, j int) bool { return pkg.CompiledGoFiles[files[i]] < pkg.CompiledGoFiles[files[j]] })
		for _, index := range files {
			test := strings.HasSuffix(pkg.CompiledGoFiles[index], "_test.go")
			if scope == "internal_test" && !test || scope == "production" && test {
				continue
			}
			walk.file(pkg.Syntax[index])
		}
	}
	return result, nil
}

// enclosingWorkspace finds a go.work file of the repository: in directory or
// above it, up to the directory that holds .git.
func enclosingWorkspace(directory string) (string, bool) {
	current, err := filepath.Abs(directory)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(current, "go.work")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			return "", false
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", false
		}
		current = parent
	}
}

// standaloneModuleEnvironment resolves the module on its own: an inherited
// GOWORK must not pull the archived base, or the candidate, into a workspace
// that lists other modules.
func standaloneModuleEnvironment(environment []string) []string {
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GOWORK=") {
			result = append(result, entry)
		}
	}
	return append(result, "GOWORK=off")
}

func moduleName(directory string) (string, error) {
	source, err := os.ReadFile(filepath.Join(directory, "go.mod")) // #nosec G304 -- the module of the checked project
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(source), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "module" {
			return strings.Trim(fields[1], "\""), nil
		}
	}
	return "", fmt.Errorf("go.mod in %s has no module declaration", directory)
}

type rootStructWalk struct {
	ownership *compositionOwnership
	module    string
	info      *types.Info
	fset      *token.FileSet
	prefix    string
	result    map[string]string
	// counters numbers repeated carrier names within one package scope;
	// keys holds the name each counted carrier node received.
	counters map[string]int
	keys     map[ast.Node]string
}

func (w *rootStructWalk) file(file *ast.File) {
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			symbol := declaration.Name.Name
			if declaration.Recv != nil && len(declaration.Recv.List) != 0 {
				symbol = compositionTypeName(declaration.Recv.List[0].Type) + "." + symbol
			}
			w.declaration(symbol, declaration, nil)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					w.declaration(spec.Name.Name, spec, spec)
				case *ast.ValueSpec:
					w.declaration(spec.Names[0].Name, spec, spec)
				}
			}
		}
	}
}

// declaration inventories the carriers of one top-level declaration. spec is
// the top-level type or value spec whose own name a carrier may take as is.
func (w *rootStructWalk) declaration(symbol string, node ast.Node, spec ast.Node) {
	var stack []ast.Node
	ast.Inspect(node, func(current ast.Node) bool {
		if current == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if structure := w.carrier(current, stack); structure != nil && w.ownership.severalOwners(w.module, structure) {
			w.count(symbol, spec, current, stack, structure)
		}
		stack = append(stack, current)
		return true
	})
}

// carrier returns the struct a node writes down: a struct type literal, an
// instantiated generic root struct, or the inferred instance of a variable.
func (w *rootStructWalk) carrier(node ast.Node, stack []ast.Node) *types.Struct {
	switch node := node.(type) {
	case *ast.StructType:
		structure, _ := w.info.TypeOf(node).(*types.Struct)
		return structure
	case *ast.IndexExpr, *ast.IndexListExpr:
		value, ok := w.info.Types[node.(ast.Expr)]
		if !ok || !value.IsType() {
			return nil
		}
		return w.rootInstance(value.Type)
	case *ast.Ident:
		variable, ok := w.info.Defs[node].(*types.Var)
		if !ok || variable.IsField() || !inferredFromCall(node, stack) {
			return nil
		}
		typ := types.Unalias(variable.Type())
		if pointer, ok := typ.(*types.Pointer); ok {
			typ = pointer.Elem()
		}
		return w.rootInstance(typ)
	}
	return nil
}

// rootInstance returns the struct of an instantiated generic root type.
func (w *rootStructWalk) rootInstance(typ types.Type) *types.Struct {
	named, ok := types.Unalias(typ).(*types.Named)
	if !ok || named.TypeArgs().Len() == 0 || named.Obj().Pkg() == nil {
		return nil
	}
	relative, internal := modulePackage(w.module, named.Obj().Pkg().Path())
	if !internal || !w.ownership.root(relative) {
		return nil
	}
	structure, _ := named.Underlying().(*types.Struct)
	return structure
}

// inferredFromCall reports whether an identifier is a variable a call's
// result types without a written type: name := f(...), var name = f(...).
func inferredFromCall(name *ast.Ident, stack []ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	var values []ast.Expr
	var names []ast.Expr
	switch parent := stack[len(stack)-1].(type) {
	case *ast.AssignStmt:
		if parent.Tok != token.DEFINE {
			return false
		}
		names, values = parent.Lhs, parent.Rhs
	case *ast.ValueSpec:
		if parent.Type != nil {
			return false
		}
		for _, id := range parent.Names {
			names = append(names, id)
		}
		values = parent.Values
	default:
		return false
	}
	// One call with several results types every name it declares.
	if len(values) == 1 && len(names) > 1 {
		_, call := ast.Unparen(values[0]).(*ast.CallExpr)
		return call
	}
	for index, candidate := range names {
		if candidate == name && index < len(values) {
			_, call := ast.Unparen(values[index]).(*ast.CallExpr)
			return call
		}
	}
	return false
}

func (w *rootStructWalk) count(symbol string, spec, node ast.Node, stack []ast.Node, structure *types.Struct) {
	path, absolute := w.lexicalName(node, stack, spec)
	base := path
	switch {
	case path == "":
		base = symbol + ".struct"
	case !absolute:
		base = symbol + "." + path
	}
	w.counters[base]++
	name := base
	switch number := w.counters[base]; {
	case path == "":
		name = base + strconv.Itoa(number)
	case number > 1:
		name = base + "#" + strconv.Itoa(number)
	}
	w.keys[node] = name
	location := w.fset.Position(node.Pos()).String()
	for index := range structure.NumFields() {
		w.result[w.prefix+name+"."+structure.Field(index).Name()] = location
	}
}

// lexicalName names a carrier by its nearest naming ancestor. absolute marks
// a name that needs no symbol prefix: the top-level spec itself, or a field
// path below a counted parent carrier. An empty name is an unnamed carrier.
func (w *rootStructWalk) lexicalName(node ast.Node, stack []ast.Node, spec ast.Node) (string, bool) {
	child := node
	for index := len(stack) - 1; index >= 0; index-- {
		switch ancestor := stack[index].(type) {
		case *ast.Field:
			field := "_"
			if len(ancestor.Names) != 0 {
				field = ancestor.Names[0].Name
			} else if name := compositionTypeName(ancestor.Type); name != "" {
				field = name
			}
			if index >= 2 {
				if parent, ok := w.keys[stack[index-2]]; ok {
					return parent + "." + field, true
				}
				if _, ok := stack[index-2].(*ast.FuncType); ok {
					return field, false
				}
			}
		case *ast.TypeSpec:
			return ancestor.Name.Name, ancestor == spec
		case *ast.ValueSpec:
			name := ancestor.Names[0].Name
			for _, id := range ancestor.Names {
				if id == child {
					name = id.Name
				}
			}
			for position, value := range ancestor.Values {
				if value == child && position < len(ancestor.Names) {
					name = ancestor.Names[position].Name
				}
			}
			return name, ancestor == spec
		case *ast.AssignStmt:
			if id, ok := child.(*ast.Ident); ok && id.Name != "_" {
				for _, lhs := range ancestor.Lhs {
					if lhs == child {
						return id.Name, false
					}
				}
			}
			for position, value := range ancestor.Rhs {
				if value != child || position >= len(ancestor.Lhs) {
					continue
				}
				if id, ok := ancestor.Lhs[position].(*ast.Ident); ok && id.Name != "_" {
					return id.Name, false
				}
			}
			return "", false
		case *ast.FuncLit, *ast.FuncDecl, *ast.BlockStmt:
			return "", false
		}
		child = stack[index]
	}
	return "", false
}
