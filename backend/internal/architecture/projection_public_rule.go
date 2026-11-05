package architecture

// reviewedProjectionPublicRule is the ADR 0058 / #4314 response-projection
// reach: an existing projection's public capability may read native domain
// and platform public/contract facts. It grants no reverse or private reach.
func reviewedProjectionPublicRule(rule Rule, owners map[string]Owner) bool {
	source, sourceKnown := owners[rule.SourceOwner]
	target, targetKnown := owners[rule.TargetOwner]
	return sourceKnown && targetKnown && source.Kind == "projection" &&
		(target.Kind == "domain" || target.Kind == "platform") &&
		rule.SourceOwnerKind == "" && rule.SourceRole == "public" &&
		rule.TargetOwnerKind == "" && (rule.TargetRole == "public" || rule.TargetRole == "contract") &&
		rule.TargetClass == "" && !rule.SameOwner &&
		len(rule.Scopes) == 1 && rule.Scopes[0] == string(ScopeProduction)
}
