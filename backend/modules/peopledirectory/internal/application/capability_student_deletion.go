package application

import (
	"context"
	"time"
)

func (s *Service) CountStudentGuardianLinks(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalidStudent("student ID is required")
	}

	count, err := s.students.CountGuardianLinks(ctx, studentID)
	return count, MapError(err)
}

func (s *Service) DeleteStudent(ctx context.Context, studentID int64) (int64, error) {
	if studentID <= 0 {
		return 0, invalidStudent("student ID is required")
	}

	rows, err := s.students.Delete(ctx, studentID)
	return rows, MapError(err)
}

func (s *Service) AnonymizeDeletedStudentPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error) {
	if personID <= 0 {
		return false, invalid("person ID is required")
	}
	if updatedAt.IsZero() {
		return false, invalid("previewed person revision is required")
	}

	anonymized, err := s.AnonymizeIfUnchanged(ctx, personID, updatedAt)
	return anonymized, MapError(err)
}
