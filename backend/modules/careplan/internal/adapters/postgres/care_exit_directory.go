package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"
)

// The care lifecycle's directory reads. They join People Directory rows, so
// they run through the named student directory projection in the caller's
// transaction; Care Plan's own tables are not involved.

// ListEndedCare reads the archive's directory half: every child whose
// enrolment interval ran out before asOf.
func (s *Store) ListEndedCare(ctx context.Context, asOf domain.Date, filter careplan.EndedCareFilter) ([]domain.EndedCareStudent, int, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, total, err := studentdirectoryview.ListEndedCare(ctx, db, tenantID, asOf.String(), studentdirectoryview.EndedCareFilter{
		Search: filter.Search, SchoolClasses: filter.SchoolClasses, Page: filter.Page, PageSize: filter.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	result := make([]domain.EndedCareStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.EndedCareStudent{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, LastCareDay: row.LastCareDay,
		})
	}
	return result, total, nil
}

// LockCareExitPeople locks the person rows the binding preview quotes.
func (s *Store) LockCareExitPeople(ctx context.Context, studentIDs []int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	return studentdirectoryview.LockCareExitPeople(ctx, db, tenantID, studentIDs)
}

// ListCareBookingStudents reads the given children, or every child in care
// on the day, for the booking evaluation.
func (s *Store) ListCareBookingStudents(ctx context.Context, on domain.Date, studentIDs []int64) ([]domain.CareBookingStudent, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := studentdirectoryview.ListCareStudents(ctx, db, tenantID, on.String(), studentIDs, studentdirectoryview.CareStudentStatuses{
		Inactive: careplan.StudentStatusInactive, Active: careplan.StudentStatusActive,
	})
	if err != nil {
		return nil, err
	}
	result := make([]domain.CareBookingStudent, 0, len(rows))
	for _, row := range rows {
		result = append(result, domain.CareBookingStudent{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			SchoolClass: row.SchoolClass, EnrolledUntil: row.EnrolledUntil,
		})
	}
	return result, nil
}
