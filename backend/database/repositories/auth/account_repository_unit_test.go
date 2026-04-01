package auth_test

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	authrepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

func TestAccountRepository_UpdateAvatar_ReturnsDatabaseError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() {
		require.NoError(t, sqlDB.Close())
	}()

	db := bun.NewDB(sqlDB, pgdialect.New())
	defer func() { _ = db.Close() }()

	repo := authrepo.NewAccountRepository(db)
	mock.ExpectExec(`UPDATE auth\.accounts SET avatar = .* WHERE \(id = .*\)`).
		WillReturnError(errors.New("update failed"))

	err = repo.UpdateAvatar(context.Background(), 42, "/uploads/avatars/global/fail.jpg")
	require.Error(t, err)

	var dbErr *modelBase.DatabaseError
	require.ErrorAs(t, err, &dbErr)
	assert.Equal(t, "update avatar", dbErr.Op)
	assert.EqualError(t, dbErr.Err, "update failed")
	require.NoError(t, mock.ExpectationsWereMet())
}
