package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// WithAttendanceStaff binds the staff identity already verified by the web
// attendance boundary to the same context key used by device authentication.
func WithAttendanceStaff(ctx context.Context, staffID, tenantID int64) context.Context {
	return context.WithValue(ctx, device.CtxStaff, &device.AuthenticatedStaff{ID: staffID, TenantID: tenantID})
}

// AttendancePrincipal projects the verified device and staff context for attendance.
// It reads the original context; no independent authentication state is created.
func AttendancePrincipal(ctx context.Context) studentpresence.RequestPrincipal {
	result := studentpresence.RequestPrincipal{IsIoT: device.IsIoTDeviceRequest(ctx)}
	if principal := device.DeviceFromCtx(ctx); principal != nil {
		result.DeviceID = principal.ID
	}
	if principal := device.StaffFromCtx(ctx); principal != nil {
		result.StaffID, result.HasStaff = principal.ID, true
	}
	return result
}
