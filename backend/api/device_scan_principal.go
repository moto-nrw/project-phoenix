package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet/deviceauth"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// DeviceScanPrincipal binds the workflow's reader to the authenticated device.
func DeviceScanPrincipal(ctx context.Context) (*devicescan.PrincipalDevice, bool) {
	principal, ok := deviceauth.VerifiedDevice(ctx)
	if !ok {
		return nil, false
	}
	return &devicescan.PrincipalDevice{
		ID: principal.ID, TenantID: principal.TenantID, DeviceID: principal.DeviceID,
		DeviceType: principal.DeviceType, Name: principal.Name, Status: string(principal.Status),
		LastSeen: principal.LastSeen, Active: principal.IsActive(),
	}, true
}

// AttendancePrincipal projects the verified context for Student Presence.
func AttendancePrincipal(ctx context.Context) studentpresence.RequestPrincipal {
	isIoT, deviceID, staffID, hasStaff := deviceauth.AttendanceContext(ctx)
	return studentpresence.RequestPrincipal{IsIoT: isIoT, DeviceID: deviceID, StaffID: staffID, HasStaff: hasStaff}
}
