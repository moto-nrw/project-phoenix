package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *StudentService) ReadEnrollment(ctx context.Context, id int64, lock string) (result domain.EnrollmentRecord, err error) {
	run := s.tx.RunRead
	if lock != "" {
		run = s.tx.RunWrite
	}
	err = s.run(ctx, "read_enrollment_student", run, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.ReadEnrollment(txCtx, id, lock)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (s *StudentService) LockEnrollmentClassWrites(ctx context.Context) error {
	return s.run(ctx, "lock_enrollment_class_writes", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(queryStats)
		return err
	})
}

func (s *StudentService) ApplyEnrollmentProfile(ctx context.Context, id int64, input domain.EnrollmentProfilePatch) error {
	return s.run(ctx, "apply_enrollment_profile", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		record, found, readStats, err := s.store.FindRecord(txCtx, id, "UPDATE")
		stats.Add(readStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStudentNotFound
		}
		domain.ApplyEnrollmentPatch(&record, input)
		_, updated, writeStats, err := s.store.UpdateRecord(txCtx, record)
		stats.Add(writeStats)
		if err != nil {
			return err
		}
		if !updated {
			return domain.ErrStudentNotFound
		}
		membershipID, err := s.owners.Renew(txCtx, record)
		if err != nil {
			return err
		}
		plan := domain.DeparturePlan{AllowedDepartureModes: record.AllowedDepartureModes, DepartureDays: record.DepartureDays, BusDays: record.BusDays, PickupDays: record.PickupDays}
		return s.owners.SaveCare(txCtx, membershipID, record, plan, record.DepartureCompanionNote, input.DepartureSet)
	})
}

func (s *StudentService) CreateEnrollment(ctx context.Context, input domain.EnrollmentStudent) (result domain.Student, err error) {
	err = s.run(ctx, "create_enrollment_student", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateEnrollment(txCtx, input)
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		record := domain.StudentRecord{ID: result.ID, PersonID: result.PersonID, SchoolClass: result.SchoolClass, Status: result.Status, EnrolledFrom: result.EnrolledFrom, EnrolledUntil: result.EnrolledUntil}
		if input.InitialProfile != nil {
			domain.ApplyEnrollmentPatch(&record, *input.InitialProfile)
		}
		membershipID, err := s.owners.Enroll(txCtx, record)
		if err != nil {
			return err
		}
		plan := domain.DeparturePlan{AllowedDepartureModes: record.AllowedDepartureModes, DepartureDays: record.DepartureDays, BusDays: record.BusDays, PickupDays: record.PickupDays}
		return s.owners.SaveCare(txCtx, membershipID, record, plan, record.DepartureCompanionNote, input.InitialProfile != nil && input.InitialProfile.DepartureSet)
	})
	return result, err
}

func (s *StudentService) RenewEnrollment(ctx context.Context, id int64, input domain.EnrollmentStudent) error {
	return s.run(ctx, "renew_enrollment_student", s.runStudentWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		gateStats, err := s.store.LockEnrollmentClassWrites(txCtx)
		stats.Add(gateStats)
		if err != nil {
			return err
		}
		record, found, queryStats, err := s.store.FindRecord(txCtx, id, "UPDATE")
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrStudentNotFound
		}
		record.SchoolClass, record.Status = input.SchoolClass, input.Status
		record.EnrolledFrom, record.EnrolledUntil = input.EnrolledFrom, input.EnrolledUntil
		_, err = s.owners.Renew(txCtx, record)
		return err
	})
}
