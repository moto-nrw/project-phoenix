package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/appointments/internal/ports"
)

type Service struct {
	store   ports.Store
	observe ports.Observer
}

func New(store ports.Store, observe ports.Observer) *Service {
	if store == nil || observe == nil {
		panic("appointments application: store and observer are required")
	}
	return &Service{store: store, observe: observe}
}

func (s *Service) FindAppointment(ctx context.Context, id int64, lock bool) (result domain.Appointment, err error) {
	operation := "find_appointment"
	if lock {
		operation = "lock_appointment"
	}
	err = s.run(operation, func(stats *domain.OperationStats) error {
		var found bool
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindAppointment(ctx, id, lock)
		stats.Add(queryStats)
		if err == nil && !found {
			return domain.ErrAppointmentNotFound
		}
		return err
	})
	return result, err
}

func (s *Service) FindReminderCandidateForUpdate(ctx context.Context, id int64) (result domain.Appointment, found bool, err error) {
	err = s.run("lock_reminder_candidate", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, found, queryStats, err = s.store.FindReminderCandidateForUpdate(ctx, id)
		stats.Add(queryStats)
		return err
	})
	return result, found, err
}

func (s *Service) FindReminderCandidatesForUpdate(ctx context.Context, ids []int64) (result []domain.Appointment, err error) {
	err = s.run("lock_reminder_candidates", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindReminderCandidatesForUpdate(ctx, ids)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListAppointmentsVisibleToStaff(ctx context.Context, staffID int64, from, to domain.Date) (result []domain.Appointment, err error) {
	err = s.run("list_staff_appointments", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListAppointmentsVisibleToStaff(ctx, staffID, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListStaffCancellationTombstones(ctx context.Context, staffID int64, since time.Time) (result []domain.Appointment, err error) {
	err = s.run("list_staff_cancellation_tombstones", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListStaffCancellationTombstones(ctx, staffID, since)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListAppointmentsVisibleToGuardians(ctx context.Context, guardianIDs, studentIDs []int64, from, to domain.Date) (result []domain.Appointment, err error) {
	err = s.run("list_guardian_appointments", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListAppointmentsVisibleToGuardians(ctx, guardianIDs, studentIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListGuardianCancellationTombstones(ctx context.Context, guardianIDs, studentIDs []int64, since time.Time) (result []domain.Appointment, err error) {
	err = s.run("list_guardian_cancellation_tombstones", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuardianCancellationTombstones(ctx, guardianIDs, studentIDs, since)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) ListGuardianReminderCandidates(ctx context.Context, from, to domain.Date) (result []domain.Appointment, err error) {
	err = s.run("list_guardian_reminder_candidates", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ListGuardianReminderCandidates(ctx, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) FindAppointmentTargets(ctx context.Context, appointmentID int64) (result []domain.AppointmentTarget, err error) {
	err = s.run("find_appointment_targets", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.FindAppointmentTargets(ctx, appointmentID)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) CreateAppointment(ctx context.Context, fields domain.AppointmentFields, targets []domain.AppointmentTargetFields) (result domain.Appointment, targetRows []domain.AppointmentTarget, err error) {
	err = s.run("create_appointment", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateAppointment(ctx, fields)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if len(targets) == 0 {
			return nil
		}
		targetRows, queryStats, err = s.store.InsertAppointmentTargets(ctx, result.ID, targets)
		stats.Add(queryStats)
		return err
	})
	return result, targetRows, err
}

func (s *Service) UpdateAppointment(ctx context.Context, id int64, fields domain.AppointmentFields) (result domain.Appointment, err error) {
	err = s.run("update_appointment", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.UpdateAppointment(ctx, id, fields)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *Service) DeleteAppointment(ctx context.Context, id int64) error {
	return s.run("delete_appointment", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.DeleteAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) BumpAppointmentRevision(ctx context.Context, id int64) error {
	return s.run("bump_appointment_revision", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.BumpAppointmentRevision(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) CancelAppointment(ctx context.Context, id int64) (transitioned bool, err error) {
	err = s.run("cancel_appointment", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		transitioned, queryStats, err = s.store.CancelAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
	return transitioned, err
}

func (s *Service) SoftDeleteAppointment(ctx context.Context, id int64) error {
	return s.run("soft_delete_appointment", func(stats *domain.OperationStats) error {
		queryStats, err := s.store.SoftDeleteAppointment(ctx, id)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) DeleteFeedTombstonesBefore(ctx context.Context, before time.Time) (rows int, err error) {
	err = s.run("delete_appointment_tombstones", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		rows, queryStats, err = s.store.DeleteFeedTombstonesBefore(ctx, before)
		stats.Add(queryStats)
		return err
	})
	return rows, err
}

func (s *Service) ReplaceAppointmentTargets(ctx context.Context, appointmentID int64, targets []domain.AppointmentTargetFields) (result []domain.AppointmentTarget, err error) {
	err = s.run("replace_appointment_targets", func(stats *domain.OperationStats) error {
		deleteStats, deleteErr := s.store.DeleteAppointmentTargets(ctx, appointmentID)
		stats.Add(deleteStats)
		if deleteErr != nil {
			return deleteErr
		}
		if len(targets) == 0 {
			return nil
		}
		var insertStats domain.OperationStats
		result, insertStats, err = s.store.InsertAppointmentTargets(ctx, appointmentID, targets)
		stats.Add(insertStats)
		return err
	})
	return result, err
}

func (s *Service) run(operation string, fn func(*domain.OperationStats) error) (err error) {
	started := time.Now()
	stats := domain.OperationStats{}
	defer func() {
		s.observe(ports.Observation{Operation: operation, Duration: time.Since(started), Stats: stats, Err: err})
	}()
	err = fn(&stats)
	return err
}
