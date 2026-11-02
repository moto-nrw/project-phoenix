package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

// StaffPermissionHolders lists the staff whose account holds a permission in
// the current school, the People Directory read the exit notices address.
type StaffPermissionHolders interface {
	ListStaffWithPermission(ctx context.Context, permissionName string) ([]*peopleValues.StaffWithRoleInfo, error)
}

// PlannedStaffExitsBuilder builds the Worker's planned staff exits (#4809)
// over the owners the Worker root built. The API root supplies it: it
// composes the staff offboarding workflow the DELETE route uses, and like
// that composition it panics on a missing dependency.
type PlannedStaffExitsBuilder func(identity identityaccess.Owner, notifications DeliveryNotifications, staff StaffPermissionHolders) schedulerPorts.PlannedStaffExits
