package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func seedTransferIntent(t *testing.T, db *testpkg.DB, school int64, state string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO audit.export_transfers
 (tenant_id,actor_name,export_kind,export_format,filename,byte_size,target_host,target_port,target_directory,status,failure_reason,dispatch_state,dispatch_expires_at)
 VALUES (?,'test actor','staff_time_tracking','csv','export.csv',4,'counterpart.example',22,'/exports','failed','internal_error',?,clock_timestamp()+INTERVAL '2 minutes') RETURNING id`, school, state).Scan(t.Context(), &id))
	return id
}

func TestTransferDispatchMigrationPreservesEvidenceAndRefusesUnsettledRollback(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	school := testpkg.Tenant(t)
	id := seedTransferIntent(t, db, school, "terminal")
	require.NoError(t, exportTransferDispatchDown(ctx, db))
	require.NoError(t, exportTransferDispatchUp(ctx, db))
	var state string
	require.NoError(t, db.NewRaw(`SELECT dispatch_state FROM audit.export_transfers WHERE id=?`, id).Scan(ctx, &state))
	assert.Equal(t, "terminal", state, "historical rows must never become upload intents")
	pending := seedTransferIntent(t, db, school, "pending")
	require.ErrorContains(t, exportTransferDispatchDown(ctx, db), "settling")
	_, err := db.NewRaw(`UPDATE audit.export_transfers SET dispatch_state='unknown' WHERE id=?`, pending).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, exportTransferDispatchDown(ctx, db))
	var count int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM audit.export_transfers WHERE tenant_id=?`, school).Scan(ctx, &count))
	assert.Equal(t, 2, count, "rollback must retain audit evidence, including unknown attempts")
	require.NoError(t, exportTransferDispatchUp(ctx, db))
}

func TestTransferDispatchFunctionsAreTenantScopedOneShotAndKeepAuditImmutable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	auth := testpkg.SetupServeTestDB(t)
	t.Cleanup(func() { require.NoError(t, auth.Close()) })
	school := testpkg.Tenant(t)
	other, _ := testpkg.CreateTestTenant(t, db)
	id := seedTransferIntent(t, db, school, "pending")
	foreign := seedTransferIntent(t, db, other, "pending")
	var token string
	err := withPhoenixTenantTx(t, auth, school, func(ctx context.Context, tx testpkg.Tx) error {
		return tx.NewRaw(`SELECT audit.claim_export_transfer(?,?)`, other, foreign).Scan(ctx, &token)
	})
	require.ErrorContains(t, err, "42501")
	require.NoError(t, withPhoenixTenantTx(t, auth, school, func(ctx context.Context, tx testpkg.Tx) error {
		return tx.NewRaw(`SELECT audit.claim_export_transfer(?,?)`, school, foreign).Scan(ctx, &token)
	}))
	assert.Empty(t, token)
	require.NoError(t, withPhoenixTenantTx(t, auth, school, func(ctx context.Context, tx testpkg.Tx) error {
		return tx.NewRaw(`SELECT audit.claim_export_transfer(?,?)`, school, id).Scan(ctx, &token)
	}))
	require.NotEmpty(t, token)
	for _, claim := range []struct {
		token string
		want  bool
	}{{"not-the-fence", false}, {token, true}, {token, false}} {
		var done bool
		require.NoError(t, withPhoenixTenantTx(t, auth, school, func(ctx context.Context, tx testpkg.Tx) error {
			return tx.NewRaw(`SELECT audit.complete_export_transfer(?,?,?,true,'')`, school, id, claim.token).Scan(ctx, &done)
		}))
		assert.Equal(t, claim.want, done, "only the live exact fence can terminalize once")
	}
	var row struct {
		Status     string
		Filename   string
		ActorName  string
		TargetHost string
	}
	require.NoError(t, db.NewRaw(`SELECT status,filename,actor_name,target_host FROM audit.export_transfers WHERE id=?`, id).Scan(t.Context(), &row))
	assert.Equal(t, "success", row.Status)
	assert.Equal(t, "export.csv", row.Filename)
	assert.Equal(t, "test actor", row.ActorName)
	assert.Equal(t, "counterpart.example", row.TargetHost)
	var canUpdate bool
	require.NoError(t, db.NewRaw(`SELECT has_table_privilege('phoenix_tenant','audit.export_transfers','UPDATE')`).Scan(t.Context(), &canUpdate))
	assert.False(t, canUpdate, "settlement must not grant arbitrary audit rewrites")
}
