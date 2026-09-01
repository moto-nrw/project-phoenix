package migrations

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestPhaseAudienceExistingStudentsUpRepairsMissingEligibilityColumns(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := testpkg.NewBunDB(sqlDB)
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
