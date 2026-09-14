// Scheduled parent-announcement reminders (#3162).
//
// A published announcement can carry one reminder moment. Nothing happens on
// a write path at that moment, so a tick has to find it: every few minutes,
// per active tenant, the tick hands the Communication capability the window
// of reminder moments that fell due since the last successful scan.
//
// The tick owns only the schedule. Which announcements are due, who they
// reach and what leaves over which channel lives in the Communication module,
// next to the publication it repeats.
package scheduler

import (
	"context"
	"log/slog"
	"time"
)

const (
	// announcementReminderInterval bounds how late a reminder can be delivered
	// relative to the moment the school chose.
	announcementReminderInterval = 5 * time.Minute

	// announcementReminderMaxLookback caps the window after downtime. A
	// reminder is tied to a moment the school picked ("the morning before the
	// early pick-up"); one that is discovered half a day late is still that
	// morning's reminder, one discovered a week late is noise the school never
	// asked for. Reminders older than this stay unsent and are shown to staff
	// as missed rather than sent stale.
	announcementReminderMaxLookback = 12 * time.Hour
)

func (s *Scheduler) scheduleAnnouncementReminderTask() {
	if s.announcementReminders == nil {
		s.getLogger().Info("announcement reminder tick not configured (no sender)")
		return
	}
	s.registerTask("announcement-reminders", "interval-poll", s.runAnnouncementReminderTaskPolling)
}

func (s *Scheduler) runAnnouncementReminderTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in announcement reminder task",
		"announcement reminder tick using interval polling",
		announcementReminderInterval,
		func() time.Duration { return announcementReminderInterval },
		s.checkAndRunAnnouncementReminders)
}

// checkAndRunAnnouncementReminders scans one window per active tenant inside
// that tenant's transaction. The claim mark (reminder_sent_at) makes a
// repeated window harmless: a tenant whose pass failed is retried from its
// old boundary, a tenant whose pass committed sends nothing twice.
func (s *Scheduler) checkAndRunAnnouncementReminders(ctx context.Context, task *ScheduledTask) {
	task.mu.Lock()
	if task.Running {
		task.mu.Unlock()
		return
	}
	task.Running = true
	task.mu.Unlock()
	defer func() {
		task.mu.Lock()
		task.Running = false
		task.mu.Unlock()
	}()

	now := time.Now()

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	scanToByTenant := make(map[int64]time.Time)
	completed := s.forEachTenantSettings(ctx, "announcement-reminders", func(tenantCtx context.Context, tenantID int64) error {
		scanFrom, scanTo := s.announcementReminderWindow(tenantID, now)
		if err := s.runAnnouncementRemindersForTenant(tenantCtx, tenantID, scanFrom, scanTo); err != nil {
			return err
		}
		scanToByTenant[tenantID] = scanTo
		return nil
	})
	for _, tenantID := range completed {
		s.markAnnouncementReminderScanned(tenantID, scanToByTenant[tenantID])
	}
}

// announcementReminderWindow returns tenantID's scan window (from, to]. On
// startup or after a long pause only the bounded lookback is recovered.
func (s *Scheduler) announcementReminderWindow(tenantID int64, now time.Time) (time.Time, time.Time) {
	s.announcementReminderScanMu.Lock()
	defer s.announcementReminderScanMu.Unlock()

	from := s.announcementReminderScannedAt[tenantID]
	if from.IsZero() || now.Sub(from) > announcementReminderMaxLookback {
		from = now.Add(-announcementReminderMaxLookback)
	}
	return from, now
}

func (s *Scheduler) markAnnouncementReminderScanned(tenantID int64, scannedAt time.Time) {
	s.announcementReminderScanMu.Lock()
	defer s.announcementReminderScanMu.Unlock()
	if s.announcementReminderScannedAt == nil {
		s.announcementReminderScannedAt = make(map[int64]time.Time)
	}
	s.announcementReminderScannedAt[tenantID] = scannedAt
}

func (s *Scheduler) runAnnouncementRemindersForTenant(ctx context.Context, tenantID int64, scanFrom, scanTo time.Time) error {
	sent, err := s.announcementReminders.SendDueParentAnnouncementReminders(ctx, scanFrom, scanTo)
	if err != nil {
		s.getLogger().Error("announcement reminder tick failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if sent > 0 {
		s.getLogger().Info("announcement reminders sent",
			slog.Int64("tenant_id", tenantID),
			slog.Int("sent", sent),
		)
	}
	return nil
}
