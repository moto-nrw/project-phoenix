package architecture

import (
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
// so wrapping owners into a local struct does not hide them. A function or
// interface type literal of the root is its own port and holds no owner value;
// a named function or interface type of another package is a value of that
// package's owner. Owners of the kernel kind (shared calendar and ID values)
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
// holds values of at least two owners under any known policy.
func (s *compositionSnapshot) severalOwners(file *compositionFile, expression ast.Expr) bool {
	ownership := s.ownership
	if ownership == nil || len(ownership.policies) == 0 {
		return false
	}
	packages := map[string]bool{}
	s.heldPackages(ownership, file, expression, packages, map[string]bool{})
	for _, policy := range ownership.policies {
		owners := map[string]bool{}
		for pkg := range packages {
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

// heldPackages collects the module-relative packages whose values a type
// expression holds, following the root's own declarations.
func (s *compositionSnapshot) heldPackages(ownership *compositionOwnership, file *compositionFile, expression ast.Expr, held map[string]bool, seen map[string]bool) {
	switch expression := expression.(type) {
	case *ast.StarExpr:
		s.heldPackages(ownership, file, expression.X, held, seen)
	case *ast.ParenExpr:
		s.heldPackages(ownership, file, expression.X, held, seen)
	case *ast.ArrayType:
		s.heldPackages(ownership, file, expression.Elt, held, seen)
	case *ast.MapType:
		s.heldPackages(ownership, file, expression.Key, held, seen)
		s.heldPackages(ownership, file, expression.Value, held, seen)
	case *ast.ChanType:
		s.heldPackages(ownership, file, expression.Value, held, seen)
	case *ast.IndexExpr:
		s.heldPackages(ownership, file, expression.X, held, seen)
		s.heldPackages(ownership, file, expression.Index, held, seen)
	case *ast.IndexListExpr:
		s.heldPackages(ownership, file, expression.X, held, seen)
		for _, index := range expression.Indices {
			s.heldPackages(ownership, file, index, held, seen)
		}
	case *ast.StructType:
		for _, field := range expression.Fields.List {
			s.heldPackages(ownership, file, field.Type, held, seen)
		}
	case *ast.Ident:
		s.heldRootType(ownership, file, expression, held, seen)
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
		if ownership.root(pkg) {
			s.heldRootType(ownership, file, expression, held, seen)
			return
		}
		held[pkg] = true
	}
}

func (s *compositionSnapshot) heldRootType(ownership *compositionOwnership, file *compositionFile, expression ast.Expr, held map[string]bool, seen map[string]bool) {
	typ, key, ok := s.resolve(file, expression)
	if !ok || seen[key] {
		return
	}
	seen[key] = true
	if !ownership.root(typ.file.pkg) {
		held[typ.file.pkg] = true
		return
	}
	s.heldPackages(ownership, typ.file, typ.expr, held, seen)
}
