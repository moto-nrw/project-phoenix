// Scheduled parent-announcement reminder tick tests (#3162). Pure in-memory:
// the sender is a fake; the tests drive the window arithmetic and the
// per-tenant pass. Which announcements are due and who they reach is the
// Communication module's business and is tested there.
package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeAnnouncementReminderSender struct {
	calls []reminderCall
	sent  int
	err   error
}

func (f *fakeAnnouncementReminderSender) SendDueParentAnnouncementReminders(_ context.Context, notBefore, dueBefore time.Time) (int, error) {
	f.calls = append(f.calls, reminderCall{from: notBefore, to: dueBefore})
	return f.sent, f.err
}

func TestAnnouncementReminderWindow(t *testing.T) {
	t.Parallel()

	t.Run("the first window after boot recovers only the bounded lookback", func(t *testing.T) {
		s := unitScheduler(&Scheduler{logger: slog.Default()})
		now := time.Date(2026, 9, 24, 8, 3, 0, 0, time.UTC)

		from, to := s.announcementReminderWindow(7, now)

		assert.Equal(t, now, to)
		assert.Equal(t, announcementReminderMaxLookback, to.Sub(from),
			"a reminder discovered after a long outage is shown as missed, not sent stale")
	})

	t.Run("consecutive ticks cover adjacent windows", func(t *testing.T) {
		s := unitScheduler(&Scheduler{logger: slog.Default()})
		first := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
		second := first.Add(announcementReminderInterval)

		s.markAnnouncementReminderScanned(7, first)
		from, to := s.announcementReminderWindow(7, second)

		assert.Equal(t, first, from, "a normal tick resumes at the prior successful boundary")
		assert.Equal(t, second, to)
	})

	t.Run("a window is clamped after downtime", func(t *testing.T) {
		s := unitScheduler(&Scheduler{logger: slog.Default()})
		bootTime := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
		s.markAnnouncementReminderScanned(7, bootTime)

		resumed := bootTime.Add(3 * 24 * time.Hour)
		from, to := s.announcementReminderWindow(7, resumed)

		assert.Equal(t, announcementReminderMaxLookback, to.Sub(from))
	})

	t.Run("a failed tenant scan keeps its boundary and is retried", func(t *testing.T) {
		s := unitScheduler(&Scheduler{logger: slog.Default()})
		first := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
		second := first.Add(announcementReminderInterval)

		_, _ = s.announcementReminderWindow(7, first)
		from, to := s.announcementReminderWindow(7, second)

		assert.Equal(t, second.Add(-announcementReminderMaxLookback), from)
		assert.Equal(t, second, to)
	})

	t.Run("each tenant has an independent boundary", func(t *testing.T) {
		s := unitScheduler(&Scheduler{logger: slog.Default()})
		first := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
		s.markAnnouncementReminderScanned(7, first)

		from, _ := s.announcementReminderWindow(8, first.Add(announcementReminderInterval))

		assert.Equal(t, first.Add(announcementReminderInterval-announcementReminderMaxLookback), from)
	})
}

func TestRunAnnouncementRemindersForTenant(t *testing.T) {
	t.Parallel()

	scanFrom := time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)
	scanTo := scanFrom.Add(announcementReminderInterval)

	t.Run("hands the exact window to the sender", func(t *testing.T) {
		sender := &fakeAnnouncementReminderSender{sent: 2}
		s := unitScheduler(&Scheduler{logger: slog.Default(), announcementReminders: sender})

		require.NoError(t, s.runAnnouncementRemindersForTenant(context.Background(), 7, scanFrom, scanTo))
		require.Len(t, sender.calls, 1)
		assert.Equal(t, scanFrom, sender.calls[0].from)
		assert.Equal(t, scanTo, sender.calls[0].to)
	})

	t.Run("a sender failure is reported so the boundary does not advance", func(t *testing.T) {
		sender := &fakeAnnouncementReminderSender{err: errors.New("outbox unavailable")}
		s := unitScheduler(&Scheduler{logger: slog.Default(), announcementReminders: sender})

		require.Error(t, s.runAnnouncementRemindersForTenant(context.Background(), 7, scanFrom, scanTo))
	})
}

func TestAnnouncementReminderTaskRegistersOnlyWithASender(t *testing.T) {
	t.Parallel()

	without := unitScheduler(&Scheduler{logger: slog.Default(), tasks: map[string]*ScheduledTask{}})
	without.scheduleAnnouncementReminderTask()
	_, registered := without.tasks["announcement-reminders"]
	assert.False(t, registered, "no sender, no tick")

	with := unitScheduler(&Scheduler{logger: slog.Default(), tasks: map[string]*ScheduledTask{}, announcementReminders: &fakeAnnouncementReminderSender{}})
	with.scheduleAnnouncementReminderTask()
	_, registered = with.tasks["announcement-reminders"]
	assert.True(t, registered)
}
