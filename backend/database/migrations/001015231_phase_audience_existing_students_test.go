package migrations

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestPhaseAudienceExistingStudentsUpRepairsMissingEligibilityColumns(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectExec("ADD COLUMN IF NOT EXISTS audience").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("CHECK \\(audience IN \\('open', 'new_students', 'existing_students', 'linked_parents'\\)\\)").
		WillReturnResult(sqlmock.NewResult(0, 0))

	require.NoError(t, phaseAudienceExistingStudentsUp(context.Background(), db))
	require.NoError(t, mock.ExpectationsWereMet())
}
