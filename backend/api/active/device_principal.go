package active

import (
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// staffPrincipal binds a verified staff row to the request the way the
// kiosk device middleware does, so the presence services attribute web
// check-ins and checkouts through one context key.
func staffPrincipal(staff *users.Staff) *device.AuthenticatedStaff {
	if staff == nil {
		return nil
	}
	return &device.AuthenticatedStaff{ID: staff.ID, TenantID: staff.TenantID}
}
