package api

import (
	"context"
	"errors"
	"log/slog"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
)

// newPersonalReminderTick builds Reminder Delivery's personal reminder
// producer for the Worker's minute tick: the reminder batch, the consent of
// the notification preferences, the staff and admin accounts, the on-duty
// staff of Workforce and the Delivery Platform's notification service.
func newPersonalReminderTick(graph *serveGraph, reads workerReads, runtime scheduler.TenantRuntime, logger *slog.Logger) (scheduler.ReminderNotificationTick, error) {
	services := graph.services
	producer, err := reminder.NewPersonalReminderNotifications(reminder.PersonalReminderDependencies{
		Computer: services.Reminders,
		Consent:  services.NotificationPreferences,
		Staff:    reads.staff,
		Admins:   reads.identity,
		OnDutyStaff: func(ctx context.Context) (map[int64]struct{}, error) {
			presence, err := reads.workSessions.GetTodayPresenceMap(ctx)
			if err != nil {
				return nil, err
			}
			return workforce.OnDutyStaffIDs(presence), nil
		},
		Notify: func(ctx context.Context, notices []reminder.PersonalReminderNotice) error {
			return services.Notifications.NotifyBatch(ctx, personalReminderEvents(notices))
		},
		Paused:      notificationDispatchPaused,
		AfterCommit: runtime.RegisterAfterCommit,
		Logger:      logger,
	})
	if err != nil {
		return nil, err
	}
	return producer.NotifyDue, nil
}

// personalReminderEvents hands the notices to the notification service as
// its events.
func personalReminderEvents(notices []reminder.PersonalReminderNotice) []deliveryCompose.NotificationEvent {
	events := make([]deliveryCompose.NotificationEvent, 0, len(notices))
	for _, notice := range notices {
		events = append(events, deliveryCompose.NotificationEvent{
			Type:           notice.Type,
			IdempotencyKey: notice.IdempotencyKey,
			RelatedType:    notice.RelatedType,
			RelatedID:      notice.RelatedID,
			Audience: deliveryCompose.NotificationAudience{
				TenantID:        notice.TenantID,
				Scope:           deliveryCompose.NotificationAudienceScope(notice.Scope),
				StaffAccountIDs: notice.StaffAccountIDs,
			},
			Priority: notice.Priority,
			Title:    notice.Title,
			Body:     notice.Body,
			DeepLink: notice.DeepLink,
			Data:     notice.Data,
		})
	}
	return events
}

// notificationDispatchPaused reports the notification service's ordinary
// refusals: notifications switched off, or outside the tenant's window.
func notificationDispatchPaused(err error) bool {
	return errors.Is(err, deliveryCompose.ErrNotificationsDisabled) || errors.Is(err, deliveryCompose.ErrOutsideNotificationWindow)
}
