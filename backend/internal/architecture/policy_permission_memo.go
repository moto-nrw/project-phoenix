package architecture

// policyPermissionMemo reuses pure source-rule selection within one import
// comparison. Its policy must remain immutable. Exact package selectors make
// paths part of a permission point; equal owner/role pairs need not share reach.
// Cache source shortlists and sparse target unions, not the Cartesian
// product of source/target points: most pairs are denied, and retaining them
// costs more than checking the few rules selected by the source.
type policyPermissionMemo struct {
	policy         *Policy
	sources        map[sourcePermissionKey][]*Rule
	sourcePackages map[string]struct{}
	targetRoles    map[string]map[string]Package
	targets        map[sourcePermissionKey]map[string]Package
}

type sourcePermissionKey struct {
	scope       Scope
	owner, role string
	path        string
}

func newPolicyPermissionMemo(policy *Policy) *policyPermissionMemo {
	memo := &policyPermissionMemo{
		policy:         policy,
		sources:        make(map[sourcePermissionKey][]*Rule),
		sourcePackages: make(map[string]struct{}),
	}
	for _, rule := range policy.Rules {
		if rule.SourcePackage != "" {
			memo.sourcePackages[rule.SourcePackage] = struct{}{}
		}
	}
	return memo
}

// sourceRules receives the scope-adjusted source. Unselected paths share a
// key because every package-restricted source rule rejects all of them.
func (m *policyPermissionMemo) sourceRules(scope Scope, source Package) []*Rule {
	key := m.sourceKey(scope, source)
	if rules, exists := m.sources[key]; exists {
		return rules
	}
	var rules []*Rule
	for i := range m.policy.Rules {
		rule := &m.policy.Rules[i]
		if m.policy.matchesSource(*rule, source) && ruleHasScope(*rule, scope) {
			rules = append(rules, rule)
		}
	}
	m.sources[key] = rules
	return rules
}

func (m *policyPermissionMemo) allowsFirstParty(scope Scope, source, target Package) bool {
	return m.firstPartyRule(scope, source, target) != nil
}

// firstPartyRule retains the unique decision for the reviewed-rule checks.
// An overlap is denied, just like decideRules, never a first-match permission.
func (m *policyPermissionMemo) firstPartyRule(scope Scope, source, target Package) *Rule {
	source = source.inScope(scope)
	var matched *Rule
	for _, rule := range m.sourceRules(scope, source) {
		if rule.TargetClass != "" || !m.policy.matchesTarget(*rule, source, target) {
			continue
		}
		if matched != nil {
			return nil
		}
		matched = rule
	}
	return matched
}

func (m *policyPermissionMemo) allowsExternal(scope Scope, source Package, target ExternalPackage) bool {
	matched := false
	for _, rule := range m.sourceRules(scope, source.inScope(scope)) {
		if rule.TargetClass != target.Class {
			continue
		}
		if matched {
			return false
		}
		matched = true
	}
	return matched
}

func (m *policyPermissionMemo) sourceKey(scope Scope, source Package) sourcePermissionKey {
	key := sourcePermissionKey{scope: scope, owner: source.Owner, role: source.Role}
	if _, selected := m.sourcePackages[source.Path]; selected {
		key.path = source.Path
	}
	return key
}

// firstPartyTargets is the union selected by this source's first-party rules.
// Targets excluded by every rule cannot be newly allowed. Cache only this
// sparse union, never the Cartesian product of denied source/target pairs.
// The ordinary evaluator still rejects overlapping matches at each endpoint.
func (m *policyPermissionMemo) firstPartyTargets(scope Scope, source Package) map[string]Package {
	source = source.inScope(scope)
	key := m.sourceKey(scope, source)
	if targets, exists := m.targets[key]; exists {
		return targets
	}
	if m.targets == nil {
		m.targets = make(map[sourcePermissionKey]map[string]Package)
		m.targetRoles = make(map[string]map[string]Package)
		for path, target := range m.policy.packageMap() {
			role := m.targetRoles[target.Role]
			if role == nil {
				role = make(map[string]Package)
				m.targetRoles[target.Role] = role
			}
			role[path] = target
		}
	}
	targets := make(map[string]Package)
	for _, rule := range m.sourceRules(scope, source) {
		if rule.TargetClass != "" {
			continue
		}
		for path, target := range m.targetRoles[rule.TargetRole] {
			if m.policy.matchesTarget(*rule, source, target) {
				targets[path] = target
			}
		}
	}
	m.targets[key] = targets
	return targets
}
