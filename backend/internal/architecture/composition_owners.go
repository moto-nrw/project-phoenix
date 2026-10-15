package architecture

import (
	"fmt"
	"go/ast"
	"strings"
)

// compositionOwnership classifies the field types of the structs a
// root-composition package declares (ADR 0059, #4071). A struct there holds
// values of several owners when its fields, taken together, hold values of at
// least two owners other than the root itself. A field holds a value of owner
// O when its declared type, after unwrapping pointers, slices, arrays, maps,
// channels and type arguments, names a type declared in a package the policy
// assigns to O. Types the root declares itself are followed into their
// definition: a root struct or alias holds what its own fields or target hold,
// so wrapping owners into a local struct does not hide them. A type argument
// is a value its instance holds; type parameters hold none. A function type
// and the own methods of an interface are ports of the root and hold no owner
// value, but an interface embedding a named interface holds what that one
// holds, so wrapping another owner's port keeps its origin. A named function
// or interface type of another package is a value of that package's owner.
//
// Counted are package-level struct types, instances of a generic root struct
// (type Wiring = Graph[alpha.Port, beta.Port]), and the structs a root
// declaration holds without a package-level name: anonymous struct variables
// and literals and types declared in a function, keyed by the enclosing
// declaration and their local name or position (anonymousStructs).
//
// Owners of the kernel kind (shared calendar and ID values)
// are vocabulary every owner shares and do not count. An internal package the
// policy does not classify counts as an owner of its own, so a missing
// classification cannot hide a value.
//
// Base and candidate are classified by every policy the comparison knows, and
// a struct counts when any of them sees several owners. A candidate policy
// that moves packages between owners therefore cannot remove a struct from
// the inventory it still holds in the base.
type compositionOwnership struct {
	roots    map[string]bool
	policies []compositionOwnerPolicy
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

// severalOwners reports whether the struct declared as expression in file
// holds values of at least two owners under any known policy. shadow names
// the type parameters in scope, which hold no owner value of their own.
func (s *compositionSnapshot) severalOwners(file *compositionFile, expression ast.Expr, shadow map[string]bool) bool {
	ownership := s.ownership
	if ownership == nil || len(ownership.policies) == 0 {
		return false
	}
	walk := &ownerWalk{snapshot: s, held: map[string]bool{}, seen: map[string]bool{}}
	walk.packages(file, expression, shadow)
	for _, policy := range ownership.policies {
		owners := map[string]bool{}
		for pkg := range walk.held {
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

// ownerWalk collects the module-relative packages whose values a type
// expression holds, following the root's own declarations.
type ownerWalk struct {
	snapshot *compositionSnapshot
	held     map[string]bool
	seen     map[string]bool
}

func (w *ownerWalk) packages(file *compositionFile, expression ast.Expr, shadow map[string]bool) {
	s := w.snapshot
	switch expression := expression.(type) {
	case *ast.StarExpr:
		w.packages(file, expression.X, shadow)
	case *ast.ParenExpr:
		w.packages(file, expression.X, shadow)
	case *ast.ArrayType:
		w.packages(file, expression.Elt, shadow)
	case *ast.MapType:
		w.packages(file, expression.Key, shadow)
		w.packages(file, expression.Value, shadow)
	case *ast.ChanType:
		w.packages(file, expression.Value, shadow)
	case *ast.IndexExpr:
		// A type argument is a value the instance holds.
		w.packages(file, expression.X, shadow)
		w.packages(file, expression.Index, shadow)
	case *ast.IndexListExpr:
		w.packages(file, expression.X, shadow)
		for _, index := range expression.Indices {
			w.packages(file, index, shadow)
		}
	case *ast.StructType:
		for _, field := range expression.Fields.List {
			w.packages(file, field.Type, shadow)
		}
	case *ast.InterfaceType:
		// Own methods make a port of the root. An embedded named interface
		// keeps its origin: a wrapper around another owner's port holds that
		// owner's service.
		for _, method := range expression.Methods.List {
			if len(method.Names) == 0 {
				w.packages(file, method.Type, shadow)
			}
		}
	case *ast.Ident:
		if !shadow[expression.Name] {
			w.rootType(file, expression)
		}
	case *ast.SelectorExpr:
		alias, ok := expression.X.(*ast.Ident)
		if !ok {
			return
		}
		path := file.imports[alias.Name]
		if s.module == "" || !strings.HasPrefix(path, s.module+"/") {
			return
		}
		pkg := strings.TrimPrefix(path, s.module+"/")
		if s.ownership.root(pkg) {
			w.rootType(file, expression)
			return
		}
		w.held[pkg] = true
	}
}

func (w *ownerWalk) rootType(file *compositionFile, expression ast.Expr) {
	s := w.snapshot
	typ, key, ok := s.resolve(file, expression)
	if !ok || w.seen[key] {
		return
	}
	w.seen[key] = true
	if !s.ownership.root(typ.file.pkg) {
		w.held[typ.file.pkg] = true
		return
	}
	w.packages(typ.file, typ.expr, typeParameters(s.typeParams[key], nil))
}

// typeParameters adds the names of a type parameter list to shadow.
func typeParameters(list *ast.FieldList, shadow map[string]bool) map[string]bool {
	if list == nil {
		return shadow
	}
	result := map[string]bool{}
	for name := range shadow {
		result[name] = true
	}
	for _, field := range list.List {
		for _, name := range field.Names {
			result[name.Name] = true
		}
	}
	return result
}

// rootStruct returns the struct a root type declaration stands for: its own
// struct literal, or the struct of the generic root type it instantiates
// (type Wiring = Graph[alpha.Port, beta.Port]). The instance holds its type
// arguments, so the owners are read from the declared expression.
func (s *compositionSnapshot) rootStruct(file *compositionFile, expression ast.Expr) *ast.StructType {
	switch typed := expression.(type) {
	case *ast.StructType:
		return typed
	case *ast.IndexExpr, *ast.IndexListExpr:
		typ, _, ok := s.resolve(file, expression)
		if !ok || !s.ownership.root(typ.file.pkg) {
			return nil
		}
		structure, _ := typ.expr.(*ast.StructType)
		return structure
	}
	return nil
}

// anonymousStructs records the struct types a root declaration holds without
// a package-level type name: anonymous struct variables and literals and
// types declared inside a function. symbol is the enclosing declaration.
// Each struct is named by its local type or variable, or by its position
// among the unnamed ones ("struct1"); a struct nested in another counts with
// the outer one.
func (s *compositionSnapshot) anonymousStructs(file *compositionFile, symbol string, node ast.Node, shadow map[string]bool, add func(name string, structure *ast.StructType)) {
	names := map[ast.Node]string{}
	instances := map[*ast.TypeSpec]bool{}
	nameLiteral := func(name string, value ast.Expr) {
		if unary, ok := value.(*ast.UnaryExpr); ok {
			value = unary.X
		}
		if literal, ok := value.(*ast.CompositeLit); ok && literal.Type != nil {
			if _, ok := literal.Type.(*ast.StructType); ok {
				names[literal.Type] = name
			}
		}
	}
	ast.Inspect(node, func(current ast.Node) bool {
		switch current := current.(type) {
		case *ast.TypeSpec:
			if _, ok := current.Type.(*ast.StructType); ok {
				names[current.Type] = current.Name.Name
			} else if structure := s.rootStruct(file, current.Type); structure != nil {
				instances[current] = true
			}
		case *ast.ValueSpec:
			if len(current.Names) == 1 {
				if _, ok := current.Type.(*ast.StructType); ok {
					names[current.Type] = current.Names[0].Name
				}
				if len(current.Values) == 1 {
					nameLiteral(current.Names[0].Name, current.Values[0])
				}
			}
		case *ast.AssignStmt:
			if len(current.Lhs) == len(current.Rhs) {
				for index, lhs := range current.Lhs {
					if id, ok := lhs.(*ast.Ident); ok && id.Name != "_" {
						nameLiteral(id.Name, current.Rhs[index])
					}
				}
			}
		}
		return true
	})
	unnamed := 0
	ast.Inspect(node, func(current ast.Node) bool {
		switch current := current.(type) {
		case *ast.TypeSpec:
			if instances[current] && s.severalOwners(file, current.Type, shadow) {
				add(symbol+"."+current.Name.Name, s.rootStruct(file, current.Type))
			}
		case *ast.FuncType:
			shadow = typeParameters(current.TypeParams, shadow)
		case *ast.StructType:
			name, ok := names[current]
			if !ok {
				unnamed++
				name = fmt.Sprintf("struct%d", unnamed)
			}
			if s.severalOwners(file, current, shadow) {
				if name != symbol {
					name = symbol + "." + name
				}
				add(name, current)
			}
			return false
		}
		return true
	})
}
