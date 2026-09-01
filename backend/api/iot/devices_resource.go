package iot

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// DevicesResource defines the Devices API resource
type DevicesResource struct {
	IoTService iotSvc.Service
}

// NewDevicesResource creates a new Devices resource
func NewDevicesResource(iotService iotSvc.Service) *DevicesResource {
	return &DevicesResource{
		IoTService: iotService,
	}
}

// Router returns a configured router for device management endpoints
// This router is mounted under /iot/ and handles all device CRUD operations
// All routes require JWT authentication with appropriate IOT permissions
func (rs *DevicesResource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Read operations require iot:read permission
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/", rs.listDevices)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/{id}", rs.getDevice)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/device/{deviceId}", rs.getDeviceByDeviceID)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/type/{type}", rs.getDevicesByType)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/status/{status}", rs.getDevicesByStatus)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/registered-by/{personId}", rs.getDevicesByRegisteredBy)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/active", rs.getActiveDevices)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/maintenance", rs.getDevicesRequiringMaintenance)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/offline", rs.getOfflineDevices)
	r.With(common.RequiresPermission(permissions.IOTRead)).Get("/statistics", rs.getDeviceStatistics)

	// Write operations require iot:update or iot:manage permission
	r.With(common.RequiresPermission(permissions.IOTManage)).Post("/", rs.createDevice)
	r.With(common.RequiresPermission(permissions.IOTUpdate)).Put("/{id}", rs.updateDevice)
	r.With(common.RequiresPermission(permissions.IOTManage)).Delete("/{id}", rs.deleteDevice)
	r.With(common.RequiresPermission(permissions.IOTUpdate)).Patch("/{deviceId}/status", rs.updateDeviceStatus)
	r.With(common.RequiresPermission(permissions.IOTUpdate)).Post("/{deviceId}/ping", rs.pingDevice)

	// Network operations require iot:manage permission
	r.With(common.RequiresPermission(permissions.IOTManage)).Post("/detect-new", rs.detectNewDevices)
	r.With(common.RequiresPermission(permissions.IOTManage)).Post("/scan-network", rs.scanNetwork)

	return r
}
