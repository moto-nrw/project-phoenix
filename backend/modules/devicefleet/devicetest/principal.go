// Package devicetest binds the principals the device authentication
// middleware binds, so handler and capability tests act as a verified kiosk
// without naming the middleware's context keys.
package devicetest

import (
	"context"

	device "github.com/moto-nrw/project-phoenix/modules/devicefleet/internal/adapters/deviceauth"
)

// Principal is the device principal handlers read from the request context.
type Principal = device.AuthenticatedDevice

// WithDevice binds principal as the authenticated device.
func WithDevice(ctx context.Context, principal *Principal) context.Context {
	return context.WithValue(ctx, device.CtxDevice, principal)
}

// WithStaff binds the staff principal a verified web boundary binds.
func WithStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxStaff, &device.AuthenticatedStaff{ID: staffID, TenantID: tenantID})
}

// WithIoTDevice marks the context as a kiosk request, the way the device
// middleware does.
func WithIoTDevice(ctx context.Context) context.Context {
	return context.WithValue(ctx, device.CtxIsIoTDevice, true)
}

// DeviceFromContext returns the bound device principal, or nil.
func DeviceFromContext(ctx context.Context) *Principal {
	return device.DeviceFromCtx(ctx)
}
