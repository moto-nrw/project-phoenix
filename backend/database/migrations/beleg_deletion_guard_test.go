package migrations

import (
	"context"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The deletion guard of the Belege of a Träger (#4720 slice 1, legal
// Löschkonzept vertragspaket/intern/loeschkonzept-traeger-finanzdaten.md
// section 4). Invoices, cancellations and credits, corrections, notices,
// care contracts with their versions and signatures, SEPA mandates and
// debits are kept for the Träger; deleting a child, a holiday registration,
// a guardian or an account must never take one along. The Belege keep their
// own copy of what they need, so no reference ties them to the operative
// child profile either.
//
// Every table of the schemas parent_billing and care_contract is a Beleg
// table unless operativeBillingTables or retentionControlTables names it. A
// new table is therefore guarded until someone states why it holds no Beleg.

// operativeBillingTables are the tables of the two schemas that hold no
// Beleg: inputs of a calculation that go with the registration or the child
// they describe. The issued documents keep their own copy.
var operativeBillingTables = map[string]string{
	"parent_billing.holiday_contributions":         "frozen contribution of a registered child; the invoice copies its lines into parent_billing.document_positions",
	"parent_billing.holiday_contribution_lines":    "lines of a frozen contribution, see parent_billing.holiday_contributions",
	"parent_billing.run_corrections":               "correction of a calculated holiday run; the release copies amount and reason into a correction position of the invoice (document_positions.note), which stays",
	"parent_billing.holiday_sibling_groups":        "sibling group of holiday registrations for the discount of a calculation",
	"parent_billing.holiday_sibling_group_members": "registration in a sibling group, goes with the registration",
}

// retentionControlTables hold the retention of the Belege, not a Beleg
// (#4720 slice 2, ADR 0184): the Weisungen and suspensions the moto team
// records, the record per Beleg, the deletion log and the reviews of the
// clarification list. They name the operator who recorded them, so beside
// the Träger they may reference platform.operators, and only with ON
// DELETE SET NULL; a Beleg table may not.
var retentionControlTables = map[string]string{
	"parent_billing.beleg_retention_instructions": "Weisung of a Träger per Beleg type, recorded by an operator",
	"parent_billing.beleg_retention_suspensions":  "suspension of the deletion of a type or Beleg, recorded and lifted by an operator",
	"parent_billing.beleg_retentions":             "retention record per Beleg: IDs and days only, no content",
	"parent_billing.beleg_deletions":              "deletion log: counts per Träger, type and day, no content",
	"parent_billing.beleg_clarification_reviews":  "review of a clarification entry with note and escalation, recorded by an operator",
}

// belegErasureFunctions are the only database functions allowed to delete
// rows of a Beleg table by name:
//   - the erasure of a retired demo school, whose Träger keeps no books,
//     with the debtor numbers of the visitor's families it performs
//     (platform.erase_demo_visitor_families, #6812);
//   - the Beleg retention (#4720 slice 2, ADR 0184), which deletes a Beleg
//     only after the period its Träger instructed, under the current
//     Weisung, without an active suspension, after locking the Träger and
//     the rows, and only when every Beleg its cascade would take passed the
//     same check. It is never reached from operative data: only the hourly
//     job calls it, in the administrative transaction;
//   - the stored PAIN.008 files of parent_billing.sepa_debit_files (#6314,
//     ADR 0187), which follow their members: a submitted file goes only with
//     the last of its debits and installment notices (so after their Beleg
//     retention), an unsubmitted download with any member, when a later
//     download supersedes it and 30 days after the download. No Beleg
//     besides the file itself is deleted by them.
//   - the release of the DATEV bookings of a handover that failed (#6824,
//     ADR 0218 section 6): parent_billing.datev_booked_documents only marks
//     which booking went to DATEV first. When the handover of that DATEV
//     file fails, the file never arrived and its markers go, so the next
//     file holds those bookings again. Invoices, Stornos, documents and the
//     export file with its protocol entry stay.
//
// The erasure of a whole Träger after its export (#4657) deletes the
// organization and reaches the Belege through its cascade.
var belegErasureFunctions = []string{
	"parent_billing.delete_due_belege",
	"parent_billing.delete_expired_sepa_debit_downloads",
	"parent_billing.drop_sepa_debit_files_with_their_members",
	"parent_billing.drop_superseded_sepa_debit_files",
	"parent_billing.release_failed_handover_bookings",
	"platform.erase_closed_demo_visitors",
	"platform.erase_demo_visitor_families",
}

type belegForeignKey struct {
	Child    string `bun:"child"`
	Parent   string `bun:"parent"`
	OnDelete string `bun:"on_delete"`
	Name     string `bun:"name"`
}

func belegSchemaTable(table string) bool {
	return strings.HasPrefix(table, "parent_billing.") || strings.HasPrefix(table, "care_contract.") ||
		table == "audit.traeger_signatures"
}

func isBelegTable(table string) bool {
	_, operative := operativeBillingTables[table]
	_, control := retentionControlTables[table]
	return belegSchemaTable(table) && !operative && !control
}

// belegIdentifier is a schema-qualified table name of the guarded schemas,
// with or without quotes around each part.
const belegIdentifier = `"?(parent_billing|care_contract|audit)"?\s*\.\s*"?(\w+)"?`

// belegDeleteStatement finds DELETE FROM [ONLY] and TRUNCATE [TABLE] [ONLY]
// with a list of tables; belegDeleteTargets splits the list.
var (
	belegDeleteStatement = regexp.MustCompile(`(?is)(?:delete\s+from|truncate(?:\s+table)?)(?:\s+only)?\s+(` +
		belegIdentifier + `(?:\s*,\s*(?:only\s+)?"?\w+"?\s*\.\s*"?\w+"?)*)`)
	belegTableName = regexp.MustCompile(`(?i)` + belegIdentifier)
)

// belegDeleteTargets returns the guarded tables a function source deletes
// from or truncates, in lower case.
func belegDeleteTargets(source string) []string {
	var targets []string
	for _, statement := range belegDeleteStatement.FindAllStringSubmatch(source, -1) {
		for _, name := range belegTableName.FindAllStringSubmatch(statement[1], -1) {
			targets = append(targets, strings.ToLower(name[1]+"."+name[2]))
		}
	}
	return targets
}

func TestBelegDeleteTargetsReadsEveryDeletionForm(t *testing.T) {
	t.Parallel()
	source := `DELETE FROM parent_billing.care_invoices WHERE true;
		delete from "care_contract"."contract_documents";
		DELETE FROM ONLY "parent_billing" . sepa_debits;
		TRUNCATE TABLE parent_billing.runs, "care_contract".contracts;
		truncate only audit.traeger_signatures;
		DELETE FROM enrollment.holiday_registrations;`
	assert.Equal(t, []string{
		"parent_billing.care_invoices", "care_contract.contract_documents", "parent_billing.sepa_debits",
		"parent_billing.runs", "care_contract.contracts", "audit.traeger_signatures",
	}, belegDeleteTargets(source))
}

func TestBelegeHaveNoDeletionPathFromOperativeData(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	var tables []string
	require.NoError(t, db.NewRaw(`SELECT table_schema || '.' || table_name FROM information_schema.tables
		WHERE table_schema IN ('parent_billing', 'care_contract') AND table_type = 'BASE TABLE'`).Scan(ctx, &tables))
	for table := range operativeBillingTables {
		assert.Contains(t, tables, table, "operativeBillingTables names a table that no longer exists")
	}
	for table := range retentionControlTables {
		assert.Contains(t, tables, table, "retentionControlTables names a table that no longer exists")
	}

	var keys []belegForeignKey
	require.NoError(t, db.NewRaw(`SELECT child_ns.nspname || '.' || child.relname AS child,
			parent_ns.nspname || '.' || parent.relname AS parent, con.confdeltype::text AS on_delete, con.conname AS name
		FROM pg_constraint AS con
		JOIN pg_class AS child ON child.oid = con.conrelid
		JOIN pg_namespace AS child_ns ON child_ns.oid = child.relnamespace
		JOIN pg_class AS parent ON parent.oid = con.confrelid
		JOIN pg_namespace AS parent_ns ON parent_ns.oid = parent.relnamespace
		WHERE con.contype = 'f'`).Scan(ctx, &keys))
	parents := map[string][]belegForeignKey{}
	for _, key := range keys {
		parents[key.Child] = append(parents[key.Child], key)
	}

	// A Beleg table references outside its schemas only the Träger, which
	// takes it along on its erasure, and accounts, whose reference it drops.
	// Any other key would tie it to operative data: a cascade deletes it, a
	// restriction keeps the operative row alive (data minimisation). A
	// retention-control table may also name its operator and drops that
	// reference when the operator goes.
	var outside []string
	for _, key := range keys {
		_, control := retentionControlTables[key.Child]
		if (!isBelegTable(key.Child) && !control) || belegSchemaTable(key.Parent) {
			continue
		}
		allowed := key.Parent == "platform.organizations" || (key.Parent == "auth.accounts" && key.OnDelete == "n") ||
			(control && key.Parent == "platform.operators" && key.OnDelete == "n")
		if !allowed {
			outside = append(outside, key.Child+" -> "+key.Parent+" ("+key.Name+", on delete "+key.OnDelete+")")
		}
	}
	sort.Strings(outside)
	assert.Empty(t, outside, "a Beleg table references operative data")

	// No chain of ON DELETE CASCADE leads from an operative table into a
	// Beleg table, also not through the operative tables of the schemas.
	var cascades []string
	for _, table := range tables {
		if !isBelegTable(table) {
			continue
		}
		seen := map[string]bool{table: true}
		queue := []string{table}
		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			for _, key := range parents[current] {
				if key.OnDelete != "c" || seen[key.Parent] {
					continue
				}
				seen[key.Parent] = true
				if key.Parent == "platform.organizations" {
					continue
				}
				if !isBelegTable(key.Parent) {
					cascades = append(cascades, table+" <- "+key.Parent+" ("+key.Name+")")
					continue
				}
				queue = append(queue, key.Parent)
			}
		}
	}
	sort.Strings(cascades)
	assert.Empty(t, cascades, "deleting operative data cascades into a Beleg table")

	// No database function deletes or truncates a Beleg table by name except
	// the demo erasure, also with quoted identifiers or ONLY. Statements built
	// at run time (EXECUTE) are outside this check.
	var functions []struct {
		Name   string `bun:"name"`
		Source string `bun:"source"`
	}
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || p.proname AS name, p.prosrc AS source
		FROM pg_proc AS p JOIN pg_namespace AS n ON n.oid = p.pronamespace
		WHERE p.prosrc ~* '(delete|truncate)'`).Scan(ctx, &functions))
	var deleters []string
	for _, function := range functions {
		for _, table := range belegDeleteTargets(function.Source) {
			if isBelegTable(table) && !slices.Contains(deleters, function.Name) {
				deleters = append(deleters, function.Name)
			}
		}
	}
	sort.Strings(deleters)
	assert.Equal(t, belegErasureFunctions, deleters, "only the demo erasure and the Beleg retention delete Belege by name")
}
