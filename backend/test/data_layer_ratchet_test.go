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
	queryOptionsListAllowlist = map[string]int{}

	updaterSetAllowlist = map[string]int{}
)

var (
	applyToQueryPattern = regexp.MustCompile(`options\.ApplyToQuery\(`)
	updaterSetPattern   = regexp.MustCompile(`Set\((["` + "`" + `])(last_login|last_used_at|password_hash|avatar|last_seen|room_id|break_minutes|duration_minutes|email_sent_at|email_retry_count) = \?`)
)

func TestDataLayerConsolidationRatchet(t *testing.T) {
	t.Parallel()

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
