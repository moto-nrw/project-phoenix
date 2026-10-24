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
	SourceOwner   string `json:"source_owner,omitempty"`
	Port          string `json:"port"`
	Binding       string `json:"binding"`
	Target        string `json:"target"`
	TargetOwner   string `json:"target_owner"`
	BindingDigest string `json:"binding_digest"`
}

func (s commandEffectSite) key() string {
	return s.sourceOwner() + "|" + s.Port + "|" + s.Binding + "|" + s.Target + "|" + s.TargetOwner
}

// Older Enrollment-only registers did not carry a source owner.
func (s commandEffectSite) sourceOwner() string {
	if s.SourceOwner == "" {
		return "enrollment"
	}
	return s.SourceOwner
}

type effectBody struct {
	pkg         *packages.Package
	declaration *ast.FuncDecl
}
type effectScanner struct {
	sourceOwner string
	ownerKinds  map[string]string
	policy      map[string]Package
	bindings    effectBindings
	values      effectValues
	bodies      map[*types.Func]effectBody
	sites       map[string]commandEffectSite
	evidence    map[string]map[string]bool
	callables   map[types.Object][]string
	// Port-independent summaries of the owner's own functions (M3).
	ownSummaries map[*types.Func]ownEffectSummary
	ownCallables map[*types.Func][]types.Object
	// Port-independent callable resolutions, replayed for every port.
	capture        *effectCapture
	callableSites  map[string]*effectCapture
	callableActive map[string]bool
	// storedFlow counts enclosing resolutions of stored or constructed
	// callables; see resolveCallable.
	storedFlow int
}

func effectTypeID(t types.Type) string {
	return types.TypeString(types.Unalias(t), func(p *types.Package) string { return p.Path() })
}

// Every domain's named input interfaces are inventoried, including aliases and
// native foreign contract fields. All typed production packages supply bindings.
func discoverCommandEffects(policy *Policy, loaded []*packages.Package) []commandEffectSite {
	s := newEffectScanner(policy, loaded)
	for owner, scoped := range domainEffectInterfaces(policy, loaded) {
		s.sourceOwner = owner
		for destination, sources := range s.bindings {
			if !scoped[destination] {
				continue
			}
			for _, source := range sources {
				if types.Identical(destination, source) && s.receiverOwner(source) == owner {
					continue
				}
				s.resolvePort(destination, source)
			}
		}
	}
	return s.sortedSites()
}

func newEffectScanner(policy *Policy, loaded []*packages.Package) *effectScanner {
	s := &effectScanner{ownerKinds: map[string]string{}, policy: policy.packageMap(), bindings: collectEffectBindings(loaded), values: collectEffectValues(loaded), bodies: map[*types.Func]effectBody{}, sites: map[string]commandEffectSite{}, evidence: map[string]map[string]bool{}, callables: map[types.Object][]string{}, ownSummaries: map[*types.Func]ownEffectSummary{}, ownCallables: map[*types.Func][]types.Object{}, callableSites: map[string]*effectCapture{}, callableActive: map[string]bool{}}
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
	if s.capture != nil {
		s.capture.add(target, owner, trail)
		return
	}
	site := commandEffectSite{SourceOwner: s.sourceOwner, Port: port, Binding: binding, Target: target, TargetOwner: owner}
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
	// A may-bind union is a least fixpoint: the identity's first visit on this
	// path already explores every source, so re-entering it adds no target.
	if seen[identity] {
		return
	}
	seen[identity] = true
	defer delete(seen, identity)
	// External/builtin contracts (notably error.Error) are outside the policy
	// binding surface. Unioning every error return invents unrelated owner edges.
	if named := namedEffectType(receiver); named != nil {
		pkg := named.Obj().Pkg()
		if pkg == nil || s.policy[pkg.Path()].Owner == "" {
			return
		}
	}
	owner := s.receiverOwner(receiver)
	if owner == s.sourceOwner {
		if effectInterface(receiver) != nil {
			s.resolveInterface(port, binding, receiver, name, seen, trail)
		} else {
			s.resolveOwnMethod(port, binding, receiver, name, seen, trail)
		}
		return
	}
	if owner != "" && owner != "root-composition" {
		s.recordTerminal(port, binding, receiver, name, owner, trail)
		if effectInterface(receiver) != nil {
			s.resolveForeignProviders(port, binding, receiver, name, seen, trail)
		}
		return
	}
	if effectInterface(receiver) != nil {
		s.resolveInterface(port, binding, receiver, name, seen, trail)
		return
	}
	s.resolveAdapter(port, binding, receiver, name, seen, trail)
}

