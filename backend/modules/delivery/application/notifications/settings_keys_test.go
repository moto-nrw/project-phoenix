package notifications_test

import (
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	"github.com/stretchr/testify/assert"
)

// TestRestatedSettingKeysMatchTheRegistry pins every registry key this package
// restates to the Settings Platform's name: a renamed key must fail here, not
// resolve as an unknown definition at runtime.
func TestRestatedSettingKeysMatchTheRegistry(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{
		configModel.KeyNotificationsDispatchEnabled,
		configModel.KeyNotificationsActiveWindowStart,
		configModel.KeyNotificationsActiveWindowEnd,
		configModel.KeyNotificationsAbsenceReportedEnabled,
		configModel.KeyNotificationsOnDutyOnly,
		configModel.KeyRemindersPickupUpcomingEnabled,
		configModel.KeyRemindersPickupOverdueEnabled,
		configModel.KeyRemindersActivityStartEnabled,
		configModel.KeyRemindersActivityOverdueEnabled,
		configModel.KeyParentNotesEnabled,
		configModel.KeyStaffMessagingEnabled,
		configModel.KeyCalendarAppointmentReminderEnabled,
	}, notifications.RestatedSettingKeys)
}
