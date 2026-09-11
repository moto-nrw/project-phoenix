package active_test

import (
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// devicePrincipal binds a device to a context the way the kiosk device
// middleware does. The presence services only read the device's primary key
// and tenant, so the helper takes exactly those.
func devicePrincipal(id, tenantID int64) *device.AuthenticatedDevice {
	return &device.AuthenticatedDevice{ID: id, TenantID: tenantID, Status: "active"}
}

// staffPrincipal binds a staff row to a context the way the kiosk device
// middleware does after a verified account PIN.
func staffPrincipal(s *users.Staff) *device.AuthenticatedStaff {
	if s == nil {
		return nil
	}
	return &device.AuthenticatedStaff{ID: s.ID, TenantID: s.TenantID}
}
