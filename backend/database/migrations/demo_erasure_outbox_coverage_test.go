package migrations

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The outbox guard of #6342. The table guard of #6015 counts an outbox
// table as erased once one statement deletes from it, but Delivery's outbox
// holds rows of many kinds, and a statement on it names only some. Before
// 1.15.808 the push of a sent contract (#5941) kept the recipient's address
// after the erasure although the push outbox had its statements (#6089).
//
// An outbox table is every table in scope with a kind, a recipient and a
// related entity (platform.email_outbox, platform.push_outbox). Its rows
// come in two scopes the guard checks apart:
//
//   - A row of a school belongs to the visitor whatever its kind: the
//     erasure deletes every row of the retired demo school with one
//     statement that filters on nothing else.
//   - A row of a Träger outlives the school it is about, so the erasure has
//     to find it by kind. The schema declares the kinds a row of a Träger
//     may have in a CHECK constraint on the table; no producer can enqueue
//     another. Each declared kind is named by a DELETE of the erasure on that
//     table (kind = '…' or kind IN (…) of the table itself, not in a comment
//     or a literal), or has its reason in demoErasureOutboxKindExceptions,
//     whose budget only falls.
//
// A mail without school or Träger (password reset, sign-in code) is the
// account's and goes with the erased account by its address; its kinds are
// not listed here.

// outboxTable is one outbox table of the schema.
type outboxTable struct {
	Name string
	// SchoolRows: the table has rows of a school (tenant_id).
	SchoolRows bool
	// TraegerRows: the table has rows of a Träger (organization_id).
	TraegerRows bool
	// TraegerKinds are the kinds its CHECK constraint allows a row of a
	// Träger; nil when no constraint declares them.
	TraegerKinds []string
}

type demoErasureOutboxCoverage struct {
	Outboxes    []outboxTable
	FunctionSQL string
	Exceptions  map[string]string
	Budget      int
}

// outboxKindKey is the key of a kind in demoErasureOutboxKindExceptions.
func outboxKindKey(table, kind string) string { return table + " " + kind }

var (
	sqlDeleteStatement = regexp.MustCompile(`(?is)\bDELETE\s+FROM\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)(?:\s+(?:AS\s+)?([a-z_][a-z0-9_]*))?`)
	sqlKindPredicate   = regexp.MustCompile(`(?is)(?:\b([a-z_][a-z0-9_]*)\.)?\bkind\s*(?:=\s*('\d+')|IN\s*\(([^)]*)\))`)
	sqlMaskedLiteral   = regexp.MustCompile(`'(\d+)'`)
	sqlSchoolRowsOnly  = regexp.MustCompile(`(?is)^\s*WHERE\s+tenant_id\s*=\s*ANY\s*\(\s*hidden_schools\s*\)\s*$`)
	traegerKindCheck   = regexp.MustCompile(`(?i)^CHECK organization_id IS NULL OR kind = ANY ARRAY\[(.*)\]$`)
	checkKindLiteral   = regexp.MustCompile(`'([^']*)'(?:::text)?`)
)

// outboxDelete is a DELETE of the erasure on one table.
type outboxDelete struct {
	// Kinds are the kinds of the table its WHERE names; empty when it names
	// none.
	Kinds []string
	// SchoolRows: it deletes every row of the retired schools, nothing else.
	SchoolRows bool
}

