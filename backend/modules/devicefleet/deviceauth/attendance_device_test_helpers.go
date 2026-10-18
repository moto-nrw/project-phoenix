//go:build testhooks

package deviceauth

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/device"
)

// Device-authentication state that production builds in middleware. Behaviour
// tests that drive a kiosk request without that middleware bind it here.

// WithAttendanceDevice binds a verified kiosk device as device authentication does.
func WithAttendanceDevice(ctx context.Context, deviceID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxDevice, &device.AuthenticatedDevice{ID: deviceID, TenantID: tenantID, Status: "active"})
}

// WithIoTAttendanceRequest marks an IoT request so attendance takes its kiosk branch.
func WithIoTAttendanceRequest(ctx context.Context) context.Context {
	return context.WithValue(ctx, device.CtxIsIoTDevice, true)
}
