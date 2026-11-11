package architecture

import (
	"go/ast"
	"go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// parentRenewalGuardViolations extends the rule of #6529 to the renewal of
// an existing child (#6638). A parent's submission or edit pins an enrolled
// child on the parent's own authority; that authority is held behind the
// parent guard (guardianhold.HoldMany) until the request commits. The rule reads
// the call graph of Enrollment's intake: every exported Intake method that
// resolves a pin and writes request children must reach the renewal's own
// check (holdGuardianReEnrollment), the entry must reach both batched held
// variants; no such method may reach the unlocked renewal check, which
// only the staff's later approvals ask (approvals and change-request
// decisions, outside this rule). A rule that finds no such entry, or no
// intake package while Enrollment is loaded, is reported as stale, so a
// renamed port or a moved package cannot switch it off.
func parentRenewalGuardViolations(project, modulePath string, loaded []*packages.Package) []Violation {
	intake, enrollmentLoaded := renewalIntakePackage(modulePath, loaded)
	if intake == nil {
		if enrollmentLoaded {
			return []Violation{parentRenewalViolation("parent-renewal.stale", renewalPackage, "package not loaded", nil)}
		}
		return nil
	}
	graph := newFlowGraph([]*packages.Package{intake})
	graph.project = project
	var violations []Violation
	renewing := 0
	for _, root := range graph.methodsOf("Intake") {
		found, renews := graph.renewalRootViolations(root)
		if renews {
			renewing++
		}
		violations = append(violations, found...)
	}
	if renewing == 0 {
		violations = append(violations, parentRenewalViolation("parent-renewal.stale", renewalPackage, renewalPinPort+"+"+renewalChildWrite, nil))
	}
	return violations
}

// renewalIntakePackage finds Enrollment's intake among the loaded packages
// and reports whether any Enrollment package is loaded at all.
func renewalIntakePackage(modulePath string, loaded []*packages.Package) (*packages.Package, bool) {
	enrollmentLoaded := false
	for _, pkg := range loaded {
		if pkg.PkgPath == modulePath+"/"+renewalPackage {
			return pkg, true
		}
		enrollmentLoaded = enrollmentLoaded || strings.HasPrefix(pkg.PkgPath, modulePath+"/modules/enrollment")
	}
	return nil, enrollmentLoaded
}

// renewalRootViolations checks one Intake entry; renews reports whether it
// resolves a pin and writes request children at all.
func (g *flowGraph) renewalRootViolations(root *types.Func) (violations []Violation, renews bool) {
	locals, ports := g.reach(root)
	if !ports[renewalPinPort] || !ports[renewalChildWrite] {
		return nil, false
	}
	name := g.name(root)
	missing := missingKeys(ports, renewalHolds)
	if !locals[renewalCheck] {
		missing = []string{renewalCheck}
	}
	if len(missing) > 0 {
		violations = append(violations, parentRenewalViolation("parent-renewal.unguarded", name, strings.Join(missing, ","), g.location(root)))
	}
	if reached := presentKeys(locals, renewalUnlockedChecks); len(reached) > 0 {
		violations = append(violations, parentRenewalViolation("parent-renewal.unlocked-check", name, strings.Join(reached, ","), g.location(root)))
	}
	return violations, true
}

const (
	renewalPackage    = "modules/enrollment/internal/application"
	renewalPinPort    = "application.StudentMatches.FindEnrolledStudentIDsByIdentities"
	renewalChildWrite = "application.IntakeChildren.InsertChild"
	renewalCheck      = "application.Intake.holdGuardianReEnrollment"
)

// renewalHolds are the held checks of the account and of the late invite's
// address.
var renewalHolds = []string{
	"application.GuardianStudentHold.HoldAccountStudentsPermission",
	"application.GuardianStudentHold.HoldGuardianEmailStudentsPermission",
}

// renewalUnlockedChecks ask the renewal permission without locks.
var renewalUnlockedChecks = []string{
	"application.Intake.assertGuardianMayReEnrollStudent",
	"application.submitterMayRenewStudent",
}

func parentRenewalViolation(rule, source, target string, locations []Location) Violation {
	return Violation{
		Scope: ScopeProduction, Rule: rule, Source: source, Target: target, Locations: locations,
		Detail: "a parent's renewal of an existing child holds the parent's authority behind the parent guard until it commits (#6638): ask GuardianStudentHold, not the unlocked check",
	}
}

// methodsOf are the exported methods of the named type, sorted by name.
func (g *flowGraph) methodsOf(typeName string) []*types.Func {
	var methods []*types.Func
	for fn := range g.decls {
		recv := fn.Type().(*types.Signature).Recv()
		if !fn.Exported() || recv == nil {
			continue
		}
		if named := namedOf(recv.Type()); named != nil && named.Obj().Name() == typeName {
			methods = append(methods, fn)
		}
	}
	sort.Slice(methods, func(i, j int) bool { return g.name(methods[i]) < g.name(methods[j]) })
	return methods
}

// reach returns the functions of the graph and the module interface methods
// root reaches, closures and method values included.
func (g *flowGraph) reach(root *types.Func) (locals, ports map[string]bool) {
	locals, ports = map[string]bool{}, map[string]bool{}
	seen := map[*types.Func]bool{root: true}
	queue := []*types.Func{root}
	for len(queue) > 0 {
		fn := queue[0]
		queue = queue[1:]
		info := g.infos[fn]
		ast.Inspect(g.decls[fn].Body, func(n ast.Node) bool {
			ident := identOf(asExpr(n))
			if ident == nil {
				return true
			}
			obj, ok := info.Uses[ident].(*types.Func)
			if !ok {
				return true
			}
			if _, local := g.decls[obj]; local {
				locals[g.name(obj)] = true
				if !seen[obj] {
					seen[obj] = true
					queue = append(queue, obj)
				}
			} else if port := interfaceMethodName(obj); port != "" {
				ports[port] = true
			}
			return true
		})
	}
	return locals, ports
}

func asExpr(n ast.Node) ast.Expr {
	expr, _ := n.(ast.Expr)
	return expr
}

func missingKeys(set map[string]bool, keys []string) []string {
	var out []string
	for _, key := range keys {
		if !set[key] {
			out = append(out, key)
		}
	}
	return out
}

func presentKeys(set map[string]bool, keys []string) []string {
	var out []string
	for _, key := range keys {
		if set[key] {
			out = append(out, key)
		}
	}
	return out
}
