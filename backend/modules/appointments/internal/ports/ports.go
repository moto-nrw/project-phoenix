package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
)

type Store interface {
	FindAppointment(context.Context, int64, bool) (domain.Appointment, bool, domain.OperationStats, error)
	FindReminderCandidateForUpdate(context.Context, int64) (domain.Appointment, bool, domain.OperationStats, error)
	FindReminderCandidatesForUpdate(context.Context, []int64) ([]domain.Appointment, domain.OperationStats, error)
	ListAppointmentsVisibleToStaff(context.Context, int64, domain.Date, domain.Date) ([]domain.Appointment, domain.OperationStats, error)
	ListStaffCancellationTombstones(context.Context, int64, time.Time) ([]domain.Appointment, domain.OperationStats, error)
	ListAppointmentsVisibleToGuardians(context.Context, []int64, []int64, domain.Date, domain.Date) ([]domain.Appointment, domain.OperationStats, error)
	ListGuardianCancellationTombstones(context.Context, []int64, []int64, time.Time) ([]domain.Appointment, domain.OperationStats, error)
	ListGuardianReminderCandidates(context.Context, domain.Date, domain.Date) ([]domain.Appointment, domain.OperationStats, error)
	FindAppointmentTargets(context.Context, int64) ([]domain.AppointmentTarget, domain.OperationStats, error)
	FindRecurrenceRule(context.Context, int64) (domain.RecurrenceRule, bool, domain.OperationStats, error)
	FindRecurrenceRules(context.Context, []int64) ([]domain.RecurrenceRule, domain.OperationStats, error)
	FindOccurrenceOverrides(context.Context, []int64, []domain.Date) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindOccurrenceOverridesByStartDates(context.Context, []int64, []domain.Date) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindCancelledOccurrenceOverrides(context.Context, []int64) ([]domain.AppointmentOccurrenceOverride, domain.OperationStats, error)
	FindAppointmentRecipient(context.Context, int64) (domain.AppointmentRecipient, bool, domain.OperationStats, error)
	FindAppointmentRecipients(context.Context, []int64) ([]domain.AppointmentRecipient, domain.OperationStats, error)
	FindAppointmentRecipientStudents(context.Context, []int64) ([]domain.AppointmentRecipientStudent, domain.OperationStats, error)
	CountAppointmentRecipientStudents(context.Context, int64) (int, domain.OperationStats, error)

	CreateAppointment(context.Context, domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error)
	InsertAppointmentTargets(context.Context, int64, []domain.AppointmentTargetFields) ([]domain.AppointmentTarget, domain.OperationStats, error)
	UpdateAppointment(context.Context, int64, domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error)
	DeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	BumpAppointmentRevision(context.Context, int64) (domain.OperationStats, error)
	CancelAppointment(context.Context, int64) (bool, domain.OperationStats, error)
	SoftDeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, domain.OperationStats, error)
	DeleteAppointmentTargets(context.Context, int64) (domain.OperationStats, error)
	CreateRecurrenceRule(context.Context, domain.RecurrenceRule) (domain.RecurrenceRule, domain.OperationStats, error)
	DeleteRecurrenceRule(context.Context, int64) (domain.OperationStats, error)
	CreateOccurrenceOverride(context.Context, domain.AppointmentOccurrenceOverride) (domain.AppointmentOccurrenceOverride, domain.OperationStats, error)
	DeleteOccurrenceOverrides(context.Context, int64) (domain.OperationStats, error)
	CancelOccurrence(context.Context, int64, domain.Date) (bool, domain.OperationStats, error)
	InsertAppointmentRecipients(context.Context, int64, []domain.AppointmentRecipientFields) ([]domain.AppointmentRecipient, domain.OperationStats, error)
	InsertAppointmentRecipientStudents(context.Context, []domain.AppointmentRecipientStudent) ([]domain.AppointmentRecipientStudent, domain.OperationStats, error)
	UpdateAppointmentRecipientResponse(context.Context, int64, string) (bool, domain.OperationStats, error)
	ClaimReminderPushDelivery(context.Context, int64, int, domain.Date, int64) (bool, domain.OperationStats, error)
	ReleaseReminderPushDelivery(context.Context, int64, int, domain.Date, int64) (bool, domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
