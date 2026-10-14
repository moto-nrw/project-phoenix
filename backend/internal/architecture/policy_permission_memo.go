package architecture

// policyPermissionMemo reuses pure rule-selection decisions within one import
// comparison. Its policy must remain immutable. Package paths are deliberately
// absent: rules select scoped owner/role points, while reviewed cutover
// exceptions and violation identities are still evaluated for each package edge.
type policyPermissionMemo struct {
	policy     *Policy
	firstParty map[firstPartyPermissionKey]bool
	external   map[externalPermissionKey]bool
}

type firstPartyPermissionKey struct {
	scope                   Scope
	sourceOwner, sourceRole string
	targetOwner, targetRole string
}

type externalPermissionKey struct {
	scope                   Scope
	sourceOwner, sourceRole string
	targetClass             string
}

func newPolicyPermissionMemo(policy *Policy) *policyPermissionMemo {
	return &policyPermissionMemo{
		policy:     policy,
		firstParty: make(map[firstPartyPermissionKey]bool),
		external:   make(map[externalPermissionKey]bool),
	}
}

func (m *policyPermissionMemo) allowsFirstParty(scope Scope, source, target Package) bool {
	scoped := source.inScope(scope)
	key := firstPartyPermissionKey{scope, scoped.Owner, scoped.Role, target.Owner, target.Role}
	if allowed, exists := m.firstParty[key]; exists {
		return allowed
	}
	allowed := policyAllowsFirstParty(m.policy, scope, source, target)
	m.firstParty[key] = allowed
	return allowed
}

func (m *policyPermissionMemo) allowsExternal(scope Scope, source Package, target ExternalPackage) bool {
	scoped := source.inScope(scope)
	key := externalPermissionKey{scope, scoped.Owner, scoped.Role, target.Class}
	if allowed, exists := m.external[key]; exists {
		return allowed
	}
	allowed := policyAllowsExternal(m.policy, scope, source, target)
	m.external[key] = allowed
	return allowed
}
