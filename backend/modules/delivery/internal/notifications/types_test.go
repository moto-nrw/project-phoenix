package notifications_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/notifications"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Personal reminder producers and the catalogue pin the same wire values
// independently, so Delivery does not import the reminder workflow (#4072).
func TestPersonalReminderNoticesSpeakTheEventVocabulary(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{"pickup_upcoming", "pickup_overdue", "activity_start", "activity_overdue", "activity_running_over", "my_activity_starting"}, []string{
		notifications.TypePickupUpcoming, notifications.TypePickupOverdue,
		notifications.TypeActivityStart, notifications.TypeActivityOverdue,
		notifications.TypeActivityRunningOver, notifications.TypeMyActivityStarting,
	})
	for _, notificationType := range []string{"pickup_upcoming", "pickup_overdue", "activity_start", "activity_overdue", "activity_running_over", "my_activity_starting"} {
		_, known := notifications.GetType(notificationType)
		assert.True(t, known, "%s is not in the notification catalogue", notificationType)
	}
	assert.Equal(t, "normal", notifications.PriorityNormal)
	assert.Equal(t, "high", notifications.PriorityHigh)
	assert.Equal(t, "staff", string(notifications.ScopeStaff))
}

func TestNotificationTypeCatalogue(t *testing.T) {
	t.Parallel()

	t.Run("staff catalogue is grouped and ordered", func(t *testing.T) {
		defs := notifications.TypesForPortal(notifications.PortalStaff)
		require.Len(t, defs, 10)

		keys := make([]string, len(defs))
		for i, def := range defs {
			keys[i] = def.Key
		}
		assert.Equal(t, []string{
			notifications.TypePickupUpcoming,
			notifications.TypePickupOverdue,
			notifications.TypeActivityStart,
			notifications.TypeActivityOverdue,
			notifications.TypeActivityRunningOver,
			notifications.TypeMyActivityStarting,
			notifications.TypeStudentAbsenceReported,
			notifications.TypeStaffParentMessage,
			notifications.TypeStaffMessage,
			notifications.TypeStaffSurvey,
		}, keys, "order is fixed by group then SortOrder, not by registration order")
	})

	t.Run("parent catalogue is separate", func(t *testing.T) {
		defs := notifications.TypesForPortal(notifications.PortalParent)
		require.Len(t, defs, 6)

		keys := make([]string, len(defs))
		for i, def := range defs {
			keys[i] = def.Key
		}
		assert.Equal(t, []string{
			notifications.TypeParentRequestDecided,
			notifications.TypeParentAnnouncement,
			notifications.TypeParentMessage,
			notifications.TypeParentAppointment,
			notifications.TypeParentAppointmentReminder,
			notifications.TypeParentCareCancelled,
		}, keys, "order is fixed by group then SortOrder, not by registration order")

		// A staff type must never surface in the parent portal and vice versa:
		// the catalogue is what the preference page renders, so a leak here
		// would offer someone consent to a notification they cannot receive.
		parentKeys := make(map[string]struct{}, len(defs))
		for _, def := range defs {
			parentKeys[def.Key] = struct{}{}
		}
		for _, def := range notifications.TypesForPortal(notifications.PortalStaff) {
			assert.NotContains(t, parentKeys, def.Key)
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
			notifications.TypePickupUpcoming:            "reminders.pickup_upcoming_enabled",
			notifications.TypePickupOverdue:             "reminders.pickup_overdue_enabled",
			notifications.TypeActivityStart:             "reminders.activity_start_enabled",
			notifications.TypeActivityOverdue:           "reminders.activity_overdue_enabled",
			notifications.TypeActivityRunningOver:       "reminders.activity_running_over_enabled",
			notifications.TypeStudentAbsenceReported:    "notifications.absence_reported_enabled",
			notifications.TypeStaffParentMessage:        "operations.parent_notes_enabled",
			notifications.TypeParentMessage:             "operations.parent_notes_enabled",
			notifications.TypeParentRequestDecided:      "operations.parent_notes_enabled",
			notifications.TypeParentAppointmentReminder: "calendar.appointment_reminder_enabled",
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
