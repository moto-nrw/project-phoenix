package architecture

// reviewedOwnerHTTPRule is the owner-transport cutover reach of ADR 0056.
// Rules are pinned to an existing HTTP package, owner, role and scope. They
// cannot grant private implementation reach or reverse the domain layering.
func reviewedOwnerHTTPRule(rule Rule, base, candidate *Policy) bool {
	if rule.SourceOwner == "" || rule.SourceOwnerKind != "" || rule.SourcePackage == "" ||
		rule.TargetOwner == "" || rule.TargetOwnerKind != "" || rule.TargetClass != "" || rule.SameOwner || len(rule.Scopes) == 0 {
		return false
	}
	source, ok := candidate.packageMap()[candidate.absolutePackage(rule.SourcePackage)]
	if !ok || source.Owner != rule.SourceOwner || source.Role != "http" {
		return false
	}
	switch rule.SourceRole {
	case "http":
		if len(rule.Scopes) != 1 || rule.Scopes[0] != string(ScopeProduction) {
			return false
		}
	case "adapter-test":
		for _, scope := range rule.Scopes {
			if scope != string(ScopeInternalTest) && scope != string(ScopeExternalTest) {
				return false
			}
		}
	default:
		return false
	}
	// HTTP runtime successors are exact packages, not an owner's whole HTTP role.
	if rule.TargetRole == "http" {
		return (rule.TargetOwner == "tenant-runtime" && rule.TargetPackage == "api/common") ||
			(rule.TargetOwner == "identity-access" && rule.TargetPackage == "modules/identityaccess/sessiontoken")
	}
	if rule.TargetRole != "public" && rule.TargetRole != "contract" {
		return false
	}
	owners := ownersByID(candidate)
	target, targetKnown := owners[rule.TargetOwner]
	owner, sourceKnown := owners[rule.SourceOwner]
	if !sourceKnown || !targetKnown {
		return false
	}
	if rule.TargetOwner == rule.SourceOwner {
		return true
	}
	if target.Kind != "domain" && target.Kind != "platform" {
		return false
	}
	if owner.Kind == "projection" || owner.Kind == "workflow" {
		return true
	}
	if owner.Kind != "domain" && owner.Kind != "platform" {
		return false
	}
	// The owner cutover also inherits the exact native contracts previously
	// consumed by the directory transport. These are the lower People Directory
	// and request-review identity contracts, not arbitrary domain directions.
	if ownerHTTPNativeSuccessor(rule, base) {
		return true
	}
	// Domain transports inherit only directions and target roles already held
	// by their owner. The rule's package/scope constraints still narrow that reach.
	for _, granted := range base.Rules {
		if !sliceContains(granted.Scopes, string(ScopeProduction)) || granted.TargetRole != rule.TargetRole || granted.TargetClass != "" {
			continue
		}
		if granted.SourceOwner != "" && granted.SourceOwner != rule.SourceOwner {
			continue
		}
		if granted.SourceOwnerKind != "" && granted.SourceOwnerKind != owner.Kind {
			continue
		}
		if granted.TargetOwner != "" && granted.TargetOwner != rule.TargetOwner {
			continue
		}
		if granted.TargetOwnerKind != "" && granted.TargetOwnerKind != target.Kind {
			continue
		}
		if granted.SameOwner {
			continue
		}
		if granted.TargetPackage != "" && granted.TargetPackage != rule.TargetPackage {
			continue
		}
		return true
	}
	return false
}

func firstPartyAllowedByReviewedOwnerHTTPRule(base, candidate *Policy, scope Scope, source, target Package) bool {
	if candidate.PolicyEpoch <= base.PolicyEpoch {
		return false
	}
	decision := decideRules(candidate.firstPartyRules(scope, source.inScope(scope), target))
	return decision.Allowed != nil && len(decision.Overlaps) == 0 && reviewedOwnerHTTPRule(*decision.Allowed, base, candidate)
}

// ownerHTTPNativeSuccessor preserves the target of an existing HTTP rule while
// assigning the handler to its actual owner. The finite domain directions are
// the approved layering; platform targets remain below every domain.
func ownerHTTPNativeSuccessor(rule Rule, base *Policy) bool {
	targets := map[string]map[string]bool{
		"care-plan":        {"people-directory/public": true, "people-directory/contract": true, "identity-access/contract": true, "delivery-platform/public": true, "security-runtime/public": true, "security-runtime/contract": true, "settings-platform/public": true},
		"student-presence": {"people-directory/public": true, "delivery-platform/public": true, "security-runtime/public": true, "security-runtime/contract": true, "settings-platform/public": true},
	}
	if !targets[rule.SourceOwner][rule.TargetOwner+"/"+rule.TargetRole] {
		return false
	}
	for _, prior := range base.Rules {
		if prior.SourceOwner == "people-directory" && prior.SourceRole == "http" &&
			prior.TargetOwner == rule.TargetOwner && prior.TargetRole == rule.TargetRole && prior.TargetClass == "" &&
			prior.TargetOwnerKind == "" && !prior.SameOwner && sliceContains(prior.Scopes, string(ScopeProduction)) &&
			(prior.TargetPackage == "" || prior.TargetPackage == rule.TargetPackage) {
			return true
		}
	}
	return false
}
