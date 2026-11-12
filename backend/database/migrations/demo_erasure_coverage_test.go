package migrations

import (
	"context"
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The demo erasure guard of #6015. platform.erase_closed_demo_visitors
// removes a visitor when its demo school retires. PR #6000 found the care
// contracts of #4678, which outlive their school (ADR 0088 section 9), still
// naming the visitor after the erasure, and no test noticed.
//
// The guard takes its tables from the two registers every table already has:
// a table is in scope when the retention register (tableRetentions) says it
// holds personal data and the data lifecycle catalog places it at a school,
// the Träger (organization), an account, the demo process or a school
// reference. Only moto's own platform data (operators, caterer businesses)
// is out of scope: a visitor is never one of them.
//
// Each table in scope is erased by the function itself (DELETE or a
// rewriting UPDATE in its current definition or in a function it performs),
// goes by ON DELETE CASCADE with a table the function deletes, or names its
// reason in
// demoErasureExceptions (test/demo_erasure_ratchet_test.go). The demo
// school itself stays soft-deleted, so most school tables are such
// exceptions until the full erasure of retired demo schools (#5559) deletes
// them, the visitor's own free texts included.

// demoErasureScopes are the catalog scopes in scope; only "platform" is not.
var demoErasureScopes = []string{"tenant", "tenantChild", "organization", "account", "demo", "tenantReference", "school"}

// demoErasureRegisterFile holds demoErasureExceptions and
// demoErasureExceptionBudget among the source ratchets, which compare them
// with the pull request's base so they only fall.
const demoErasureRegisterFile = "../../test/demo_erasure_ratchet_test.go"

type noImports struct{}

func (noImports) Import(path string) (*types.Package, error) {
	return nil, fmt.Errorf("the register needs no import, not %s", path)
}

// readDemoErasureRegister evaluates the registers with go/types, as the
// ratchet does: their reasons are constant expressions of that file. It
// returns the table register with its budget and the outbox kind register
// of #6342 with its budget.
func readDemoErasureRegister(t *testing.T) (demoErasureRegister, demoErasureRegister) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, demoErasureRegisterFile, nil, 0)
	require.NoError(t, err)
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: noImports{}, Error: func(error) {}}
	_, _ = config.Check(file.Name.Name, fset, []*ast.File{file}, info)
	maps, budgets := map[string]map[string]string{}, map[string]int{}
	for _, declaration := range file.Decls {
		general, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range general.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 {
				continue
			}
			switch name := value.Names[0].Name; name {
			case "demoErasureExceptionBudget", "demoErasureOutboxKindExceptionBudget":
				number, exact := constant.Int64Val(info.Types[value.Values[0]].Value)
				require.True(t, exact, "the budget %s is an integer constant", name)
				budgets[name] = int(number)
			case "demoErasureExceptions", "demoErasureOutboxKindExceptions":
				literal, ok := value.Values[0].(*ast.CompositeLit)
				require.True(t, ok, "the register %s is a map literal", name)
				entries := map[string]string{}
				for _, element := range literal.Elts {
					entry := element.(*ast.KeyValueExpr)
					key, reason := info.Types[entry.Key].Value, info.Types[entry.Value].Value
					require.NotNil(t, key, "a key of %s is a constant", name)
					require.NotNil(t, reason, "a reason of %s is a constant", name)
					entries[constant.StringVal(key)] = constant.StringVal(reason)
				}
				maps[name] = entries
			}
		}
	}
	for _, name := range []string{"demoErasureExceptionBudget", "demoErasureOutboxKindExceptionBudget"} {
		_, declared := budgets[name]
		require.True(t, declared, "%s declares %s", demoErasureRegisterFile, name)
	}
	for _, name := range []string{"demoErasureExceptions", "demoErasureOutboxKindExceptions"} {
		_, declared := maps[name]
		require.True(t, declared, "%s declares %s", demoErasureRegisterFile, name)
	}
	require.NotEmpty(t, maps["demoErasureExceptions"], "%s declares demoErasureExceptions", demoErasureRegisterFile)
	return demoErasureRegister{Exceptions: maps["demoErasureExceptions"], Budget: budgets["demoErasureExceptionBudget"]},
		demoErasureRegister{Exceptions: maps["demoErasureOutboxKindExceptions"], Budget: budgets["demoErasureOutboxKindExceptionBudget"]}
}

// demoErasureRegister is one register of exceptions with its budget.
type demoErasureRegister struct {
	Exceptions map[string]string
	Budget     int
}

type cascadeEdge struct{ Child, Parent string }

type demoErasureCoverage struct {
	// InScope are the tables with personal data of a school, Träger or account.
	InScope []string
	// FunctionSQL is the current definition of platform.erase_closed_demo_visitors.
	FunctionSQL string
	// Cascades are the ON DELETE CASCADE foreign keys of the schema.
	Cascades   []cascadeEdge
	Exceptions map[string]string
	Budget     int
}

