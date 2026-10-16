package deviceauth

import (
	"context"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/auth/device"
)

// ContextDevice returns exactly the native principal facts the Root bindings need.
func ContextDevice(ctx context.Context) (id int64, deviceID string, tenantID int64, deviceType string, ok bool) {
	principal := device.DeviceFromCtx(ctx)
	if principal == nil {
		return 0, "", 0, "", false
	}
	return principal.ID, principal.DeviceID, principal.TenantID, principal.DeviceType, true
}

// Required retains the device adapter's fail-closed behavior for an unbound route group.
func Required(name string, middleware Middleware) Middleware {
	return device.Required(name, middleware)
}

// Unauthorized returns the same native device error renderer used by the kiosk contract.
func Unauthorized(err error) render.Renderer { return device.ErrDeviceUnauthorized(err) }
