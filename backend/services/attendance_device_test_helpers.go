package services

import (
	"context"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
)

// The kiosk half of the attendance context. Production builds it in device
// authentication middleware; these entry points exist for the behaviour tests
// that drive a kiosk request without that middleware.

// WithAttendanceDevice binds a verified kiosk device to the attendance context.
func WithAttendanceDevice(ctx context.Context, deviceID, tenantID int64) context.Context {
	return devicescanCompose.WithAttendanceDevice(ctx, deviceID, tenantID)
}

// WithIoTAttendanceRequest marks the context as an IoT device request.
func WithIoTAttendanceRequest(ctx context.Context) context.Context {
	return devicescanCompose.WithIoTAttendanceRequest(ctx)
}
