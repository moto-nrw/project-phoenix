package test

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// TestModelCeremonyRatchet enforces .claude/rules/backend-conventions.md
// Rule 3: model entities embed base.Model / base.StringIDModel and never
// redeclare the trivial GetID/GetCreatedAt/GetUpdatedAt accessors, and never
// declare GORM-style TableName() methods (bun has no such convention — table
// names come from struct tags and ModelTableExpr strings).
//
// PR A1 of the 2026-07-05 code-reduction audit deleted 255 shadow getters and
// 117 TableName methods; this ratchet keeps them from growing back.
//
// Two patterns, scanned over models/ excluding models/base (which hosts the
// one canonical getter set):
//
//	M1 — GetID/GetCreatedAt/GetUpdatedAt method declarations. The allowlist
//	     holds exactly the load-bearing shadows: structs that do NOT embed
//	     base.Model, or audit models with intentionally different semantics
//	     (GetUpdatedAt returning AccessedAt/ChangedAt).
//	M2 — TableName() method declarations. Zero tolerated.
//	M3 — bun-incompatible BeforeAppendModel(query any) hook declarations.
//	     bun dispatches only the two-arg (ctx, query) interface via reflection;
//	     the one-arg shape never runs and 93 dead copies were deleted in PR A2.
//	     Zero tolerated; a correctly-signed (ctx, query) hook stays legal.
//
// Allowlist rules (identical to the other ratchets): a file not listed must
// have zero hits, a file may never exceed its count, and counts only shrink.
var (
	modelGetterDeclPattern = regexp.MustCompile(`^func \([^)]*\) (GetID|GetCreatedAt|GetUpdatedAt)\(`)
	modelTableNamePattern  = regexp.MustCompile(`^func \([^)]*\) TableName\(`)
	modelDeadHookPattern   = regexp.MustCompile(`^func \([^)]*\) BeforeAppendModel\(\w+ (any|interface\{\})\)`)
)

// M1 — load-bearing getter declarations (structs without base.Model embed or
// with divergent semantics). Seeded 2026-07-05 from the audit's keep list.
var modelGetterAllowlist = map[string]int{
	"models/audit/auth_event.go":                     3,
	"models/audit/data_access_log.go":                3,
	"models/audit/data_deletion.go":                  3,
	"models/audit/deviation_event.go":                3,
	"models/audit/enrollment_offering_adjustment.go": 3,
	"models/audit/guardian_change.go":                3,
	"models/auth/passkey.go":                         3,
	"models/config/setting_audit.go":                 3,
	"models/platform/operator_passkey.go":            3,
}

// M2 — TableName() declarations in models/. Empty: bun never calls them.
var modelTableNameAllowlist = map[string]int{}

// M3 — one-arg BeforeAppendModel hooks in models/. Empty: bun never ran them.
var modelDeadHookAllowlist = map[string]int{}

func TestModelCeremonyRatchet(t *testing.T) {
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	skipDirs := map[string]bool{"models/base": true}
	modelsRoot := filepath.Join(backendRoot, "models")

	checks := []struct {
		name      string
		pattern   *regexp.Regexp
		allowlist map[string]int
		fix       string
	}{
		{
			name:      "M1 trivial getter declarations in models/",
			pattern:   modelGetterDeclPattern,
			allowlist: modelGetterAllowlist,
			fix:       "base.Model / base.StringIDModel already provide GetID/GetCreatedAt/GetUpdatedAt (Rule 3) — delete the shadow; only genuinely different semantics belong in the allowlist",
		},
		{
			name:      "M2 TableName() declarations in models/",
			pattern:   modelTableNamePattern,
			allowlist: modelTableNameAllowlist,
			fix:       "bun never calls TableName() — set the table via struct tags / ModelTableExpr and delete the method",
		},
		{
			name:      "M3 one-arg BeforeAppendModel hooks in models/",
			pattern:   modelDeadHookPattern,
			allowlist: modelDeadHookAllowlist,
			fix:       "BeforeAppendModel(query any) never runs — bun dispatches only BeforeAppendModel(ctx context.Context, query schema.Query); use the two-arg signature or set ModelTableExpr in the repository",
		},
	}

	var violations []string
	for _, c := range checks {
		pattern := c.pattern
		counts, scanErr := scanTree(backendRoot, modelsRoot, skipDirs, func(line string) int {
			return len(pattern.FindAllString(line, -1))
		})
		if scanErr != nil {
			t.Fatalf("ratchet scan %s failed: %v", c.name, scanErr)
		}
		violations = append(violations, ratchetViolations(c.name, counts, c.allowlist, c.fix)...)
	}

	if len(violations) > 0 {
		sort.Strings(violations)
		t.Errorf("Model-ceremony ratchet check failed (%d issue(s)):\n\n%s",
			len(violations), strings.Join(violations, "\n\n"))
	}
}
