package architecture

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"hash/fnv"
	"reflect"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

type commandEffectSite struct {
	Port          string `json:"port"`
	Binding       string `json:"binding"`
	Target        string `json:"target"`
	TargetOwner   string `json:"target_owner"`
	BindingDigest string `json:"binding_digest"`
}

func (s commandEffectSite) key() string {
	return s.Port + "|" + s.Binding + "|" + s.Target + "|" + s.TargetOwner
}

type effectBody struct {
	pkg         *packages.Package
	declaration *ast.FuncDecl
}
type effectScanner struct {
	ownerKinds map[string]string
	policy     map[string]Package
	bindings   effectBindings
	bodies     map[*types.Func]effectBody
	sites      map[string]commandEffectSite
	evidence   map[string]map[string]bool
}

func effectTypeID(t types.Type) string {
	return types.TypeString(types.Unalias(t), func(p *types.Package) string { return p.Path() })
}

// This first cut inventories Enrollment's named interface binding surface,
// not call-site liveness. Every policy-classified package can supply a binding.
func discoverCommandEffects(policy *Policy, loaded []*packages.Package) []commandEffectSite {
	s := newEffectScanner(policy, loaded)
	scoped := enrollmentEffectInterfaces(policy, loaded)
	for destination, sources := range s.bindings {
		if !scoped[destination] {
			continue
		}
		for _, source := range sources {
			if types.Identical(destination, source) && s.receiverOwner(source) == "enrollment" {
				continue
			}
			s.resolvePort(destination, source)
		}
	}
	return s.sortedSites()
}

func newEffectScanner(policy *Policy, loaded []*packages.Package) *effectScanner {
	s := &effectScanner{ownerKinds: map[string]string{}, policy: policy.packageMap(), bindings: collectEffectBindings(loaded), bodies: map[*types.Func]effectBody{}, sites: map[string]commandEffectSite{}, evidence: map[string]map[string]bool{}}
	for _, owner := range policy.Owners {
		s.ownerKinds[owner.ID] = owner.Kind
	}
	for _, pkg := range loaded {
		s.indexBodies(pkg)
	}
	return s
}

func (s *effectScanner) indexBodies(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if obj, ok := pkg.TypesInfo.Defs[fn.Name].(*types.Func); ok {
				s.bodies[obj] = effectBody{pkg, fn}
			}
		}
	}
}

func (s *effectScanner) resolvePort(destination, source types.Type) {
	contract := effectInterface(destination).Complete()
	for i := 0; i < contract.NumMethods(); i++ {
		name := contract.Method(i).Name()
		s.resolve(effectTypeID(destination)+"."+name, effectTypeID(source)+"."+name, source, name, map[string]bool{}, nil)
	}
}

func (s *effectScanner) sortedSites() []commandEffectSite {
	result := make([]commandEffectSite, 0, len(s.sites))
	for key, site := range s.sites {
		var evidence []string
		for value := range s.evidence[key] {
			evidence = append(evidence, value)
		}
		sort.Strings(evidence)
		site.BindingDigest = effectFingerprint(strings.Join(evidence, "\n"))
		result = append(result, site)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].key() < result[j].key() })
	return result
}

func (s *effectScanner) record(port, binding, target, owner string, trail []string) {
	site := commandEffectSite{Port: port, Binding: binding, Target: target, TargetOwner: owner}
	s.sites[site.key()] = site
	if s.evidence[site.key()] == nil {
		s.evidence[site.key()] = map[string]bool{}
	}
	for _, value := range trail {
		s.evidence[site.key()][value] = true
	}
}

func (s *effectScanner) resolve(port, binding string, receiver types.Type, name string, seen map[string]bool, trail []string) {
	identity := effectTypeID(receiver) + "." + name
	if seen[identity] {
		s.record(port, binding, "unresolved:cycle:"+identity, "", trail)
		return
	}
	seen[identity] = true
	defer delete(seen, identity)
	owner := s.receiverOwner(receiver)
	if owner == "enrollment" {
		if effectInterface(receiver) != nil {
			s.resolveInterface(port, binding, receiver, name, seen, trail)
		}
		return
	}
	if owner != "" && owner != "root-composition" {
		s.recordTerminal(port, binding, receiver, name, owner, trail)
		return
	}
	if effectInterface(receiver) != nil {
		s.resolveInterface(port, binding, receiver, name, seen, trail)
		return
	}
	s.resolveAdapter(port, binding, receiver, name, seen, trail)
}

