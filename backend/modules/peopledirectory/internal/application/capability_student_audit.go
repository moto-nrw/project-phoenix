package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

func (s *Service) ListStudentChangeHistory(ctx context.Context, studentID int64) ([]peopledirectory.StudentFieldEdit, error) {
	if studentID <= 0 {
		return nil, invalidStudent("student ID is required")
	}

	values, err := s.studentAudit.ListChangeHistory(ctx, studentID)
	if err != nil {
		return nil, MapError(err)
	}
	return toPublicStudentFieldEdits(values), nil
}

func (s *Service) RecordStudentChanges(
	ctx context.Context,
	before, after peopledirectory.StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	if after.StudentID <= 0 {
		return invalidStudent("student ID is required")
	}

	return MapError(s.studentAudit.RecordChanges(
		ctx, domain.StudentAuditSnapshot(before), domain.StudentAuditSnapshot(after), editedBy, editedByName))
}

func (s *Service) RecordStudentPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	if studentID <= 0 {
		return invalidStudent("student ID is required")
	}

	return MapError(s.studentAudit.RecordPickupPlan(ctx, studentID, before, after, result, reason, editedBy, editedByName))
}
