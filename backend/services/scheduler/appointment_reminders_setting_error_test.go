package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
)

func TestRunAppointmentRemindersForTenantLeadSettingErrorDoesNotScan(t *testing.T) {
	t.Parallel()

	queuer := &fakeReminderQueuer{}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		appointmentReminders: queuer,
		settings: &fakeSettingsResolver{boolValues: map[string]bool{
			configModel.KeyCalendarAppointmentReminderEnabled: true,
		}}})

	err := s.runAppointmentRemindersForTenant(
		context.Background(),
		7,
		time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 8, 1, 10, 5, 0, 0, time.UTC),
	)

	assert.Error(t, err)
	assert.Empty(t, queuer.calls)
}
