package architecture

// sameOwnerContractRule is one implementation role of a platform owner that
// speaks its own owner's published contract package (ADR 0074, #4461).
type sameOwnerContractRule struct {
	owner         string
	sourceRole    string
	targetPackage string
}

// sameOwnerContractRules are the three directions ADR 0074 approves: Delivery's
// ports and Postgres adapters name the intent and provider values of
// modules/delivery/contract, and the Export Transfer service names the request
// and result values of modules/exporttransfer. Each is pinned to one owner,
// one source role and one target package; the target stays inside the owner,
// so no rule adds an edge to the owner graph.
var sameOwnerContractRules = []sameOwnerContractRule{
	{owner: "delivery-platform", sourceRole: "port", targetPackage: "modules/delivery/contract"},
	{owner: "delivery-platform", sourceRole: "postgres", targetPackage: "modules/delivery/contract"},
	{owner: "export-transfer", sourceRole: "application", targetPackage: "modules/exporttransfer"},
}

// reviewedSameOwnerContractRule reports whether a rule is one of the finite
// same-owner contract directions. It never permits another target owner or
// role, an owner-kind or same_owner selector, an unpinned or different target
// package, a source package, an external class or a scope other than
// production.
func reviewedSameOwnerContractRule(rule Rule) bool {
	if rule.SourceOwnerKind != "" || rule.TargetOwnerKind != "" || rule.SourcePackage != "" || rule.TargetClass != "" ||
		rule.SameOwner || rule.TargetOwner != rule.SourceOwner || rule.TargetRole != "contract" ||
		len(rule.Scopes) != 1 || rule.Scopes[0] != string(ScopeProduction) {
		return false
	}
	for _, allowed := range sameOwnerContractRules {
		if rule.SourceOwner == allowed.owner && rule.SourceRole == allowed.sourceRole && rule.TargetPackage == allowed.targetPackage {
			return true
		}
	}
	return false
}

func firstPartyAllowedByReviewedSameOwnerContractRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedSameOwnerContractRule(*decision.Allowed)
}
