package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAppointmentRemindersForTenantDecreasedLeadUsesActiveWindow(t *testing.T) {
	scanFrom := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	scanTo := scanFrom.Add(appointmentReminderInterval)
	queuer := &fakeReminderQueuer{}
	s := &Scheduler{
		logger:               slog.Default(),
		appointmentReminders: queuer,
		settings: &fakeSettingsResolver{intValues: map[string]int{
			configModel.KeyCalendarAppointmentReminderLeadHours: 12,
		}},
		appointmentReminderLeadHours: map[int64]int{7: 24},
	}

	require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))
	require.Len(t, queuer.calls, 1)
	assert.Equal(t, scanFrom.Add(12*time.Hour), queuer.calls[0].from)
	assert.Equal(t, scanTo.Add(12*time.Hour), queuer.calls[0].to)
}
