package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
)

// Store persists the Appointments records in the public vocabulary. A find
// returns nil when no row matches.
type Store interface {
	FindAppointment(context.Context, int64, bool) (*appointments.Appointment, domain.OperationStats, error)
	FindReminderCandidateForUpdate(context.Context, int64) (*appointments.Appointment, domain.OperationStats, error)
	FindReminderCandidatesForUpdate(context.Context, []int64) ([]*appointments.Appointment, domain.OperationStats, error)
	ListAppointmentsVisibleToStaff(context.Context, int64, appointments.Date, appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error)
	ListStaffCancellationTombstones(context.Context, int64, time.Time) ([]*appointments.Appointment, domain.OperationStats, error)
	ListAppointmentsVisibleToGuardians(context.Context, []int64, []int64, appointments.Date, appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error)
	ListGuardianCancellationTombstones(context.Context, []int64, []int64, time.Time) ([]*appointments.Appointment, domain.OperationStats, error)
	ListGuardianReminderCandidates(context.Context, appointments.Date, appointments.Date) ([]*appointments.Appointment, domain.OperationStats, error)
	FindAppointmentTargets(context.Context, int64) ([]*appointments.AppointmentTarget, domain.OperationStats, error)
	FindRecurrenceRule(context.Context, int64) (*appointments.RecurrenceRule, domain.OperationStats, error)
	FindRecurrenceRules(context.Context, []int64) ([]*appointments.RecurrenceRule, domain.OperationStats, error)
	FindOccurrenceOverrides(context.Context, []int64, []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindOccurrenceOverridesByStartDates(context.Context, []int64, []appointments.Date) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindCancelledOccurrenceOverrides(context.Context, []int64) ([]*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindAppointmentRecipient(context.Context, int64) (*appointments.AppointmentRecipient, domain.OperationStats, error)
	FindAppointmentRecipients(context.Context, []int64) ([]*appointments.AppointmentRecipient, domain.OperationStats, error)
	FindAppointmentRecipientStudents(context.Context, []int64) ([]*appointments.AppointmentRecipientStudent, domain.OperationStats, error)
	CountAppointmentRecipientStudents(context.Context, int64) (int, domain.OperationStats, error)

	CreateAppointment(context.Context, appointments.AppointmentFields) (*appointments.Appointment, domain.OperationStats, error)
	InsertAppointmentTargets(context.Context, int64, []appointments.AppointmentTargetFields) ([]*appointments.AppointmentTarget, domain.OperationStats, error)
	UpdateAppointment(context.Context, int64, appointments.AppointmentFields) (*appointments.Appointment, domain.OperationStats, error)
	DeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	BumpAppointmentRevision(context.Context, int64) (domain.OperationStats, error)
	CancelAppointment(context.Context, int64) (bool, domain.OperationStats, error)
	SoftDeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, domain.OperationStats, error)
	DeleteAppointmentTargets(context.Context, int64) (domain.OperationStats, error)
	CreateRecurrenceRule(context.Context, appointments.RecurrenceRule) (*appointments.RecurrenceRule, domain.OperationStats, error)
	DeleteRecurrenceRule(context.Context, int64) (domain.OperationStats, error)
	CreateOccurrenceOverride(context.Context, appointments.AppointmentOccurrenceOverride) (*appointments.AppointmentOccurrenceOverride, domain.OperationStats, error)
	DeleteOccurrenceOverrides(context.Context, int64) (domain.OperationStats, error)
	CancelOccurrence(context.Context, int64, appointments.Date) (bool, domain.OperationStats, error)
	InsertAppointmentRecipients(context.Context, int64, []appointments.AppointmentRecipientFields) ([]*appointments.AppointmentRecipient, domain.OperationStats, error)
	InsertAppointmentRecipientStudents(context.Context, []appointments.AppointmentRecipientStudent) ([]*appointments.AppointmentRecipientStudent, domain.OperationStats, error)
	UpdateAppointmentRecipientResponse(context.Context, int64, string) (bool, domain.OperationStats, error)
	ClaimReminderPushDelivery(context.Context, int64, int, appointments.Date, int64) (bool, domain.OperationStats, error)
	ReleaseReminderPushDelivery(context.Context, int64, int, appointments.Date, int64) (bool, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
