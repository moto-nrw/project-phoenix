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

	CreateAppointment(context.Context, domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error)
	InsertAppointmentTargets(context.Context, int64, []domain.AppointmentTargetFields) ([]domain.AppointmentTarget, domain.OperationStats, error)
	UpdateAppointment(context.Context, int64, domain.AppointmentFields) (domain.Appointment, domain.OperationStats, error)
	DeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	BumpAppointmentRevision(context.Context, int64) (domain.OperationStats, error)
	CancelAppointment(context.Context, int64) (bool, domain.OperationStats, error)
	SoftDeleteAppointment(context.Context, int64) (domain.OperationStats, error)
	DeleteFeedTombstonesBefore(context.Context, time.Time) (int, domain.OperationStats, error)
	DeleteAppointmentTargets(context.Context, int64) (domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