// outboxDeletes reads the DELETE statements of the function per table.
// Comments and literals are no statements; a kind is a literal its WHERE
// compares the kind of the deleted table with.
func outboxDeletes(functionSQL string) map[string][]outboxDelete {
	masked, literals := maskedSQL(functionSQL)
	deletes := map[string][]outboxDelete{}
	for _, statement := range strings.Split(masked, ";") {
		match := sqlDeleteStatement.FindStringSubmatchIndex(statement)
		if match == nil {
			continue
		}
		table := strings.ToLower(statement[match[2]:match[3]])
		name := table[strings.Index(table, ".")+1:]
		alias, rest := name, statement[match[1]:]
		if match[4] >= 0 {
			// The word after the table is its alias, or already WHERE or USING.
			if word := strings.ToLower(statement[match[4]:match[5]]); word == "where" || word == "using" {
				rest = statement[match[4]:]
			} else {
				alias = word
			}
		}
		found := outboxDelete{SchoolRows: alias == name && sqlSchoolRowsOnly.MatchString(rest)}
		for _, predicate := range sqlKindPredicate.FindAllStringSubmatch(rest, -1) {
			if qualifier := strings.ToLower(predicate[1]); qualifier != "" && qualifier != alias {
				continue
			}
			for _, literal := range sqlMaskedLiteral.FindAllStringSubmatch(predicate[2]+predicate[3], -1) {
				index, err := strconv.Atoi(literal[1])
				if err == nil && index < len(literals) {
					found.Kinds = append(found.Kinds, literals[index])
				}
			}
		}
		deletes[table] = append(deletes[table], found)
	}
	return deletes
}

// traegerKindsOfCheck reads the kinds of a CHECK constraint
// "organization_id IS NULL OR kind IN (…)", as pg_get_constraintdef prints
// it; nil for any other constraint.
func traegerKindsOfCheck(definition string) []string {
	// The kinds hold no parentheses; without them the definition reads
	// CHECK organization_id IS NULL OR kind = ANY ARRAY[…].
	flat := strings.Join(strings.Fields(strings.NewReplacer("(", " ", ")", " ").Replace(definition)), " ")
	match := traegerKindCheck.FindStringSubmatch(flat)
	if match == nil {
		return nil
	}
	var kinds []string
	for _, literal := range checkKindLiteral.FindAllStringSubmatch(match[1], -1) {
		kinds = append(kinds, literal[1])
	}
	return kinds
}

func demoErasureOutboxProblems(in demoErasureOutboxCoverage) []string {
	var problems []string
	if len(in.Outboxes) == 0 {
		problems = append(problems, "no outbox table is in scope; the scope query is broken and the guard would pass vacuously")
	}
	deletes := outboxDeletes(in.FunctionSQL)
	declared := map[string]bool{}
	for _, outbox := range in.Outboxes {
		if outbox.SchoolRows && !slices.ContainsFunc(deletes[outbox.Name], func(d outboxDelete) bool { return d.SchoolRows }) {
			problems = append(problems, fmt.Sprintf("%s keeps rows of the retired demo school; platform.erase_closed_demo_visitors needs DELETE FROM %s WHERE tenant_id = ANY (hidden_schools), whatever their kind", outbox.Name, outbox.Name))
		}
		if !outbox.TraegerRows {
			continue
		}
		if len(outbox.TraegerKinds) == 0 {
			problems = append(problems, fmt.Sprintf("%s has rows of a Träger, but no CHECK (organization_id IS NULL OR kind IN (…)) declares their kinds; declare them, so the guard sees every kind", outbox.Name))
			continue
		}
		for _, kind := range outbox.TraegerKinds {
			key := outboxKindKey(outbox.Name, kind)
			declared[key] = true
			erased := slices.ContainsFunc(deletes[outbox.Name], func(d outboxDelete) bool { return slices.Contains(d.Kinds, kind) })
			reason, excepted := in.Exceptions[key]
			switch {
			case erased && excepted:
				problems = append(problems, fmt.Sprintf("%s is erased now; remove it from demoErasureOutboxKindExceptions and lower the budget", key))
			case erased:
			case !excepted:
				problems = append(problems, fmt.Sprintf("%s: a row of a Träger of this kind can name the demo school's visitor, but no DELETE of platform.erase_closed_demo_visitors on %s names kind %q; erase it there (with a test in organizationtenancy/compose) or give a reason in demoErasureOutboxKindExceptions", key, outbox.Name, kind))
			case strings.TrimSpace(reason) == "":
				problems = append(problems, fmt.Sprintf("%s needs a non-empty reason in demoErasureOutboxKindExceptions", key))
			}
		}
	}
	for key := range in.Exceptions {
		if !declared[key] {
			problems = append(problems, fmt.Sprintf("%s is in demoErasureOutboxKindExceptions but is no declared kind of a Träger; remove it", key))
		}
	}
	switch count := len(in.Exceptions); {
	case count > in.Budget:
		problems = append(problems, fmt.Sprintf("%d outbox kind exceptions, more than the %d recorded; erase the new kind instead, the budget only falls", count, in.Budget))
	case count < in.Budget:
		problems = append(problems, fmt.Sprintf("%d outbox kind exceptions, fewer than the %d recorded; lower demoErasureOutboxKindExceptionBudget", count, in.Budget))
	}
	sort.Strings(problems)
	return problems
}

