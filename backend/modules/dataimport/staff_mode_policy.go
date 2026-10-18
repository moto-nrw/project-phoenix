package dataimport

import (
	"context"
	"fmt"
)

// StaffImportModePolicy admits file modes with the same write rights as the staff screens.
type StaffImportModePolicy struct{ authorization Authorization }

func NewStaffImportModePolicy(authorization Authorization) StaffImportModePolicy {
	return StaffImportModePolicy{authorization: authorization}
}

// staffUpdateImportPermissions are the permissions an importer needs before
// the staff import may change existing records (#2906). Update and upsert
// mode write the same fields as the personnel screens: notes, employment
// type, personnel number and position belong to staff:manage (PUT
// /api/staff/{id}); names, birthday, contact, contract and qualifications to
// staff:stammdaten (PUT /api/staff/{id}/stammdaten/*). users:create alone
// files new records only. The literals mirror permissions.StaffManage and
// permissions.StaffStammdaten; the architecture policy keeps that package
// out of the import services, as auth/authorize/role_grant.go does it too.
var staffUpdateImportPermissions = []string{"staff:manage", "staff:stammdaten"}

// AuthorizeImportMode refuses update-capable modes unless the importer holds
// every permission the personnel screens demand for the same writes. The
// import service calls it before any row is validated, so a create-only
// caller cannot even preview an update.
func (c StaffImportModePolicy) AuthorizeImportMode(ctx context.Context, mode ImportMode) error {
	if mode == ImportModeCreate {
		return nil
	}
	importerPermissions := ImporterPermissionsFromContext(ctx)
	for _, required := range staffUpdateImportPermissions {
		if c.authorization == nil || !c.authorization.HasPermission(required, importerPermissions) {
			return fmt.Errorf("%w: mode %q requires %s", ErrImportModeForbidden, mode, required)
		}
	}
	return nil
}
