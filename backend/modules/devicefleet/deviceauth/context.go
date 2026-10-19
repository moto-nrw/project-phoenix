package deviceauth

import (
	"context"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	device "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/deviceauth"
)

// ContextDevice returns exactly the native principal facts the Root bindings need.
func ContextDevice(ctx context.Context) (id int64, deviceID string, tenantID int64, deviceType string, ok bool) {
	principal := device.DeviceFromCtx(ctx)
	if principal == nil {
		return 0, "", 0, "", false
	}
	return principal.ID, principal.DeviceID, principal.TenantID, principal.DeviceType, true
}

// VerifiedDevice returns the device facts authenticated by the middleware.
func VerifiedDevice(ctx context.Context) (devicefleet.Device, bool) {
	principal := device.DeviceFromCtx(ctx)
	if principal == nil {
		return devicefleet.Device{}, false
	}
	return devicefleet.Device{
		ID: principal.ID, TenantID: principal.TenantID, DeviceID: principal.DeviceID,
		DeviceType: principal.DeviceType, Name: principal.Name,
		Status: devicefleet.DeviceStatus(principal.Status), LastSeen: principal.LastSeen,
	}, true
}

// WithAttendanceStaff binds the staff identity already verified by the web boundary.
func WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxStaff, &device.AuthenticatedStaff{ID: staffID, TenantID: tenantID})
}

// AttendanceContext reads the original verified device and staff context.
func AttendanceContext(ctx context.Context) (isIoT bool, deviceID, staffID int64, hasStaff bool) {
	isIoT = device.IsIoTDeviceRequest(ctx)
	if principal := device.DeviceFromCtx(ctx); principal != nil {
		deviceID = principal.ID
	}
	if principal := device.StaffFromCtx(ctx); principal != nil {
		staffID, hasStaff = principal.ID, true
	}
	return
}

// Required retains the device adapter's fail-closed behavior for an unbound route group.
func Required(name string, middleware Middleware) Middleware {
	return device.Required(name, middleware)
}

// Unauthorized returns the same native device error renderer used by the kiosk contract.
func Unauthorized(err error) render.Renderer { return device.ErrDeviceUnauthorized(err) }
