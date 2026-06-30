package calendar

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

type AppointmentRepository interface {
	Create(ctx context.Context, appointment *Appointment) error
	FindByID(ctx context.Context, id int64) (*Appointment, error)
	Update(ctx context.Context, appointment *Appointment) error
	Delete(ctx context.Context, id any) error
	ListVisibleForStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
	ListVisibleForGuardianProfiles(ctx context.Context, guardianProfileIDs []int64, from, to timezone.Date) ([]*Appointment, error)
	ListOrganizedByStaff(ctx context.Context, staffID int64, from, to timezone.Date) ([]*Appointment, error)
}

type RecurrenceRuleRepository interface {
	Create(ctx context.Context, rule *RecurrenceRule) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) (*RecurrenceRule, error)
	FindByAppointmentIDs(ctx context.Context, appointmentIDs []int64) ([]*RecurrenceRule, error)
	DeleteByAppointmentID(ctx context.Context, appointmentID int64) error
}

type AppointmentRecipientRepository interface {
	CreateMany(ctx context.Context, recipients []*AppointmentRecipient) error
	ReplaceForAppointment(ctx context.Context, appointmentID int64, recipients []*AppointmentRecipient) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*AppointmentRecipient, error)
	UpdateResponse(ctx context.Context, recipientID int64, status string) error
}

type AppointmentRecipientStudentRepository interface {
	CreateMany(ctx context.Context, links []*AppointmentRecipientStudent) error
	FindByRecipientIDs(ctx context.Context, recipientIDs []int64) ([]*AppointmentRecipientStudent, error)
}

type AppointmentTargetRepository interface {
	ReplaceForAppointment(ctx context.Context, appointmentID int64, targets []*AppointmentTarget) error
	FindByAppointmentID(ctx context.Context, appointmentID int64) ([]*AppointmentTarget, error)
}

type AppointmentOccurrenceOverrideRepository interface {
	Create(ctx context.Context, override *AppointmentOccurrenceOverride) error
	Update(ctx context.Context, override *AppointmentOccurrenceOverride) error
	FindByAppointmentIDsInRange(ctx context.Context, appointmentIDs []int64, from, to timezone.Date) ([]*AppointmentOccurrenceOverride, error)
	Delete(ctx context.Context, id any) error
}
