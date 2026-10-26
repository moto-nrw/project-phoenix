package architecture

import (
	"fmt"
	"regexp"
	"strings"
)

// targetOnlyRulesEpoch is the policy epoch from which a rule may only state
// the target (ADR 0057, #4070). Older epochs stay loadable as immutable PR
// bases; every candidate from this epoch on is validated against the rule.
const targetOnlyRulesEpoch = 47

// Earlier policy epochs remain loadable as immutable PR bases (#4540).
const domainProjectionRulesEpoch = 91

// privateRoles are the roles no other owner may import (ADR 0057 point 1).
var privateRoles = map[string]bool{
	"application": true, "domain": true, "adapter": true, "postgres": true, "cli": true,
}

// platformForbiddenTargetKinds are the owner kinds a platform may not import
// (ADR 0057 point 1, ADR 0058 point 2).
var platformForbiddenTargetKinds = map[string]bool{"domain": true, "projection": true, "workflow": true}

// compositionGlueSourceKinds are the owner kinds whose composition builds only
// its own owner. Their compose role reaching another owner's compose or http
// role is glue that belongs in the root (ADR 0059 point 6, #4004). The typed
// roots and the legacy composition are composition owners and stay outside.
var compositionGlueSourceKinds = map[string]bool{"domain": true, "workflow": true, "projection": true, "platform": true}

// compositionGlueTargetRoles are the roles an owner composition may not
// import from another owner. A composition owner (the typed roots) is a target
// only in the test scopes, the standing test reach of ADR 0053 point 7.2: no
// owner imports a root in production (ADR 0058).
var compositionGlueTargetRoles = map[string]bool{"compose": true, "http": true}

// TransitionMarker is one phrase by which a rule description declares the
// rule temporary. A temporary allowance is an exact legacy.jsonl key with an
// open carrier issue, never a rule (ADR 0057 point 2).
type TransitionMarker struct {
	Name    string
	Pattern *regexp.Regexp
}

// TransitionMarkers is the fixed list of transition phrases. Matching is
// case-insensitive and only reads the description.
var TransitionMarkers = []TransitionMarker{
	{Name: "convert to exact debt", Pattern: regexp.MustCompile(`(?i)\bconvert\b[^.]*\bto exact debt\b`)},
	{Name: "compatibility permission", Pattern: regexp.MustCompile(`(?i)\bcompatibility permissions?\b`)},
	{Name: "compatibility binding", Pattern: regexp.MustCompile(`(?i)\bcompatibility bindings?\b`)},
	{Name: "compatibility adapter", Pattern: regexp.MustCompile(`(?i)\bcompatibility adapters?\b`)},
	{Name: "temporary", Pattern: regexp.MustCompile(`(?i)\btemporar(?:y|ily)\b`)},
	{Name: "until #", Pattern: regexp.MustCompile(`(?i)\buntil #\d+`)},
	{Name: "tracked by #", Pattern: regexp.MustCompile(`(?i)\btracked by #\d+`)},
}

// RuleTransitionMarkers returns the names of the transition markers in a rule
// description, in TransitionMarkers order.
func RuleTransitionMarkers(description string) []string {
	var names []string
	for _, marker := range TransitionMarkers {
		if marker.Pattern.MatchString(description) {
			names = append(names, marker.Name)
		}
	}
	return names
}

// validateTargetOnlyRule rejects a rule the end state of #2580 does not keep:
// the criteria of ADR 0057 point 1 except cycles, and owner compositions
// reaching another owner's compose or http role (ADR 0059 point 6). The
// directions and cycles of ADR 0058 are judged by #3417.
func validateTargetOnlyRule(rule Rule, owners map[string]Owner) error {
	if rule.Issue != "" {
		return fmt.Errorf("rule %q carries issue %s: a temporary allowance is an exact legacy.jsonl key, not a rule (ADR 0057)", rule.ID, rule.Issue)
	}
	if markers := RuleTransitionMarkers(rule.Description); len(markers) > 0 {
		return fmt.Errorf("rule %q describes itself as temporary (%s): convert it to exact legacy.jsonl keys (ADR 0057)", rule.ID, strings.Join(markers, ", "))
	}
	if rule.TargetClass != "" {
		return nil
	}
	sources := selectedOwners(rule.SourceOwner, rule.SourceOwnerKind, owners)
	targets := selectedOwners(rule.TargetOwner, rule.TargetOwnerKind, owners)
	for _, source := range sources {
		for _, target := range targets {
			if source.ID == target.ID {
				continue
			}
			if rule.SameOwner {
				continue
			}
			if privateRoles[rule.TargetRole] {
				return fmt.Errorf("rule %q lets %s import the %s role of %s: another owner's private role is never target (ADR 0057)", rule.ID, source.ID, rule.TargetRole, target.ID)
			}
			if (rule.SourceRole == "" || rule.SourceRole == "compose") && compositionGlueSourceKinds[source.Kind] &&
				compositionGlueTargetRoles[rule.TargetRole] && (target.Kind != "composition" || ruleHasScope(rule, ScopeProduction)) {
				return fmt.Errorf("rule %q lets the composition of %s import the %s role of %s: cross-owner glue belongs in the root (ADR 0059 point 6)", rule.ID, source.ID, rule.TargetRole, target.ID)
			}
			if source.Kind == "platform" && platformForbiddenTargetKinds[target.Kind] {
				return fmt.Errorf("rule %q lets platform %s import %s owner %s: a platform depends on platforms and the shared kernel only (ADR 0057, ADR 0058)", rule.ID, source.ID, target.Kind, target.ID)
			}
		}
	}
	return nil
}

func selectedOwners(id, kind string, owners map[string]Owner) []Owner {
	var selected []Owner
	for _, owner := range owners {
		if ownerMatches(id, kind, owner) {
			selected = append(selected, owner)
		}
	}
	return selected
}

// validateDomainProjectionRule enforces ADR 0071 point 2, including public
// and contract targets. There are no domain-to-projection exceptions.
func validateDomainProjectionRule(rule Rule, owners map[string]Owner) error {
	if rule.TargetClass != "" || rule.SameOwner {
		return nil
	}
	for _, source := range selectedOwners(rule.SourceOwner, rule.SourceOwnerKind, owners) {
		for _, target := range selectedOwners(rule.TargetOwner, rule.TargetOwnerKind, owners) {
			if source.Kind == "domain" && target.Kind == "projection" {
				return fmt.Errorf("rule %q lets domain %s import projection %s: a domain declares its own reader port and the root binds it (ADR 0071 point 2)", rule.ID, source.ID, target.ID)
			}
		}
	}
	return nil
}
