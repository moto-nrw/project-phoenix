package users_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationPreferenceRepository pins the store that decides whether a
// notification may be delivered to a person at all.
func TestNotificationPreferenceRepository(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := repositories.NewFactory(db).NotificationPreference
	ctx := testpkg.TenantContext(1)

	suffix := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pref-%d@example.com", suffix))
	other := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pref-other-%d@example.com", suffix))
	defer testpkg.CleanupAuthFixtures(t, db, account.ID, other.ID)

	upsert := func(t *testing.T, accountID int64, notificationType string, enabled bool) {
		t.Helper()
		pref := &userModels.NotificationPreference{
			AccountID:        accountID,
			NotificationType: notificationType,
			Enabled:          enabled,
		}
		require.NoError(t, repo.Upsert(ctx, pref))
	}

	t.Run("upsert stores and then overwrites one decision", func(t *testing.T) {
		upsert(t, account.ID, "pickup_upcoming", true)

		prefs, err := repo.ListByAccount(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, prefs, 1)
		assert.Equal(t, "pickup_upcoming", prefs[0].NotificationType)
		assert.True(t, prefs[0].Enabled)

		upsert(t, account.ID, "pickup_upcoming", false)

		prefs, err = repo.ListByAccount(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, prefs, 1, "the same type must not create a second row")

		// An explicit opt-out is kept as a row rather than deleted: it records a
		// decision, which a later change of defaults must not silently overrule.
		assert.False(t, prefs[0].Enabled)
	})

	t.Run("filter returns only accounts that agreed", func(t *testing.T) {
		upsert(t, account.ID, "student_absence_reported", true)
		upsert(t, other.ID, "student_absence_reported", false)

		optedIn, err := repo.FilterOptedIn(ctx, "student_absence_reported",
			[]int64{account.ID, other.ID})
		require.NoError(t, err)

		assert.Equal(t, []int64{account.ID}, optedIn)
	})

	t.Run("an account with no row at all is not opted in", func(t *testing.T) {
		optedIn, err := repo.FilterOptedIn(ctx, "activity_start", []int64{account.ID, other.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn, "no row means no consent")
	})

	t.Run("empty input yields nothing rather than everything", func(t *testing.T) {
		// The dangerous default: a producer that resolved no candidates must end
		// up with no recipients, never with the whole school.
		optedIn, err := repo.FilterOptedIn(ctx, "student_absence_reported", nil)
		require.NoError(t, err)
		assert.Empty(t, optedIn)

		optedIn, err = repo.FilterOptedIn(ctx, "", []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn)
	})

	t.Run("disable all switches every decision off", func(t *testing.T) {
		upsert(t, account.ID, "pickup_overdue", true)
		upsert(t, account.ID, "activity_overdue", true)

		require.NoError(t, repo.DisableAllForAccount(ctx, account.ID))

		prefs, err := repo.ListByAccount(ctx, account.ID)
		require.NoError(t, err)
		require.NotEmpty(t, prefs)
		for _, pref := range prefs {
			assert.False(t, pref.Enabled, "type %s", pref.NotificationType)
		}
	})

	t.Run("does not leak another tenant's decisions", func(t *testing.T) {
		const otherTenant int64 = 99047
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignCtx := testpkg.TenantContext(otherTenant)
		foreign := &userModels.NotificationPreference{
			AccountID:        account.ID,
			NotificationType: "pickup_upcoming",
			Enabled:          true,
		}
		require.NoError(t, repo.Upsert(foreignCtx, foreign))

		// The same account agreed at another school. Tenant 1 must neither see
		// that decision nor act on it.
		optedIn, err := repo.FilterOptedIn(ctx, "pickup_upcoming", []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn, "consent is per school, not global")

		prefs, err := repo.ListByAccount(foreignCtx, account.ID)
		require.NoError(t, err)
		require.Len(t, prefs, 1, "the other school keeps its own row")
	})
}
