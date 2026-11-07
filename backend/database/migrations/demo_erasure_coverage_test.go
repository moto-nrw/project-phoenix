package migrations

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The demo erasure guard of #6015. platform.erase_closed_demo_visitors
// removes a visitor when its demo school retires, but the school itself stays
// soft-deleted: its tenant rows go only with a full school erasure (#5559).
// Rows that do not go with the school are the risk: PR #6000 found the care
// contracts of #4678, which outlive their school (ADR 0088 section 9), still
// naming the visitor, and no test noticed.
//
// The guard takes its tables from the two registers every table already has:
// a table is in scope when the retention register (tableRetentions) says it
// holds personal data and the data lifecycle catalog places its rows outside
// the school, at the Träger (organization), the account, the demo process or
// a school reference. Tenant rows wait for #5559; moto's own platform data
// (operators, caterer businesses) never holds a visitor.
//
// Each table in scope is erased by the function itself (DELETE or a
// rewriting UPDATE in its current definition), goes by ON DELETE CASCADE
// with a table the function deletes, or names its reason in
// demoErasureExceptions.

// demoErasureOutsideSchoolScopes are the catalog scopes whose rows the
// school does not take along.
var demoErasureOutsideSchoolScopes = []string{"organization", "account", "demo", "tenantReference", "school"}

// The Träger-Büro of the shared demo Träger is the bootstrap admin of the
// seed (seedTraegerBueroStep); no visitor account is ever a member of it.
const traegerBueroOnly = "Träger-Büro of the shared demo Träger: the seed assigns only its bootstrap admin, never a visitor account (seedTraegerBueroStep)"

// The public holiday registration has neither a school nor an account of the
// demo school; the seed writes the invented family Ferienkind or the seed
// name of the displaced parent with its synthetic address, never the
// visitor's name (seedHolidayRegistrationByParent takes the seed key).
const holidayRegistrationSeed = "holiday registration of the shared demo Träger without a school or account link: the seed writes the invented family Ferienkind or the seed name of the displaced parent with its synthetic address, never the visitor's name (seedHolidayRegistrationByParent)"

// The billing of a holiday registration copies the registration's family.
const holidayBillingSeed = "billing of a holiday registration of the shared demo Träger: copies the seeded family, never the visitor's name; " + holidayRegistrationNoLink

const holidayRegistrationNoLink = "no school or visitor account to select it by"

