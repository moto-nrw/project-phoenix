package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"strconv"

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
			logger.Error(message, "event_type", event.EventType, "error", event.Err)
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

// studentDeletionRecords are the deletion log entry and the tombstone of one
// deleted child. Actor account 0 is the nightly retention run (#3962): no
// actor, written by system.
func studentDeletionRecords(actor studentdeletion.Actor, result studentdeletion.Result) []any {
	deletionType, deletedBy, actorID := auditModels.DeletionTypeManual, "account:"+strconv.FormatInt(actor.AccountID, 10), &actor.AccountID
	if actor.AccountID == 0 {
		deletionType, deletedBy, actorID = auditModels.DeletionTypeStudentRetention, "system", nil
	}
	dataDeletion := auditModels.NewDataDeletion(result.StudentID, deletionType, result.Counts.Total()+result.PrimaryRowsDeleted, deletedBy)
	dataDeletion.DeletionReason = result.Reason
	dataDeletion.SetTenantID(actor.TenantID)
	dataDeletion.SetMetadata("student_deletion", true)
	dataDeletion.SetMetadata("counts", result.Counts)
	tombstone := &auditModels.StudentDeletion{StudentID: result.StudentID, ActorAccountID: actorID, Reason: result.Reason, Counts: auditModels.StudentDeletionCounts(result.Counts)}
	tombstone.SetTenantID(actor.TenantID)
	return []any{dataDeletion, tombstone}
}

// staffAbsenceDeletionRecord is the time-tracking deletion record of one
// absence the offboarding removes.
func staffAbsenceDeletionRecord(absence workforce.StaffAbsence, actorID int64) (any, error) {
	payload, err := json.Marshal(absence)
	if err != nil {
		return nil, err
	}
	return &auditModels.TimeTrackingDeletion{StaffID: absence.StaffID, Source: auditModels.TimeTrackingDeletionSourceAbsence, SourceID: absence.ID, DeletedBy: actorID, Payload: payload, Note: "Personal-Offboarding"}, nil
}

// staffOffboardingRecord is the data deletion record of one offboarded staff
// member, with the removed row counts as metadata.
func staffOffboardingRecord(actor staffoffboarding.Actor, result staffoffboarding.Result) any {
	counts := map[string]int64{
		"group_teacher": result.Membership.GroupAssignments, "class_teachers": result.Membership.ClassAssignments,
		"activity_supervisors": result.Timetable.PlannedSupervisors, "timetable_instance_staff": result.Timetable.InstanceAssignments,
		"group_substitutions": result.Workforce.Substitutions, "staff_shifts": result.Workforce.Shifts,
		"staff_shift_series": result.Workforce.Series, "staff_absences": result.Workforce.Absences,
		"account_permissions": result.Access.PermissionsRevoked,
	}
	records := 1
	for _, count := range counts {
		records += int(count)
	}
	deletedBy := actor.Username
	if deletedBy == "" {
		// Preserve the legacy provider's audit value for authenticated
		// sessions without a display name, including the API seeder.
		deletedBy = "system"
	}
	event := auditModels.NewStaffDataDeletion(result.StaffID, auditModels.DeletionTypeManual, records, deletedBy)
	event.DeletionReason = "staff offboarding"
	event.SetTenantID(actor.TenantID)
	for key, count := range counts {
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