var sqlErasureStatement = regexp.MustCompile(`(?i)\b(DELETE\s+FROM|UPDATE)\s+(?:ONLY\s+)?([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)`)

// sqlWithoutCommentsAndStrings blanks line comments, nested block comments
// and single-quoted literals (” inside one, E'\” escapes included), so text
// there is never read as a statement. Dollar quotes stay: the function body
// itself is one.
func sqlWithoutCommentsAndStrings(sql string) string {
	masked, _ := maskedSQL(sql)
	return masked
}

// maskedSQL is sqlWithoutCommentsAndStrings that keeps the literals apart:
// the n-th literal reads 'n' in the text and literals[n] holds its value.
// A predicate can name a literal; text inside one is never a statement.
func maskedSQL(sql string) (masked string, literals []string) {
	var out strings.Builder
	for i := 0; i < len(sql); i++ {
		switch {
		case strings.HasPrefix(sql[i:], "--"):
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			out.WriteByte('\n')
		case strings.HasPrefix(sql[i:], "/*"):
			depth := 0
			for ; i < len(sql); i++ {
				if strings.HasPrefix(sql[i:], "/*") {
					depth++
					i++
				} else if strings.HasPrefix(sql[i:], "*/") {
					depth--
					i++
					if depth == 0 {
						break
					}
				}
			}
			out.WriteByte(' ')
		case sql[i] == '\'':
			escapes := i > 0 && (sql[i-1] == 'E' || sql[i-1] == 'e')
			var value strings.Builder
			for i++; i < len(sql); i++ {
				if escapes && sql[i] == '\\' && i+1 < len(sql) {
					i++
					value.WriteByte(sql[i])
				} else if sql[i] == '\'' {
					if i+1 < len(sql) && sql[i+1] == '\'' {
						i++
						value.WriteByte('\'')
						continue
					}
					break
				} else {
					value.WriteByte(sql[i])
				}
			}
			fmt.Fprintf(&out, "'%d'", len(literals))
			literals = append(literals, value.String())
		default:
			out.WriteByte(sql[i])
		}
	}
	return out.String(), literals
}

var sqlPerformedFunction = regexp.MustCompile(`(?i)\bPERFORM\s+([a-z_][a-z0-9_]*\.[a-z_][a-z0-9_]*)\s*\(`)

// demoErasureCalledFunctions returns the schema-qualified functions the
// erasure performs, outside pg_catalog. Comments and literals call none.
func demoErasureCalledFunctions(functionSQL string) []string {
	var called []string
	for _, match := range sqlPerformedFunction.FindAllStringSubmatch(sqlWithoutCommentsAndStrings(functionSQL), -1) {
		name := strings.ToLower(match[1])
		if !strings.HasPrefix(name, "pg_catalog.") && !slices.Contains(called, name) {
			called = append(called, name)
		}
	}
	return called
}

