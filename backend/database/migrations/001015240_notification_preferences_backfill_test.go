package migrations

import (
	"context"
	"testing"

	iotModels "github.com/moto-nrw/project-phoenix/models/iot"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The backfill exists because two changes land together: consent becomes
// mandatory for every producer, and the school-wide switch defaults to on.
// Without it, every phone registered today would go silent in the same minute.
//
// It therefore has to give a registered device exactly the consent it used to
// act on — and nothing beyond that, because inventing consent is the failure
// this whole epic is built to avoid.
func TestNotificationPreferencesBackfill(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	parentAccount := testpkg.CreateTestAccount(t, db, "backfill-parent@example.com")
	staffAccount := testpkg.CreateTestAccount(t, db, "backfill-staff@example.com")
	silentAccount := testpkg.CreateTestAccount(t, db, "backfill-silent@example.com")
	optedOutAccount := testpkg.CreateTestAccount(t, db, "backfill-optout@example.com")

	insertSubscription := func(accountID int64, portal, endpoint string) {
		t.Helper()
		sub := &iotModels.PushSubscription{
			AccountID: accountID,
			Portal:    portal,
			Endpoint:  endpoint,
			P256dh:    "test-p256dh",
			Auth:      "test-auth",
		}
		sub.SetTenantID(1)
		_, err := db.NewInsert().Model(sub).ModelTableExpr("iot.push_subscriptions").Exec(ctx)
		require.NoError(t, err)
	}

	insertSubscription(parentAccount.ID, iotModels.PushPortalParent, "https://fcm.googleapis.com/backfill-parent")
	// Two devices of one person: the backfill must still produce one row per
	// type, not one per device.
	insertSubscription(staffAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-staff-1")
	insertSubscription(staffAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-staff-2")
	insertSubscription(optedOutAccount.ID, iotModels.PushPortalStaff, "https://fcm.googleapis.com/backfill-optout")

	// Somebody who already said no. A backfill must never overwrite a decision.
	_, err := db.ExecContext(ctx, `
		INSERT INTO users.notification_preferences (tenant_id, account_id, notification_type, enabled)
		VALUES (1, ?, 'pickup_upcoming', FALSE)`, optedOutAccount.ID)
	require.NoError(t, err)

	accountIDs := []int64{parentAccount.ID, staffAccount.ID, silentAccount.ID, optedOutAccount.ID}
	t.Cleanup(func() {
		_, cerr := db.NewDelete().
			Table("users.notification_preferences").
			Where("account_id IN (?)", bun.List(accountIDs)).
			Exec(context.Background())
		require.NoError(t, cerr)
		_, cerr = db.NewDelete().
			Table("iot.push_subscriptions").
			Where("account_id IN (?)", bun.List(accountIDs)).
			Exec(context.Background())
		require.NoError(t, cerr)
		testpkg.CleanupAuthFixtures(t, db, accountIDs...)
	})

	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))

	consentOf := func(accountID int64) map[string]bool {
		t.Helper()
		var rows []struct {
			NotificationType string `bun:"notification_type"`
			Enabled          bool   `bun:"enabled"`
		}
		require.NoError(t, db.NewSelect().
			TableExpr("users.notification_preferences").
			ColumnExpr("notification_type, enabled").
			Where("account_id = ?", accountID).
			Scan(ctx, &rows))
		out := make(map[string]bool, len(rows))
		for _, row := range rows {
			out[row.NotificationType] = row.Enabled
		}
		return out
	}

	assert.Equal(t, map[string]bool{"parent_announcement": true}, consentOf(parentAccount.ID),
		"a guardian device delivered exactly one kind of notification")

	assert.Equal(t, map[string]bool{
		"pickup_upcoming":  true,
		"pickup_overdue":   true,
		"activity_start":   true,
		"activity_overdue": true,
	}, consentOf(staffAccount.ID),
		"a staff device gets the four reminder kinds it could already receive, and nothing this epic invented")

	assert.Empty(t, consentOf(silentAccount.ID),
		"somebody who never registered a device is not opted into anything")

	optedOut := consentOf(optedOutAccount.ID)
	assert.False(t, optedOut["pickup_upcoming"], "an explicit no must survive the backfill")
	assert.True(t, optedOut["pickup_overdue"], "the untouched kinds are still backfilled")

	// Re-running must be a no-op rather than a duplicate-key failure: migrations
	// are re-applied on restored databases often enough that this matters.
	require.NoError(t, notificationPreferencesBackfillUp(ctx, db))
	assert.False(t, consentOf(optedOutAccount.ID)["pickup_upcoming"])
}
