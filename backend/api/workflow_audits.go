package api

import (
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/gradetransition"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"
)

// The workflows append their audit records through the Audit owner's command;
// the root builds the records from the workflow values, so no workflow
// composition names the Audit owner's records (#4004, #4085).

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

// staffAbsenceDeletionRecord is the time-tracking deletion record of one
// absence the offboarding removes, as Workforce builds it.
func staffAbsenceDeletionRecord(absence workforce.StaffAbsence, actorID int64) (any, error) {
	deletion, err := workforce.NewOffboardingAbsenceDeletion(absence, actorID)
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
