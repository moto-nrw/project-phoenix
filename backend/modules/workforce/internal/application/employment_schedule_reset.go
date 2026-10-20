package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// NewEmploymentScheduleReset ends the live schedule, preserving its versions,
// and clears the template binding with its previous value in the audit trail.
func NewEmploymentScheduleReset(capability workforce.Capability, employment workforce.StaffEmployments, audit workforce.StaffAdminAudit) func(context.Context, int64, string) error {
	return func(ctx context.Context, staffID int64, today string) error {
		profiles, err := employment.StaffEmployments(ctx, []int64{staffID})
		if err != nil {
			return err
		}
		profile, found := profiles[staffID]
		if !found {
			return workforce.ErrStaffEmploymentNotFound
		}
		if err := capability.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{StaffID: staffID, RotationAnchorDate: today}); err != nil {
			return err
		}
		if err := auditEmploymentModelReset(ctx, audit, staffID, profile); err != nil {
			return err
		}
		profile.WorkTimeModelID = nil
		profile.RotationAnchorDate = ""
		return employment.SaveStaffEmployment(ctx, profile)
	}
}

func auditEmploymentModelReset(ctx context.Context, audit workforce.StaffAdminAudit, staffID int64, profile workforce.StaffEmployment) error {
	modelID := ""
	if profile.WorkTimeModelID != nil {
		modelID = fmt.Sprint(*profile.WorkTimeModelID)
	}
	for _, field := range []struct{ name, value string }{
		{"work_time_model_id", modelID}, {"rotation_anchor_date", profile.RotationAnchorDate},
	} {
		if field.value == "" {
			continue
		}
		if err := audit.RecordMasterDataChange(ctx, workforce.StaffMasterDataChange{StaffID: staffID, ChangedBy: staffID, Section: workforce.StaffAuditSectionArbeitsvertrag, Field: field.name, OldValue: field.value, NewValue: "", Note: "Arbeitszeit bei Wiederaufnahme zurückgesetzt"}); err != nil {
			return err
		}
	}
	return nil
}
