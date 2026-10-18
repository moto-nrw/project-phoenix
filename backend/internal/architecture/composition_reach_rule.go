package architecture

// The standing composition and test reaches of ADR 0053 (#4001). Each reach is
// a finite selector set. Its acceptance function admits a candidate rule when
// everything the rule allows lies inside the reach: a subset test over the
// source owner or kind, the source role, the target owner or kind, the target
// role, same_owner and the scopes. A narrower rule is accepted; a rule that
// reaches one point outside is not. The comparison admits these rules only in
// an increased epoch.

// rootCompositionOwner owns the typed roots: `.`, `api` and `cmd`.
const rootCompositionOwner = "root-composition"

var (
	// rootReachTargetKinds and rootReachTargetRoles span the owner points the
	// typed roots compose (ADR 0053 point 2). Another owner's application,
	// domain, adapter, postgres or cli role, the legacy composition and the
	// tool owners stay outside.
	rootReachTargetKinds = map[string]bool{"domain": true, "platform": true, "workflow": true, "projection": true}
	rootReachTargetRoles = map[string]bool{"compose": true, "public": true, "contract": true, "port": true, "http": true}

	// ownTestTargetRoles are the roles of its own owner a test package may
	// import, and ownTestSupportTargetRoles those an owner's test support may
	// import in production (ADR 0053 point 7.1).
	ownTestTargetRoles        = map[string]bool{"public": true, "contract": true, "compose": true, "port": true, "adapter": true, "test-support": true}
	ownTestSupportTargetRoles = map[string]bool{"public": true, "contract": true, "compose": true, "port": true, "adapter": true}

	reachProductionScopes = []string{string(ScopeProduction)}
	reachTestScopes       = []string{string(ScopeInternalTest), string(ScopeExternalTest)}
)

// reviewedRootCompositionRule reports whether a rule lies inside the standing
// reach of the typed roots (ADR 0053 point 2): any role of root-composition,
// in any scope, importing the compose, public, contract, port or http role of
// a domain, platform, workflow or projection owner, the migrations, or the
// root's own packages.
func reviewedRootCompositionRule(rule Rule, owners map[string]Owner) bool {
	if rule.SourceOwner != rootCompositionOwner || rule.SourceOwnerKind != "" || rule.TargetClass != "" || len(rule.Scopes) == 0 {
		return false
	}
	switch {
	case rule.SameOwner:
		return rule.TargetOwnerKind == "" && (rule.TargetOwner == "" || rule.TargetOwner == rootCompositionOwner)
	case rule.TargetOwner == rootCompositionOwner:
		return true
	case rule.TargetOwner == "migrations":
		return rule.TargetRole == "migration"
	case rule.TargetOwner != "":
		return rootReachTargetKinds[owners[rule.TargetOwner].Kind] && rootReachTargetRoles[rule.TargetRole]
	default:
		return rootReachTargetKinds[rule.TargetOwnerKind] && rootReachTargetRoles[rule.TargetRole]
	}
}

// reviewedOwnerCompositionRule reports whether a rule lies inside the
// standing reach of an owner's composition (ADR 0053 point 3): the compose
// role importing any role of its own owner, in production only. It grants
// nothing across owners. A composition-kind source stays outside: the typed
// roots reach their own packages through reviewedRootCompositionRule, and the
// legacy composition gains no reach (ADR 0053 point 1).
func reviewedOwnerCompositionRule(rule Rule, owners map[string]Owner) bool {
	return rule.SourceRole == "compose" && targetsOwnOwner(rule) && rule.TargetClass == "" &&
		sourceExcludesComposition(rule, owners) && len(rule.Scopes) > 0 && scopesContain(reachProductionScopes, rule.Scopes)
}

// reviewedOwnTestRule reports whether a rule lies inside the own-owner test
// reach (ADR 0053 point 7.1): every test package of an owner, in both test
// scopes, importing that owner's public, contract, compose, port, adapter or
// test-support role, and the owner's test support, in production, importing
// its public, contract, compose, port or adapter role. Like ADR 0053, it
// names no owner kind: the repository's rules select non-composition kinds,
// because the root's own tests already reach the root through
// reviewedRootCompositionRule.
func reviewedOwnTestRule(rule Rule) bool {
	if !targetsOwnOwner(rule) || rule.TargetClass != "" || len(rule.Scopes) == 0 {
		return false
	}
	if rule.SourceRole == "test-support" && scopesContain(reachProductionScopes, rule.Scopes) {
		return ownTestSupportTargetRoles[rule.TargetRole]
	}
	return scopesContain(reachTestScopes, rule.Scopes) && ownTestTargetRoles[rule.TargetRole]
}

// reviewedRootTestRule reports whether a rule lies inside the reach of a test
// that composes several owners (ADR 0053 point 7.2): every test package, in
// both test scopes, importing the compose role of the typed roots. That
// includes the legacy composition's external tests; Go keeps its internal
// tests out, because the root imports it.
func reviewedRootTestRule(rule Rule) bool {
	return rule.TargetOwner == rootCompositionOwner && rule.TargetOwnerKind == "" && rule.TargetRole == "compose" &&
		!rule.SameOwner && rule.TargetClass == "" && len(rule.Scopes) > 0 && scopesContain(reachTestScopes, rule.Scopes)
}

// reviewedCompositionReachRule reports whether a rule lies inside one of the
// four standing reaches of ADR 0053.
func reviewedCompositionReachRule(rule Rule, owners map[string]Owner) bool {
	return reviewedRootCompositionRule(rule, owners) || reviewedOwnerCompositionRule(rule, owners) ||
		reviewedOwnTestRule(rule) || reviewedRootTestRule(rule)
}

// firstPartyAllowedByReviewedCompositionReach reports whether the single rule
// that admits this first-party import in the candidate policy lies inside a
// standing reach of ADR 0053. The import loosening it implies is then the
// intended effect of that reach, not a separate widening.
func firstPartyAllowedByReviewedCompositionReach(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedCompositionReachRule(*decision.Allowed, ownersByID(candidate))
}

// targetsOwnOwner reports whether every target a rule selects belongs to the
// source's own owner: through same_owner, or by naming the source owner on
// both sides.
func targetsOwnOwner(rule Rule) bool {
	return rule.SameOwner || rule.SourceOwner != "" && rule.TargetOwner == rule.SourceOwner
}

// sourceExcludesComposition reports whether a rule's source selector names an
// owner or owner kind other than a composition. A rule without a source
// selector selects the compositions too. Only the production reach of an
// owner's composition needs it; the test reaches follow ADR 0053 point 7.
func sourceExcludesComposition(rule Rule, owners map[string]Owner) bool {
	if rule.SourceOwner != "" {
		owner, known := owners[rule.SourceOwner]
		return known && owner.Kind != "composition" && rule.SourceOwnerKind == ""
	}
	return rule.SourceOwnerKind != "" && rule.SourceOwnerKind != "composition"
}
