package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

func toPublicStudentFieldEdits(values []domain.StudentFieldEdit) []peopledirectory.StudentFieldEdit {
	result := make([]peopledirectory.StudentFieldEdit, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentFieldEdit(value))
	}
	return result
}

func (e Engine) RecordStudentChanges(
	ctx context.Context,
	before, after peopledirectory.StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	return MapError(e.studentAudit.RecordChanges(
		ctx, domain.StudentAuditSnapshot(before), domain.StudentAuditSnapshot(after), editedBy, editedByName))
}

func (e Engine) RecordStudentPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	return MapError(e.studentAudit.RecordPickupPlan(ctx, studentID, before, after, result, reason, editedBy, editedByName))
}

func (e Engine) ListStudentChangeHistory(ctx context.Context, studentID int64) ([]peopledirectory.StudentFieldEdit, error) {
	values, err := e.studentAudit.ListChangeHistory(ctx, studentID)
	if err != nil {
		return nil, MapError(err)
	}
	return toPublicStudentFieldEdits(values), nil
}

// StudentFieldAuditTrail is the Audit Platform trail behind the per-child
// change history, in the public vocabulary the composition root binds.
type StudentFieldAuditTrail interface {
	Append(context.Context, []peopledirectory.StudentFieldEdit) error
	ListByStudent(context.Context, int64) ([]peopledirectory.StudentFieldEdit, error)
}

// NewStudentFieldAuditLog adapts the root's public-typed trail to the
// module's internal port.
func NewStudentFieldAuditLog(trail StudentFieldAuditTrail) ports.StudentFieldAuditLog {
	return studentFieldAuditLog{log: trail}
}

type studentFieldAuditLog struct{ log StudentFieldAuditTrail }

func (a studentFieldAuditLog) Append(ctx context.Context, edits []domain.StudentFieldEdit) error {
	return a.log.Append(ctx, toPublicStudentFieldEdits(edits))
}

func (a studentFieldAuditLog) ListByStudent(ctx context.Context, studentID int64) ([]domain.StudentFieldEdit, error) {
	values, err := a.log.ListByStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.StudentFieldEdit, 0, len(values))
	for _, value := range values {
		result = append(result, domain.StudentFieldEdit(value))
	}
	return result, nil
}
