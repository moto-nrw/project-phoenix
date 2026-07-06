package test

import (
	"regexp"
	"strings"
	"testing"
)

// TestDataLayerConsolidationRatchet guards the audit-B8 data-layer
// consolidations against regrowth (report §8). Three patterns, all
// shrink-only — never add an entry, never raise a count:
//
//  1. Hand-rolled QueryOptions list bodies in database/repositories/
//     (detected via options.ApplyToQuery calls outside base/) — use the
//     generic base.Repository[T].ListWithOptions. The allowlist is the
//     verified-divergent remainder (joins, pseudo-filters, post-processing,
//     custom ordering) that must NOT be blindly consolidated.
//  2. Hand-rolled single-column updater SETs for the column families
//     consolidated onto base.Repository[T].UpdateColumns.
//
// The B8e DeleteExpired consolidation (base.Repository[T].DeleteBefore) has
// no regex ratchet: the thin wrappers keep the same method names and column
// predicates as a hand-roll, and any line-level pattern also matches
// legitimate validity-filter SELECTs. Reviewers: new DeleteExpired-style
// methods must delegate to DeleteBefore unless the predicate is compound.
var (
	// Files whose QueryOptions list bodies are behaviorally divergent from
	// the generic (verified 2026-07-06, audit B8/dup-54 skeptic list).
	queryOptionsListAllowlist = map[string]int{
		"database/repositories/education/group.go":            1, // LEFT JOIN facilities.rooms + sort rewrite
		"database/repositories/education/grade_transition.go": 1, // returns data+count pair
		"database/repositories/activities/group.go":           1, // LEFT JOIN categories
		"database/repositories/active/group_supervisor.go":    1, // applyActiveOnlyFilter pseudo-filter
		"database/repositories/active/combined_group.go":      1, // applyActiveOnlyFilter pseudo-filter
		"database/repositories/users/guardian_profile.go":     1, // no tenant filter, default ordering, fmt.Errorf
		"database/repositories/users/student.go":              1, // hydrateBusDaysForStudents post-processing
		"database/repositories/schedule/dateframe.go":         1, // parked vertical, no base.Repository embed
		"database/repositories/schedule/recurrence_rule.go":   1, // parked vertical, no base.Repository embed
		// Deliberately alias-free ModelTableExpr: a custom alias historically
		// broke bun's SELECT column expansion here ("missing FROM-clause
		// entry"); see the comment above its List method.
		"database/repositories/active/staff_vacation_quota.go": 1,
	}

	// Hand-rolled .Set("col = ?") sites for the consolidated updater column
	// families. Everything else was migrated to UpdateColumns in audit B8c.
	updaterSetAllowlist = map[string]int{
		// Tenant WHERE not expressible via UpdateColumns without flipping
		// repo-wide TenantScoped (AccountParent is a cross-tenant table).
		"database/repositories/auth/account_parent.go": 2,
		// Out of B8c scope (post-audit additions, listed for the recount).
		"database/repositories/auth/guardian_invitation.go": 2, // UpdateEmailStatus (email_sent_at + email_retry_count)
		"database/repositories/users/profile.go":            1, // UpdateAvatar
		// Compound updates UpdateColumns cannot express (state-guard WHERE
		// beyond the PK, or a ::jsonb cast on a sibling column).
		"database/repositories/active/work_session_break.go": 1, // EndBreak: WHERE ended_at IS NULL guard
		"database/repositories/auth/passkey.go":              1, // UpdateAfterUse: revoked_at IS NULL + jsonb cast
		"database/repositories/platform/operator_passkey.go": 1, // UpdateAfterUse: revoked_at IS NULL + jsonb cast
	}
)

var (
	applyToQueryPattern = regexp.MustCompile(`options\.ApplyToQuery\(`)
	updaterSetPattern   = regexp.MustCompile(`Set\((["` + "`" + `])(last_login|last_used_at|password_hash|avatar|last_seen|room_id|break_minutes|duration_minutes|email_sent_at|email_retry_count) = \?`)
)

func TestDataLayerConsolidationRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	var violations []string

	check := func(counts map[string]int, allowlist map[string]int, what, fix string) {
		for file, got := range counts {
			allowed, ok := allowlist[file]
			switch {
			case !ok:
				violations = append(violations, file+": "+what+" site(s) found, but the file is not in the allowlist.\n  "+fix)
			case got > allowed:
				violations = append(violations, file+": more "+what+" site(s) than allowed.\n  "+fix+" Never raise the allowlist.")
			case got < allowed:
				violations = append(violations, file+": fewer "+what+" site(s) than the allowlist says. Ratchet down: lower/delete the entry.")
			}
		}
		for file := range allowlist {
			if _, ok := counts[file]; !ok {
				violations = append(violations, file+": allowlisted for "+what+" but has no hits. Remove the entry.")
			}
		}
	}

	skipBase := func(rel string) bool {
		return strings.HasPrefix(rel, "database/repositories/base/")
	}

	listCounts, err := scanRatchetPattern(backendRoot, "database/repositories", applyToQueryPattern, skipBase)
	if err != nil {
		t.Fatalf("scanning for ApplyToQuery sites failed: %v", err)
	}
	check(listCounts, queryOptionsListAllowlist, "hand-rolled QueryOptions list",
		"Use the generic base.Repository[T].ListWithOptions (with a thin List shim / empty-slice coercion where the interface needs it).")

	setCounts, err := scanRatchetPattern(backendRoot, "database/repositories", updaterSetPattern, skipBase)
	if err != nil {
		t.Fatalf("scanning for per-field updater sites failed: %v", err)
	}
	check(setCounts, updaterSetAllowlist, "per-field updater Set",
		"Build the entity with PK + target fields and call base.Repository[T].UpdateColumns.")

	if len(violations) > 0 {
		t.Errorf("Data-layer consolidation ratchet violations (%d):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}
