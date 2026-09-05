package repositories

import (
	"context"
	"errors"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	educationRepo "github.com/moto-nrw/project-phoenix/database/repositories/education"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	scheduleRepo "github.com/moto-nrw/project-phoenix/database/repositories/schedule"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// bindStudentDirectories hands the People Directory to every legacy
// repository that used to read or write users.students through a foreign
// join (#2662). Each repository declares the narrow projection it needs;
// the adapters below map the owner's Student onto it. Binding happens on
// the raw repositories, before the person and school projections wrap them.
func (f *Factory) bindStudentDirectories(students peopledirectory.StudentQuery, commands peopledirectory.StudentCommand) {
	if repo, ok := f.CrossTenant.(*activeRepo.CrossTenantRepository); ok {
		repo.BindStudentDirectory(activeStudentDirectory{students: students, commands: commands})
	}
	if repo, ok := f.ActiveVisit.(*activeRepo.VisitRepository); ok {
		repo.BindStudentDirectory(activeStudentDirectory{students: students, commands: commands})
	}
	if repo, ok := f.RequestChildOffering.(*enrollmentRepo.RequestChildOfferingRepository); ok {
		repo.BindStudentDirectory(enrollmentStudentDirectory{students})
	}
	if repo, ok := f.PhaseExpiry.(*enrollmentRepo.PhaseExpiryRepository); ok {
		repo.BindStudentDirectory(enrollmentStudentDirectory{students})
	}
	if repo, ok := f.ParentChild.(*parentRepo.ChildRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{students})
	}
	if repo, ok := f.ParentEnrollablePhase.(*parentRepo.EnrollablePhaseRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{students})
	}
	if repo, ok := f.GradeTransition.(*educationRepo.GradeTransitionRepository); ok {
		repo.BindStudentDirectory(educationStudentDirectory{students: students, commands: commands})
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

type activeStudentDirectory struct {
	students peopledirectory.StudentQuery
	commands peopledirectory.StudentCommand
}

func (d activeStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]activeRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	return toActiveStudents(students), err
}

func (d activeStudentDirectory) ListStudentsAcrossTenantsByID(ctx context.Context, ids []int64) ([]activeRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsAcrossTenantsByID(ctx, ids)
	return toActiveStudents(students), err
}

func (d activeStudentDirectory) ListActiveStudents(ctx context.Context) ([]activeRepo.DirectoryStudent, error) {
	students, err := d.students.ListEnrolledStudents(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]activeRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		if student.Status == "active" {
			result = append(result, toActiveStudent(student))
		}
	}
	return result, nil
}

func (d activeStudentDirectory) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]activeRepo.DirectoryStudent, error) {
	flags, ok := d.students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return nil, errors.New("people directory: student status flag capability is not configured")
	}
	students, err := flags.ListStudentsWithStatusFlag(ctx, status)
	return toActiveStudents(students), err
}

func (d activeStudentDirectory) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	flags, ok := d.commands.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return 0, errors.New("people directory: student status flag capability is not configured")
	}
	return flags.ClearStudentStatusFlags(ctx, ids, status)
}

func (d activeStudentDirectory) LockStudent(ctx context.Context, studentID int64) error {
	return d.commands.LockStudent(ctx, studentID)
}

func toActiveStudents(students []peopledirectory.Student) []activeRepo.DirectoryStudent {
	result := make([]activeRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, toActiveStudent(student))
	}
	return result
}

func toActiveStudent(student peopledirectory.Student) activeRepo.DirectoryStudent {
	return activeRepo.DirectoryStudent{
		ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID,
		SchoolClass: student.SchoolClass, GroupID: student.GroupID, Status: student.Status,
		Sick: student.Sick, SickSince: student.SickSince, Excused: student.Excused, ExcusedSince: student.ExcusedSince,
		PhotoPath: student.PhotoPath,
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

type enrollmentStudentDirectory struct{ students peopledirectory.StudentQuery }

func (d enrollmentStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]enrollmentRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	return toEnrollmentStudents(students), err
}

func (d enrollmentStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]enrollmentRepo.DirectoryStudent, error) {
	students, err := d.students.ListEnrolledStudents(ctx)
	return toEnrollmentStudents(students), err
}

func toEnrollmentStudents(students []peopledirectory.Student) []enrollmentRepo.DirectoryStudent {
	result := make([]enrollmentRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, enrollmentRepo.DirectoryStudent{
			ID: student.ID, SchoolClass: student.SchoolClass, Status: student.Status, Alumnus: student.IsAlumnus(),
			EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		})
	}
	return result
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

type scheduleStudentDirectory struct {
	students peopledirectory.StudentQuery
	commands peopledirectory.StudentCommand
}

func (d scheduleStudentDirectory) LockStudent(ctx context.Context, studentID int64) error {
	err := d.commands.LockStudent(ctx, studentID)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return scheduleRepo.ErrStudentNotFound
	}
	return err
}

func (d scheduleStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]scheduleRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]scheduleRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, scheduleRepo.DirectoryStudent{ID: student.ID, Alumnus: student.IsAlumnus()})
	}
	return result, nil
}

type educationStudentDirectory struct {
	students peopledirectory.StudentQuery
	commands peopledirectory.StudentCommand
}

func (d educationStudentDirectory) ListStudentsByID(ctx context.Context, ids []int64) ([]educationRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByID(ctx, ids)
	return toEducationStudents(students), err
}

func (d educationStudentDirectory) ListStudentsByClasses(ctx context.Context, classes []string) ([]educationRepo.DirectoryStudent, error) {
	students, err := d.students.ListStudentsByClasses(ctx, classes)
	return toEducationStudents(students), err
}

func (d educationStudentDirectory) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return d.students.ListSchoolClasses(ctx)
}

func (d educationStudentDirectory) PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error) {
	return d.commands.PromoteStudents(ctx, ids, fromClass, toClass)
}

func (d educationStudentDirectory) RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error) {
	return d.commands.RevertStudentClass(ctx, id, fromClass, toClass)
}

func (d educationStudentDirectory) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	return d.commands.GraduateStudentsByClasses(ctx, classes)
}

func (d educationStudentDirectory) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	return d.commands.GraduateStudents(ctx, ids)
}

func (d educationStudentDirectory) ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error) {
	return d.commands.ReactivateStudents(ctx, ids, status)
}

func toEducationStudents(students []peopledirectory.Student) []educationRepo.DirectoryStudent {
	result := make([]educationRepo.DirectoryStudent, 0, len(students))
	for _, student := range students {
		result = append(result, educationRepo.DirectoryStudent{
			ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status,
		})
	}
	return result
}
