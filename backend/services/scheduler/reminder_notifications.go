// Reminder → notification bridge (#1624 follow-up): the first real consumer
// of the notification abstraction. Every minute, per tenant, the tick
// computes the current reminder list (issue #1457 — pickups, activity
// starts, overdue variants) and dispatches ONE aggregated, display-safe
// notification for reminders that became due since the last tick.
//
// Deliberately generic: the tick knows nothing about channels — it builds a
// notifications.Event and calls Notify. Whatever channels the abstraction
// grows (Web Push, e-mail) automatically apply here.
//
// GDPR: reminder rows carry student names, but the dispatched notification
// NEVER does. Title/body only say how many reminders of which kind are due;
// the deep link points to /reminders where the names are shown to the
// authenticated, permission-filtered viewer.
//
// Re-fire guard: an in-memory sync.Map keyed by (tenant, reminder identity),
// rotated on civil-date rollover — the same shape as the instance-overdue
// tick. A mid-day restart re-fires currently-due reminders once; toasts are
// ephemeral, that cost is acceptable for zero disk state.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/notifications"
	"github.com/moto-nrw/project-phoenix/services/reminders"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// reminderNotificationKey identifies one dispatched reminder occurrence.
type reminderNotificationKey struct {
	tenantID int64
	reminder string // type + entity id + due time
}

// SetReminderNotificationDeps wires the reminder-notification tick. Both nil
// (or either nil) → the task does not register, matching the scheduler's
// opt-in setter pattern.
func (s *Scheduler) SetReminderNotificationDeps(computer reminders.Service, notifier notifications.Service) {
	s.reminderComputer = computer
	s.reminderNotifier = notifier
}

// scheduleReminderNotificationTask registers the minute tick when both
// dependencies are wired.
func (s *Scheduler) scheduleReminderNotificationTask() {
	if s.reminderComputer == nil || s.reminderNotifier == nil {
		s.getLogger().Info("reminder notification tick not configured (missing reminders or notifications service)")
		return
	}
	s.registerTask("reminder-notifications", "1m-poll", s.runReminderNotificationTaskPolling)
}

func (s *Scheduler) runReminderNotificationTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in reminder notification task",
		"reminder notification tick using minute-polling",
		s.checkAndRunReminderNotifications)
}

// checkAndRunReminderNotifications rotates the day cache, then iterates
// active tenants. The per-tenant work lives in
// runReminderNotificationsForTenant so unit tests can drive it directly.
func (s *Scheduler) checkAndRunReminderNotifications(task *ScheduledTask) {
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

	s.rotateReminderNotificationCacheIfNewDay(time.Now())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "reminder-notifications", func(tenantCtx context.Context, tenantID int64) error {
		// Cheap gate first: skip the reminder computation entirely while the
		// notification feature flag is off for this tenant. Notify would also
		// refuse (ErrDisabled), but there is no point computing the list.
		if !s.resolveBoolSetting(tenantCtx, configModel.KeyNotificationsDispatchEnabled, "", false) {
			return nil
		}
		s.runReminderNotificationsForTenant(tenantCtx, tenantID, time.Now())
		return nil
	})
}

