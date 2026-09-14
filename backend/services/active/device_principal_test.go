package active_test

import (
	"github.com/moto-nrw/project-phoenix/auth/device"
)

// devicePrincipal binds a device to a context the way the kiosk device
// middleware does. The presence services only read the device's primary key
// and tenant, so the helper takes exactly those.
func devicePrincipal(id, tenantID int64) *device.AuthenticatedDevice {
	return &device.AuthenticatedDevice{ID: id, TenantID: tenantID, Status: "active"}
}

// staffPrincipal binds a staff identity to a context the way the kiosk device
// middleware does after a verified account PIN. It takes the two fields the
// principal carries rather than a staff row, so callers that only know the id
// do not have to build one.
func staffPrincipal(id, tenantID int64) *device.AuthenticatedStaff {
	if id == 0 {
		return nil
	}
	return &device.AuthenticatedStaff{ID: id, TenantID: tenantID}
}
