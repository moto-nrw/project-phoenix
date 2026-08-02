// Guardian appointment reminder tick tests (#1671). Pure in-memory: the queuer
// is a fake and the tests drive the window arithmetic and the per-tenant pass
// directly. What each occurrence means is the calendar service's business and is
// tested there.
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

type fakeReminderQueuer struct {
	calls   []reminderCall
	queued  int
	err     error
	callErr error
}

type reminderCall struct {
	from time.Time
	to   time.Time
}

func (f *fakeReminderQueuer) EnqueueDueAppointmentReminders(_ context.Context, from, to time.Time) (int, error) {
	f.calls = append(f.calls, reminderCall{from: from, to: to})
	if f.callErr != nil {
		return 0, f.callErr
	}
	return f.queued, f.err
}

func TestAppointmentReminderWindow(t *testing.T) {
	t.Run("the first window after boot recovers the bounded lookback", func(t *testing.T) {
		s := &Scheduler{logger: slog.Default()}
		now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)

		from, to := s.appointmentReminderWindow(7, now)

		assert.Equal(t, now, to)
		assert.Equal(t, appointmentReminderMaxLookback, to.Sub(from),
			"a zero-valued last-scan recovers useful reminders without scanning indefinitely")
	})

	t.Run("consecutive ticks cover adjacent windows", func(t *testing.T) {
		s := &Scheduler{logger: slog.Default()}
		first := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		second := first.Add(7 * time.Minute)

		s.markAppointmentReminderScanned(7, first)
		secondFrom, secondTo := s.appointmentReminderWindow(7, second)

		assert.Equal(t, first, secondFrom,
			"a normal tick resumes at the prior scan boundary")
		assert.Equal(t, second, secondTo)
	})

	t.Run("a window is clamped after downtime", func(t *testing.T) {
		s := &Scheduler{logger: slog.Default()}
		bootTime := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		s.markAppointmentReminderScanned(7, bootTime)

		// The process was down for a day.
		resumed := bootTime.Add(24 * time.Hour)
		from, to := s.appointmentReminderWindow(7, resumed)

		assert.Equal(t, appointmentReminderMaxLookback, to.Sub(from),
			"a day-long catch-up would mail reminders whose moment is long gone")
	})

	t.Run("a failed tenant scan is retried without advancing its boundary", func(t *testing.T) {
		s := &Scheduler{logger: slog.Default()}
		first := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		second := first.Add(appointmentReminderInterval)

		_, _ = s.appointmentReminderWindow(7, first)
		from, to := s.appointmentReminderWindow(7, second)

		assert.Equal(t, second.Add(-appointmentReminderMaxLookback), from)
		assert.Equal(t, second, to)
	})

	t.Run("each tenant has an independent successful boundary", func(t *testing.T) {
		s := &Scheduler{logger: slog.Default()}
		first := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		s.markAppointmentReminderScanned(7, first)

		from, _ := s.appointmentReminderWindow(8, first.Add(appointmentReminderInterval))

		assert.Equal(t, first.Add(appointmentReminderInterval-appointmentReminderMaxLookback), from)
	})
}

func TestRunAppointmentRemindersForTenant(t *testing.T) {
	scanFrom := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	scanTo := scanFrom.Add(appointmentReminderInterval)

	t.Run("an increased lead bridges the newly exposed reminder interval", func(t *testing.T) {
		queuer := &fakeReminderQueuer{}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings: &fakeSettingsResolver{intValues: map[string]int{
				configModel.KeyCalendarAppointmentReminderLeadHours: 24,
			}},
			appointmentReminderLeadHours: map[int64]int{7: 12},
		}

		require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))
		require.Len(t, queuer.calls, 1)
		assert.Equal(t, scanFrom.Add(12*time.Hour), queuer.calls[0].from)
		assert.Equal(t, scanTo.Add(24*time.Hour), queuer.calls[0].to)
	})

	t.Run("the lead time shifts the window into the future", func(t *testing.T) {
		queuer := &fakeReminderQueuer{}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings: &fakeSettingsResolver{
				intValues: map[string]int{
					configModel.KeyCalendarAppointmentReminderLeadHours: 12,
				},
			},
		}

		require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))

		require.Len(t, queuer.calls, 1)
		assert.Equal(t, scanFrom.Add(12*time.Hour), queuer.calls[0].from)
		assert.Equal(t, scanTo.Add(12*time.Hour), queuer.calls[0].to)
		assert.Equal(t, appointmentReminderInterval, queuer.calls[0].to.Sub(queuer.calls[0].from),
			"shifting both bounds keeps the window exactly one tick long")
	})

	t.Run("a school that switched reminders off is not scanned at all", func(t *testing.T) {
		queuer := &fakeReminderQueuer{}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings: &fakeSettingsResolver{
				boolValues: map[string]bool{
					configModel.KeyCalendarAppointmentReminderEnabled: false,
				},
			},
		}

		require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))

		assert.Empty(t, queuer.calls, "the off switch has to stop the work, not just the delivery")
	})

	t.Run("without an override the registered default applies", func(t *testing.T) {
		queuer := &fakeReminderQueuer{}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings:             &fakeSettingsResolver{},
		}

		require.NoError(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))

		require.Len(t, queuer.calls, 1, "the feature defaults on")
		assert.Equal(t, scanFrom.Add(defaultAppointmentReminderLeadHours*time.Hour), queuer.calls[0].from)
	})

	t.Run("a failing tenant is logged, not fatal", func(t *testing.T) {
		queuer := &fakeReminderQueuer{callErr: assertAnError{}}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings:             &fakeSettingsResolver{},
		}

		assert.Error(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo),
			"the caller must leave the failed tenant's checkpoint unchanged")
	})

	t.Run("a failed reminder-enabled lookup fails closed", func(t *testing.T) {
		queuer := &fakeReminderQueuer{}
		s := &Scheduler{
			logger:               slog.Default(),
			appointmentReminders: queuer,
			settings:             &fakeSettingsResolver{overrideErr: assertAnError{}},
		}

		assert.Error(t, s.runAppointmentRemindersForTenant(context.Background(), 7, scanFrom, scanTo))

		assert.Empty(t, queuer.calls)
	})
}

type assertAnError struct{}

func (assertAnError) Error() string { return "calendar unavailable" }
