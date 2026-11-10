package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// The enrollment batch of the data import (#5552, ADR 0067 addendum of
// 2026-11-08, step 7): CreateEnrollment split into its three owner steps,
// each for many children, in the caller's transaction and without a
// savepoint of its own.

// EnrollmentCareEntry is one care profile of a child the batch created.
type EnrollmentCareEntry struct {
	MembershipID int64
	Student      peopledirectory.Student
	Input        domain.EnrollmentStudent
}

// requireTransaction runs fn in the caller's transaction and never opens one.
func (s *StudentService) requireTransaction(ctx context.Context, fn func(context.Context) error) error {
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return err
	}
	return fn(ctx)
}

// CreateEnrollments inserts the profiles of the inputs in one statement.
func (s *StudentService) CreateEnrollments(ctx context.Context, inputs []domain.EnrollmentStudent) (result []peopledirectory.Student, err error) {
	err = s.run(ctx, "create_enrollment_profiles", s.requireTransaction, func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.CreateEnrollments(txCtx, inputs)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

// enrollmentRecord is the record CreateEnrollment hands its owners for a
// created child: the created row with the input's status and the initial
// profile applied.
func enrollmentRecord(student peopledirectory.Student, input domain.EnrollmentStudent) domain.StudentRecord {
	status := input.Status
	if status == "" {
		status = "active"
	}
	record := domain.StudentRecord{ID: student.ID, PersonID: student.PersonID, SchoolClass: input.SchoolClass, Status: status, EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil}
	if input.InitialProfile != nil {
		domain.ApplyEnrollmentPatch(&record, *input.InitialProfile)
	}
	return record
}

// EnrollCreated writes the membership of a created child as CreateEnrollment
// does and returns it.
func (s *StudentService) EnrollCreated(ctx context.Context, student peopledirectory.Student, input domain.EnrollmentStudent) (membershipID int64, err error) {
	err = s.run(ctx, "enroll_created_student", s.requireTransaction, func(txCtx context.Context, _ *domain.OperationStats) error {
		membershipID, err = s.owners.Enroll(txCtx, enrollmentRecord(student, input))
		return err
	})
	return membershipID, err
}

// SaveEnrollmentCares writes the care profiles of created children in one
// Care Plan statement, each with the values CreateEnrollment saves.
func (s *StudentService) SaveEnrollmentCares(ctx context.Context, entries []EnrollmentCareEntry) error {
	saves := make([]domain.StudentCareSave, len(entries))
	for i, entry := range entries {
		record := enrollmentRecord(entry.Student, entry.Input)
		saves[i] = domain.StudentCareSave{
			MembershipID: entry.MembershipID, Record: record, Note: record.DepartureCompanionNote,
			Plan:    domain.DeparturePlan{AllowedDepartureModes: record.AllowedDepartureModes, DepartureDays: record.DepartureDays, BusDays: record.BusDays, PickupDays: record.PickupDays},
			Touched: entry.Input.InitialProfile != nil && entry.Input.InitialProfile.DepartureSet,
		}
	}
	return s.run(ctx, "save_enrollment_cares", s.requireTransaction, func(txCtx context.Context, _ *domain.OperationStats) error {
		return s.owners.SaveCareBatch(txCtx, saves)
	})
}
