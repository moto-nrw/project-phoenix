package iot_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	iotRepo "github.com/moto-nrw/project-phoenix/database/repositories/iot"
	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func cleanupPWAUsage(t *testing.T, db *bun.DB, accountIDs ...int64) {
	t.Helper()
	_, err := db.NewDelete().
		Model((*iotModels.PWAStandaloneUsage)(nil)).
		ModelTableExpr("iot.pwa_standalone_usage").
		Where("account_id IN (?)", bun.List(accountIDs)).
		Exec(context.Background())
	require.NoError(t, err)
}

func fetchPWAUsageRows(t *testing.T, db *bun.DB, accountID int64) []*iotModels.PWAStandaloneUsage {
	t.Helper()
	var rows []*iotModels.PWAStandaloneUsage
	err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(`iot.pwa_standalone_usage AS "pwa_standalone_usage"`).
		Where("account_id = ?", accountID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func TestPWAStandaloneUsageRepository(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repo := iotRepo.NewPWAStandaloneUsageRepository(db)
	ctx := tenant.WithTenantID(context.Background(), 1)

	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pwa-usage-%d@example.com", time.Now().UnixNano()))
	defer testpkg.CleanupAuthFixtures(t, db, account.ID)
	defer cleanupPWAUsage(t, db, account.ID)
	createAccountTenantMapping(t, db, account.ID, 1)

	newUsage := func(portal string) *iotModels.PWAStandaloneUsage {
		usage := &iotModels.PWAStandaloneUsage{AccountID: account.ID, Portal: portal}
		usage.SetTenantID(1)
		return usage
	}

	t.Run("record seen is idempotent per (tenant, account, portal)", func(t *testing.T) {
		require.NoError(t, repo.RecordSeen(ctx, newUsage(iotModels.PushPortalStaff)))

		rows := fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1)
		firstSeen := rows[0].FirstSeenAt

		// Backdate, then report again: the row must stay unique and only
		// last_seen_at may advance.
		_, err := db.ExecContext(context.Background(),
			`UPDATE iot.pwa_standalone_usage SET last_seen_at = NOW() - INTERVAL '1 hour' WHERE account_id = ?`,
			account.ID)
		require.NoError(t, err)

		require.NoError(t, repo.RecordSeen(ctx, newUsage(iotModels.PushPortalStaff)))

		rows = fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1, "repeated reports must not duplicate the row")
		assert.WithinDuration(t, time.Now(), rows[0].LastSeenAt, time.Minute)
		assert.WithinDuration(t, firstSeen, rows[0].FirstSeenAt, time.Second, "first_seen_at must not move on refresh")
	})

	t.Run("portals are separate rows", func(t *testing.T) {
		require.NoError(t, repo.RecordSeen(ctx, newUsage(iotModels.PushPortalParent)))
		rows := fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 2)
	})

	t.Run("tenant filter hides rows from other tenants", func(t *testing.T) {
		otherCtx := tenant.WithTenantID(context.Background(), 2)
		rows, err := repo.List(otherCtx, map[string]any{"account_id": account.ID})
		require.NoError(t, err)
		assert.Empty(t, rows)
	})

	t.Run("delete older than removes only stale rows of the tenant", func(t *testing.T) {
		// Age the staff row past the cutoff; the parent row stays fresh.
		_, err := db.ExecContext(context.Background(),
			`UPDATE iot.pwa_standalone_usage SET last_seen_at = NOW() - INTERVAL '40 days'
			 WHERE account_id = ? AND portal = ?`,
			account.ID, iotModels.PushPortalStaff)
		require.NoError(t, err)

		deleted, err := repo.DeleteLastSeenBefore(ctx, time.Now().AddDate(0, 0, -30))
		require.NoError(t, err)
		assert.GreaterOrEqual(t, deleted, 1)

		rows := fetchPWAUsageRows(t, db, account.ID)
		require.Len(t, rows, 1)
		assert.Equal(t, iotModels.PushPortalParent, rows[0].Portal)
	})
}
