package owners

import (
	"context"
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// newWorkflowAuditCommand builds the fail-closed append command a workflow
// writes in its caller's tenant transaction; a failed append is logged at
// error level under message.
func NewWorkflowAuditCommand(db transactionruntime.Database, logger *slog.Logger, message string) (auditModels.Command, error) {
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
