package architecture

// reviewedParentHTTPRule is the finite standing reach of the guardian portal
// HTTP composition (inbound-parent), approved in ADR 0051 for #3421. It grants
// the three runtime points every other HTTP adapter reaches through a standing
// rule: the shared HTTP runtime, the session adapter and the tenant runtime,
// and the session adapter to the handler suites for token construction. It
// never permits application, ORM, model or composition imports.
func reviewedParentHTTPRule(rule Rule) bool {
	if rule.SourceOwner != "inbound-parent" || rule.SourceOwnerKind != "" ||
		rule.TargetOwnerKind != "" || rule.TargetClass != "" || rule.SameOwner {
		return false
	}
	if rule.SourceRole == "adapter-test" {
		return rule.TargetOwner == "identity-access" && rule.TargetRole == "adapter" &&
			len(rule.Scopes) == 2 && sliceContains(rule.Scopes, "internal_test") && sliceContains(rule.Scopes, "external_test")
	}
	if rule.SourceRole != "http" || len(rule.Scopes) != 1 || rule.Scopes[0] != "production" {
		return false
	}
	switch rule.TargetOwner + "/" + rule.TargetRole {
	case "inbound-common/http", "identity-access/adapter", "tenant-runtime/public":
		return true
	default:
		return false
	}
}

func firstPartyAllowedByReviewedParentHTTPRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedParentHTTPRule(*decision.Allowed)
}
