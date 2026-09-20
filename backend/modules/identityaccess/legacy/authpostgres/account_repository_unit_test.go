package authpostgres_test

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	authrepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestAccountRepository_FindAvatarsByAccountIDs_EmptyIDs(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	// sqlmock without ExpectClose complains on Close; the close error is noise.
	defer func() { _ = sqlDB.Close() }()

	db := bun.NewDB(sqlDB, pgdialect.New())

	repo := authrepo.NewAccountRepository(db)

	result, err := repo.FindAvatarsByAccountIDs(context.Background(), nil)
	require.NoError(t, err)
	assert.Empty(t, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepository_FindAvatarsByAccountIDs_Success(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	// sqlmock without ExpectClose complains on Close; the close error is noise.
	defer func() { _ = sqlDB.Close() }()

	db := bun.NewDB(sqlDB, pgdialect.New())

	repo := authrepo.NewAccountRepository(db)

	rows := sqlmock.NewRows([]string{"id", "avatar"}).
		AddRow(1, "/uploads/avatars/global/one.jpg").
		AddRow(2, "/uploads/avatars/global/two.jpg")
	mock.ExpectQuery(`SELECT "id", "avatar" FROM auth\.accounts`).
		WillReturnRows(rows)

	result, err := repo.FindAvatarsByAccountIDs(context.Background(), []int64{1, 2})
	require.NoError(t, err)
	assert.Equal(t, map[int64]string{
		1: "/uploads/avatars/global/one.jpg",
		2: "/uploads/avatars/global/two.jpg",
	}, result)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountRepository_FindAvatarsByAccountIDs_ReturnsDatabaseError(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	// sqlmock without ExpectClose complains on Close; the close error is noise.
	defer func() { _ = sqlDB.Close() }()

	db := bun.NewDB(sqlDB, pgdialect.New())

	repo := authrepo.NewAccountRepository(db)
	mock.ExpectQuery(`SELECT "id", "avatar" FROM auth\.accounts`).
		WillReturnError(errors.New("select failed"))

	result, err := repo.FindAvatarsByAccountIDs(context.Background(), []int64{1})
	require.Nil(t, result)
	require.Error(t, err)

	var dbErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &dbErr)
	assert.Equal(t, "find avatars by account IDs", dbErr.Op)
	assert.EqualError(t, dbErr.Err, "select failed")
	require.NoError(t, mock.ExpectationsWereMet())
}
