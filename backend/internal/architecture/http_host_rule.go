package architecture

// httpHostRule is one standing external-class reach of the typed roots as
// the HTTP host (ADR 0056, #4063).
type httpHostRule struct {
	sourceRole string
	class      string
	scope      string
}

// httpHostRules are the reaches the root needs to host HTTP beyond ADR 0053
// point 2: the router class and the CLI class in
// every internal test of the typed roots. Each is pinned to one source role
// and one scope. ADR 0057 removed two of ADR 0056's reaches: the session token
// runtime is Identity's http role, which point 2 already covers, and the dev
// tools' cli role is another owner's private role, so their two imports are
// keys (#4096).
var httpHostRules = []httpHostRule{
	{sourceRole: "compose", class: "http-router", scope: "production"},
	{sourceRole: "module-internal-test", class: "cli-config", scope: "internal_test"},
}

// reviewedHTTPHostRule reports whether a rule is one of the finite standing
// reaches of the HTTP host. It never permits an application, domain, postgres
// or ORM target, another source owner, an owner kind or a wider scope.
func reviewedHTTPHostRule(rule Rule) bool {
	if rule.SourceOwner != rootCompositionOwner || rule.SourceOwnerKind != "" ||
		rule.TargetOwnerKind != "" || rule.TargetOwner != "" || rule.TargetRole != "" || rule.SameOwner || len(rule.Scopes) != 1 {
		return false
	}
	for _, allowed := range httpHostRules {
		if rule.SourceRole == allowed.sourceRole &&
			rule.TargetClass == allowed.class && rule.Scopes[0] == allowed.scope {
			return true
		}
	}
	return false
}

func externalAllowedByReviewedHTTPHostRule(candidate *Policy, scope Scope, source Package, target ExternalPackage) bool {
	decision := decideRules(candidate.externalRules(scope, source.inScope(scope), target.Class))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedHTTPHostRule(*decision.Allowed)
}
