package repositories

import (
	"context"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// bindStudentDirectories hands the People Directory to every legacy
// repository that used to read or write users.students through a foreign
// join (#2662). Each repository declares the narrow projection it needs;
// the adapters below map the owner's Student onto it. Binding happens on
// the raw repositories, before the person and school projections wrap them.
func (f *Factory) bindStudentDirectories(students peopledirectory.StudentQuery, commands peopledirectory.StudentCommand) {
	if repo, ok := f.CrossTenant.(*visitorProjection); ok {
		repo.students = students
	}

	if repo, ok := f.ParentChild.(*parentRepo.ChildRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{students})
	}
	if repo, ok := f.ParentEnrollablePhase.(*parentRepo.EnrollablePhaseRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{students})
	}
	f.bindAuditStudentDirectory()
}

// bindAuditStudentDirectory binds the booking-consistency audit, which
// ConfigureAuditRuntime rebuilds after the directory was bound.
func (f *Factory) bindAuditStudentDirectory() {
	if f.students == nil {
		return
	}
	if repo, ok := f.BookingConsistency.(interface {
		BindStudentDirectory(auditRepo.StudentDirectory)
	}); ok {
		repo.BindStudentDirectory(auditStudentDirectory{f.students})
	}
}

type auditStudentDirectory struct{ students peopledirectory.StudentQuery }

func (d auditStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]auditRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]auditRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, auditRepo.DirectoryStudent{ID: student.ID, Alumnus: student.IsAlumnus()})
	}
	return result, nil
}

type parentStudentDirectory struct{ students peopledirectory.StudentQuery }

func (d parentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]parentRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]parentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, parentRepo.DirectoryStudent{
			ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID,
			SchoolClass: student.SchoolClass, Status: student.Status,
			EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		})
	}
	return result, nil
}