// demoErasureOutboxCoverageOfSchema reads the outbox tables, their kinds of
// a Träger and the function from the migrated schema.
func demoErasureOutboxCoverageOfSchema(t *testing.T) demoErasureOutboxCoverage {
	t.Helper()
	tables := demoErasureCoverageOfSchema(t)
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	var columns []struct {
		Table   string
		Columns []string `bun:",array"`
	}
	require.NoError(t, db.NewRaw(`
		SELECT table_schema || '.' || table_name AS "table", array_agg(column_name::text ORDER BY column_name) AS columns
		FROM information_schema.columns
		WHERE column_name IN ('kind', 'recipient', 'related_entity_type', 'tenant_id', 'organization_id')
		GROUP BY 1`).Scan(ctx, &columns))
	var outboxes []outboxTable
	for _, table := range columns {
		has := func(column string) bool { return slices.Contains(table.Columns, column) }
		if !slices.Contains(tables.InScope, table.Table) || !has("kind") || !has("recipient") || !has("related_entity_type") {
			continue
		}
		outbox := outboxTable{Name: table.Table, SchoolRows: has("tenant_id"), TraegerRows: has("organization_id")}
		var checks []string
		require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conrelid = ?::regclass AND contype = 'c'`, table.Table).Scan(ctx, &checks))
		for _, check := range checks {
			outbox.TraegerKinds = append(outbox.TraegerKinds, traegerKindsOfCheck(check)...)
		}
		outboxes = append(outboxes, outbox)
	}
	sort.Slice(outboxes, func(i, j int) bool { return outboxes[i].Name < outboxes[j].Name })
	_, kinds := readDemoErasureRegister(t)
	return demoErasureOutboxCoverage{Outboxes: outboxes, FunctionSQL: tables.FunctionSQL, Exceptions: kinds.Exceptions, Budget: kinds.Budget}
}

func TestDemoErasureCoversEveryOutboxKind(t *testing.T) {
	t.Parallel()
	coverage := demoErasureOutboxCoverageOfSchema(t)
	var names []string
	for _, outbox := range coverage.Outboxes {
		names = append(names, outbox.Name)
	}
	require.Equal(t, []string{"platform.email_outbox", "platform.push_outbox"}, names, "the outbox tables of Delivery are in scope")
	require.Empty(t, demoErasureOutboxProblems(coverage),
		"demo erasure outbox coverage (#6342): every row of a retired demo school goes, and every kind of a Träger is named by a DELETE of platform.erase_closed_demo_visitors or has a reason in demoErasureOutboxKindExceptions")
}

// The gap of #6089 is caught on the real function: without the statement
// that deletes the push of a sent contract, its kind is reported, although
// other statements still delete from the push outbox.
func TestDemoErasureOutboxGuardCatchesTheContractPushGapOf6089(t *testing.T) {
	t.Parallel()
	coverage := demoErasureOutboxCoverageOfSchema(t)
	contractPush := regexp.MustCompile(`(?s)DELETE FROM platform\.push_outbox AS push\s+USING care_contract\.contracts AS contract[^;]*;`)
	require.Len(t, contractPush.FindAllString(coverage.FunctionSQL, -1), 1, "the erasure deletes the push of a sent contract (1.15.808)")
	coverage.FunctionSQL = contractPush.ReplaceAllString(coverage.FunctionSQL, "PERFORM 1;")
	require.Contains(t, outboxDeletes(coverage.FunctionSQL), "platform.push_outbox", "other statements still delete from the push outbox")
	problems := demoErasureOutboxProblems(coverage)
	require.Len(t, problems, 1, "problems: %q", problems)
	require.Contains(t, problems[0], `platform.push_outbox care_contract_document: a row of a Träger of this kind`)
}

// A new kind of a Träger in a table the erasure already deletes from, with
// no statement widened for it, is reported on the real schema; so is an
// outbox whose rows of the school no longer all go.
func TestDemoErasureOutboxGuardCatchesANewKindInACoveredTable(t *testing.T) {
	t.Parallel()
	coverage := demoErasureOutboxCoverageOfSchema(t)
	for i := range coverage.Outboxes {
		coverage.Outboxes[i].TraegerKinds = append(coverage.Outboxes[i].TraegerKinds, "traeger_new_notice")
	}
	problems := strings.Join(demoErasureOutboxProblems(coverage), "\n")
	require.Contains(t, problems, `platform.email_outbox traeger_new_notice: a row of a Träger of this kind`)
	require.Contains(t, problems, `platform.push_outbox traeger_new_notice: a row of a Träger of this kind`)

	coverage = demoErasureOutboxCoverageOfSchema(t)
	schoolPushes := regexp.MustCompile(`DELETE FROM platform\.push_outbox\s+WHERE tenant_id = ANY \(hidden_schools\);`)
	require.Len(t, schoolPushes.FindAllString(coverage.FunctionSQL, -1), 1, "the erasure deletes every push of the school")
	coverage.FunctionSQL = schoolPushes.ReplaceAllString(coverage.FunctionSQL, "DELETE FROM platform.push_outbox WHERE tenant_id = ANY (hidden_schools) AND kind = 'parent_message';")
	require.Equal(t, []string{"platform.push_outbox keeps rows of the retired demo school; platform.erase_closed_demo_visitors needs DELETE FROM platform.push_outbox WHERE tenant_id = ANY (hidden_schools), whatever their kind"},
		demoErasureOutboxProblems(coverage))
}

func outboxCoverageFixture() demoErasureOutboxCoverage {
	return demoErasureOutboxCoverage{
		Outboxes: []outboxTable{{Name: "mail.outbox", SchoolRows: true, TraegerRows: true, TraegerKinds: []string{"contract", "receipt"}}},
		FunctionSQL: `BEGIN
			DELETE FROM mail.outbox WHERE tenant_id = ANY (hidden_schools);
			DELETE FROM mail.outbox AS outbox USING care.contracts AS contract
			WHERE contract.school_id = ANY (hidden_schools) AND outbox.kind = 'contract' AND outbox.related_entity_id = contract.id;
		END`,
		Exceptions: map[string]string{},
	}
}

func TestDemoErasureOutboxProblems(t *testing.T) {
	t.Parallel()
	receiptMissing := "mail.outbox receipt: a row of a Träger of this kind"
	tests := []struct {
		name   string
		change func(*demoErasureOutboxCoverage)
		want   []string
	}{
		{name: "a kind no statement names is reported although the table has its DELETEs", change: func(*demoErasureOutboxCoverage) {}, want: []string{receiptMissing}},
		{name: "a kind named in a comment, a literal or the kind of another table fails", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL += "\n-- DELETE FROM mail.outbox WHERE kind = 'receipt';" +
				"\n/* DELETE FROM mail.outbox WHERE kind = 'receipt'; */" +
				"\nRAISE LOG 'DELETE FROM mail.outbox WHERE kind = ''receipt''';" +
				"\nDELETE FROM mail.outbox AS outbox USING mail.letters AS letter WHERE letter.kind = 'receipt' AND outbox.id = letter.mail_id;" +
				"\nDELETE FROM mail.letters WHERE kind = 'receipt';"
		}, want: []string{receiptMissing}},
		{name: "a kind in an IN list of the table counts, qualified or not", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL += "\nDELETE FROM mail.outbox WHERE organization_id = 1 AND kind IN ('other', 'receipt');"
		}},
		{name: "a kind of the table under its alias counts after a comment and a literal", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL += "\n/* note */ PERFORM 'a''b'; DELETE FROM mail.outbox AS o WHERE o.kind = 'receipt' AND o.payload->>'x' = 'y';"
		}},
		{name: "a deletion of the kind from another table does not count", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL += "\nDELETE FROM mail.archive WHERE kind = 'receipt';"
		}, want: []string{receiptMissing}},
		{name: "a narrower school statement does not delete every row of the school", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL = strings.Replace(c.FunctionSQL, "WHERE tenant_id = ANY (hidden_schools);", "WHERE tenant_id = ANY (hidden_schools) AND kind = 'x';", 1)
		}, want: []string{"mail.outbox keeps rows of the retired demo school", receiptMissing}},
		{name: "a school statement only in a comment fails", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL = strings.Replace(c.FunctionSQL, "DELETE FROM mail.outbox WHERE tenant_id", "-- DELETE FROM mail.outbox WHERE tenant_id", 1)
		}, want: []string{"mail.outbox keeps rows of the retired demo school", receiptMissing}},
		{name: "a table without rows of a school needs no school statement", change: func(c *demoErasureOutboxCoverage) {
			c.Outboxes[0].SchoolRows = false
			c.FunctionSQL = strings.Replace(c.FunctionSQL, "DELETE FROM mail.outbox WHERE tenant_id", "-- DELETE FROM mail.outbox WHERE tenant_id", 1)
		}, want: []string{receiptMissing}},
		{name: "rows of a Träger without declared kinds fail", change: func(c *demoErasureOutboxCoverage) {
			c.Outboxes[0].TraegerKinds = nil
		}, want: []string{"mail.outbox has rows of a Träger, but no CHECK"}},
		{name: "a reasoned exception within budget passes", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": "names no one"}
			c.Budget = 1
		}},
		{name: "an exception without reason fails", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": " "}
			c.Budget = 1
		}, want: []string{"mail.outbox receipt needs a non-empty reason"}},
		{name: "an exception for an erased kind fails", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": "kept", "mail.outbox contract": "kept"}
			c.Budget = 2
		}, want: []string{"mail.outbox contract is erased now"}},
		{name: "an exception for an undeclared kind fails", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": "kept", "mail.outbox gone": "kept"}
			c.Budget = 2
		}, want: []string{"mail.outbox gone is in demoErasureOutboxKindExceptions but is no declared kind"}},
		{name: "the budget only falls", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": "kept"}
		}, want: []string{"1 outbox kind exceptions, more than the 0 recorded"}},
		{name: "a lower count lowers the budget", change: func(c *demoErasureOutboxCoverage) {
			c.Exceptions = map[string]string{"mail.outbox receipt": "kept"}
			c.Budget = 2
		}, want: []string{"1 outbox kind exceptions, fewer than the 2 recorded"}},
		{name: "no outbox table does not pass vacuously", change: func(c *demoErasureOutboxCoverage) {
			c.Outboxes = nil
		}, want: []string{"no outbox table is in scope"}},
		{name: "an unread function does not pass vacuously", change: func(c *demoErasureOutboxCoverage) {
			c.FunctionSQL = ""
		}, want: []string{"mail.outbox keeps rows of the retired demo school", "mail.outbox contract: a row of a Träger", receiptMissing}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			coverage := outboxCoverageFixture()
			tt.change(&coverage)
			problems := demoErasureOutboxProblems(coverage)
			require.Len(t, problems, len(tt.want), "problems: %q", problems)
			for _, want := range tt.want {
				require.True(t, slices.ContainsFunc(problems, func(p string) bool { return strings.Contains(p, want) }), "want %q in %q", want, problems)
			}
		})
	}
}

func TestTraegerKindsOfCheck(t *testing.T) {
	t.Parallel()
	require.Equal(t, []string{"a_kind", "b"}, traegerKindsOfCheck(
		`CHECK (((organization_id IS NULL) OR (kind = ANY (ARRAY['a_kind'::text, 'b'::text]))))`))
	require.Nil(t, traegerKindsOfCheck(`CHECK (((tenant_id IS NULL) OR (organization_id IS NULL)))`))
	require.Nil(t, traegerKindsOfCheck(`CHECK ((status = ANY (ARRAY['pending'::text, 'sent'::text])))`))
}
