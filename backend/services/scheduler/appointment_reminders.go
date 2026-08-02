// Guardian appointment reminders (#1671).
//
// The calendar already mails parents when an appointment is published, changed
// or cancelled. This tick adds the one notice that cannot be sent from a write
// path because nothing happens at that moment: the reminder shortly before the
// appointment itself.
//
// The tick owns only the schedule. Which occurrences fall due, who is reachable
// and what the mail says lives in services/calendar, which owns occurrence
// expansion — a second copy of that arithmetic here would drift from the
// calendar the parent is looking at.
package scheduler

import (
	"context"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

const (
	// appointmentReminderInterval is how often the tick looks for due
	// occurrences. It bounds how late a reminder can be, so it is deliberately
	// much shorter than the smallest configurable lead time (1 hour).
	appointmentReminderInterval = 5 * time.Minute

	// appointmentReminderMaxLookback caps the window after downtime. A process
	// that was down for a day would otherwise wake up and scan a day's worth of
	// occurrences at once, mailing reminders for appointments whose reminder
	// moment is long gone. One hour late is still a useful reminder; six hours
	// late is noise.
	appointmentReminderMaxLookback = time.Hour

	defaultAppointmentReminderLeadHours = 24
)

// AppointmentReminderQueuer is the narrow slice of the calendar service the
// scheduler needs. Declared here so the scheduler does not depend on the whole
// calendar service interface.
type AppointmentReminderQueuer interface {
	// EnqueueDueAppointmentReminders queues reminders for every guardian-facing
	// occurrence starting in [from, to) and returns how many mails it queued.
	EnqueueDueAppointmentReminders(ctx context.Context, from, to time.Time) (int, error)
}

// SetAppointmentReminderQueuer wires the guardian reminder tick. Nil disables it.
func (s *Scheduler) SetAppointmentReminderQueuer(q AppointmentReminderQueuer) {
	s.appointmentReminders = q
}

func (s *Scheduler) scheduleAppointmentReminderTask() {
	if s.appointmentReminders == nil {
		s.getLogger().Info("appointment reminder tick not configured (no queuer)")
		return
	}
	s.registerTask("appointment-reminders", "interval-poll", s.runAppointmentReminderTaskPolling)
}

func (s *Scheduler) runAppointmentReminderTaskPolling(task *ScheduledTask) {
	s.runIntervalPolling(task, "panic in appointment reminder task",
		"appointment reminder tick using interval polling",
		appointmentReminderInterval,
		func() time.Duration { return appointmentReminderInterval },
		s.checkAndRunAppointmentReminders)
}

// checkAndRunAppointmentReminders scans one overlapping window per active
// tenant. The overlap retries a tenant that failed during the previous pass;
// reminder outbox keys make the repeat harmless for tenants that succeeded.
func (s *Scheduler) checkAndRunAppointmentReminders(task *ScheduledTask) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	scanToByTenant := make(map[int64]time.Time)
	completed := s.forEachTenantSettings(ctx, "appointment-reminders", func(tenantCtx context.Context, tenantID int64) error {
		scanFrom, scanTo := s.appointmentReminderWindow(tenantID, now)
		if err := s.runAppointmentRemindersForTenant(tenantCtx, tenantID, scanFrom, scanTo); err != nil {
			return err
		}
		scanToByTenant[tenantID] = scanTo
		return nil
	})
	for _, tenantID := range completed {
		s.markAppointmentReminderScanned(tenantID, scanToByTenant[tenantID])
	}
}

// appointmentReminderWindow returns tenantID's scan window [from, to). The
// successful boundary is recorded separately, after the tenant's work commits.
func (s *Scheduler) appointmentReminderWindow(tenantID int64, now time.Time) (time.Time, time.Time) {
	s.appointmentReminderScanMu.Lock()
	defer s.appointmentReminderScanMu.Unlock()

	from := s.appointmentReminderScannedAt[tenantID]
	if from.IsZero() || now.Sub(from) > appointmentReminderMaxLookback {
		// On startup or after a long pause, recover only the useful bounded
		// lookback so a prolonged outage does not mail stale reminders.
		from = now.Add(-appointmentReminderMaxLookback)
	}
	return from, now
}

func (s *Scheduler) markAppointmentReminderScanned(tenantID int64, scannedAt time.Time) {
	s.appointmentReminderScanMu.Lock()
	defer s.appointmentReminderScanMu.Unlock()
	if s.appointmentReminderScannedAt == nil {
		s.appointmentReminderScannedAt = make(map[int64]time.Time)
	}
	s.appointmentReminderScannedAt[tenantID] = scannedAt
}

// runAppointmentRemindersForTenant resolves the school's reminder settings and
// asks the calendar service for the occurrences that fall due. The lead time
// shifts both window bounds, so the window stays exactly as long as the tick
// interval and every occurrence is offered to exactly one tick.
func (s *Scheduler) runAppointmentRemindersForTenant(ctx context.Context, tenantID int64, scanFrom, scanTo time.Time) error {
	enabled := true
	if s.settings != nil {
		hasOverride, err := s.settings.HasTenantOverride(ctx, configModel.KeyCalendarAppointmentReminderEnabled)
		if err != nil {
			s.getLogger().Error("appointment reminder setting lookup failed",
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
			return err
		}
		if hasOverride {
			enabled, err = s.settings.ResolveBool(ctx, configModel.KeyCalendarAppointmentReminderEnabled)
			if err != nil {
				s.getLogger().Error("appointment reminder setting resolution failed",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
				return err
			}
		}
	}
	if !enabled {
		return nil
	}
	leadHours := s.resolveIntSetting(ctx, configModel.KeyCalendarAppointmentReminderLeadHours, "", defaultAppointmentReminderLeadHours)
	if leadHours < 1 {
		leadHours = defaultAppointmentReminderLeadHours
	}
	lead := time.Duration(leadHours) * time.Hour

	queued, err := s.appointmentReminders.EnqueueDueAppointmentReminders(ctx, scanFrom.Add(lead), scanTo.Add(lead))
	if err != nil {
		s.getLogger().Error("appointment reminder tick failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return err
	}
	if queued > 0 {
		s.getLogger().Info("appointment reminders queued",
			slog.Int64("tenant_id", tenantID),
			slog.Int("queued", queued),
			slog.Int("lead_hours", leadHours),
		)
	}
	return nil
}
