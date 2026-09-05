package calendar

import (
	"context"
	"errors"
	"log/slog"

	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/emailbranding"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	platformService "github.com/moto-nrw/project-phoenix/services/platform"
)

// ReminderEffects exposes notification adapters, not delivery orchestration.
// Nil channel or preference functions preserve deployments without that channel.
type ReminderEffects struct {
	Appointments appointments.Capability
	Audiences    func(context.Context, []int64) (map[int64]GuardianNotificationAudience, error)
	Email        func(context.Context, string, int64, map[string]any) (bool, error)
	Push         func(context.Context, int64, []int64, []int64, string) (bool, error)
	FilterEmail  func(context.Context, []int64) ([]int64, error)
	FilterPush   func(context.Context, []int64) ([]int64, error)
	Brand        func(context.Context, int64) (string, string, string)
	WhenText     func(*appointments.Appointment) string
	NoDelivery   func(error) bool
	ParentsURL   string
	Logger       *slog.Logger
}

func (s *service) ReminderEffects() ReminderEffects {
	result := ReminderEffects{
		Appointments: s.cfg.Appointments, Audiences: s.GuardianNotificationAudiences,
		ParentsURL: s.cfg.ParentsURL, Logger: s.logger(),
		Brand: func(ctx context.Context, tenantID int64) (string, string, string) {
			return s.resolveSchoolName(ctx, tenantID), s.resolveSchoolLogo(ctx, tenantID), emailbranding.MotoLogoURL(s.cfg.ParentsURL)
		},
		WhenText: func(value *appointments.Appointment) string { return appointmentWhenText(calendarAppointment(value)) },
		NoDelivery: func(err error) bool {
			return errors.Is(err, notifications.ErrNoWebPushSubscribers) || errors.Is(err, notifications.ErrDisabled) || errors.Is(err, notifications.ErrOutsideActiveWindow)
		},
	}
	if s.cfg.Outbox != nil {
		result.Email = func(ctx context.Context, key string, appointmentID int64, payload map[string]any) (bool, error) {
			row, err := s.cfg.Outbox.Enqueue(ctx, platformService.EnqueueRequest{
				Kind: platformModels.EmailKindAppointmentReminder, Payload: payload, IdempotencyKey: key,
				RelatedEntityType: platformModels.EmailRelatedTypeAppointment, RelatedEntityID: appointmentID,
			})
			if err != nil {
				return false, err
			}
			return row.ID != 0, nil
		}
	}
	if s.cfg.ReminderNotifier != nil {
		result.Push = s.dispatchGuardianAccountReminderDevicesLocalized
	}
	if s.cfg.Preferences != nil {
		result.FilterEmail = func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.cfg.Preferences.FilterNotOptedOut(ctx, notifications.TypeParentAppointmentReminder, ids)
		}
		result.FilterPush = func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.cfg.Preferences.FilterOptedIn(ctx, notifications.TypeParentAppointmentReminder, ids)
		}
	}
	return result
}
