package notifications_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRestatedSettingKeysMatchTheRegistry pins every registry key this package
// restates to the Settings Platform's name: a renamed key must fail here, not
// resolve as an unknown definition at runtime.
func TestRestatedSettingKeysMatchTheRegistry(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		"notifications.dispatch_enabled",
		"notifications.active_window_start",
		"notifications.active_window_end",
		"notifications.absence_reported_enabled",
		"notifications.on_duty_only",
		"reminders.pickup_upcoming_enabled",
		"reminders.pickup_overdue_enabled",
		"reminders.activity_start_enabled",
		"reminders.activity_overdue_enabled",
		"operations.parent_notes_enabled",
		"operations.staff_messaging_enabled",
		"calendar.appointment_reminder_enabled",
	}, notifications.RestatedSettingKeys)
	db := testpkg.SetupTestDB(t)
	module, err := api.NewSettingsTestModule(testpkg.NewDatabase(t, db), testpkg.TenantRuntime(t, db))
	require.NoError(t, err)
	for _, key := range notifications.RestatedSettingKeys {
		_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), db, module.Settings.Resolve, key)
		require.NoError(t, err, "Delivery must use a registered setting: %s", key)
	}
}
