package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

var ErrPickupPermissionContactGone = care.ErrPickupPermissionContactGone
var ErrPickupPermissionChanged = care.ErrPickupPermissionChanged

type PickupPermissionApproval = care.PickupPermissionApproval
type PickupPermissionChange = care.PickupPermissionChange
type PickupPermissionState = care.PickupPermissionState
type PickupPermissionWrite = care.PickupPermissionWrite

// PickupPermissionApprovals is the review-time coordinator for live pickup
// contact eligibility. It joins the caller's tenant transaction.
type PickupPermissionApprovals interface {
	ApplyPickupPermission(context.Context, PickupPermissionApproval) error
	ValidatePickupPerson(context.Context, int64, int64) error
}

// NewPickupPermissionApprovals exposes the workflow's approval coordinator.
func NewPickupPermissionApprovals(
	guardianProfiles GuardianProfiles,
	studentGuardians StudentGuardianReads,
	guardianPolicy GuardianPolicy,
	pickupPermissionWrite PickupPermissionWrite,
) PickupPermissionApprovals {
	return &care.PickupPermissionApprovals{
		GuardianProfiles: guardianProfiles, StudentGuardians: studentGuardians,
		GuardianPolicy: guardianPolicy, PickupPermissionWrite: pickupPermissionWrite,
	}
}
