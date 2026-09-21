package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestPolicyTemporaryRulesRatchet makes the second debt surface of the #2580
// architecture migration countable and shrink-only.
//
// The known surface is backend/architecture/legacy.jsonl: a finite, shrink-only
// baseline of exact import tuples, 682 of them at seed time, each naming its
// open migration issue. `scripts/backend-architecture.sh check` requires exact
// equality with it and pull requests may only remove tuples.
//
// The unknown surface is backend/architecture/policy.json itself. Beside the
// target rules it carries 603 rules whose own description declares them
// temporary ("Compatibility permission: …" / "Compatibility binding: …"), 501
// of them literally promising to "convert it to exact debt once the package
// exists at a base SHA". Those rules are allow-rules: while they stand, the
// evaluator does not report the edges they cover. Running the same evaluator
// with the same policy minus the 557 such rules of the first seed reported
// 1314 violations instead of 688 — roughly 626 real import edges that no
// number tracks today.
//
// That regime has no monotonicity of its own. The rule set grew from 0 when the
// ratchet was switched on (2026-08-28) to 501 while legacy.jsonl fell from 3663
// to 682: converting exact debt into a policy rule looks like progress in the
// only number CI prints. The evaluator has neither an `issue` field on rules
// nor a staleness check for them, so a temporary permission outlives its issue
// silently — 34 of the 39 families below cite only issues that are closed
// today. The five exceptions all hang on #2725 or #2736: communication,
// inbound-parent, inbound-usercontext (gone with #3501), organization-tenancy,
// settings-platform.
//
// This test does not fix any of that. It puts a number on the surface and lets
// the number move one way only.
//
// #3416 adds per-rule cleanup issues and stale-rule detection. The account
// above records the original seed, not the current evaluator. Its 167 stale
// rule deletions lower these seeds to 483 conversion promises and 581 wider
// compatibility markers. The #3351 care-schedule cutover and the seven
// file-storage rules #3461 removed lower them to 395 and 492, and the #3226
// identity persistence rewrite to 392 and 487, and the #2725 conversion of the
// 46 user-context permissions to exact debt to 346 and 441, and the #3501
// dissolution of the user-context package, which took its four Presence
// rules along, to 342 and 437. The #3427 care-lifecycle cutover deleted the
// 46 permissions #3350 had added and lowers them to 296 and 359, and the
// #3422 statistics cutover, which dropped the statistics HTTP grant to the
// retained Presence adapter, to 295 and 358, and the #3422 Care Plan row
// handover, which moved the status-day and excused-request rows out of the
// Presence nest and so dropped eleven stale grants to it, to 290 and 352,
// and the #3422 Workforce row handover, which left the scheduler's grant to
// the retained Presence rows stale, to 289 and 351, and the two remaining
// #3422 slices, which retired the presence services and the session rows and
// left 23 further grants to the nest stale, to 236 and 295. Dissolving the
// nest rewrote the nine surviving grants of its own behaviour suites, which
// describe the module's suites now and promise no conversion, and lowers them
// to 227 and 286; this prose counter remains as an independent guard.
//
// Three shrink-only measurements, all read-only on policy.json:
//
//	policyTempRulesTotal        — rules whose description contains the literal
//	                              "convert it to exact debt".
//	policyTempRulesCompatTotal  — rules whose description mentions a
//	                              "compatibility permission" or a
//	                              "compatibility binding" (case-insensitive);
//	                              the wider set, which includes the rules that
//	                              skip the conversion promise.
//	policyTempRulesFamilies     — the same rules as the first measurement,
//	                              grouped by the part of the rule id before the
//	                              first dot, i.e. the source owner.
//
// Failure modes, exactly like every other ratchet in this package:
//
//   - A number above its seed means a new temporary permission. Each one is
//     debt that legacy.jsonl does NOT count: resolve it in the same PR, or
//     justify the entry in the ticket. Never raise a seed.
//   - A number below its seed is the win: ratchet the seed down so it cannot
//     regress ("nice — lower the seed to N").
//   - A family that is not seeded is a new rule family — add its entry together
//     with the issue that created it ("new rule family, add the entry").
//   - A seeded family with no hits left is the success case: the family was
//     converted, so remove the entry ("family converted, remove the entry").
//
// Seed measured 2026-09-18 at merge commit ecf0003369 with a throwaway program
// that decoded architecture/policy.json into {id, description} and grouped the
// matches, run as:
//
//	cd backend && ../scripts/run-go-toolchain.sh go run ./tmp/policyTempRules
//
// The first seed was taken at 19feca2822 and re-measured here after the merge
// of origin/development (a47f77b2c3): PR #3408 (issue #3350) added 46 new
// temporary permissions for the care-exit, companion and care-document move,
// which the ratchet cannot absorb without a fresh measurement.
//
// The two totals cross-check without Go:
//
//	grep -c 'convert it to exact debt' backend/architecture/policy.json          # 501
//	grep -ci 'compatibility permission\|compatibility binding' \
//	  backend/architecture/policy.json                                           # 603
const (
	// policyTempRulesMarker is the conversion promise the temporary rules carry
	// in their own description. It is the marker the migration itself writes, so
	// matching on it costs nothing to maintain.
	policyTempRulesMarker = "convert it to exact debt"

	// policyTempRulesTotal seeds the count of rules carrying that marker.
	policyTempRulesTotal = 227

	// policyTempRulesCompatTotal seeds the wider count: every rule that calls
	// itself a compatibility permission or a compatibility binding, whether or
	// not it promises the conversion.
	policyTempRulesCompatTotal = 286
)

