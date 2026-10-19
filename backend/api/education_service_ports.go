package api

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// The ports of the retained School Structure services the root's service
// graph binds over the retained repositories (#2742). Each adapter translates the
// People Directory, Facilities, Workforce and Audit Platform rows into the
// School Structure values the services own (#3556), and changes nothing about
// which reads and writes run.

// The substitution module may not name the Audit Platform, so School Structure
// mirrors the actions the trail stores, and the binding below passes them
// through unchanged. A drift must not compile: a false comparison repeats the
// false key of this map literal.
var _ = map[bool]struct{}{
	false: {},
	auditModels.SubstitutionAssigned == educationRepo.SubstitutionAssigned &&
		auditModels.SubstitutionEnded == educationRepo.SubstitutionEnded: {},
}

// EducationSubstitutionAudit appends the substitution module's changes to the
// audit.substitution_changes trail.
type EducationSubstitutionAudit struct {
	changes auditModels.SubstitutionChangeCreator
}

// NewEducationSubstitutionAudit binds the trail to its repository.
func NewEducationSubstitutionAudit(changes auditModels.SubstitutionChangeCreator) EducationSubstitutionAudit {
	return EducationSubstitutionAudit{changes: changes}
}

// RecordSubstitutionChange appends one change.
func (a EducationSubstitutionAudit) RecordSubstitutionChange(ctx context.Context, change educationRepo.SubstitutionChange) error {
	row := &auditModels.SubstitutionChange{
		SubstitutionID: change.SubstitutionID, TargetType: change.TargetType, Action: string(change.Action),
		GroupID: change.GroupID, TargetStaffID: change.TargetStaffID, ActorAccountID: change.ActorAccountID,
		StartDate: auditModels.Date(change.StartDate),
	}
	if change.EndDate != nil {
		endDate := auditModels.Date(*change.EndDate)
		row.EndDate = &endDate
	}
	return a.changes.Create(ctx, row)
}

// EducationClassAssignmentAudit appends class assignment rewrites (#1772) to
// the Stammdaten audit trail.
type EducationClassAssignmentAudit struct {
	changes auditModels.StaffMasterDataChangeCreator
}

// NewEducationClassAssignmentAudit binds the trail to its repository.
func NewEducationClassAssignmentAudit(changes auditModels.StaffMasterDataChangeCreator) EducationClassAssignmentAudit {
	return EducationClassAssignmentAudit{changes: changes}
}

// RecordSchoolClassChange appends one rewrite in the school classes section.
func (a EducationClassAssignmentAudit) RecordSchoolClassChange(ctx context.Context, change educationRepo.SchoolClassChange) error {
	return a.changes.Create(ctx, &auditModels.StaffMasterDataChange{
		StaffID:   change.StaffID,
		ChangedBy: change.ChangedBy,
		Section:   auditModels.StammdatenSectionSchoolClasses,
		FieldName: auditModels.StammdatenFieldSchoolClasses,
		OldValue:  change.OldValue,
		NewValue:  change.NewValue,
	})
}
