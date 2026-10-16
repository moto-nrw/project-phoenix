//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

type routerStaffClock struct{}

func (routerStaffClock) StaffClockState(context.Context, string) (*devicescan.StaffClockState, error) {
	return &devicescan.StaffClockState{}, nil
}

func (routerStaffClock) ExecuteStaffClock(context.Context, devicescan.StaffClockCommand) (*devicescan.StaffClockState, error) {
	return &devicescan.StaffClockState{}, nil
}

// routerDeviceScan satisfies the kiosk scan contract during route
// composition; every call answers as an unauthenticated device.
type routerDeviceScan struct{}

func (routerDeviceScan) LockFeedbackStudent(context.Context, int64) (devicescan.FeedbackStudent, error) {
	return devicescan.FeedbackStudent{}, devicescan.ErrDeviceUnauthorized
}

func (routerDeviceScan) Device(context.Context) (devicescan.Device, error) {
	return devicescan.Device{}, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Scan(context.Context, devicescan.ScanCommand) (*devicescan.ScanResult, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) PickupInfo(context.Context, string) (*devicescan.PickupInfo, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Ping(context.Context) (*devicescan.DevicePing, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) Status(context.Context) (*devicescan.DeviceStatus, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) AttendanceStatus(context.Context, string) (*devicescan.AttendanceStatus, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}
func (routerDeviceScan) ToggleAttendance(context.Context, devicescan.AttendanceToggleCommand) (*devicescan.AttendanceToggleResult, error) {
	return nil, devicescan.ErrDeviceUnauthorized
}

// iotSessionConflictInfo returns the exact public conflict facts used by the route fixture.
func iotSessionConflictInfo(has bool, deviceID *int64, message string, override bool) devicescan.ConflictInfoResponse {
	return devicescan.ConflictInfoResponse{HasConflict: has, ConflictingDevice: deviceID, ConflictMessage: message, CanOverride: override}
}
