package parentportal

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// PickupPermissionApprovals is the review-time coordinator for live pickup
// contact eligibility. It joins the caller's tenant transaction.
type PickupPermissionApprovals interface {
	carerequests.PickupPermissionApprovals
}

// NewPickupPermissionApprovals exposes the workflow's approval coordinator.
func NewPickupPermissionApprovals(
	guardianProfiles GuardianProfiles,
	studentGuardians StudentGuardianReads,
	guardianPolicy GuardianPolicy,
	pickupPermissionWrite careplan.GuardianPickupPermissions,
) PickupPermissionApprovals {
	return &care.PickupPermissionApprovals{
		GuardianProfiles: guardianProfiles, StudentGuardians: studentGuardians,
		GuardianPolicy: guardianPolicy, PickupPermissionWrite: pickupPermissionWrite,
	}
}
