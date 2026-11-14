package migrations

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The Träger counterpart of TestEveryTenantTableHasRLS (#4692): a table with
// an organization_id column that the request role reaches is a Träger table.
// The Träger transaction of ADR 0105 runs as phoenix_tenant with
// app.current_org_id, so only a forced policy keeps one Träger out of another
// Träger's rows. A Träger table the request role cannot reach at all, such as
// audit.traeger_account_changes (administrative role only), needs none.

type organizationRLSCoverage struct {
	TableName     string
	Enabled       bool
	Forced        bool
	HasPolicy     bool
	RequestPolicy bool
	RequestWrite  bool
}

// Only read-only shared tables belong here, never Träger isolation gaps. Keep
// the budget in sync downwards; removed or repaired entries fail.
const organizationRLSExceptionBudget = 1

var organizationRLSExceptions = map[string]string{
	"platform.schools": "School master data the request role only reads (SELECT, no write privilege): a school and a Träger transaction resolve their school rows by ID; organization_id is the school's Träger, not a Träger-private row.",
}

func readOrganizationRLSCoverage(ctx context.Context, db bun.IDB) ([]organizationRLSCoverage, error) {
	// Like the tenant guard this checks the catalog, not the predicate; the
	// predicate behavior stays in the migration tests of each Träger table.
	var tables []organizationRLSCoverage
	err := db.NewRaw(`
		SELECT n.nspname || '.' || c.relname AS table_name,
		       c.relrowsecurity AS enabled, c.relforcerowsecurity AS forced,
		       EXISTS (SELECT FROM pg_policy p WHERE p.polrelid = c.oid) AS has_policy,
		       EXISTS (
		           SELECT FROM pg_policy p, unnest(p.polroles) AS role_oid
		           WHERE p.polrelid = c.oid
		             AND CASE WHEN role_oid = 0 THEN true
		                      ELSE pg_has_role('phoenix_tenant', role_oid, 'USAGE') END
		       ) AS request_policy,
		       (has_table_privilege('phoenix_tenant', c.oid, 'INSERT,UPDATE,DELETE,TRUNCATE')
		        OR has_any_column_privilege('phoenix_tenant', c.oid, 'INSERT,UPDATE')) AS request_write
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
		  AND EXISTS (
		      SELECT FROM pg_attribute a WHERE a.attrelid = c.oid
		        AND a.attname = 'organization_id' AND a.attnum > 0 AND NOT a.attisdropped
		  )
		  AND (has_table_privilege('phoenix_tenant', c.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
		       OR has_any_column_privilege('phoenix_tenant', c.oid, 'SELECT,INSERT,UPDATE,REFERENCES'))
		ORDER BY 1`).Scan(ctx, &tables)
	return tables, err
}

func organizationRLSProblems(tables []organizationRLSCoverage, exceptions map[string]string) []string {
	var problems []string
	seen := make(map[string]bool, len(tables))
	for _, table := range tables {
		seen[table.TableName] = true
		var missing []string
		if !table.Enabled {
			missing = append(missing, "ENABLE RLS")
		}
		if !table.Forced {
			missing = append(missing, "FORCE RLS")
		}
		if !table.HasPolicy {
			missing = append(missing, "policy")
		}
		if !table.RequestPolicy {
			missing = append(missing, "policy applicable to phoenix_tenant")
		}
		if reason, excepted := exceptions[table.TableName]; excepted {
			if table.RequestWrite {
				problems = append(problems, table.TableName+" is read-only shared data but phoenix_tenant can write it")
			}
			if strings.TrimSpace(reason) == "" {
				problems = append(problems, table.TableName+" needs an exception reason")
			}
			if len(missing) == 0 {
				problems = append(problems, table.TableName+" now satisfies RLS coverage; remove its exception")
			}
			continue
		}
		if len(missing) > 0 {
			problems = append(problems, fmt.Sprintf("%s missing %s", table.TableName, strings.Join(missing, ", ")))
		}
	}
	for table := range exceptions {
		if !seen[table] {
			problems = append(problems, table+" is excepted but is no Träger table the request role reaches")
		}
	}
	sort.Strings(problems)
	return problems
}

func TestEveryTraegerTableHasRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tables, err := readOrganizationRLSCoverage(context.Background(), db)
	require.NoError(t, err)
	require.NotEmpty(t, tables, "Träger table discovery must not pass vacuously")
	require.Len(t, organizationRLSExceptions, organizationRLSExceptionBudget, "Träger RLS exceptions may only shrink; lower the budget when removing one")
	require.Empty(t, organizationRLSProblems(tables, organizationRLSExceptions), "Träger RLS coverage")
}

