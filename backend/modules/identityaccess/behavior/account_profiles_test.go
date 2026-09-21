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

func TestAccountProfiles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	profiles, err := repositories.NewIdentityAccessForTests(db)
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "account-profile")

	_, _, found, err := profiles.FindAccountProfile(ctx, account.ID)
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, profiles.SetAccountBio(ctx, account.ID, "First bio"))
	bio, settings, found, err := profiles.FindAccountProfile(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "First bio", bio)
	require.JSONEq(t, `{}`, settings)

	// A bio change must not overwrite independently stored settings or avatar.
	_, err = db.NewRaw(`UPDATE users.profiles SET settings = '{"theme":"dark"}'::jsonb, avatar = 'profile.png'
		WHERE account_id = ? AND tenant_id = ?`, account.ID, testpkg.Tenant(t)).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, profiles.SetAccountBio(ctx, account.ID, ""))
	bio, settings, found, err = profiles.FindAccountProfile(ctx, account.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.Empty(t, bio)
	require.JSONEq(t, `{"theme":"dark"}`, settings)
	var avatar string
	require.NoError(t, db.NewRaw(`SELECT avatar FROM users.profiles WHERE account_id = ? AND tenant_id = ?`,
		account.ID, testpkg.Tenant(t)).Scan(ctx, &avatar))
	require.Equal(t, "profile.png", avatar)

	// The owner joins the caller's transaction; both reads and writes see it.
	rollback := errors.New("roll back profile change")
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		if writeErr := profiles.SetAccountBio(txCtx, account.ID, "Uncommitted"); writeErr != nil {
			return writeErr
		}
		value, _, exists, readErr := profiles.FindAccountProfile(txCtx, account.ID)
		require.NoError(t, readErr)
		require.True(t, exists)
		require.Equal(t, "Uncommitted", value)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
	bio, _, _, err = profiles.FindAccountProfile(ctx, account.ID)
	require.NoError(t, err)
	require.Empty(t, bio)

	t.Run("same account in another school is isolated", func(t *testing.T) {
		other := testpkg.OwnCtx(t)
		_, _, exists, readErr := profiles.FindAccountProfile(other, account.ID)
		require.NoError(t, readErr)
		require.False(t, exists)
		require.NoError(t, profiles.SetAccountBio(other, account.ID, "Other school"))
		value, _, exists, readErr := profiles.FindAccountProfile(other, account.ID)
		require.NoError(t, readErr)
		require.True(t, exists)
		require.Equal(t, "Other school", value)
		original, originalSettings, _, readErr := profiles.FindAccountProfile(ctx, account.ID)
		require.NoError(t, readErr)
		require.Empty(t, original)
		require.JSONEq(t, `{"theme":"dark"}`, originalSettings)
	})

	_, _, _, err = profiles.FindAccountProfile(context.Background(), account.ID)
	require.ErrorIs(t, err, identityaccess.ErrTenantRequired)
	require.ErrorIs(t, profiles.SetAccountBio(context.Background(), account.ID, "No school"), identityaccess.ErrTenantRequired)
	for _, invalidID := range []int64{0, -1} {
		require.Error(t, profiles.SetAccountBio(ctx, invalidID, "Invalid"))
	}
}
