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
// Three patterns, scanned over models/ excluding models/base (which hosts the
// one canonical getter set):
//
//	M1 — GetID/GetCreatedAt/GetUpdatedAt method declarations. Zero tolerated;
//	     conventional models embed a base shape, while audit models keep their
//	     explicit AccessedAt/DeletedAt/OccurredAt/ChangedAt fields.
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

// M1 — model entities use the shared base shapes or semantic audit fields;
// no per-entity GetID/GetCreatedAt/GetUpdatedAt declarations remain.
var modelGetterAllowlist = map[string]int{}

// M2 — TableName() declarations in models/. Empty: bun never calls them.
var modelTableNameAllowlist = map[string]int{}

// M3 — one-arg BeforeAppendModel hooks in models/. Empty: bun never ran them.
var modelDeadHookAllowlist = map[string]int{}

func TestModelCeremonyRatchet(t *testing.T) {
	t.Parallel()

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
			fix:       "embed the matching base identity/timestamp shape, or keep the audit timestamp's domain name; per-model generic getters are not allowed (Rule 3)",
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
