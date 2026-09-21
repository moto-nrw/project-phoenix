package behavior_test

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestAccountMetadataSelfEdits(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	profiles, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	account := testpkg.CreateTestAccount(t, db, "self-metadata")
	ctx := testpkg.Ctx(t)

	before, err := profiles.FindAccountMetadata(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.Email, before.Email)
	require.Nil(t, before.Username)
	require.Empty(t, before.Avatar)

	// Display edits cannot rewrite credentials or activation state from an
	// earlier read, even when those facts change after that read.
	_, err = db.NewRaw(`UPDATE auth.accounts SET active = FALSE, pin_attempts = pin_attempts + 1, is_password_otp = TRUE
		WHERE id = ?`, account.ID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, profiles.SetAccountUsername(ctx, account.ID, account.Email))
	require.NoError(t, profiles.SetAccountAvatar(ctx, account.ID, "/uploads/avatars/global/owner.jpg"))
	after, err := profiles.FindAccountMetadata(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, account.Email, *after.Username)
	require.Equal(t, "/uploads/avatars/global/owner.jpg", after.Avatar)
	require.False(t, after.Active)
	require.True(t, after.IsPasswordOTP)
	require.Equal(t, before.CreatedAt, after.CreatedAt)
	require.False(t, after.UpdatedAt.Before(before.UpdatedAt))
	var attempts int
	require.NoError(t, db.NewRaw(`SELECT pin_attempts FROM auth.accounts WHERE id = ?`, account.ID).Scan(ctx, &attempts))
	require.Equal(t, 1, attempts)

	rollback := errors.New("roll back self edits")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		require.NoError(t, profiles.SetAccountUsername(txCtx, account.ID, ""))
		require.NoError(t, profiles.SetAccountAvatar(txCtx, account.ID, ""))
		value, readErr := profiles.FindAccountMetadata(txCtx, account.ID)
		require.NoError(t, readErr)
		require.Nil(t, value.Username)
		require.Empty(t, value.Avatar)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	rolledBack, err := profiles.FindAccountMetadata(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, after, rolledBack)

	require.NoError(t, profiles.SetAccountUsername(ctx, account.ID, ""))
	require.NoError(t, profiles.SetAccountAvatar(ctx, account.ID, ""))
	cleared, err := profiles.FindAccountMetadata(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.Username)
	require.Empty(t, cleared.Avatar)

	_, err = profiles.FindAccountMetadata(ctx, 0)
	require.ErrorIs(t, err, identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, profiles.SetAccountUsername(ctx, 0, "missing"), identityaccess.ErrAccountNotFound)
	require.ErrorIs(t, profiles.SetAccountAvatar(ctx, 0, "missing"), identityaccess.ErrAccountNotFound)
}

func TestAccountMetadataPropagatesDatabaseFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupClosableTestDB(t)
	profiles, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	_, err = profiles.FindAccountMetadata(context.Background(), 0)
	require.ErrorContains(t, err, "database is closed")
	require.NotErrorIs(t, err, identityaccess.ErrAccountNotFound)
	require.ErrorContains(t, profiles.SetAccountUsername(context.Background(), 0, "failed"), "database is closed")
	require.ErrorContains(t, profiles.SetAccountAvatar(context.Background(), 0, "failed"), "database is closed")
}