func (s *effectScanner) receiverOwner(receiver types.Type) string {
	named := namedEffectType(receiver)
	if named == nil || named.Obj().Pkg() == nil {
		return ""
	}
	return s.policy[named.Obj().Pkg().Path()].Owner
}

func (s *effectScanner) recordTerminal(port, binding string, receiver types.Type, name, owner string, trail []string) {
	if !s.crossOwnerTarget(owner) {
		return
	}
	identity := effectTypeID(receiver) + "." + name
	if obj, _, _ := types.LookupFieldOrMethod(receiver, true, nil, name); obj != nil {
		trail = append(trail, identity+":"+effectTypeID(obj.Type()))
	}
	s.record(port, binding, identity, owner, trail)
}

func (s *effectScanner) resolveInterface(port, binding string, receiver types.Type, name string, seen map[string]bool, trail []string) {
	var sources []types.Type
	for destination, candidates := range s.bindings {
		if types.Identical(destination, receiver) {
			sources = append(sources, distinctEffectSources(receiver, candidates)...)
		}
	}
	if len(sources) == 0 {
		s.record(port, binding, "unresolved:"+effectTypeID(receiver)+"."+name, "", trail)
		return
	}
	for _, source := range sources {
		s.resolve(port, binding, source, name, seen, trail)
	}
}

func distinctEffectSources(receiver types.Type, candidates []types.Type) []types.Type {
	var sources []types.Type
	for _, source := range candidates {
		if !types.Identical(source, receiver) {
			sources = append(sources, source)
		}
	}
	return sources
}

func (s *effectScanner) resolveAdapter(port, binding string, receiver types.Type, name string, seen map[string]bool, trail []string) {
	identity := effectTypeID(receiver) + "." + name
	obj, indexes, _ := types.LookupFieldOrMethod(receiver, true, nil, name)
	if len(indexes) > 1 {
		s.resolve(port, binding, effectEmbeddedReceiver(receiver, indexes), name, seen, trail)
		return
	}
	fn, _ := obj.(*types.Func)
	body, ok := s.bodies[fn]
	if !ok || body.declaration.Body == nil {
		s.record(port, binding, "unresolved:"+identity, "", trail)
		return
	}
	normalized, err := normalizeEffectBody(body)
	if err != nil {
		s.record(port, binding, "unresolved:"+identity, "", trail)
		return
	}
	s.walkBody(port, binding, body.pkg, body.declaration.Body, seen, append(trail, identity+":"+normalized))
}

func effectEmbeddedReceiver(receiver types.Type, indexes []int) types.Type {
	for _, index := range indexes[:len(indexes)-1] {
		if pointer, ok := receiver.(*types.Pointer); ok {
			receiver = pointer.Elem()
		}
		shape, ok := receiver.Underlying().(*types.Struct)
		if !ok {
			break
		}
		receiver = shape.Field(index).Type()
	}
	return receiver
}

func normalizeEffectBody(body effectBody) (string, error) {
	var normalized bytes.Buffer
	err := ast.Fprint(&normalized, nil, body.declaration.Body, effectASTField)
	return normalized.String(), err
}

func (s *effectScanner) walkBody(port, binding string, pkg *packages.Package, body *ast.BlockStmt, seen map[string]bool, trail []string) {
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.SelectorExpr:
			s.walkSelector(port, binding, pkg, n, seen, trail)
		case *ast.Ident:
			if fn, ok := pkg.TypesInfo.Uses[n].(*types.Func); ok && fn.Type().(*types.Signature).Recv() == nil {
				s.resolveFunction(port, binding, fn, seen, trail)
			}
		}
		return true
	})
}

func (s *effectScanner) walkSelector(port, binding string, pkg *packages.Package, selector *ast.SelectorExpr, seen map[string]bool, trail []string) {
	selection := pkg.TypesInfo.Selections[selector]
	if selection == nil {
		if fn, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func); ok {
			s.resolveFunction(port, binding, fn, seen, trail)
		}
		return
	}
	if _, ok := selection.Obj().(*types.Func); ok {
		s.resolve(port, binding, pkg.TypesInfo.TypeOf(selector.X), selector.Sel.Name, seen, trail)
	}
	if _, ok := selection.Obj().Type().Underlying().(*types.Signature); ok && selection.Kind() == types.FieldVal {
		s.record(port, binding, "unresolved:function:"+effectTypeID(pkg.TypesInfo.TypeOf(selector.X))+"."+selector.Sel.Name, "", trail)
	}
}

