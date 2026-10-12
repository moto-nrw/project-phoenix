package architecture

// reviewedCommonHTTPRule is the finite standing reach of api/common, approved
// in ADR 0046. It permits owner contracts for HTTP plumbing, never application,
// ORM or retained-model imports. Only session construction needs a test rule;
// the other owner public/contract test reaches already exist.
func reviewedCommonHTTPRule(rule Rule) bool {
	if rule.SourceOwner != "inbound-common" || rule.SourceOwnerKind != "" ||
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
	case "identity-access/adapter", "tenant-runtime/public", "security-runtime/contract",
		"security-runtime/public", "settings-platform/public", "delivery-platform/public", "transaction-runtime/public":
		return true
	default:
		return false
	}
}

func firstPartyAllowedByReviewedCommonHTTPRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedCommonHTTPRule(*decision.Allowed)
}
