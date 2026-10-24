package architecture

// policyPermissionMemo reuses pure rule-selection decisions within one import
// comparison. Its policy must remain immutable. Exact package selectors make
// paths part of a permission point; equal owner/role pairs need not share reach.
// Unselected paths still share an owner/role cache key: all package-restricted
// rules reject them. One narrow rule must not expand the cache to every path pair.
type policyPermissionMemo struct {
	policy         *Policy
	firstParty     map[firstPartyPermissionKey]bool
	external       map[externalPermissionKey]bool
	sourcePackages map[string]struct{}
	targetPackages map[string]struct{}
}

type firstPartyPermissionKey struct {
	scope                   Scope
	sourceOwner, sourceRole string
	sourcePath              string
	targetOwner, targetRole string
	targetPath              string
}

type externalPermissionKey struct {
	scope                   Scope
	sourceOwner, sourceRole string
	sourcePath              string
	targetClass             string
}

func newPolicyPermissionMemo(policy *Policy) *policyPermissionMemo {
	memo := &policyPermissionMemo{
		policy:         policy,
		firstParty:     make(map[firstPartyPermissionKey]bool),
		external:       make(map[externalPermissionKey]bool),
		sourcePackages: make(map[string]struct{}),
		targetPackages: make(map[string]struct{}),
	}
	for _, rule := range policy.Rules {
		if rule.SourcePackage != "" {
			memo.sourcePackages[rule.SourcePackage] = struct{}{}
		}
		if rule.TargetPackage != "" {
			memo.targetPackages[rule.TargetPackage] = struct{}{}
		}
	}
	return memo
}

func (m *policyPermissionMemo) allowsFirstParty(scope Scope, source, target Package) bool {
	scoped := source.inScope(scope)
	key := firstPartyPermissionKey{scope: scope, sourceOwner: scoped.Owner, sourceRole: scoped.Role, targetOwner: target.Owner, targetRole: target.Role}
	if _, selected := m.sourcePackages[scoped.Path]; selected {
		key.sourcePath = scoped.Path
	}
	if _, selected := m.targetPackages[target.Path]; selected {
		key.targetPath = target.Path
	}
	if allowed, exists := m.firstParty[key]; exists {
		return allowed
	}
	allowed := policyAllowsFirstParty(m.policy, scope, source, target)
	m.firstParty[key] = allowed
	return allowed
}

func (m *policyPermissionMemo) allowsExternal(scope Scope, source Package, target ExternalPackage) bool {
	scoped := source.inScope(scope)
	key := externalPermissionKey{scope: scope, sourceOwner: scoped.Owner, sourceRole: scoped.Role, targetClass: target.Class}
	if _, selected := m.sourcePackages[scoped.Path]; selected {
		key.sourcePath = scoped.Path
	}
	if allowed, exists := m.external[key]; exists {
		return allowed
	}
	allowed := policyAllowsExternal(m.policy, scope, source, target)
	m.external[key] = allowed
	return allowed
}