func TestTraegerRLSCoverageDiscoversSchemaChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	const table = `CREATE TABLE public.org_rls_probe (organization_id bigint); GRANT SELECT, INSERT ON public.org_rls_probe TO phoenix_tenant;`
	const forced = table + ` ALTER TABLE public.org_rls_probe ENABLE ROW LEVEL SECURITY; ALTER TABLE public.org_rls_probe FORCE ROW LEVEL SECURITY;`
	for _, tc := range []struct {
		name       string
		ddl        string
		discovered bool
		protected  bool
	}{
		{"new Träger table", table, true, false},
		{"enable only", table + ` ALTER TABLE public.org_rls_probe ENABLE ROW LEVEL SECURITY`, true, false},
		{"no policy", forced, true, false},
		{"wrong role", forced + ` CREATE POLICY probe ON public.org_rls_probe TO phoenix_auth USING (true)`, true, false},
		{"Träger policy", forced + ` CREATE POLICY probe ON public.org_rls_probe FOR ALL
			USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)`, true, true},
		{"request role policy", forced + ` CREATE POLICY probe ON public.org_rls_probe TO phoenix_tenant USING (true)`, true, true},
		{"administrative only", `CREATE TABLE public.org_rls_probe (organization_id bigint)`, false, false},
		{"dropped column", table + ` ALTER TABLE public.org_rls_probe DROP COLUMN organization_id`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			require.NoError(t, err)
			defer func() { require.NoError(t, tx.Rollback()) }()
			_, err = tx.ExecContext(ctx, tc.ddl)
			require.NoError(t, err)
			tables, err := readOrganizationRLSCoverage(ctx, tx)
			require.NoError(t, err)
			var probes []organizationRLSCoverage
			for _, table := range tables {
				if table.TableName == "public.org_rls_probe" {
					probes = append(probes, table)
				}
			}
			if !tc.discovered {
				require.Empty(t, probes)
				return
			}
			require.Len(t, probes, 1)
			require.Equal(t, tc.protected, len(organizationRLSProblems(probes, nil)) == 0)
		})
	}
}

func TestTraegerRLSExceptionsRejectDrift(t *testing.T) {
	t.Parallel()
	exceptions := map[string]string{"public.old_table": "read-only shared data"}
	require.Contains(t, organizationRLSProblems(nil, exceptions), "public.old_table is excepted but is no Träger table the request role reaches")
	protected := organizationRLSCoverage{TableName: "public.old_table", Enabled: true, Forced: true, HasPolicy: true, RequestPolicy: true}
	require.Contains(t, organizationRLSProblems([]organizationRLSCoverage{protected}, exceptions), "public.old_table now satisfies RLS coverage; remove its exception")
	writable := organizationRLSCoverage{TableName: "public.old_table", RequestWrite: true}
	require.Contains(t, organizationRLSProblems([]organizationRLSCoverage{writable}, exceptions), "public.old_table is read-only shared data but phoenix_tenant can write it")
	require.Contains(t, organizationRLSProblems([]organizationRLSCoverage{writable}, map[string]string{"public.old_table": " "}), "public.old_table needs an exception reason")
}