func (s *effectScanner) resolveFunction(port, binding string, fn *types.Func, seen map[string]bool, trail []string) {
	if fn.Pkg() == nil {
		return
	}
	classification := s.policy[fn.Pkg().Path()]
	identity := fn.Pkg().Path() + "." + fn.Name()
	if classification.Owner == "" || classification.Owner == "enrollment" {
		return
	}
	if classification.Owner != "root-composition" {
		if s.crossOwnerTarget(classification.Owner) {
			s.record(port, binding, identity, classification.Owner, append(trail, identity+":"+effectTypeID(fn.Type())))
		}
		return
	}
	if seen[identity] {
		s.record(port, binding, "unresolved:cycle:"+identity, "", trail)
		return
	}
	seen[identity] = true
	defer delete(seen, identity)
	body, ok := s.bodies[fn]
	if !ok || body.declaration.Body == nil {
		s.record(port, binding, "unresolved:"+identity, "", trail)
		return
	}
	var normalized bytes.Buffer
	if err := ast.Fprint(&normalized, nil, body.declaration.Body, effectASTField); err != nil {
		s.record(port, binding, "unresolved:"+identity, "", trail)
		return
	}
	s.walkBody(port, binding, body.pkg, body.declaration.Body, seen, append(trail, identity+":"+normalized.String()))

}

func commandEffectsEnabled(policy *Policy) bool {
	for _, pkg := range policy.Packages {
		if pkg.Owner == "enrollment" {
			return true
		}
	}
	return false
}

func effectFingerprint(evidence string) string {
	hash := fnv.New128a()
	_, _ = hash.Write([]byte(evidence)) // hash.Hash.Write always returns nil.
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func commandEffectViolation(rule string, site commandEffectSite, detail string) Violation {
	return Violation{Scope: ScopeProduction, Rule: "effects." + rule, Source: site.Port, Target: site.Binding + " → " + site.Target, Detail: detail}
}

func validateEffectEntry(entry commandEffectEntry) error {
	if strings.TrimSpace(entry.Reason) == "" {
		return fmt.Errorf("effect entry %s needs exact review evidence", entry.key())
	}
	switch entry.Effect {
	case "query":
		if entry.Classification != "query" {
			return fmt.Errorf("query %s must be classified query", entry.key())
		}
	case "mutation", "lock", "scope", "durable":
		switch entry.Classification {
		case "leaf", "workflow", "E1", "E2", "E3", "E4", "E5":
		default:
			return fmt.Errorf("command %s needs leaf, workflow or named E1–E5 classification", entry.key())
		}
		if entry.ADR != "0067" {
			return fmt.Errorf("command %s must cite ADR 0067", entry.key())
		}
	case "unresolved":
		if !strings.HasPrefix(entry.Target, "unresolved:") || entry.Classification != "unresolved" || entry.ADR != "0089" {
			return fmt.Errorf("unresolved %s needs exact ADR 0089 explanation", entry.key())
		}
	default:
		return fmt.Errorf("unclassified effect %q for %s", entry.Effect, entry.key())
	}
	if strings.HasPrefix(entry.Target, "unresolved:") && entry.Effect != "unresolved" {
		return fmt.Errorf("unresolved binding %s cannot be treated as a query or command", entry.key())
	}
	return nil
}

// Consumer aliases and native foreign contracts in Enrollment inputs count too.
func enrollmentEffectInterfaces(policy *Policy, loaded []*packages.Package) map[types.Type]bool {
	result := map[types.Type]bool{}
	classified := policy.packageMap()
	for _, pkg := range loaded {
		if classified[pkg.PkgPath].Owner != "enrollment" {
			continue
		}
		for _, object := range pkg.TypesInfo.Defs {
			if isEffectInputInterface(object, classified) {
				result[types.Unalias(object.Type())] = true
			}
		}
	}
	return result
}

func isEffectInputInterface(object types.Object, classified map[string]Package) bool {
	if object == nil || effectInterface(object.Type()) == nil {
		return false
	}
	_, declaration := object.(*types.TypeName)
	field, isVariable := object.(*types.Var)
	if !declaration && (!isVariable || !field.IsField()) {
		return false
	}
	named := namedEffectType(object.Type())
	return named != nil && named.Obj().Pkg() != nil && classified[named.Obj().Pkg().Path()].Owner != ""
}

func (s *effectScanner) crossOwnerTarget(owner string) bool {
	switch s.ownerKinds[owner] {
	case "domain", "workflow", "projection":
		return true
	}
	return false
}

func effectASTField(name string, value reflect.Value) bool {
	switch name {
	case "Obj", "Scope", "Doc", "Comment", "Comments":
		return false
	}
	return value.Type() != reflect.TypeFor[token.Pos]()
}
