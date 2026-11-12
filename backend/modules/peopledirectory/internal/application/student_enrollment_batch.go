package application

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
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

// The batch steps carry no owner observation: the data import observes them
// (create_enrollment_profiles, enroll_created_student,
// save_enrollment_cares), as it does CreatePersons; the owner observer is a
// callable the command-effects inventory cannot follow (ADR 0083, 0098).

// CreateEnrollments takes the shared class-writes gate, as the single
// profile insert does before it locks its person, and inserts the profiles
// of the inputs in one statement.
func (s *StudentService) CreateEnrollments(ctx context.Context, inputs []domain.EnrollmentStudent) ([]peopledirectory.Student, error) {
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	if _, err := s.store.LockEnrollmentClassWrites(ctx); err != nil {
		return nil, err
	}
	result, _, err := s.store.CreateEnrollments(ctx, inputs)
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

// enrollCreated writes the membership of a created child as CreateEnrollment
// does and returns it; the capability requires the caller's transaction.
func (s *StudentService) enrollCreated(ctx context.Context, student peopledirectory.Student, input domain.EnrollmentStudent) (int64, error) {
	return s.owners.Enroll(ctx, enrollmentRecord(student, input))
}

// SaveEnrollmentCares writes the care profiles of created children in one
// Care Plan statement, each with the values CreateEnrollment saves.
func (s *StudentService) SaveEnrollmentCares(ctx context.Context, entries []EnrollmentCareEntry) error {
	saves := make([]peopledirectory.StudentCareSave, len(entries))
	for i, entry := range entries {
		record := enrollmentRecord(entry.Student, entry.Input)
		saves[i] = peopledirectory.StudentCareSave{
			MembershipID: entry.MembershipID, Record: peopledirectory.StudentRecord(record), Note: record.DepartureCompanionNote,
			Plan:    peopledirectory.StudentPlan{AllowedDepartureModes: record.AllowedDepartureModes, DepartureDays: record.DepartureDays, BusDays: record.BusDays, PickupDays: record.PickupDays},
			Touched: entry.Input.InitialProfile != nil && entry.Input.InitialProfile.DepartureSet,
		}
	}
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return err
	}
	return s.owners.SaveCareBatch(ctx, saves)
}