// demoErasureExceptions are the tables in scope the erasure leaves on
// purpose, each with its reason. The list only shrinks: a new table outside
// the school gets an erasure path, not an entry.
var demoErasureExceptions = map[string]string{
	"audit.traeger_account_changes":                traegerBueroOnly,
	"audit.traeger_document_downloads":             traegerBueroOnly + "; only the Träger-Büro downloads",
	"auth.account_avatar_file_cleanup":             "the erasure writes the avatar file intent itself; the hourly drain deletes it with the file",
	"auth.caterer_accounts":                        "caterer login of a caterer business; the demo seed gives no visitor account a caterer membership",
	"auth.organization_account_roles":              traegerBueroOnly,
	"auth.traeger_invitations":                     traegerBueroOnly + "; invitations name synthetic example.org addresses",
	"config.traeger_setting_audit":                 traegerBueroOnly,
	"enrollment.holiday_registration_audit":        holidayRegistrationSeed + "; the trail names the Träger-Büro",
	"enrollment.holiday_registration_bookings":     holidayRegistrationSeed,
	"enrollment.holiday_registration_children":     holidayRegistrationSeed + "; its home school ID is no visitor data",
	"enrollment.holiday_registration_contacts":     holidayRegistrationSeed,
	"enrollment.holiday_registration_deletions":    "counts per holiday period of the demo Träger, no name",
	"enrollment.holiday_registration_proofs":       holidayRegistrationSeed + "; the proof is a generated sample PDF",
	"enrollment.holiday_registration_rate_limits":  "abuse counter of the public holiday form, expires after a day",
	"enrollment.holiday_registrations":             holidayRegistrationSeed,
	"parent_billing.document_letters":              holidayBillingSeed,
	"parent_billing.document_mailings":             holidayBillingSeed + "; the visitor's parent account loses its address by anonymization",
	"parent_billing.document_positions":            holidayBillingSeed,
	"parent_billing.documents":                     holidayBillingSeed,
	"parent_billing.holiday_contribution_lines":    holidayBillingSeed,
	"parent_billing.holiday_contributions":         holidayBillingSeed,
	"parent_billing.holiday_sibling_group_members": holidayBillingSeed,
	"parent_billing.holiday_sibling_groups":        holidayBillingSeed,
	"parent_billing.run_correction_dismissals":     holidayBillingSeed,
	"parent_billing.run_corrections":               holidayBillingSeed,
	"parent_billing.run_events":                    holidayBillingSeed + "; the trail names the Träger-Büro",
	"parent_billing.run_letters":                   holidayBillingSeed,
	"platform.billing_key_date_counts":             "school name and counts of an invoice key date, no person",
	"platform.organization_accounts":               traegerBueroOnly,
	"platform.traeger_bank_accounts":               traegerBueroOnly + "; invented bank details of the demo Träger",
	"platform.traeger_document_reads":              "an opening keeps only the school ID and the account ID, which the erasure anonymizes; expires three years after the opening (#4692)",
	"platform.traeger_document_releases":           traegerBueroOnly + "; a release names the school, not a person",
	"platform.traeger_document_templates":          traegerBueroOnly,
	"platform.traeger_documents":                   traegerBueroOnly,
	"platform.traeger_master_data_audit":           traegerBueroOnly,
	"platform.traeger_profiles":                    traegerBueroOnly + "; invented master data of the demo Träger",
}

// demoErasureExceptionBudget is the recorded size of demoErasureExceptions.
// It may only fall: lower it when an entry leaves.
const demoErasureExceptionBudget = 36

type cascadeEdge struct{ Child, Parent string }

type demoErasureCoverage struct {
	// InScope are the tables with personal data whose rows outlive the school.
	InScope []string
	// FunctionSQL is the current definition of platform.erase_closed_demo_visitors.
	FunctionSQL string
	// Cascades are the ON DELETE CASCADE foreign keys of the schema.
	Cascades   []cascadeEdge
	Exceptions map[string]string
	Budget     int
}

