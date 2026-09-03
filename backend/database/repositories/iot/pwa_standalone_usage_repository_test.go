package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type testPWAUsageRow struct {
	bun.BaseModel `bun:"schema:iot,table:pwa_standalone_usage"`
	TenantID      int64     `bun:"tenant_id"`
	AccountID     int64     `bun:"account_id"`
	Portal        string    `bun:"portal"`
	FirstSeenAt   time.Time `bun:"first_seen_at"`
	LastSeenAt    time.Time `bun:"last_seen_at"`
}

func fetchPWAUsageRows(t *testing.T, db *bun.DB, accountID int64) []*testPWAUsageRow {
	t.Helper()
	var rows []*testPWAUsageRow
	err := db.NewSelect().Model(&rows).
		ModelTableExpr(`iot.pwa_standalone_usage AS "test_pwa_usage_row"`).
		Where("account_id = ?", accountID).
		Order("tenant_id ASC", "portal ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func TestPWAStandaloneUsageRepository(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	repo := iotRepo.NewPWAStandaloneUsageRepository(db)
	tenantID := testpkg.Tenant(t)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-usage-%d@example.com", time.Now().UnixNano()))
	testpkg.EnsureAccountTenant(t, db, account.ID, tenantID)
	recordSeen := func(tenantID int64, portal string) error {
		return testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			return repo.RecordSeen(txCtx, tenantID, account.ID, portal)
		})
	}

	t.Run("record seen is idempotent per tenant account and portal", func(t *testing.T) {
		require.NoError(t, recordSeen(tenantID, "staff"))
		rows := fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1)
		firstSeen := rows[0].FirstSeenAt

		_, err := db.ExecContext(context.Background(),
			`UPDATE iot.pwa_standalone_usage SET last_seen_at = NOW() - INTERVAL '1 hour' WHERE account_id = ?`,
			account.ID)
		require.NoError(t, err)
		require.NoError(t, recordSeen(tenantID, "staff"))

		rows = fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1)
		var databaseTime time.Time
		require.NoError(t, db.NewRaw("SELECT CURRENT_TIMESTAMP").Scan(context.Background(), &databaseTime))
		assert.WithinDuration(t, databaseTime, rows[0].LastSeenAt, time.Minute)
		assert.WithinDuration(t, firstSeen, rows[0].FirstSeenAt, time.Second)
	})

	t.Run("portals are separate rows", func(t *testing.T) {
		require.NoError(t, recordSeen(tenantID, "parent"))
		assert.Len(t, fetchPWAUsageRows(t, db, account.ID), 2)
	})

	t.Run("tenant role cannot bypass the write capability", func(t *testing.T) {
		err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			_, updateErr := tx.ExecContext(txCtx,
				`UPDATE iot.pwa_standalone_usage SET last_seen_at = NOW() WHERE account_id = ?`, account.ID)
			return updateErr
		})
		require.Error(t, err)
	})

	t.Run("delete is tenant scoped", func(t *testing.T) {
		otherTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureAccountTenant(t, db, account.ID, otherTenantID)
		err := testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			return repo.RecordSeen(txCtx, otherTenantID, account.ID, "staff")
		})
		require.Error(t, err, "the database capability must not bypass tenant RLS")
		require.NoError(t, recordSeen(otherTenantID, "staff"))
		_, err = db.ExecContext(context.Background(),
			`UPDATE iot.pwa_standalone_usage SET last_seen_at = NOW() - INTERVAL '40 days' WHERE account_id = ?`,
			account.ID)
		require.NoError(t, err)

		var deleted int
		err = testpkg.WithTenantTx(t, context.Background(), db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			var deleteErr error
			deleted, deleteErr = repo.DeleteLastSeenBefore(txCtx, tenantID, time.Now().AddDate(0, 0, -30))
			return deleteErr
		})
		require.NoError(t, err)
		assert.Equal(t, 2, deleted)

		rows := fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1)
		assert.Equal(t, otherTenantID, rows[0].TenantID)
	})
}
