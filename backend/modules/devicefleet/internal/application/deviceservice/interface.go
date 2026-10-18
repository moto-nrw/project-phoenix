package iot

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// Service defines the IoT service operations
type Service interface {
	// Core device operations
	CreateDevice(ctx context.Context, device *iot.Device) error
	GetDeviceByID(ctx context.Context, id int64) (*iot.Device, error)
	GetDeviceByDeviceID(ctx context.Context, deviceID string) (*iot.Device, error)
	UpdateDevice(ctx context.Context, device *iot.Device) error
	DeleteDevice(ctx context.Context, id int64) error
	ListDevices(ctx context.Context, filters map[string]interface{}) ([]*iot.Device, error)

	// Status operations
	UpdateDeviceStatus(ctx context.Context, deviceID string, status iot.DeviceStatus) error
	PingDevice(ctx context.Context, deviceID string) error

	// Filtered lookups
	GetDevicesByType(ctx context.Context, deviceType string) ([]*iot.Device, error)
	GetDevicesByStatus(ctx context.Context, status iot.DeviceStatus) ([]*iot.Device, error)
	GetDevicesByRegisteredBy(ctx context.Context, personID int64) ([]*iot.Device, error)

	// Monitoring and reporting
	GetActiveDevices(ctx context.Context) ([]*iot.Device, error)
	GetDevicesRequiringMaintenance(ctx context.Context) ([]*iot.Device, error)
	GetOfflineDevices(ctx context.Context, offlineDuration time.Duration) ([]*iot.Device, error)
	GetDeviceTypeStatistics(ctx context.Context) (map[string]int, error)

	// Online/offline decision (issue #586 — Rule 12). The window is resolved
	// by the Device Fleet owner from the per-tenant setting
	// iot.device_online_window_minutes.
	DeviceOnlineWindow(ctx context.Context) time.Duration
	IsDeviceOnline(ctx context.Context, device *iot.Device) bool
	IsDeviceOnlineAt(ctx context.Context, device *iot.Device, now time.Time) bool
	IsLastSeenOnline(ctx context.Context, lastSeen *time.Time) bool

	// Network operations
	DetectNewDevices(ctx context.Context) ([]*iot.Device, error)
	ScanNetwork(ctx context.Context) (map[string]string, error)

	// Fleet exposes the Device Fleet owner this service delegates to, so the
	// composition root binds one instance for every entry point (#2676).
	Fleet() devicefleet.Capability
}
