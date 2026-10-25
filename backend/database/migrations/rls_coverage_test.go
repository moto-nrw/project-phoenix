package migrations

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type tenantRLSCoverage struct {
	TableName     string
	Enabled       bool
	Forced        bool
	HasPolicy     bool
	RequestPolicy bool
	RequestAccess bool
}

// Only documented administrative tables belong here, never tenant isolation
// gaps. Keep the budget in sync downwards; removed or repaired entries fail.
const tenantRLSExceptionBudget = 4

var tenantRLSExceptions = map[string]string{
	"audit.room_color_migration_backup":             "Administrative restore backup only; 1.15.225 revokes all tenant table/sequence privileges, guarded by TestAuditSchemaAppendOnlyForTenantRole.",
	"audit.wc_alias_migration_backup":               "Administrative restore backup only; 1.15.225 revokes all tenant table/sequence privileges, guarded by TestAuditSchemaAppendOnlyForTenantRole.",
	"calendar.appointment_reminder_push_deliveries": "No direct tenant access since 1.15.255; claim/release use SECURITY DEFINER functions with app.current_tenant_id predicates.",
	"platform.demo_school_states":                   "Demo control plane, not a school request table; 1.15.402 revokes tenant access, 1.15.403 forces RLS with a phoenix_demo-only policy.",
}

func readTenantRLSCoverage(ctx context.Context, db bun.IDB) ([]tenantRLSCoverage, error) {
	// This is a coverage guard, not a SQL theorem prover: matching words in
	// pg_policies.qual/with_check cannot prove isolation (OR true, function
	// bodies and permissive/restrictive policy composition change semantics).
	// Role applicability is catalog-checkable; predicate behavior stays in
	// the module RLS tests and TestAuthTenantRLSRejectsCrossSchoolAccess.
	var tables []tenantRLSCoverage
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
		       (has_table_privilege('phoenix_tenant', c.oid, 'SELECT,INSERT,UPDATE,DELETE,TRUNCATE,REFERENCES,TRIGGER,MAINTAIN')
		        OR has_any_column_privilege('phoenix_tenant', c.oid, 'SELECT,INSERT,UPDATE,REFERENCES')) AS request_access
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('r', 'p') AND NOT c.relispartition
		  AND n.nspname !~ '^pg_' AND n.nspname <> 'information_schema'
		  AND EXISTS (
		      SELECT FROM pg_attribute a WHERE a.attrelid = c.oid
		        AND a.attname = 'tenant_id' AND a.attnum > 0 AND NOT a.attisdropped
		  )
		ORDER BY 1`).Scan(ctx, &tables)
	return tables, err
}

func tenantRLSProblems(tables []tenantRLSCoverage, exceptions map[string]string) []string {
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
			if table.RequestAccess {
				problems = append(problems, table.TableName+" is administrative but phoenix_tenant has table or column privileges")
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
			problems = append(problems, table+" is excepted but has no tenant_id table")
		}
	}
	sort.Strings(problems)
	return problems
}

func TestEveryTenantTableHasRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tables, err := readTenantRLSCoverage(context.Background(), db)
	require.NoError(t, err)
	require.NotEmpty(t, tables, "tenant table discovery must not pass vacuously")
	require.Len(t, tenantRLSExceptions, tenantRLSExceptionBudget, "RLS exceptions may only shrink; lower the budget when removing one")
	require.Empty(t, tenantRLSProblems(tables, tenantRLSExceptions), "tenant RLS coverage")
}
