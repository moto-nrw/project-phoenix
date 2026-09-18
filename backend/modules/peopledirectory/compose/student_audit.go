package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentFieldAuditLog is the Audit Platform seam behind the per-child change
// history. The composition root binds it: Audit Platform owns the trail and
// its retention, People Directory only decides which of its fields are
// tracked and how a change reads.
type StudentFieldAuditLog interface {
	Append(context.Context, []peopledirectory.StudentFieldEdit) error
	ListByStudent(context.Context, int64) ([]peopledirectory.StudentFieldEdit, error)
}

// studentFieldAuditLog adapts the composition root's public-typed seam to the
// module's internal port.
type studentFieldAuditLog struct{ log StudentFieldAuditLog }

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

func toPublicStudentFieldEdits(values []domain.StudentFieldEdit) []peopledirectory.StudentFieldEdit {
	result := make([]peopledirectory.StudentFieldEdit, 0, len(values))
	for _, value := range values {
		result = append(result, peopledirectory.StudentFieldEdit(value))
	}
	return result
}

func (e engine) RecordStudentChanges(
	ctx context.Context,
	before, after peopledirectory.StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	return mapError(e.studentAudit.RecordChanges(
		ctx, domain.StudentAuditSnapshot(before), domain.StudentAuditSnapshot(after), editedBy, editedByName))
}

func (e engine) RecordStudentPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	return mapError(e.studentAudit.RecordPickupPlan(ctx, studentID, before, after, result, reason, editedBy, editedByName))
}

func (e engine) ListStudentChangeHistory(ctx context.Context, studentID int64) ([]peopledirectory.StudentFieldEdit, error) {
	values, err := e.studentAudit.ListChangeHistory(ctx, studentID)
	if err != nil {
		return nil, mapError(err)
	}
	return toPublicStudentFieldEdits(values), nil
}