// Unexported methods and fields are only visible from their declaring package.
func effectTypePackage(receiver types.Type) *types.Package {
	if named := namedEffectType(receiver); named != nil {
		return named.Obj().Pkg()
	}
	return nil
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
	if obj, _, _ := types.LookupFieldOrMethod(receiver, true, effectTypePackage(receiver), name); obj != nil {
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
		if s.receiverOwner(receiver) == s.sourceOwner {
			return
		}
		s.record(port, binding, "unresolved:"+effectTypeID(receiver)+"."+name, "", trail)
		return
	}
	for _, source := range sources {
		s.resolve(port, binding, source, name, seen, trail)
	}
}

// A foreign contract remains a first-hop terminal. Its root-bound provider can
// nevertheless belong to another owner (notably a workflow); expose that fact
// without walking the foreign owner's command implementation.
func (s *effectScanner) resolveForeignProviders(port, binding string, receiver types.Type, name string, seen map[string]bool, trail []string) {
	owner := s.receiverOwner(receiver)
	for destination, candidates := range s.bindings {
		if !types.Identical(destination, receiver) {
			continue
		}
		for _, source := range distinctEffectSources(receiver, candidates) {
			if s.receiverOwner(source) == owner {
				if effectInterface(source) == nil || seen[effectTypeID(source)+"."+name] {
					continue
				}
			}
			s.resolve(port, binding, source, name, seen, trail)
		}
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
	obj, indexes, _ := types.LookupFieldOrMethod(receiver, true, effectTypePackage(receiver), name)
	if len(indexes) > 1 {
		s.resolve(port, binding, effectEmbeddedReceiver(receiver, indexes), name, seen, trail)
		return
	}
	fn, _ := obj.(*types.Func)
	if fn != nil {
		fn = fn.Origin()
	}
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
	return normalizeEffectNode(body.declaration.Body)
}

func normalizeEffectNode(node ast.Node) (string, error) {
	var normalized bytes.Buffer
	err := ast.Fprint(&normalized, nil, node, effectASTField)
	return normalized.String(), err
}

func (s *effectScanner) walkBody(port, binding string, pkg *packages.Package, body *ast.BlockStmt, seen map[string]bool, trail []string) {
	known := knownEffectCallables(pkg, body)
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			s.walkCallable(port, binding, pkg, n, seen, trail, known)
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
	s.walkCallableField(port, binding, pkg, selector, selection, seen, trail)
}

func (s *effectScanner) walkCallableField(port, binding string, pkg *packages.Package, selector *ast.SelectorExpr, selection *types.Selection, seen map[string]bool, trail []string) {
	if selection.Kind() != types.FieldVal || !effectCallableCarrier(selection.Obj().Type(), map[types.Type]bool{}) {
		return
	}
	s.resolveOrDiagnose(port, binding, selection.Obj(), "unresolved:function:"+effectTypeID(pkg.TypesInfo.TypeOf(selector.X))+"."+selector.Sel.Name, trail)
}

func (s *effectScanner) resolveFunction(port, binding string, fn *types.Func, seen map[string]bool, trail []string) {
	if fn.Pkg() == nil {
		return
	}
	classification := s.policy[fn.Pkg().Path()]
	identity := fn.Pkg().Path() + "." + fn.Name()
	if classification.Owner == s.sourceOwner {
		// A root body calling the owner's own function hands it callables;
		// follow those it invokes, never the owner's other effects.
		s.resolveOwnCallables(port, binding, s.ownReachable(fn.Origin()), seen, trail)
		return
	}
	if classification.Owner == "" {
		return
	}
	if classification.Owner != "root-composition" {
		if s.crossOwnerTarget(classification.Owner) && (s.ownerKinds[classification.Owner] != "platform" || platformProtocolFunction(fn)) {
			s.record(port, binding, identity, classification.Owner, append(trail, identity+":"+effectTypeID(fn.Type())))
		}
		return
	}
	if seen[identity] {
		return // Recursive root helpers: see the fixpoint note in resolve.
	}
	seen[identity] = true
	defer delete(seen, identity)
	body, ok := s.bodies[fn.Origin()]
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

// Platform methods are protocol contracts and always inventoried. A platform
// package function enters a protocol when it runs or registers a callback
// (transaction, savepoint, after-commit, retry), can fail (lock acquisition,
// current-unit lookup) or changes the unit carried by its context (rollback
// marks). Pure value helpers and context-free metric sinks are not protocol
// effects (#4836, ADR 0098).
func platformProtocolFunction(fn *types.Func) bool {
	signature := fn.Signature()
	takesContext := false
	for i := 0; i < signature.Params().Len(); i++ {
		parameter := signature.Params().At(i).Type()
		if effectCallableCarrier(parameter, map[types.Type]bool{}) {
			return true
		}
		if named := namedEffectType(parameter); named != nil && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context" {
			takesContext = true
		}
	}
	for i := 0; i < signature.Results().Len(); i++ {
		if types.Identical(signature.Results().At(i).Type(), types.Universe.Lookup("error").Type()) {
			return true
		}
	}
	return signature.Results().Len() == 0 && takesContext
}

func commandEffectsEnabled(policy *Policy) bool {
	for _, owner := range policy.Owners {
		if owner.Kind == "domain" {
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
	return Violation{Scope: ScopeProduction, Rule: "effects." + rule, Source: site.sourceOwner() + ": " + site.Port, Target: site.Binding + " → " + site.Target, Detail: detail}
}

func validateEffectEntry(entry commandEffectEntry) error {
	if strings.TrimSpace(entry.Reason) == "" {
		return fmt.Errorf("effect entry %s needs exact review evidence", entry.key())
	}
	if strings.HasPrefix(entry.Target, "unresolved:") && entry.Effect != "unresolved" {
		return fmt.Errorf("unresolved binding %s cannot be treated as a query or command", entry.key())
	}
	switch entry.Effect {
	case "query":
		switch entry.Classification {
		case "query":
		case "debt", "workflow", "E1", "E2", "E3", "E4", "E5":
			return validateCommandEffectEntry(entry)
		default:
			return fmt.Errorf("query %s must be classified query, debt or a named exception", entry.key())
		}
	case "mutation", "lock", "scope", "durable", "notice":
		return validateCommandEffectEntry(entry)
	case "unresolved":
		if !strings.HasPrefix(entry.Target, "unresolved:") || entry.Classification != "unresolved" || entry.ADR != "0083" {
			return fmt.Errorf("unresolved %s needs exact ADR 0083 addendum #4739 explanation", entry.key())
		}
	default:
		return fmt.Errorf("unclassified effect %q for %s", entry.Effect, entry.key())
	}
	return nil
}

// Platform protocol effects (#4836, ADR 0098) are reviewed participation in
// an infrastructure protocol, not a cross-owner command or an exception.
const platformEffectADR = "0098"

func validateCommandEffectEntry(entry commandEffectEntry) error {
	switch entry.Classification {
	case "platform":
		if entry.ADR != platformEffectADR {
			return fmt.Errorf("platform effect %s must cite ADR %s", entry.key(), platformEffectADR)
		}
	case "debt":
		if entry.ADR != "0083" {
			return fmt.Errorf("known debt %s must cite ADR 0083 without granting an exception", entry.key())
		}
	case "leaf", "workflow", "E1", "E2", "E3", "E4", "E5":
		if entry.ADR != "0067" {
			return fmt.Errorf("command %s must cite ADR 0067", entry.key())
		}
	default:
		return fmt.Errorf("command %s needs leaf, workflow, platform, known debt or named E1–E5 classification", entry.key())
	}
	return nil
}

func domainEffectInterfaces(policy *Policy, loaded []*packages.Package) map[string]map[types.Type]bool {
	result := map[string]map[types.Type]bool{}
	classified := policy.packageMap()
	domains := map[string]bool{}
	for _, owner := range policy.Owners {
		domains[owner.ID] = owner.Kind == "domain"
	}
	for _, pkg := range loaded {
		owner := classified[pkg.PkgPath].Owner
		if !domains[owner] {
			continue
		}
		if result[owner] == nil {
			result[owner] = map[types.Type]bool{}
		}
		for _, object := range pkg.TypesInfo.Defs {
			if isEffectInputInterface(object, classified) {
				result[owner][types.Unalias(object.Type())] = true
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
	case "domain", "workflow", "projection", "platform":
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

func (s *effectScanner) walkCallable(port, binding string, pkg *packages.Package, call *ast.CallExpr, seen map[string]bool, trail []string, known map[*types.Var]bool) {
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return
	}
	variable, ok := pkg.TypesInfo.Uses[id].(*types.Var)
	if !ok || known[variable] {
		return
	}
	if _, callable := variable.Type().Underlying().(*types.Signature); callable {
		s.resolveOrDiagnose(port, binding, variable, "unresolved:callable:"+pkg.PkgPath+"."+id.Name, trail)
	}
}
