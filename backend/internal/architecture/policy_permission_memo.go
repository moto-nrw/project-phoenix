package architecture

// policyPermissionMemo reuses pure source-rule selection within one import
// comparison. Its policy must remain immutable. Exact package selectors make
// paths part of a permission point; equal owner/role pairs need not share reach.
// Cache only source shortlists, not the Cartesian product of source/target
// points: most pairs are denied, and retaining them costs more than checking
// the few rules selected by the source.
type policyPermissionMemo struct {
	policy         *Policy
	sources        map[sourcePermissionKey][]*Rule
	sourcePackages map[string]struct{}
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
	key := sourcePermissionKey{scope: scope, owner: source.Owner, role: source.Role}
	if _, selected := m.sourcePackages[source.Path]; selected {
		key.path = source.Path
	}
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
	source = source.inScope(scope)
	matched := false
	for _, rule := range m.sourceRules(scope, source) {
		if rule.TargetClass != "" || !m.policy.matchesTarget(*rule, source, target) {
			continue
		}
		if matched {
			return false // An overlap is denied, just like decideRules.
		}
		matched = true
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
