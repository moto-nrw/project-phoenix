package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) ListStudentsByID(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.Student{}, nil
	}

	values, err := s.students.ListByIDs(ctx, ids)
	return nonNilStudents(values), MapError(err)
}

func (s *Service) ListStudentNamesByID(ctx context.Context, ids []int64) ([]peopledirectory.StudentName, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.StudentName{}, nil
	}

	values, err := s.students.ListNamesByIDs(ctx, ids)
	result := append([]peopledirectory.StudentName{}, values...)
	return result, MapError(err)
}

func (s *Service) ListStudentsAcrossTenantsByID(ctx context.Context, ids []int64) ([]peopledirectory.Student, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return []peopledirectory.Student{}, nil
	}

	values, err := s.students.ListAcrossTenantsByIDs(ctx, ids)
	return nonNilStudents(values), MapError(err)
}

func (s *Service) ListStudentsByClasses(ctx context.Context, classes []string) ([]peopledirectory.Student, error) {
	classes = peopledirectory.SchoolClasses(classes)
	if len(classes) == 0 {
		return []peopledirectory.Student{}, nil
	}

	values, err := s.students.ListByClasses(ctx, classes)
	return nonNilStudents(values), MapError(err)
}

func (s *Service) ListStudentsByPersonID(ctx context.Context, personIDs []int64) ([]peopledirectory.Student, error) {
	personIDs = peopledirectory.PositiveIDs(personIDs)
	if len(personIDs) == 0 {
		return []peopledirectory.Student{}, nil
	}

	values, err := s.students.ListByPersonIDs(ctx, personIDs)
	return nonNilStudents(values), MapError(err)
}

func (s *Service) ListEnrolledStudents(ctx context.Context) ([]peopledirectory.Student, error) {
	values, err := s.students.ListEnrolled(ctx)
	return nonNilStudents(values), MapError(err)
}

func (s *Service) ListSchoolClasses(ctx context.Context) ([]string, error) {
	values, err := s.students.ListClasses(ctx)
	return values, MapError(err)
}

func (s *Service) LockStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return invalidStudent("student ID is required")
	}

	return MapError(s.students.Lock(ctx, id))
}

func (s *Service) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]peopledirectory.Student, error) {
	if status != "sick" && status != "excused" {
		return nil, invalidStudent("student status flag is invalid")
	}
	values, err := s.students.ListByStatusFlag(ctx, status)
	return nonNilStudents(values), MapError(err)
}

func invalidStudent(reason string) error { return &peopledirectory.InvalidStudentError{Reason: reason} }