// runReminderNotificationsForTenant computes the tenant-wide reminder list
// and dispatches one aggregated notification for occurrences not yet seen
// today. `now` is injected for deterministic tests.
func (s *Scheduler) runReminderNotificationsForTenant(ctx context.Context, tenantID int64, now time.Time) {
	// Admin scope = all present children and all activities: the notification
	// is tenant-wide, so it must reflect the full list, not one staffer's
	// supervision slice. It carries no per-child data either way.
	result, err := s.reminderComputer.Compute(ctx, reminders.Scope{IsAdmin: true})
	if err != nil {
		s.getLogger().Warn("reminder notification tick: compute failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}
	if result == nil || !result.Enabled || len(result.Reminders) == 0 {
		return
	}

	fresh := make([]reminders.Reminder, 0, len(result.Reminders))
	freshKeys := make([]reminderNotificationKey, 0, len(result.Reminders))
	for _, r := range result.Reminders {
		key := reminderNotificationKey{tenantID: tenantID, reminder: reminderIdentity(r)}
		if _, seen := s.reminderNotified.Load(key); seen {
			continue
		}
		fresh = append(fresh, r)
		freshKeys = append(freshKeys, key)
	}
	if len(fresh) == 0 {
		return
	}

	event := buildReminderNotificationEvent(tenantID, fresh)
	if err := s.reminderNotifier.Notify(ctx, event); err != nil {
		// ErrDisabled can still race the settings gate above (flag flipped
		// between checks); leave the occurrences unmarked so they fire on the
		// next tick once dispatch is possible.
		s.getLogger().Warn("reminder notification tick: notify failed",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return
	}
	// Notify registered delivery first, so this hook runs after delivery and
	// only after the surrounding tenant transaction commits. A rollback or
	// commit failure drops both hooks and leaves the occurrences eligible for
	// the next tick.
	tenant.RegisterAfterCommit(ctx, func() {
		for _, key := range freshKeys {
			s.reminderNotified.Store(key, now)
		}
		s.getLogger().Info("reminder notification dispatched",
			slog.Int64("tenant_id", tenantID),
			slog.Int("reminder_count", len(fresh)),
		)
	})
}

// reminderIdentity builds the dedup identity of one reminder occurrence:
// type + entity + due time. A rescheduled pickup (new DueTime) is a new
// occurrence on purpose — the plan changed, staff should hear about it again.
func reminderIdentity(r reminders.Reminder) string {
	entity := ""
	switch {
	case r.StudentID != nil:
		entity = "s" + *r.StudentID
	case r.ActivityInstanceID != nil:
		entity = "a" + *r.ActivityInstanceID
	}
	return r.Type + "|" + entity + "|" + r.DueTime
}

// reminderTypeLabels maps reminder types to display-safe German fragments.
// Singular/plural chosen by count at build time.
var reminderTypeLabels = map[string]struct{ singular, plural string }{
	reminders.TypePickupUpcoming:  {"anstehende Abholung", "anstehende Abholungen"},
	reminders.TypePickupOverdue:   {"überfällige Abholung", "überfällige Abholungen"},
	reminders.TypeActivityStart:   {"beginnende Aktivität", "beginnende Aktivitäten"},
	reminders.TypeActivityOverdue: {"überfällige Aktivität", "überfällige Aktivitäten"},
}

// buildReminderNotificationEvent aggregates fresh reminders into ONE
// display-safe event: counts per type, no student names, deep link to the
// reminders page. Overdue reminders raise the priority.
func buildReminderNotificationEvent(tenantID int64, fresh []reminders.Reminder) notifications.Event {
	counts := make(map[string]int, len(reminderTypeLabels))
	order := make([]string, 0, len(reminderTypeLabels))
	overdue := false
	for _, r := range fresh {
		if counts[r.Type] == 0 {
			order = append(order, r.Type)
		}
		counts[r.Type]++
		if r.Type == reminders.TypePickupOverdue || r.Type == reminders.TypeActivityOverdue {
			overdue = true
		}
	}

	parts := make([]string, 0, len(order))
	data := make(map[string]string, len(order))
	for _, typ := range order {
		n := counts[typ]
		label, known := reminderTypeLabels[typ]
		if !known {
			label = struct{ singular, plural string }{"fällige Erinnerung", "fällige Erinnerungen"}
		}
		if n == 1 {
			parts = append(parts, fmt.Sprintf("1 %s", label.singular))
		} else {
			parts = append(parts, fmt.Sprintf("%d %s", n, label.plural))
		}
		data[typ] = fmt.Sprintf("%d", n)
	}

	priority := notifications.PriorityNormal
	if overdue {
		priority = notifications.PriorityHigh
	}

	return notifications.Event{
		Type: "reminders_due",
		// The counts come from Compute(IsAdmin: true), so only clients with the
		// same effective admin scope may receive them. Caregivers fetch their
		// own supervision-filtered counts from /reminders.
		Audience: notifications.Audience{TenantID: tenantID, Scope: notifications.ScopeAdmin},
		Priority: priority,
		Title:    "Erinnerungen",
		Body:     strings.Join(parts, ", ") + ".",
		DeepLink: "/reminders",
		Data:     data,
	}
}

// rotateReminderNotificationCacheIfNewDay clears the re-fire guard on civil-
// date rollover, mirroring rotateOverdueCacheIfNewDay. Reminder identities
// embed only wall-clock due times, so yesterday's keys would otherwise
// suppress today's identical schedule.
func (s *Scheduler) rotateReminderNotificationCacheIfNewDay(now time.Time) {
	today := timezone.DateFromTime(now)
	s.reminderNotifiedDayMu.Lock()
	defer s.reminderNotifiedDayMu.Unlock()
	if s.reminderNotifiedDay != today {
		s.reminderNotified.Range(func(k, _ any) bool {
			s.reminderNotified.Delete(k)
			return true
		})
		s.reminderNotifiedDay = today
	}
}
