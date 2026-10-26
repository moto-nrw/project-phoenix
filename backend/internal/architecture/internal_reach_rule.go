package architecture

// The reviewed reach of a domain owner's implementation (ADR 0058 addendum
// #4084). #4084 moves domain logic, row mapping and repository code out of
// the owner compositions into `application` and `postgres`. That code names
// the owner's own public contract, the tenant runtime and the public
// contracts of the domains the owner already depends on. A reviewed epoch may
// add exactly these rules:
//
//   - the two kind-wide rules from a domain's `application` or `postgres`
//     role into its own `public` role (same_owner);
//   - one rule per owner from a domain's `application` or `postgres` role
//     into the `public` role of the tenant runtime or the transaction
//     runtime. A domain depends on platforms (ADR 0058 point 2), so these
//     targets cannot close a cycle;
//   - one rule per owner from a domain's `application` or `postgres` role
//     into another domain's `public` role, when the base already grants that
//     owner a production rule into the same target. Such a rule adds no edge
//     to the owner graph; it moves an edge the owner's composition or HTTP
//     role already holds into its implementation, so the direction ADR 0058
//     fixed stays.
//
// The internal tests of that code (role module-internal-test, scope
// internal_test) may reach exactly what the per-owner production rules
// above may reach; they test unexported functions and move with the code.
//
// A domain's `port` role holds the dependency types and ports the roots name
// when they build the owner's services (#4084, option A); it reaches what the
// implementation reaches. Two more shapes serve it:
//
//   - a rule into another owner's `port` role from a source selector that
//     the base already grants that owner's `compose` role. A composition may
//     import its own ports, so the rule narrows a reach that exists;
//   - one rule per owner from a domain's `postgres` role into the same
//     owner's `port` role, production only: the owner's repositories
//     implement the repository contracts of its ports (#4084). It stays
//     inside one owner, so it adds no edge to the owner graph.
//
// Every other owner kind, role, target, external class and scope stays under
// the ordinary loosening guards.

// internalReachSourceRoles are the implementation roles the reach admits.
var internalReachSourceRoles = map[string]bool{"application": true, "postgres": true, "port": true}

// internalReachPlatformTargets are the platform owners whose public role a
// domain's implementation may reach: the ambient tenant transaction and the
// classification of database failures.
var internalReachPlatformTargets = map[string]bool{"tenant-runtime": true, "transaction-runtime": true}

// internalReachTestRole is the role of the internal tests of a domain's
// implementation packages.
const internalReachTestRole = "module-internal-test"

// reviewedInternalReachRule reports whether a rule lies inside the reviewed
// reach of a domain owner's implementation.
func reviewedInternalReachRule(rule Rule, base *Policy, owners map[string]Owner) bool {
	if reviewedPortNarrowingRule(rule, base) || reviewedPostgresPortRule(rule, owners) {
		return true
	}
	if rule.TargetRole != "public" || rule.TargetClass != "" || rule.TargetOwnerKind != "" || len(rule.Scopes) != 1 {
		return false
	}
	switch {
	case internalReachSourceRoles[rule.SourceRole] && rule.Scopes[0] == string(ScopeProduction):
	case rule.SourceRole == internalReachTestRole && rule.Scopes[0] == string(ScopeInternalTest) && !rule.SameOwner:
	default:
		return false
	}
	if rule.SameOwner {
		return rule.SourceOwner == "" && rule.SourceOwnerKind == "domain" && rule.TargetOwner == ""
	}
	if rule.SourceOwnerKind != "" || rule.SourceOwner == "" || rule.TargetOwner == "" || rule.TargetOwner == rule.SourceOwner {
		return false
	}
	source, sourceKnown := owners[rule.SourceOwner]
	target, targetKnown := owners[rule.TargetOwner]
	if !sourceKnown || !targetKnown || source.Kind != "domain" {
		return false
	}
	if target.Kind == "platform" {
		return internalReachPlatformTargets[rule.TargetOwner]
	}
	return target.Kind == "domain" && baseGrantsOwnerEdge(base, rule.SourceOwner, rule.TargetOwner)
}

// reviewedPostgresPortRule reports the per-owner rule from a domain's
// postgres role into its own port role.
func reviewedPostgresPortRule(rule Rule, owners map[string]Owner) bool {
	source, known := owners[rule.SourceOwner]
	return known && source.Kind == "domain" && rule.SourceOwnerKind == "" && rule.SourceRole == "postgres" &&
		rule.TargetOwner == rule.SourceOwner && rule.TargetOwnerKind == "" && rule.TargetRole == "port" &&
		rule.TargetClass == "" && !rule.SameOwner && len(rule.Scopes) == 1 && rule.Scopes[0] == string(ScopeProduction)
}

// reviewedPortNarrowingRule reports a rule into another owner's port role
// whose source selector the base already grants that owner's compose role in
// every scope the rule names.
func reviewedPortNarrowingRule(rule Rule, base *Policy) bool {
	if rule.TargetRole != "port" || rule.TargetOwner == "" || rule.TargetOwnerKind != "" || rule.TargetClass != "" ||
		rule.SameOwner || rule.SourceOwner == rule.TargetOwner || len(rule.Scopes) == 0 {
		return false
	}
	for _, granted := range base.Rules {
		if granted.SourceOwner == rule.SourceOwner && granted.SourceOwnerKind == rule.SourceOwnerKind &&
			granted.SourceRole == rule.SourceRole && granted.TargetOwner == rule.TargetOwner &&
			granted.TargetOwnerKind == "" && granted.TargetRole == "compose" && granted.TargetClass == "" &&
			!granted.SameOwner && scopesContain(granted.Scopes, rule.Scopes) {
			return true
		}
	}
	return false
}

// baseGrantsOwnerEdge reports whether the base policy holds a production rule
// from the source owner into the target owner's public role.
func baseGrantsOwnerEdge(base *Policy, sourceOwner, targetOwner string) bool {
	for _, rule := range base.Rules {
		if rule.SourceOwner == sourceOwner && rule.TargetOwner == targetOwner && rule.TargetRole == "public" &&
			!rule.SameOwner && sliceContains(rule.Scopes, string(ScopeProduction)) {
			return true
		}
	}
	return false
}

// firstPartyAllowedByReviewedInternalReach reports whether the single rule
// that admits this first-party import in the candidate policy lies inside the
// reviewed reach of a domain owner's implementation.
func firstPartyAllowedByReviewedInternalReach(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 &&
		reviewedInternalReachRule(*decision.Allowed, base, ownersByID(candidate))
}
