package checkin_test

import (
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// staffPrincipal binds a staff row to a context the way the kiosk device
// middleware does after a verified account PIN.
func staffPrincipal(s *users.Staff) *device.AuthenticatedStaff {
	if s == nil {
		return nil
	}
	return &device.AuthenticatedStaff{ID: s.ID, TenantID: s.TenantID}
}
