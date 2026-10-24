package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The care lifecycle's directory reads. They join People Directory rows, so
// they run through the named student directory projection in the caller's
// transaction; Care Plan's own tables are not involved.

// ListEndedCare reads the archive's directory half: every child whose
// enrolment interval ran out before asOf.
func (s *DirectoryStore) ListEndedCare(ctx context.Context, asOf domain.Date, filter careplan.EndedCareFilter) ([]domain.EndedCareStudent, int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := []domain.EndedCareStudent{}
	total, err := s.directory.ListEndedCare(ctx, db, tenantID, asOf.String(), filter.Search, filter.SchoolClasses, filter.Page, filter.PageSize,
		func(studentID int64, firstName, lastName, schoolClass string, lastCareDay calendar.Date) {
			result = append(result, domain.EndedCareStudent{
				StudentID: studentID, FirstName: firstName, LastName: lastName, SchoolClass: schoolClass, LastCareDay: lastCareDay,
			})
		})
	if err != nil {
		return nil, 0, err
	}
	return result, total, nil
}

// LockCareExitPeople locks the person rows the binding preview quotes.
func (s *DirectoryStore) LockCareExitPeople(ctx context.Context, studentIDs []int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	return s.directory.LockCareExitPeople(ctx, db, tenantID, studentIDs)
}

// ListCareBookingStudents reads the given children, or every child in care
// on the day, for the booking evaluation.
func (s *DirectoryStore) ListCareBookingStudents(ctx context.Context, on domain.Date, studentIDs []int64) ([]domain.CareBookingStudent, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	result := []domain.CareBookingStudent{}
	err = s.directory.ListCareStudents(ctx, db, tenantID, on.String(), studentIDs, careplan.StudentStatusInactive, careplan.StudentStatusActive,
		func(studentID int64, firstName, lastName, schoolClass string, enrolledUntil *calendar.Date) {
			result = append(result, domain.CareBookingStudent{
				StudentID: studentID, FirstName: firstName, LastName: lastName, SchoolClass: schoolClass, EnrolledUntil: enrolledUntil,
			})
		})
	if err != nil {
		return nil, err
	}
	return result, nil
}
