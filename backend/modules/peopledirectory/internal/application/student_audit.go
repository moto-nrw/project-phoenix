package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentAuditService records and reads the per-child change history (#1455):
// who changed which student profile field, from what to what, and when.
//
// It is a separate service because the trail lives in the Audit Platform: this
// owner decides which of its fields are tracked and how a change reads, and
// appends through a consumer-owned port.
type StudentAuditService struct {
	log     ports.StudentFieldAuditLog
	observe ports.Observer
}

func NewStudentAudit(log ports.StudentFieldAuditLog, observe ports.Observer) *StudentAuditService {
	if observe == nil {
		panic("people directory application: student audit observer is required")
	}
	return &StudentAuditService{log: log, observe: observe}
}

// RecordChanges diffs the snapshots and appends one row per changed tracked
// field. A no-op when nothing tracked changed, so an untouched child never
// grows a history row.
func (s *StudentAuditService) RecordChanges(
	ctx context.Context,
	before, after domain.StudentAuditSnapshot,
	editedBy int64,
	editedByName string,
) error {
	return s.run(ctx, "record_student_changes", func() error {
		if after.StudentID <= 0 {
			return domain.ErrStudentNotFound
		}
		changes := domain.DiffStudentFields(before, after)
		if len(changes) == 0 {
			return nil
		}
		if s.log == nil {
			return ports.ErrStudentAuditUnavailable
		}
		name := strings.TrimSpace(editedByName)
		if name == "" {
			name = unknownEditorName
		}
		edits := make([]domain.StudentFieldEdit, 0, len(changes))
		for _, change := range changes {
			oldValue, newValue := change.OldValue, change.NewValue
			edits = append(edits, domain.StudentFieldEdit{
				StudentID:    after.StudentID,
				EditedBy:     editedBy,
				EditedByName: name,
				FieldName:    change.FieldName,
				OldValue:     &oldValue,
				NewValue:     &newValue,
			})
		}
		return s.log.Append(ctx, edits)
	})
}

// RecordPickupPlan appends the permanent weekly pickup-plan change as one row.
// result and reason are folded into the new value, mirroring how the change
// history renders a plan adjustment.
func (s *StudentAuditService) RecordPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	return s.run(ctx, "record_student_pickup_plan", func() error {
		if studentID <= 0 {
			return domain.ErrStudentNotFound
		}
		if s.log == nil {
			return ports.ErrStudentAuditUnavailable
		}
		name := strings.TrimSpace(editedByName)
		if name == "" {
			name = unknownEditorName
		}
		newValue := strings.TrimSpace(result) + " · " + strings.TrimSpace(after)
		if reason = strings.TrimSpace(reason); reason != "" {
			newValue += " · Grund: " + reason
		}
		oldValue := strings.TrimSpace(before)
		return s.log.Append(ctx, []domain.StudentFieldEdit{{
			StudentID:    studentID,
			EditedBy:     editedBy,
			EditedByName: name,
			FieldName:    domain.StudentFieldPickupSchedule,
			OldValue:     &oldValue,
			NewValue:     &newValue,
		}})
	})
}

// ListChangeHistory returns the child's recorded changes, newest first.
func (s *StudentAuditService) ListChangeHistory(ctx context.Context, studentID int64) (result []domain.StudentFieldEdit, err error) {
	err = s.run(ctx, "list_student_change_history", func() error {
		if s.log == nil {
			return ports.ErrStudentAuditUnavailable
		}
		result, err = s.log.ListByStudent(ctx, studentID)
		return err
	})
	return result, err
}

// unknownEditorName is the stored stand-in when the caller cannot name the
// editor. It is a recorded value, so it stays German and stable.
const unknownEditorName = "Unbekannt"

func (s *StudentAuditService) run(ctx context.Context, operation string, fn func() error) error {
	return observeRun(ctx, s.observe, operation, passthroughRun, func(context.Context, *domain.OperationStats) error {
		return fn()
	})
}

// passthroughRun keeps the audit seam in the caller's transaction: the append
// must commit or roll back with the write it describes.
func passthroughRun(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
