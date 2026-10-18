package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRunAppointmentRemindersForTenantLeadSettingErrorDoesNotScan(t *testing.T) {
	t.Parallel()

	queuer := &fakeReminderQueuer{}
	s := unitScheduler(func(unit *Scheduler) {
		unit.logger = slog.Default()
		unit.appointmentReminders = queuer
		unit.settings = &fakeSettingsResolver{boolValues: map[string]bool{
			settingCalendarAppointmentReminderEnabled: true,
		}}
	})

	err := s.runAppointmentRemindersForTenant(
		context.Background(),
		7,
		time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC),
	)

	assert.Error(t, err)
	assert.Empty(t, queuer.calls)
}