// policyTempRulesCompatMarkers are matched case-insensitively against the
// description, because the phrase opens a sentence ("Compatibility permission:
// …") as often as it sits inside one.
var policyTempRulesCompatMarkers = []string{
	"compatibility permission",
	"compatibility binding",
}

// policyTempRulesFamilies seeds the per-owner breakdown of the rules carrying
// policyTempRulesMarker, keyed by the rule id up to its first dot.
//
// Each entry names the issues whose descriptions the family's rules cite and
// whether those issues are still open, checked 2026-09-18. Two are open:
// #2725 (legacy identity models/repositories/services/inbound adapters) and
// #2736 (api/auth and api/operator edges). Everything else below was permitted
// by an issue that is already closed, which is precisely the stale-permission
// problem this test surfaces: a closed issue no longer forces the conversion.
//
// Shrink-only. Lower an entry when rules are converted to exact debt, delete it
// when the family is empty, and never raise one.
var policyTempRulesFamilies = map[string]int{
	// #3214, #3218, #3220, #3224 — all closed.
	// #3422 removed 2 that went stale with legacy/services/active.
	"calendar-view": 2,

	// #2736 (OPEN), #3232, #3224.
	"communication": 4,

	// #3218, #3219, #3220 — closed.
	"document-rendering": 5,

	// #3214, #3218, #3220 — closed; #3427 removed the three #3350 (PR #3408)
	// had added.
	"enrollment": 3,

	// #3214, #3218, #3220, #3224 — closed; #3427 removed the one #3350 added.
	// #3422 removed 1 that went stale with legacy/services/active.
	"group-live-view": 1,

	// #3364 — closed.
	"identity-access": 18,

	// #3229, #3214, #3220 — closed; #2725 (OPEN) for one rule.
	"inbound-parent": 27,

	// #3219, #3218, #2730 — closed.
	"inbound-staff-shifts": 37,

	// #3214, #3218, #3220, #3224 — closed. #3427 removed the 36 that #3350
	// (PR #3408) had added with the care-lifecycle adapter.
	// #3422 removed 4 that went stale with legacy/services/active.
	"inbound-students": 2,

	// #3218, #3220, #3214, #3224 — closed. The largest family: 90 rules, every
	// one of them permitted by an issue that is done.
	// #3422 removed 5 that went stale with legacy/services/active.
	"inbound-timetable": 72,

	// #2725 (OPEN) and #3224 (closed) name most rules jointly; #3214 the rest.
	// The only large family with a live issue behind it.

	// #3214, #3218, #3219, #3220, #3224 — closed; #3427 removed the one #3350
	// (PR #3408) added.
	// #3422 removed 1 that went stale with legacy/services/active.
	"legacy-composition": 2,

	// #2736 (OPEN) and #3232 (closed) name most rules jointly; #3214 the rest.
	"organization-tenancy": 14,

	// #3214, #3218 — closed; #3427 removed the one #3350 (PR #3408) added.
	// #3422 removed 1 that went stale with legacy/services/active.
	"people-directory": 1,

	// #3214, #3218, #3220 — closed.
	// #3422 removed 1 that went stale with legacy/services/active.
	"process-device-scan": 1,

	// #3207, #3214, #3217, #3218, #3219, #3220, #3224, #3229 — all closed.
	// #3422 removed 2 that went stale with legacy/services/active.
	"root-composition": 6,

	// #3214, #3218, #3220 — closed.
	// #3422 removed 3 that went stale with legacy/services/active.
	"scheduler-runtime": 3,

	// #3214, #3218 — closed.
	"school-structure": 1,

	// #2736 (OPEN) and #3232 (closed) name most rules jointly; #3207, #3214 the
	// rest; #3427 removed the one #3350 (PR #3408) added.
	// #3422 removed 1 that went stale with legacy/services/active.
	"settings-platform": 15,

	// #3214, #3207, #3218, #3224 — closed.
	// #3422 removed 7 that went stale with legacy/services/active and rewrote
	// the 9 of the dissolved nest's behaviour suites, which now describe the
	// module's own suites and promise no conversion.
	"student-presence": 5,

	// #3214, #3218, #3229 — closed.
	"test-support": 2,

	// #3214, #3218, #3220, #3224 — closed.
	"timetable-activities": 2,

	// #3219, #3218 — closed.
	"workforce": 4,
}

