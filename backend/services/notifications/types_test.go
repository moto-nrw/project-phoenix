package notifications_test

import (
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNotificationTypeKeysMatchReminders pins the four reminder keys against
// their source of truth.
//
// The strings are declared twice on purpose — services/notifications must not
// depend on services/reminders — but they are one wire contract: the frontend
// switches on Reminder.Type and stores consent under the same string. A silent
// rename on either side would leave people opted into a type that no longer
// exists, which looks exactly like a working opt-in.
func TestNotificationTypeKeysMatchReminders(t *testing.T) {
	assert.Equal(t, reminders.TypePickupUpcoming, notifications.TypePickupUpcoming)
	assert.Equal(t, reminders.TypePickupOverdue, notifications.TypePickupOverdue)
	assert.Equal(t, reminders.TypeActivityStart, notifications.TypeActivityStart)
	assert.Equal(t, reminders.TypeActivityOverdue, notifications.TypeActivityOverdue)
}

func TestNotificationTypeCatalogue(t *testing.T) {
	t.Run("staff catalogue is grouped and ordered", func(t *testing.T) {
		defs := notifications.TypesForPortal(notifications.PortalStaff)
		require.Len(t, defs, 6)

		keys := make([]string, len(defs))
		for i, def := range defs {
			keys[i] = def.Key
		}
		assert.Equal(t, []string{
			notifications.TypePickupUpcoming,
			notifications.TypePickupOverdue,
			notifications.TypeActivityStart,
			notifications.TypeActivityOverdue,
			notifications.TypeMyActivityStarting,
			notifications.TypeStudentAbsenceReported,
		}, keys, "order is fixed by group then SortOrder, not by registration order")
	})

	t.Run("parent catalogue is separate", func(t *testing.T) {
		defs := notifications.TypesForPortal(notifications.PortalParent)
		require.Len(t, defs, 1)
		assert.Equal(t, notifications.TypeParentAnnouncement, defs[0].Key)

		// A staff type must never surface in the parent portal and vice versa:
		// the catalogue is what the preference page renders, so a leak here
		// would offer someone consent to a notification they cannot receive.
		for _, def := range notifications.TypesForPortal(notifications.PortalStaff) {
			assert.NotEqual(t, notifications.TypeParentAnnouncement, def.Key)
		}
	})

	t.Run("every definition carries German copy", func(t *testing.T) {
		for _, portal := range []string{notifications.PortalStaff, notifications.PortalParent} {
			for _, def := range notifications.TypesForPortal(portal) {
				assert.NotEmpty(t, def.Label, "label for %s", def.Key)
				assert.NotEmpty(t, def.Description, "description for %s", def.Key)
			}
		}
	})

	t.Run("tenant gates point at registered settings", func(t *testing.T) {
		gated := map[string]string{
			notifications.TypePickupUpcoming:         configModel.KeyRemindersPickupUpcomingEnabled,
			notifications.TypePickupOverdue:          configModel.KeyRemindersPickupOverdueEnabled,
			notifications.TypeActivityStart:          configModel.KeyRemindersActivityStartEnabled,
			notifications.TypeActivityOverdue:        configModel.KeyRemindersActivityOverdueEnabled,
			notifications.TypeStudentAbsenceReported: configModel.KeyNotificationsAbsenceReportedEnabled,
		}
		for key, expected := range gated {
			def, ok := notifications.GetType(key)
			require.True(t, ok, "type %s must be registered", key)
			assert.Equal(t, expected, def.TenantGate, "gate for %s", key)
		}

		// Whether a person wants to hear about their own shift is nobody
		// else's decision, so this one deliberately has no school-wide gate.
		def, ok := notifications.GetType(notifications.TypeMyActivityStarting)
		require.True(t, ok)
		assert.Empty(t, def.TenantGate)
	})

	t.Run("unknown key is not registered", func(t *testing.T) {
		_, ok := notifications.GetType("does_not_exist")
		assert.False(t, ok)
	})
}
