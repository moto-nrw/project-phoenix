package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/device"
)

// Device-authentication state that production builds in middleware. Behaviour
// tests that drive a kiosk request without that middleware bind it here.

// WithAttendanceDevice binds a verified kiosk device to the same context key
// device authentication uses. It is the device half of WithAttendanceStaff.
func WithAttendanceDevice(ctx context.Context, deviceID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxDevice, &device.AuthenticatedDevice{ID: deviceID, TenantID: tenantID, Status: "active"})
}

// WithIoTAttendanceRequest marks the context as coming from an IoT device, the
// way the device middleware does, so attendance takes its kiosk branch.
func WithIoTAttendanceRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, device.CtxIsIoTDevice, true)
}