var (
	sqlLineComment      = regexp.MustCompile(`--[^\n]*`)
	sqlErasureStatement = regexp.MustCompile(`(?i)\b(DELETE\s+FROM|UPDATE)\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)
)

// demoErasureTargets returns the tables the function deletes from and the
// tables it deletes from or rewrites. Comments are no statements.
func demoErasureTargets(functionSQL string) (deleted, erased map[string]bool) {
	deleted, erased = map[string]bool{}, map[string]bool{}
	for _, match := range sqlErasureStatement.FindAllStringSubmatch(sqlLineComment.ReplaceAllString(functionSQL, ""), -1) {
		table := strings.ToLower(match[2])
		erased[table] = true
		if strings.HasPrefix(strings.ToUpper(match[1]), "DELETE") {
			deleted[table] = true
		}
	}
	return deleted, erased
}

// cascadeClosure returns every table whose rows go with a deleted table, by
// ON DELETE CASCADE over any number of steps, with the step it goes with.
func cascadeClosure(deleted map[string]bool, edges []cascadeEdge) map[string]string {
	via := map[string]string{}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if _, done := via[edge.Child]; done || deleted[edge.Child] {
				continue
			}
			if _, reached := via[edge.Parent]; deleted[edge.Parent] || reached {
				via[edge.Child] = edge.Parent
				changed = true
			}
		}
	}
	return via
}

func demoErasureCoverageProblems(in demoErasureCoverage) []string {
	var problems []string
	if len(in.InScope) == 0 {
		problems = append(problems, "no table outlives the school; the scope query is broken and the guard would pass vacuously")
	}
	deleted, erased := demoErasureTargets(in.FunctionSQL)
	if len(deleted) == 0 {
		problems = append(problems, "the erasure function deletes from no table; its definition was not read and the guard would pass vacuously")
	}
	cascaded := cascadeClosure(deleted, in.Cascades)
	for _, table := range in.InScope {
		_, viaCascade := cascaded[table]
		reason, excepted := in.Exceptions[table]
		switch {
		case (erased[table] || viaCascade) && excepted:
			problems = append(problems, fmt.Sprintf("%s is erased now; remove it from demoErasureExceptions and lower the budget", table))
		case erased[table] || viaCascade:
		case !excepted:
			problems = append(problems, fmt.Sprintf("%s holds personal data that outlives the demo school, but platform.erase_closed_demo_visitors neither erases it nor deletes a table it cascades from; erase it there (with visitorTraceCounts) or give a reason in demoErasureExceptions", table))
		case strings.TrimSpace(reason) == "":
			problems = append(problems, fmt.Sprintf("%s needs a non-empty reason in demoErasureExceptions", table))
		}
	}
	for table := range in.Exceptions {
		if !slices.Contains(in.InScope, table) {
			problems = append(problems, fmt.Sprintf("%s is in demoErasureExceptions but is no table with personal data outside the school; remove it", table))
		}
	}
	switch count := len(in.Exceptions); {
	case count > in.Budget:
		problems = append(problems, fmt.Sprintf("%d demo erasure exceptions, more than the %d recorded; erase the new table instead", count, in.Budget))
	case count < in.Budget:
		problems = append(problems, fmt.Sprintf("%d demo erasure exceptions, fewer than the %d recorded; lower demoErasureExceptionBudget", count, in.Budget))
	}
	sort.Strings(problems)
	return problems
}

// demoErasureCoverageOfSchema reads the scope, the function and the
// cascades from the migrated schema.
func demoErasureCoverageOfSchema(t *testing.T) demoErasureCoverage {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	var tables []string
	require.NoError(t, db.NewRaw(`
		SELECT n.nspname || '.' || c.relname
		FROM pg_class AS c JOIN pg_namespace AS n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'pg_toast')`).Scan(ctx, &tables))
	scopes := map[string]string{}
	for _, entry := range readLifecycleCatalog(t).Tables {
		scopes[entry.Name] = entry.Scope
	}
	var inScope []string
	for _, table := range tables {
		retention, classified := tableRetentions[table]
		if classified && retention.class != noPersonalData && slices.Contains(demoErasureOutsideSchoolScopes, scopes[table]) {
			inScope = append(inScope, table)
		}
	}
	var functionSQL string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &functionSQL))
	var cascades []cascadeEdge
	require.NoError(t, db.NewRaw(`
		SELECT DISTINCT conrelid::regclass::text AS child, confrelid::regclass::text AS parent
		FROM pg_constraint WHERE contype = 'f' AND confdeltype = 'c'`).Scan(ctx, &cascades))
	sort.Strings(inScope)
	return demoErasureCoverage{
		InScope: inScope, FunctionSQL: functionSQL, Cascades: cascades,
		Exceptions: demoErasureExceptions, Budget: demoErasureExceptionBudget,
	}
}

func TestDemoErasureCoversEveryTableOutsideTheSchool(t *testing.T) {
	t.Parallel()
	coverage := demoErasureCoverageOfSchema(t)
	require.Empty(t, demoErasureCoverageProblems(coverage),
		"demo erasure coverage (#6015): every table with personal data outside the school is erased by platform.erase_closed_demo_visitors, goes with one of its tables by cascade, or has a reason in demoErasureExceptions")
}

// The gap of PR #6000 is caught: without its contract deletion, the parties
// and every other contract table outliving the school are reported.
func TestDemoErasureGuardCatchesTheContractGapOfPR6000(t *testing.T) {
	t.Parallel()
	coverage := demoErasureCoverageOfSchema(t)
	withoutContracts := regexp.MustCompile(`(?i)DELETE\s+FROM\s+care_contract\.contracts\b`)
	require.True(t, withoutContracts.MatchString(coverage.FunctionSQL), "the erasure deletes the contracts of the demo school (1.15.786)")
	coverage.FunctionSQL = withoutContracts.ReplaceAllString(coverage.FunctionSQL, "SELECT 1 FROM care_contract.contracts")
	problems := strings.Join(demoErasureCoverageProblems(coverage), "\n")
	for _, table := range []string{"care_contract.contracts", "care_contract.contract_parties", "care_contract.contract_documents", "care_contract.contract_mailings"} {
		require.Contains(t, problems, table+" holds personal data that outlives the demo school")
	}
}

func coverageFixture() demoErasureCoverage {
	return demoErasureCoverage{
		InScope: []string{"care.contracts", "care.parties", "care.party_notes", "care.trail"},
		FunctionSQL: `BEGIN
			-- DELETE FROM care.trail is only a comment
			DELETE FROM care.contracts WHERE school_id = ANY (hidden_schools);
		END`,
		Cascades: []cascadeEdge{
			{Child: "care.party_notes", Parent: "care.parties"},
			{Child: "care.parties", Parent: "care.contracts"},
		},
		Exceptions: map[string]string{},
		Budget:     0,
	}
}

func TestDemoErasureCoverageProblems(t *testing.T) {
	t.Parallel()
	trailMissing := "care.trail holds personal data that outlives the demo school"
	tests := []struct {
		name   string
		change func(*demoErasureCoverage)
		want   []string
	}{
		{name: "a table without erasure path is reported, a comment is no statement", change: func(*demoErasureCoverage) {}, want: []string{trailMissing}},
		{name: "a cascade over two steps counts, a rewrite counts", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\nUPDATE care.trail SET name = 'Demo Besuch';"
		}},
		{name: "a cascade from a table the function only rewrites does not count", change: func(c *demoErasureCoverage) {
			c.FunctionSQL = "UPDATE care.contracts SET name = 'Demo'; DELETE FROM care.trail;"
		}, want: []string{"care.parties holds personal data", "care.party_notes holds personal data"}},
		{name: "a reasoned exception within budget passes", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept for the Träger"}
			c.Budget = 1
		}},
		{name: "an exception without reason fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": " "}
			c.Budget = 1
		}, want: []string{"care.trail needs a non-empty reason"}},
		{name: "an exception for an erased table fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept", "care.parties": "kept"}
			c.Budget = 2
		}, want: []string{"care.parties is erased now"}},
		{name: "an exception outside the scope fails", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept", "care.gone": "kept"}
			c.Budget = 2
		}, want: []string{"care.gone is in demoErasureExceptions but is no table"}},
		{name: "the budget only falls", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept"}
		}, want: []string{"1 demo erasure exceptions, more than the 0 recorded"}},
		{name: "a lower count lowers the budget", change: func(c *demoErasureCoverage) {
			c.Exceptions = map[string]string{"care.trail": "kept"}
			c.Budget = 2
		}, want: []string{"1 demo erasure exceptions, fewer than the 2 recorded"}},
		{name: "an empty scope does not pass vacuously", change: func(c *demoErasureCoverage) {
			c.InScope = nil
		}, want: []string{"no table outlives the school"}},
		{name: "an unread function does not pass vacuously", change: func(c *demoErasureCoverage) {
			c.FunctionSQL = ""
		}, want: []string{"deletes from no table", "care.contracts holds personal data", "care.parties holds personal data", "care.party_notes holds personal data", trailMissing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			coverage := coverageFixture()
			tt.change(&coverage)
			problems := demoErasureCoverageProblems(coverage)
			require.Len(t, problems, len(tt.want), "problems: %q", problems)
			for _, want := range tt.want {
				require.True(t, slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }), "want %q in %q", want, problems)
			}
		})
	}
}
