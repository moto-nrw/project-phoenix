package api

import (
	"context"
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// The workflows append their audit records through the Audit owner's command;
// the root builds the records from the workflow values, so no workflow
// composition names the Audit owner's records (#4004, #4085).

// newWorkflowAuditCommand builds the fail-closed append command a workflow
// writes in its caller's tenant transaction; a failed append is logged at
// error level under message.
func newWorkflowAuditCommand(db transactionruntime.Database, logger *slog.Logger, message string) (auditModels.Command, error) {
	audits, err := auditCompose.NewTenantPersistence(db.Pool(), false)
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.Default()
	}
	command, err := auditCompose.NewCommand(audits.AppendStore, func(event auditCompose.AppendObservation) {
		if event.Err != nil {
			logger.Error(message,
				"event_type", event.EventType,
				"error", event.Err,
			)
		}
	})
	if err != nil {
		return nil, err
	}
	return command, nil
}

// GradeTransitionClassListRecord is the audit record of one class-list entry
// a grade transition rewrites.
func GradeTransitionClassListRecord(actor gradetransition.Actor, change gradetransition.ClassListEntryAudit) any {
	event := &auditModels.ClassListEntryChange{
		EntryID: change.EntryID, Action: change.Action, OldValue: change.OldValue, NewValue: change.NewValue,
		ChangedBy: actor.AccountID,
	}
	event.SetTenantID(actor.TenantID)
	return event
}

// The student deletion names the Audit Platform's deletion types itself; a
// drift must not compile: a false comparison repeats the false key of this
// map literal.
var _ = map[bool]struct{}{
	false: {},
	studentdeletion.DeletionTypeManual == auditModels.DeletionTypeManual &&
		studentdeletion.DeletionTypeRetention == auditModels.DeletionTypeStudentRetention: {},
}

// studentDeletionRecords are the deletion log entry and the tombstone of one
// deleted child, as the student deletion workflow attributes them.
func studentDeletionRecords(actor studentdeletion.Actor, result studentdeletion.Result) []any {
	deletion := studentdeletion.NewDeletionAudit(actor, result)
	dataDeletion := auditModels.NewDataDeletion(deletion.StudentID, deletion.DeletionType, deletion.RecordsDeleted, deletion.DeletedBy)
	dataDeletion.DeletionReason = deletion.Reason
	dataDeletion.SetTenantID(deletion.TenantID)
	deletion.EachMetadata(dataDeletion.SetMetadata)
	tombstone := &auditModels.StudentDeletion{StudentID: deletion.StudentID, ActorAccountID: deletion.ActorAccountID, Reason: deletion.Reason, Counts: auditModels.StudentDeletionCounts(deletion.Counts)}
	tombstone.SetTenantID(deletion.TenantID)
	return []any{dataDeletion, tombstone}
}

// staffAbsenceDeletionRecord is the time-tracking deletion record of one
// absence the offboarding removes, as the workflow builds it.
func staffAbsenceDeletionRecord(absence workforce.StaffAbsence, actorID int64) (any, error) {
	deletion, err := staffoffboarding.NewAbsenceDeletion(absence, actorID)
	if err != nil {
		return nil, err
	}
	return &auditModels.TimeTrackingDeletion{StaffID: deletion.StaffID, Source: auditModels.TimeTrackingDeletionSourceAbsence, SourceID: deletion.AbsenceID, DeletedBy: deletion.DeletedBy, Payload: deletion.Payload, Note: deletion.Note}, nil
}

// staffOffboardingRecord is the manual data deletion record of one
// offboarded staff member, as the workflow builds it.
func staffOffboardingRecord(actor staffoffboarding.Actor, result staffoffboarding.Result) any {
	deletion := staffoffboarding.NewStaffDeletion(actor, result)
	event := auditModels.NewStaffDataDeletion(deletion.StaffID, auditModels.DeletionTypeManual, deletion.RecordsDeleted, deletion.DeletedBy)
	event.DeletionReason = deletion.Reason
	event.SetTenantID(deletion.TenantID)
	for key, count := range deletion.Counts {
		event.Metadata[key] = count
	}
	return event
}

// deviationEventLog appends the Audit owner's deviation events.
type deviationEventLog interface {
	Create(context.Context, *auditModels.DeviationEvent) error
}

// NewShiftMoveAudit stores Workforce's shift_moved Änderungsprotokoll entry
// (#1884) as a shift-anchored audit deviation event; nil events skip it.
func NewShiftMoveAudit(events deviationEventLog) func(context.Context, workforce.ShiftMovedEvent) error {
	if events == nil {
		return nil
	}
	return func(ctx context.Context, moved workforce.ShiftMovedEvent) error {
		return events.Create(ctx, &auditModels.DeviationEvent{
			OccurrenceDate: auditModels.Date(moved.OccurrenceDate), StartTime: moved.StartTime,
			StaffShiftID: &moved.StaffShiftID, SubjectStaffID: &moved.SubjectStaffID, RelatedStaffID: moved.RelatedStaffID,
			ActorAccountID: moved.ActorAccountID, EventType: auditModels.DeviationEventShiftMoved,
			OldValue: moved.OldValue, NewValue: moved.NewValue,
		})
	}
}
