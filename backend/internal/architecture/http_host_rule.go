package architecture

// httpHostRule is one standing reach of the typed roots as the HTTP host
// (ADR 0056, #4063). Exactly one of target and class is set.
type httpHostRule struct {
	sourceRole string
	target     string
	class      string
	scope      string
}

// httpHostRules are the reaches the root needs to host HTTP beyond ADR 0053
// point 2: the router class, the shared HTTP runtime, the session token
// adapter, the CLI class in every internal test of the typed roots, and the
// dev tools the commands root starts. Each is pinned to one source role and
// one scope.
var httpHostRules = []httpHostRule{
	{sourceRole: "compose", class: "http-router", scope: "production"},
	{sourceRole: "compose", target: "inbound-common/http", scope: "production"},
	{sourceRole: "compose", target: "identity-access/adapter", scope: "production"},
	{sourceRole: "module-internal-test", target: "identity-access/adapter", scope: "internal_test"},
	{sourceRole: "module-internal-test", class: "cli-config", scope: "internal_test"},
	{sourceRole: "cli", target: "dev-tools/cli", scope: "production"},
}

// reviewedHTTPHostRule reports whether a rule is one of the finite standing
// reaches of the HTTP host. It never permits an application, domain, postgres
// or ORM target, another source owner, an owner kind or a wider scope.
func reviewedHTTPHostRule(rule Rule) bool {
	if rule.SourceOwner != rootCompositionOwner || rule.SourceOwnerKind != "" ||
		rule.TargetOwnerKind != "" || rule.SameOwner || len(rule.Scopes) != 1 {
		return false
	}
	target := ""
	if rule.TargetOwner != "" || rule.TargetRole != "" {
		target = rule.TargetOwner + "/" + rule.TargetRole
	}
	for _, allowed := range httpHostRules {
		if rule.SourceRole == allowed.sourceRole && target == allowed.target &&
			rule.TargetClass == allowed.class && rule.Scopes[0] == allowed.scope {
			return true
		}
	}
	return false
}

func firstPartyAllowedByReviewedHTTPHostRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedHTTPHostRule(*decision.Allowed)
}

func externalAllowedByReviewedHTTPHostRule(candidate *Policy, scope Scope, source Package, target ExternalPackage) bool {
	decision := decideRules(candidate.externalRules(scope, source.inScope(scope), target.Class))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedHTTPHostRule(*decision.Allowed)
}
