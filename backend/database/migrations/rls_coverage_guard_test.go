package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestTenantRLSCoverageDiscoversSchemaChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := t.Context()
	for _, tc := range []struct {
		name       string
		ddl        string
		discovered bool
		protected  bool
	}{
		{"new table", `CREATE TABLE public.rls_probe (tenant_id bigint)`, true, false},
		{"enable only", `CREATE TABLE public.rls_probe (tenant_id bigint); ALTER TABLE public.rls_probe ENABLE ROW LEVEL SECURITY`, true, false},
		{"no policy", `CREATE TABLE public.rls_probe (tenant_id bigint); ALTER TABLE public.rls_probe ENABLE ROW LEVEL SECURITY; ALTER TABLE public.rls_probe FORCE ROW LEVEL SECURITY`, true, false},
		{"wrong role", `CREATE TABLE public.rls_probe (tenant_id bigint); ALTER TABLE public.rls_probe ENABLE ROW LEVEL SECURITY; ALTER TABLE public.rls_probe FORCE ROW LEVEL SECURITY; CREATE POLICY probe ON public.rls_probe TO phoenix_auth USING (true)`, true, false},
		{"public policy", `CREATE TABLE public.rls_probe (tenant_id bigint); ALTER TABLE public.rls_probe ENABLE ROW LEVEL SECURITY; ALTER TABLE public.rls_probe FORCE ROW LEVEL SECURITY; CREATE POLICY probe ON public.rls_probe USING (true)`, true, true},
		{"request role policy", `CREATE TABLE public.rls_probe (tenant_id bigint); ALTER TABLE public.rls_probe ENABLE ROW LEVEL SECURITY; ALTER TABLE public.rls_probe FORCE ROW LEVEL SECURITY; CREATE POLICY probe ON public.rls_probe TO phoenix_tenant USING (true)`, true, true},
		{"partitioned parent", `CREATE TABLE public.rls_probe (tenant_id bigint) PARTITION BY RANGE (tenant_id); CREATE TABLE public.rls_probe_leaf PARTITION OF public.rls_probe DEFAULT`, true, false},
		{"dropped column", `CREATE TABLE public.rls_probe (tenant_id bigint, value text); ALTER TABLE public.rls_probe DROP COLUMN tenant_id`, false, false},
		{"no tenant column", `CREATE TABLE public.rls_probe (value text)`, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := db.Begin()
			require.NoError(t, err)
			defer func() { require.NoError(t, tx.Rollback()) }()
			_, err = tx.ExecContext(ctx, tc.ddl)
			require.NoError(t, err)
			tables, err := readTenantRLSCoverage(ctx, tx)
			require.NoError(t, err)
			var probes []tenantRLSCoverage
			for _, table := range tables {
				require.NotEqual(t, "public.rls_probe_leaf", table.TableName, "partition leaves are covered by the parent")
				if table.TableName == "public.rls_probe" {
					probes = append(probes, table)
				}
			}
			if !tc.discovered {
				require.Empty(t, probes)
				return
			}
			require.Len(t, probes, 1)
			require.Equal(t, tc.protected, len(tenantRLSProblems(probes, nil)) == 0)
		})
	}
}

func TestTenantRLSExceptionsRejectDrift(t *testing.T) {
	t.Parallel()
	exceptions := map[string]string{"public.old_table": "administrative only"}
	require.Contains(t, tenantRLSProblems(nil, exceptions), "public.old_table is excepted but has no tenant_id table")
	protected := tenantRLSCoverage{TableName: "public.old_table", Enabled: true, Forced: true, HasPolicy: true, RequestPolicy: true}
	require.Contains(t, tenantRLSProblems([]tenantRLSCoverage{protected}, exceptions), "public.old_table now satisfies RLS coverage; remove its exception")
	granted := tenantRLSCoverage{TableName: "public.old_table", RequestAccess: true}
	require.Contains(t, tenantRLSProblems([]tenantRLSCoverage{granted}, exceptions), "public.old_table is administrative but phoenix_tenant has table or column privileges")
	require.Contains(t, tenantRLSProblems([]tenantRLSCoverage{granted}, map[string]string{"public.old_table": " "}), "public.old_table needs an exception reason")
}
