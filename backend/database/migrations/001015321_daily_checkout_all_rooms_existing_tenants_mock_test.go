package migrations

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func newDailyCheckoutMigrationMockDB(t *testing.T) (*testpkg.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := testpkg.NewBunDB(sqlDB)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	return db, mock
}

func TestDailyCheckoutAllRoomsExistingTenantsBackfill(t *testing.T) {
	t.Parallel()

	db, mock := newDailyCheckoutMigrationMockDB(t)

	mock.ExpectExec(`(?s)WITH inserted AS \(.*INSERT INTO config\.setting_values .*SELECT id, 'checkout\.daily_checkout_from_all_rooms_enabled', 'false'::jsonb.*FROM platform\.schools.*ON CONFLICT \(tenant_id, setting_key\) DO NOTHING.*RETURNING tenant_id, setting_key, value.*\).*INSERT INTO config\.setting_audit .*SELECT tenant_id, setting_key, NULL, value, 'set', NULL.*FROM inserted`).
		WillReturnResult(sqlmock.NewResult(0, 4))

	require.NoError(t, dailyCheckoutAllRoomsExistingTenantsUp(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDailyCheckoutAllRoomsExistingTenantsRollbackPreservesValues(t *testing.T) {
	t.Parallel()

	db, mock := newDailyCheckoutMigrationMockDB(t)

	require.NoError(t, dailyCheckoutAllRoomsExistingTenantsDown(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}
