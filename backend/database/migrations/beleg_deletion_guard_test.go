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

	testpkg "github.com/moto-nrw/project-phoenix/test"
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
// table unless operativeBillingTables names it. A new table is therefore
// guarded until someone states why it is operative.

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

// belegErasureFunctions are the only database functions allowed to delete
// rows of a Beleg table by name: the erasure of a retired demo school, whose
// Träger keeps no books. The erasure of a whole Träger after its export
// (#4657) deletes the organization and reaches the Belege through its
// cascade.
var belegErasureFunctions = []string{
	"platform.erase_closed_demo_visitors",
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
	return belegSchemaTable(table) && !operative
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
	// restriction keeps the operative row alive (data minimisation).
	var outside []string
	for _, key := range keys {
		if !isBelegTable(key.Child) || belegSchemaTable(key.Parent) {
			continue
		}
		allowed := key.Parent == "platform.organizations" || (key.Parent == "auth.accounts" && key.OnDelete == "n")
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

	// No database function deletes a Beleg row except the erasure of a
	// whole Träger.
	var functions []struct {
		Name   string `bun:"name"`
		Source string `bun:"source"`
	}
	require.NoError(t, db.NewRaw(`SELECT n.nspname || '.' || p.proname AS name, p.prosrc AS source
		FROM pg_proc AS p JOIN pg_namespace AS n ON n.oid = p.pronamespace
		WHERE p.prosrc ~* 'delete\s+from\s+(parent_billing|care_contract|audit)\.'`).Scan(ctx, &functions))
	deleteFrom := regexp.MustCompile(`(?i)delete\s+from\s+((?:parent_billing|care_contract|audit)\.\w+)`)
	var deleters []string
	for _, function := range functions {
		for _, match := range deleteFrom.FindAllStringSubmatch(function.Source, -1) {
			if isBelegTable(strings.ToLower(match[1])) && !slices.Contains(deleters, function.Name) {
				deleters = append(deleters, function.Name)
			}
		}
	}
	sort.Strings(deleters)
	assert.Equal(t, belegErasureFunctions, deleters, "only the demo erasure deletes Belege by name")
}
