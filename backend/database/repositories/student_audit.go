package repositories

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

// This file is the translation seam between the retained users.Student
// snapshots the legacy services still pass around and the People Directory
// owner capability that decides and records the per-child change history
// (#1455, moved in #3349). It holds no rules of its own: the tracked fields,
// their German values and the append live with the owner and the Audit
// Platform.

// StudentAuditCapability is the owner surface this seam translates to.
type StudentAuditCapability interface {
	peopleModule.StudentAuditQuery
	peopleModule.StudentAuditCommand
}

// StudentAudit adapts the People Directory change-history capability to the
// retained model-typed contract the legacy services still call
// (services/users.StudentAuditService, satisfied structurally so this seam
// does not depend on them).
type StudentAudit struct{ capability StudentAuditCapability }

// NewStudentAuditFor binds an already composed owner capability, so the
// serve root keeps one observed People Directory.
func NewStudentAuditFor(capability StudentAuditCapability) *StudentAudit {
	return &StudentAudit{capability: capability}
}

// NewStudentAudit composes an unobserved owner for test graphs and CLI roots.
func NewStudentAudit(db *bun.DB) *StudentAudit {
	return NewStudentAuditFor(MustNewPeopleDirectory(db))
}

func (s *StudentAudit) RecordChanges(
	ctx context.Context,
	before, after *userModels.Student,
	editedBy int64,
	editedByName string,
) error {
	if before == nil || after == nil {
		return nil
	}
	return s.capability.RecordStudentChanges(
		ctx, studentAuditSnapshot(before), studentAuditSnapshot(after), editedBy, editedByName)
}

func (s *StudentAudit) RecordPickupPlan(
	ctx context.Context,
	studentID int64,
	before, after, result, reason string,
	editedBy int64,
	editedByName string,
) error {
	return s.capability.RecordStudentPickupPlan(
		ctx, studentID, before, after, result, reason, editedBy, editedByName)
}

func (s *StudentAudit) RecordSystemStatusChange(
	ctx context.Context,
	studentID int64,
	before userModels.StudentStatus,
	after userModels.StudentStatus,
) error {
	return s.capability.RecordStudentChanges(
		ctx,
		peopleModule.StudentAuditSnapshot{Status: string(before)},
		peopleModule.StudentAuditSnapshot{StudentID: studentID, Status: string(after)},
		peopleModule.StudentAuditSystemActorID,
		peopleModule.StudentAuditSystemActorName,
	)
}

func (s *StudentAudit) GetChangeHistory(ctx context.Context, studentID int64) ([]*auditModels.StudentFieldEdit, error) {
	edits, err := s.capability.ListStudentChangeHistory(ctx, studentID)
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
func studentAuditSnapshot(student *userModels.Student) peopleModule.StudentAuditSnapshot {
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