// demoErasureTargets returns the tables the function deletes from and the
// tables it deletes from or rewrites. Comments and literals are no
// statements.
func demoErasureTargets(functionSQL string) (deleted, erased map[string]bool) {
	deleted, erased = map[string]bool{}, map[string]bool{}
	for _, match := range sqlErasureStatement.FindAllStringSubmatch(sqlWithoutCommentsAndStrings(functionSQL), -1) {
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
		problems = append(problems, "no table with personal data is in scope; the scope query is broken and the guard would pass vacuously")
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
			problems = append(problems, fmt.Sprintf("%s holds personal data of the demo school or its visitor, but platform.erase_closed_demo_visitors neither erases it nor deletes a table it cascades from; erase it there (with visitorTraceCounts) or give a reason in demoErasureExceptions", table))
		case strings.TrimSpace(reason) == "":
			problems = append(problems, fmt.Sprintf("%s needs a non-empty reason in demoErasureExceptions", table))
		}
	}
	for table := range in.Exceptions {
		if !slices.Contains(in.InScope, table) {
			problems = append(problems, fmt.Sprintf("%s is in demoErasureExceptions but is no table with personal data in scope; remove it", table))
		}
	}
	switch count := len(in.Exceptions); {
	case count > in.Budget:
		problems = append(problems, fmt.Sprintf("%d demo erasure exceptions, more than the %d recorded; erase the new table instead, the budget only falls", count, in.Budget))
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
		if classified && retention.class != noPersonalData && slices.Contains(demoErasureScopes, scopes[table]) {
			inScope = append(inScope, table)
		}
	}
	var functionSQL string
	require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef('platform.erase_closed_demo_visitors()'::regprocedure)`).Scan(ctx, &functionSQL))
	// A function the erasure performs erases as part of it (#6812).
	read := map[string]bool{}
	for queue := demoErasureCalledFunctions(functionSQL); len(queue) > 0; queue = queue[1:] {
		if read[queue[0]] {
			continue
		}
		read[queue[0]] = true
		var definitions []string
		require.NoError(t, db.NewRaw(`SELECT pg_get_functiondef(p.oid) FROM pg_proc AS p
			JOIN pg_namespace AS n ON n.oid = p.pronamespace
			WHERE n.nspname || '.' || p.proname = ? AND p.prokind = 'f'`, queue[0]).Scan(ctx, &definitions))
		require.NotEmpty(t, definitions, "the erasure performs %s, which exists", queue[0])
		for _, definition := range definitions {
			functionSQL += "\n" + definition
			queue = append(queue, demoErasureCalledFunctions(definition)...)
		}
	}
	var cascades []cascadeEdge
	require.NoError(t, db.NewRaw(`
		SELECT DISTINCT cn.nspname || '.' || cc.relname AS child, pn.nspname || '.' || pc.relname AS parent
		FROM pg_constraint AS f
		JOIN pg_class AS cc ON cc.oid = f.conrelid JOIN pg_namespace AS cn ON cn.oid = cc.relnamespace
		JOIN pg_class AS pc ON pc.oid = f.confrelid JOIN pg_namespace AS pn ON pn.oid = pc.relnamespace
		WHERE f.contype = 'f' AND f.confdeltype = 'c'
		  -- A row that may outlive its parent by another foreign key to it
		  -- (ON DELETE SET NULL or SET DEFAULT) does not go with the parent:
		  -- the cascade holds only some of its rows (#6812).
		  AND NOT EXISTS (SELECT 1 FROM pg_constraint AS kept
			WHERE kept.contype = 'f' AND kept.conrelid = f.conrelid AND kept.confrelid = f.confrelid
			  AND kept.confdeltype IN ('n', 'd'))`).Scan(ctx, &cascades))
	sort.Strings(inScope)
	register, _ := readDemoErasureRegister(t)
	return demoErasureCoverage{
		InScope: inScope, FunctionSQL: functionSQL, Cascades: cascades,
		Exceptions: register.Exceptions, Budget: register.Budget,
	}
}

func TestDemoErasureCoversEveryTableWithPersonalData(t *testing.T) {
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
		require.Contains(t, problems, table+" holds personal data of the demo school or its visitor")
	}
}

// The gap of #6812 is caught: a row of parent_billing.family_billing that a
// holiday registration used only loses its contract when the contract goes
// (ON DELETE SET NULL), and a row without a contract never had one, so the
// cascade of the contract no longer counts for the table.
func TestDemoErasureGuardCatchesTheFamilyGapOf6812(t *testing.T) {
	t.Parallel()
	coverage := demoErasureCoverageOfSchema(t)
	withoutFamilies := regexp.MustCompile(`(?i)DELETE\s+FROM\s+parent_billing\.family_billing\b(\s+AS\s+family)`)
	require.True(t, withoutFamilies.MatchString(coverage.FunctionSQL), "the erasure deletes the families of the visitor (1.15.951)")
	coverage.FunctionSQL = withoutFamilies.ReplaceAllString(coverage.FunctionSQL, "SELECT 1 FROM parent_billing.family_billing$1")
	require.Contains(t, strings.Join(demoErasureCoverageProblems(coverage), "\n"),
		"parent_billing.family_billing holds personal data of the demo school or its visitor")
}

func TestDemoErasureCalledFunctions(t *testing.T) {
	t.Parallel()
	called := demoErasureCalledFunctions(`BEGIN
		PERFORM pg_advisory_xact_lock(1);
		PERFORM pg_catalog.pg_sleep(0);
		-- PERFORM care.in_a_comment(hidden_schools);
		RAISE LOG 'PERFORM care.in_a_literal(1)';
		PERFORM Care.Families(hidden_schools);
		PERFORM 1 FROM care.contracts;
		PERFORM care.families (hidden_schools);
	END`)
	require.Equal(t, []string{"care.families"}, called)
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
	trailMissing := "care.trail holds personal data of the demo school or its visitor"
	tests := []struct {
		name   string
		change func(*demoErasureCoverage)
		want   []string
	}{
		{name: "a table without erasure path is reported, a comment is no statement", change: func(*demoErasureCoverage) {}, want: []string{trailMissing}},
		{name: "an erasure path only in a block comment or a literal fails", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\n/* UPDATE care.trail SET name = 'Demo'; /* nested */ DELETE FROM care.trail; */" +
				"\nRAISE LOG 'DELETE FROM care.trail', E'it''s \\' UPDATE care.trail';"
		}, want: []string{trailMissing}},
		{name: "a statement after a comment and a literal still counts", change: func(c *demoErasureCoverage) {
			c.FunctionSQL += "\n/* note */ PERFORM 'a''b'; DELETE FROM care.trail;"
		}},
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
		}, want: []string{"no table with personal data is in scope"}},
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
