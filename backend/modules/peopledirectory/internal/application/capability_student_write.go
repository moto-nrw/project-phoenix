package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

func (s *Service) CreateStudent(ctx context.Context, write peopledirectory.StudentWrite) (peopledirectory.StudentRecord, error) {
	if write.Record.PersonID <= 0 {
		return peopledirectory.StudentRecord{}, invalidStudent("person ID is required")
	}

	record, err := s.students.CreateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (s *Service) UpdateStudent(ctx context.Context, write peopledirectory.StudentWrite) (peopledirectory.StudentRecord, error) {
	if write.Record.ID <= 0 {
		return peopledirectory.StudentRecord{}, invalidStudent("student ID is required")
	}

	record, err := s.students.UpdateStudent(ctx, toApplicationStudentWrite(write))
	return peopledirectory.StudentRecord(record), MapError(err)
}

func (s *Service) DeleteStudentRecord(ctx context.Context, studentID int64) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}

	return MapError(s.students.DeleteStudent(ctx, studentID))
}

func (s *Service) VerifyStudentStrandingBatch(ctx context.Context) error {
	return MapError(s.students.VerifyStrandingBatch(ctx))
}

func (s *Service) ListStudentCareEnds(ctx context.Context, ids []int64) (map[int64]string, error) {
	ids = peopledirectory.PositiveIDs(ids)
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}

	bounds, err := s.students.ListStudentCareEnds(ctx, ids)
	return bounds, MapError(err)
}
