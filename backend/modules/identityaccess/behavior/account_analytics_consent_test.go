package behavior_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The usage analytics consent (#4901) belongs to the account: it starts
// undecided, records every answer with its time, and holds in every school.
func TestAccountAnalyticsConsent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	consent := setupAuthFactory(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "analytics-consent")

	answer, err := consent.FindAnalyticsConsent(ctx, account.ID)
	require.NoError(t, err)
	require.Nil(t, answer.Granted, "a new account has not decided")
	require.Nil(t, answer.DecidedAt)

	stored, err := consent.DecideAnalyticsConsent(ctx, account.ID, true)
	require.NoError(t, err)
	answer, err = consent.FindAnalyticsConsent(ctx, account.ID)
	require.NoError(t, err)
	require.Equal(t, answer, stored, "the decision returns the answer as stored")
	require.NotNil(t, answer.Granted)
	require.True(t, *answer.Granted)
	require.NotNil(t, answer.DecidedAt)
	granted := *answer.DecidedAt

	_, err = consent.DecideAnalyticsConsent(ctx, account.ID, false)
	require.NoError(t, err)
	answer, err = consent.FindAnalyticsConsent(ctx, account.ID)
	require.NoError(t, err)
	require.NotNil(t, answer.Granted)
	require.False(t, *answer.Granted, "a revocation replaces the consent")
	require.False(t, answer.DecidedAt.Before(granted), "the latest answer carries its own time")

	t.Run("the answer holds in every school", func(t *testing.T) {
		other, readErr := consent.FindAnalyticsConsent(testpkg.OwnCtx(t), account.ID)
		require.NoError(t, readErr)
		require.NotNil(t, other.Granted)
		require.False(t, *other.Granted)
	})

	t.Run("an erased account has no consent to read or record", func(t *testing.T) {
		erased := testpkg.CreateTestAccount(t, db, "analytics-consent-erased")
		_, execErr := db.NewRaw(`UPDATE auth.accounts SET personal_data_erased_at = NOW() WHERE id = ?`, erased.ID).Exec(ctx)
		require.NoError(t, execErr)

		_, readErr := consent.FindAnalyticsConsent(ctx, erased.ID)
		require.ErrorIs(t, readErr, identityaccess.ErrAccountNotFound)
		_, decideErr := consent.DecideAnalyticsConsent(ctx, erased.ID, true)
		require.ErrorIs(t, decideErr, identityaccess.ErrAccountNotFound)
	})
}
