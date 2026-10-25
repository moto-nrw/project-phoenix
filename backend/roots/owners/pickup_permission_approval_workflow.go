package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewPickupPermissionApprovals binds the Parent Portal review workflow to
// Care Plan's pickup-permission command without making either owner import the
// other. The root adapter translates the two public vocabularies.
func NewPickupPermissionApprovals(
	guardianProfiles parentportal.GuardianProfiles,
	studentGuardians parentportal.StudentGuardianReads,
	guardianPolicy parentportal.GuardianPolicy,
	pickupPermissions careplan.GuardianPickupPermissions,
) carerequests.PickupPermissionApprovals {
	workflow := parentportal.NewPickupPermissionApprovals(
		guardianProfiles, studentGuardians, guardianPolicy,
		pickupPermissionWriter{commands: pickupPermissions},
	)
	return pickupPermissionApprovalAdapter{workflow: workflow}
}

type pickupPermissionWriter struct {
	commands careplan.GuardianPickupPermissions
}

func (w pickupPermissionWriter) ChangePickupPermission(ctx context.Context, change parentportal.PickupPermissionChange) (bool, error) {
	return changeParentPickupPermission(ctx, w.commands, change)
}

func changeParentPickupPermission(ctx context.Context, commands careplan.GuardianPickupPermissions, change parentportal.PickupPermissionChange) (bool, error) {
	grant := true
	return commands.ChangeGuardianPickupPermission(ctx, careplan.GuardianPickupPermissionChange{
		TenantID: change.TenantID, RelationshipID: change.RelationshipID,
		CanPickup: &grant, SetWeekdays: true, Weekdays: change.Weekdays,
	})
}

type pickupPermissionApprovalAdapter struct {
	workflow parentportal.PickupPermissionApprovals
}

func (a pickupPermissionApprovalAdapter) ApplyPickupPermission(ctx context.Context, input carerequests.PickupPermissionApproval) error {
	return applyParentPickupPermission(ctx, a.workflow, parentportal.PickupPermissionApproval{
		TenantID: input.TenantID, StudentID: input.StudentID,
		GuardianProfileID: input.GuardianProfileID, RelationshipID: input.RelationshipID,
		Weekdays: input.Weekdays,
	})
}

func (a pickupPermissionApprovalAdapter) ValidatePickupPerson(ctx context.Context, studentID, guardianProfileID int64) error {
	return validateParentPickupPerson(ctx, a.workflow, studentID, guardianProfileID)
}

func validateParentPickupPerson(ctx context.Context, workflow parentportal.PickupPermissionApprovals, studentID, guardianProfileID int64) error {
	if workflow == nil {
		return errors.New("schedule: pickup permission approvals not configured")
	}
	return pickupPermissionApprovalError(workflow.ValidatePickupPerson(ctx, studentID, guardianProfileID))
}

func applyParentPickupPermission(ctx context.Context, workflow parentportal.PickupPermissionApprovals, input parentportal.PickupPermissionApproval) error {
	if workflow == nil {
		return errors.New("schedule: pickup permission approvals not configured")
	}
	return pickupPermissionApprovalError(workflow.ApplyPickupPermission(ctx, input))
}

func pickupPermissionApprovalError(err error) error {
	if errors.Is(err, parentportal.ErrPickupPermissionContactGone) {
		return carerequests.ErrPickupPermissionContactGone
	}
	return err
}
