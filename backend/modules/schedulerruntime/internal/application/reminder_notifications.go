// Personal reminder notifications: the minute tick of the producer in
// Reminder Delivery (workflows/reminderdelivery), which turns the #1457
// reminder list into notifications for the people it concerns. The Worker
// only runs the tick: it iterates the active schools, skips a school whose
// notification feature flag is off and reads the school's on-duty setting.
package application

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// scheduleReminderNotificationTask registers the minute tick when the producer
// is wired.
func (s *Scheduler) scheduleReminderNotificationTask() {
	if s.reminderNotifications == nil {
		s.getLogger().Info("reminder notification tick not configured (no producer)")
		return
	}
	s.registerTask("reminder-notifications", "1m-poll", s.runReminderNotificationTaskPolling)
}

func (s *Scheduler) runReminderNotificationTaskPolling(task *ScheduledTask) {
	s.runMinutePolling(task, "panic in reminder notification task",
		"reminder notification tick using minute-polling",
		s.checkAndRunReminderNotifications)
}

// checkAndRunReminderNotifications iterates the active tenants and hands each
// one whose notification feature flag is on to the producer.
func (s *Scheduler) checkAndRunReminderNotifications(ctx context.Context, task *ScheduledTask) {
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

	ctx, cancel := s.taskContext(ctx, 5*time.Minute)
	defer cancel()

	s.forEachTenantSettings(ctx, "reminder-notifications", func(tenantCtx context.Context, tenantID int64) error {
		// Cheap gate first: skip the whole resolution while the notification
		// feature flag is off for this tenant. The notification service would
		// also refuse, but there is no point resolving anybody. That mirroring
		// only holds while both sides read the flag the same way, hence
		// notificationDispatchEnabled instead of the scheduler's
		// literal-default helpers.
		if !s.notificationDispatchEnabled(tenantCtx, tenantID) {
			return nil
		}
		onDutyOnly, err := s.settings.ResolveBool(tenantCtx, settingNotificationsOnDutyOnly)
		if err != nil {
			return fmt.Errorf("resolve notification duty scope: %w", err)
		}
		s.reminderNotifications(tenantCtx, tenantID, time.Now(), onDutyOnly)
		return nil
	})
}

// notificationDispatchEnabled answers the notification feature flag exactly the
// way the notification router does (services/notifications/service.go): the
// tenant override when one exists, otherwise the value registered in the
// settings registry. ResolveBool already returns the registry default, so this
// gate carries no default of its own. A literal here would be a second source of
// truth beside the registry: a registry default of "on" against a literal "off"
// silenced the whole tick for every school without an explicit override.
//
// Fails closed when the flag cannot be read and when no settings service is
// wired: without one the router cannot dispatch either.
func (s *Scheduler) notificationDispatchEnabled(ctx context.Context, tenantID int64) bool {
	if s.settings == nil {
		return false
	}
	enabled, err := s.settings.ResolveBool(ctx, settingNotificationsDispatchEnabled)
	if err != nil {
		s.getLogger().Warn("reminder notification tick: notification feature flag unreadable, skipping tenant",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return enabled
}
