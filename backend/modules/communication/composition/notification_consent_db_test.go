package compose

import (
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationConsentStore pins the store that decides whether a
// notification may be delivered to a person at all.
func TestNotificationConsentStore(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	consent, err := NewNotificationConsent(NotificationConsentConfig{DB: db})
	require.NoError(t, err)
	ctx := testpkg.Ctx(t)

	suffix := time.Now().UnixNano()
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pref-%d@example.com", suffix))
	other := testpkg.CreateTestAccount(t, db, fmt.Sprintf("pref-other-%d@example.com", suffix))

	record := func(t *testing.T, accountID int64, notificationType string, enabled bool) {
		t.Helper()
		require.NoError(t, consent.RecordConsent(ctx, accountID, notificationType, enabled))
	}

	t.Run("record stores and then overwrites one decision", func(t *testing.T) {
		record(t, account.ID, "pickup_upcoming", true)

		stored, err := consent.StoredConsent(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, stored, 1)
		assert.True(t, stored["pickup_upcoming"])

		record(t, account.ID, "pickup_upcoming", false)

		stored, err = consent.StoredConsent(ctx, account.ID)
		require.NoError(t, err)
		require.Len(t, stored, 1, "the same type must not create a second row")

		// An explicit opt-out is kept as a row rather than deleted: it records a
		// decision, which a later change of defaults must not silently overrule.
		enabled, present := stored["pickup_upcoming"]
		require.True(t, present, "the decline stays stored")
		assert.False(t, enabled)
	})

	t.Run("filter returns only accounts that agreed", func(t *testing.T) {
		record(t, account.ID, "student_absence_reported", true)
		record(t, other.ID, "student_absence_reported", false)

		optedIn, err := consent.FilterOptedIn(ctx, "student_absence_reported",
			[]int64{account.ID, other.ID})
		require.NoError(t, err)

		assert.Equal(t, []int64{account.ID}, optedIn)
	})

	t.Run("an account with no row at all is not opted in", func(t *testing.T) {
		optedIn, err := consent.FilterOptedIn(ctx, "activity_start", []int64{account.ID, other.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn, "no row means no consent")
	})

	t.Run("tenant gate and multi-type filter use enabled rows only", func(t *testing.T) {
		record(t, account.ID, "my_activity_starting", true)
		record(t, other.ID, "activity_start", true)
		record(t, other.ID, "pickup_overdue", false)

		hasAny, err := consent.HasAnyOptedIn(ctx, []string{"my_activity_starting", "activity_start"})
		require.NoError(t, err)
		assert.True(t, hasAny)

		hasAny, err = consent.HasAnyOptedIn(ctx, []string{"pickup_overdue"})
		require.NoError(t, err)
		assert.False(t, hasAny, "an explicit opt-out must not open the scheduler gate")

		byType, err := consent.FilterOptedInByType(
			ctx,
			[]string{"my_activity_starting", "activity_start", "pickup_overdue"},
			[]int64{account.ID, other.ID},
		)
		require.NoError(t, err)
		assert.Equal(t, []int64{account.ID}, byType["my_activity_starting"])
		assert.Equal(t, []int64{other.ID}, byType["activity_start"])
		assert.NotContains(t, byType, "pickup_overdue")
	})

	// The mirror rule, for channels that already reached people before consent
	// existed: only a stored "no" filters, a missing row does not.
	t.Run("not-opted-out drops an explicit no and keeps a missing decision", func(t *testing.T) {
		// From the subtest above: account agreed to student_absence_reported,
		// other declined it. Neither has a row for activity_start.
		remaining, err := consent.FilterNotOptedOut(ctx, "student_absence_reported",
			[]int64{account.ID, other.ID})
		require.NoError(t, err)
		assert.Equal(t, []int64{account.ID}, remaining)

		remaining, err = consent.FilterNotOptedOut(ctx, "activity_start", []int64{account.ID, other.ID})
		require.NoError(t, err)
		assert.Equal(t, []int64{account.ID, other.ID}, remaining, "no row means no objection")

		remaining, err = consent.FilterNotOptedOut(ctx, "activity_start", nil)
		require.NoError(t, err)
		assert.Empty(t, remaining)
	})

	t.Run("empty input yields nothing rather than everything", func(t *testing.T) {
		// The dangerous default: a producer that resolved no candidates must end
		// up with no recipients, never with the whole school.
		optedIn, err := consent.FilterOptedIn(ctx, "student_absence_reported", nil)
		require.NoError(t, err)
		assert.Empty(t, optedIn)

		optedIn, err = consent.FilterOptedIn(ctx, "", []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn)
	})

	t.Run("disable switches only the named decisions off", func(t *testing.T) {
		record(t, account.ID, "pickup_overdue", true)
		record(t, account.ID, "activity_overdue", true)
		record(t, account.ID, "parent_announcement", true)

		require.NoError(t, consent.DisableConsent(ctx, account.ID,
			[]string{"pickup_overdue", "activity_overdue"}))

		stored, err := consent.StoredConsent(ctx, account.ID)
		require.NoError(t, err)
		require.NotEmpty(t, stored)
		assert.False(t, stored["pickup_overdue"])
		assert.False(t, stored["activity_overdue"])
		assert.True(t, stored["parent_announcement"],
			"a bulk action in one portal must preserve the other portal")
	})

	t.Run("does not leak another tenant's decisions", func(t *testing.T) {
		otherTenant := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, db, otherTenant)

		foreignCtx := testpkg.TenantContext(otherTenant)
		require.NoError(t, consent.RecordConsent(foreignCtx, account.ID, "pickup_upcoming", true))

		// The same account agreed at another school. Tenant 1 must neither see
		// that decision nor act on it.
		optedIn, err := consent.FilterOptedIn(ctx, "pickup_upcoming", []int64{account.ID})
		require.NoError(t, err)
		assert.Empty(t, optedIn, "consent is per school, not global")

		hasAny, err := consent.HasAnyOptedIn(ctx, []string{"pickup_upcoming"})
		require.NoError(t, err)
		assert.False(t, hasAny, "another tenant's consent must not open this tenant's gate")

		byType, err := consent.FilterOptedInByType(
			ctx,
			[]string{"pickup_upcoming"},
			[]int64{account.ID},
		)
		require.NoError(t, err)
		assert.Empty(t, byType, "another tenant's consent must not enter a bulk audience")

		stored, err := consent.StoredConsent(foreignCtx, account.ID)
		require.NoError(t, err)
		require.Len(t, stored, 1, "the other school keeps its own row")
	})
}
