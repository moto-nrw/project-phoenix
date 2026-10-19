package owners

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// StudentAuditCapability is the owner surface this seam translates to.
type StudentAuditCapability interface {
	peopleModule.StudentAuditQuery
	peopleModule.StudentAuditCommand
}

// StudentAudit adapts the People Directory change-history capability to the
// retained model-typed contract the legacy services still call
// (StudentAuditRecorder in modules/peopledirectory/compose, satisfied
// structurally).
type StudentAudit struct{ Capability StudentAuditCapability }

// NewStudentAuditFor binds an already composed owner capability, so the
// serve root keeps one observed People Directory.
func NewStudentAuditFor(capability StudentAuditCapability) *StudentAudit {
	return &StudentAudit{Capability: capability}
}

func (s *StudentAudit) RecordChanges(
	ctx context.Context,
	before, after *peopleModule.StudentState,
	editedBy int64,
	editedByName string,
) error {
	if before == nil || after == nil {
		return nil
	}
	return s.Capability.RecordStudentChanges(
		ctx, studentAuditSnapshot(before), studentAuditSnapshot(after), editedBy, editedByName)
}

func (s *StudentAudit) RecordPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	return s.Capability.RecordStudentPickupPlan(
		ctx, studentID, before, after, result, reason, editedBy, editedByName)
}

func (s *StudentAudit) RecordSystemStatusChange(
	ctx context.Context,
	studentID int64,
	before string,
	after string,
) error {
	return s.Capability.RecordStudentChanges(
		ctx,
		peopleModule.StudentAuditSnapshot{Status: string(before)},
		peopleModule.StudentAuditSnapshot{StudentID: studentID, Status: string(after)},
		peopleModule.StudentAuditSystemActorID,
		peopleModule.StudentAuditSystemActorName,
	)
}

func (s *StudentAudit) GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error) {
	edits, err := s.Capability.ListStudentChangeHistory(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]*auditModels.StudentFieldEdit, 0, len(edits))
	for _, edit := range edits {
		result = append(result, &auditModels.StudentFieldEdit{
			ID: edit.ID, StudentID: edit.StudentID, EditedBy: edit.EditedBy, EditedByName: edit.EditedByName,
			FieldName: edit.FieldName, OldValue: edit.OldValue, NewValue: edit.NewValue, CreatedAt: edit.CreatedAt,
		})
	}
	return result, nil
}

// studentAuditSnapshot projects the retained row onto the owner's tracked
// slice. CareEnd travels as a calendar date so the owner renders it.
func studentAuditSnapshot(student *peopleModule.StudentState) peopleModule.StudentAuditSnapshot {
	snapshot := peopleModule.StudentAuditSnapshot{
		StudentID:              student.ID,
		Status:                 string(student.Status),
		SupervisorNotes:        student.SupervisorNotes,
		ExtraInfo:              student.ExtraInfo,
		HealthInfo:             student.HealthInfo,
		PickupStatus:           student.PickupStatus,
		DepartureCompanionNote: student.DepartureCompanionNote,
		AllowedDepartureModes:  student.AllowedDepartureModes,
		DepartureDays:          student.DepartureDays,
	}
	if student.EnrolledUntil != nil {
		snapshot.CareEnd = student.EnrolledUntil.String()
	}
	return snapshot
}

// RecordStudentChanges exposes the already-composed native owner command to
// the actor-aware owner port, without a legacy-row reconstruction.
func (s *StudentAudit) RecordStudentChanges(ctx context.Context, before, after peopleModule.StudentAuditSnapshot, editedBy int64, editedByName string) error {
	return s.Capability.RecordStudentChanges(ctx, before, after, editedBy, editedByName)
}
