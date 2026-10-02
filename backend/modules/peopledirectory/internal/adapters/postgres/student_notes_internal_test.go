package postgres

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestStudentNoteStoreSoftDeleteReturnsAffectedRowsError(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	db := testpkg.NewBunDB(sqlDB)
	store := NewStudentNoteStore(func(context.Context) (bun.IDB, int64, error) {
		return db, 42, nil
	})
	rowsAffectedErr := errors.New("affected rows unavailable")
	mock.ExpectExec("UPDATE.*student_notes").
		WillReturnResult(sqlmock.NewErrorResult(rowsAffectedErr))

	_, err = store.SoftDelete(context.Background(), domain.DeleteStudentNote{
		ID: 7, ActorAccountID: 8,
	})
	require.ErrorIs(t, err, rowsAffectedErr)
}
