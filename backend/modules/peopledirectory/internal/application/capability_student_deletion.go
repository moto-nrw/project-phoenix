package application

import (
	"context"
	"time"
)

func (s *Service) CountStudentGuardianLinks(ctx context.Context, studentID, personID int64) (int, error) {
	if studentID <= 0 || personID <= 0 {
		return 0, invalidStudent("student and person IDs are required")
	}

	count, err := s.students.CountGuardianLinks(ctx, studentID, personID)
	return count, MapError(err)
}

func (s *Service) DeleteLegacyGuardianLinks(ctx context.Context, personID int64) (int64, error) {
	if personID <= 0 {
		return 0, invalid("person ID is required")
	}

	rows, err := s.students.DeleteLegacyGuardianLinks(ctx, personID)
	return rows, MapError(err)
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
