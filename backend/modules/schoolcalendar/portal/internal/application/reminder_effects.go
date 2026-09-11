package application

import (
	"context"
	"log/slog"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/internal/ports"
)

// ReminderEffects exposes notification adapters, not delivery orchestration.
// Nil channel or preference functions preserve deployments without that channel.
type ReminderEffects struct {
	Appointments appointmentcap.Capability
	Audiences    func(context.Context, []int64) (map[int64]GuardianNotificationAudience, error)
	Email        func(context.Context, string, int64, map[string]any) (bool, error)
	Push         func(context.Context, int64, []int64, []int64, string) (bool, error)
	FilterEmail  func(context.Context, []int64) ([]int64, error)
	FilterPush   func(context.Context, []int64) ([]int64, error)
	Brand        func(context.Context, int64) (string, string, string)
	WhenText     func(*appointmentcap.Appointment) string
	NoDelivery   func(error) bool
	ParentsURL   string
	Logger       *slog.Logger
}

func (s *service) ReminderEffects() ReminderEffects {
	result := ReminderEffects{
		Appointments: s.cfg.Appointments, Audiences: s.GuardianNotificationAudiences,
		ParentsURL: s.cfg.ParentsURL, Logger: s.logger(),
		Brand: func(ctx context.Context, tenantID int64) (string, string, string) {
			return s.resolveSchoolName(ctx, tenantID), s.resolveSchoolLogo(ctx, tenantID), s.cfg.MotoLogoURL(s.cfg.ParentsURL)
		},
		WhenText: func(value *appointmentcap.Appointment) string { return appointmentWhenText(value) },
		NoDelivery: func(err error) bool {
			return s.cfg.NoDelivery(err)
		},
	}
	if s.cfg.Outbox != nil {
		result.Email = func(ctx context.Context, key string, appointmentID int64, payload map[string]any) (bool, error) {
			row, err := s.cfg.Outbox.Enqueue(ctx, ports.EnqueueRequest{
				Kind: ports.EmailKindAppointmentReminder, Payload: payload, IdempotencyKey: key,
				RelatedEntityType: ports.EmailRelatedTypeAppointment, RelatedEntityID: appointmentID,
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
			return s.cfg.Preferences.FilterNotOptedOut(ctx, "parent_appointment_reminder", ids)
		}
		result.FilterPush = func(ctx context.Context, ids []int64) ([]int64, error) {
			return s.cfg.Preferences.FilterOptedIn(ctx, "parent_appointment_reminder", ids)
		}
	}
	return result
}
