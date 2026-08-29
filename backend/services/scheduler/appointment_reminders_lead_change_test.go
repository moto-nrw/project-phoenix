package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunAppointmentRemindersForTenantDecreasedLeadUsesActiveWindow(t *testing.T) {
	t.Parallel()

	scanFrom := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	scanTo := scanFrom.Add(appointmentReminderInterval)
	queuer := &fakeReminderQueuer{}
	s := unitScheduler(&Scheduler{
		logger:               slog.Default(),
		appointmentReminders: queuer,
		settings:             appointmentReminderSettings(12, true)})

	require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))
	require.Len(t, queuer.calls, 1)
	assert.Equal(t, scanFrom.Add(12*time.Hour), queuer.calls[0].from)
	assert.Equal(t, scanTo.Add(12*time.Hour), queuer.calls[0].to)
}