// policyTempRulesFamilyFix is the guidance ratchetViolations prints for a family
// that is new or has grown. Its stale-entry branch ("Remove the entry") covers
// the success case, a family converted to exact debt. The helper says "file"
// where the key here is a rule family — it is the shared ratchet helper and
// worth more than a bespoke wording.
const policyTempRulesFamilyFix = "that is at least one new temporary permission. Every one of them is debt " +
	"that legacy.jsonl does NOT count. Resolve it in the same PR or justify the entry in the ticket; " +
	"if the family is new, add its entry together with the issue that created it (new rule family, add the entry)"

// policyTempRulesRule is the minimal shape this test needs. policy.json is
// ~960 KB, so it is decoded into these two fields rather than into
// map[string]any; encoding/json drops every other field on its own.
type policyTempRulesRule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
}

type policyTempRulesPolicy struct {
	Rules []policyTempRulesRule `json:"rules"`
}

// policyTempRulesCounts carries the three measurements of one policy read.
type policyTempRulesCounts struct {
	total    int
	compat   int
	families map[string]int
}

func TestPolicyTemporaryRulesRatchet(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	counts, err := policyTempRulesScan(filepath.Join(backendRoot, "architecture", "policy.json"))
	if err != nil {
		t.Fatalf("policy scan failed: %v", err)
	}

	violations := policyTempRulesCountViolations(
		"temporary rules carrying \""+policyTempRulesMarker+"\"", counts.total, policyTempRulesTotal)
	violations = append(violations, policyTempRulesCountViolations(
		"compatibility permission/binding", counts.compat, policyTempRulesCompatTotal)...)
	violations = append(violations, ratchetViolations(
		"rule families", counts.families, policyTempRulesFamilies, policyTempRulesFamilyFix)...)

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Policy temporary-rules ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}

// policyTempRulesCountViolations compares one aggregate against its seed. It
// cannot reuse ratchetViolations: that helper compares per-key maps, while
// these two measurements are single numbers with no key to report.
func policyTempRulesCountViolations(check string, got, seed int) []string {
	switch {
	case got > seed:
		return []string{fmt.Sprintf(
			"[%s] %d rule(s), seed %d: %d more, so at least one new temporary permission. "+
				"Every one of them is debt that legacy.jsonl does NOT count. "+
				"Resolve it in the same PR or justify the entry in the ticket. Never raise a seed.",
			check, got, seed, got-seed)}
	case got < seed:
		return []string{fmt.Sprintf(
			"[%s] %d rule(s), seed %d: nice — lower the seed to %d so the progress cannot regress.",
			check, got, seed, got)}
	}
	return nil
}

// policyTempRulesScan reads policy.json and returns the three measurements.
// It only reads: nothing in this test writes to the policy.
func policyTempRulesScan(path string) (policyTempRulesCounts, error) {
	counts := policyTempRulesCounts{families: make(map[string]int)}

	raw, err := os.ReadFile(path) // #nosec G304 -- test reads a repo-local policy file
	if err != nil {
		return counts, fmt.Errorf("read %s: %w", path, err)
	}

	var policy policyTempRulesPolicy
	if err := json.Unmarshal(raw, &policy); err != nil {
		return counts, fmt.Errorf("decode %s: %w", path, err)
	}
	if len(policy.Rules) == 0 {
		return counts, fmt.Errorf("no rules decoded from %s", path)
	}

	for _, rule := range policy.Rules {
		if strings.Contains(rule.Description, policyTempRulesMarker) {
			counts.total++
			counts.families[policyTempRulesFamily(rule.ID)]++
		}
		lowered := strings.ToLower(rule.Description)
		for _, marker := range policyTempRulesCompatMarkers {
			if strings.Contains(lowered, marker) {
				counts.compat++
				break
			}
		}
	}
	return counts, nil
}

// policyTempRulesFamily returns the part of a rule id before the first dot —
// the source owner that the permission was granted to.
func policyTempRulesFamily(id string) string {
	if i := strings.Index(id, "."); i >= 0 {
		return id[:i]
	}
	return id
}
