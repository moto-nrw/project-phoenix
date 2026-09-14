package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/services/active"
)

// WithAttendanceStaff binds the staff identity already verified by the web
// attendance boundary to the same context key used by device authentication.
func WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxStaff, &device.AuthenticatedStaff{ID: staffID, TenantID: tenantID})
}

// AttendancePrincipal projects the verified device and staff context for attendance.
// It reads the original context; no independent authentication state is created.
func AttendancePrincipal(ctx context.Context) active.RequestPrincipal {
	result := active.RequestPrincipal{IsIoT: device.IsIoTDeviceRequest(ctx)}
	if principal := device.DeviceFromCtx(ctx); principal != nil {
		result.DeviceID = principal.ID
	}
	if principal := device.StaffFromCtx(ctx); principal != nil {
		result.StaffID, result.HasStaff = principal.ID, true
	}
	return result
}

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
