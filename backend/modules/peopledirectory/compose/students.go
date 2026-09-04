package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (e engine) ListStudentsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByIDs(ctx, ids)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListStudentNamesByIDs(ctx context.Context, ids []int64) ([]peopledirectory.StudentName, error) {
	values, err := e.students.ListNamesByIDs(ctx, ids)
	result := make([]peopledirectory.StudentName, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentName(value))
	}
	return result, mapError(err)
}

func (e engine) ListStudentsAcrossTenantsByIDs(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	values, err := e.students.ListAcrossTenantsByIDs(ctx, ids)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListStudentsByClasses(ctx context.Context, classes []string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByClasses(ctx, classes)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListEnrolledStudents(ctx context.Context) ([]peopledirectory.Student, error) {
	values, err := e.students.ListEnrolled(ctx)
	return toPublicStudents(values), mapError(err)
}

func (e engine) ListSchoolClasses(ctx context.Context) ([]string, error) {
	values, err := e.students.ListClasses(ctx)
	return values, mapError(err)
}

func (e engine) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]peopledirectory.Student, error) {
	values, err := e.students.ListByStatusFlag(ctx, status)
	return toPublicStudents(values), mapError(err)
}

func (e engine) LockStudent(ctx context.Context, id int64) error {
	return mapError(e.students.Lock(ctx, id))
}

func (e engine) PromoteStudents(ctx context.Context, ids []int64, fromClass, toClass string) (int64, error) {
	affected, err := e.students.Promote(ctx, ids, fromClass, toClass)
	return affected, mapError(err)
}

func (e engine) RevertStudentClass(ctx context.Context, id int64, fromClass, toClass string) (int64, error) {
	affected, err := e.students.RevertClass(ctx, id, fromClass, toClass)
	return affected, mapError(err)
}

func (e engine) GraduateStudentsByClasses(ctx context.Context, classes []string) (int64, error) {
	affected, err := e.students.GraduateByClasses(ctx, classes)
	return affected, mapError(err)
}

func (e engine) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	affected, err := e.students.GraduateByIDs(ctx, ids)
	return affected, mapError(err)
}

func (e engine) ReactivateStudents(ctx context.Context, ids []int64, status string) ([]int64, error) {
	values, err := e.students.Reactivate(ctx, ids, status)
	return values, mapError(err)
}

func (e engine) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	affected, err := e.students.ClearStatusFlags(ctx, ids, status)
	return affected, mapError(err)
}

func toPublicStudents(values []domain.Student) []peopledirectory.Student {
	result := make([]peopledirectory.Student, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.Student{
			ID: value.ID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt, TenantID: value.TenantID,
			PersonID: value.PersonID, SchoolClass: value.SchoolClass, GroupID: value.GroupID, Status: value.Status,
			EnrolledFrom: value.EnrolledFrom, EnrolledUntil: value.EnrolledUntil,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
			PhotoPath: value.PhotoPath,
		})
	}
	return result
}
