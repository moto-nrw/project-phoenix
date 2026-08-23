package migrations

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestDailyCheckoutAllRoomsExistingTenantsBackfill(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})

	mock.ExpectExec(`(?s)INSERT INTO config\.setting_values .*SELECT id, 'checkout\.daily_checkout_from_all_rooms_enabled', 'false'::jsonb.*FROM platform\.schools.*ON CONFLICT \(tenant_id, setting_key\) DO NOTHING`).
		WillReturnResult(sqlmock.NewResult(0, 4))

	require.NoError(t, dailyCheckoutAllRoomsExistingTenantsUp(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}
