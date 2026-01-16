package devices

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/authorize/permissions"
	iotSvc "github.com/moto-nrw/project-phoenix/internal/core/service/iot"
)

// Resource defines the Devices API resource
type Resource struct {
	IoTService iotSvc.Service
}

// NewResource creates a new Devices resource
func NewResource(iotService iotSvc.Service) *Resource {
	return &Resource{
		IoTService: iotService,
	}
}

// Router returns a configured router for device management endpoints
// This router is mounted under /iot/ and handles all device CRUD operations
// All routes require JWT authentication with appropriate IOT permissions
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Read operations require iot:read permission
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/", rs.listDevices)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/{id}", rs.getDevice)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/device/{deviceId}", rs.getDeviceByDeviceID)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/type/{type}", rs.getDevicesByType)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/status/{status}", rs.getDevicesByStatus)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/registered-by/{personId}", rs.getDevicesByRegisteredBy)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/active", rs.getActiveDevices)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/maintenance", rs.getDevicesRequiringMaintenance)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/offline", rs.getOfflineDevices)
	r.With(authorize.RequiresPermission(permissions.IOTRead)).Get("/statistics", rs.getDeviceStatistics)

	// Write operations require iot:update or iot:manage permission
	r.With(authorize.RequiresPermission(permissions.IOTManage)).Post("/", rs.createDevice)
	r.With(authorize.RequiresPermission(permissions.IOTUpdate)).Put("/{id}", rs.updateDevice)
	r.With(authorize.RequiresPermission(permissions.IOTManage)).Delete("/{id}", rs.deleteDevice)
	r.With(authorize.RequiresPermission(permissions.IOTUpdate)).Patch("/{deviceId}/status", rs.updateDeviceStatus)
	r.With(authorize.RequiresPermission(permissions.IOTUpdate)).Post("/{deviceId}/ping", rs.pingDevice)

	// Network operations require iot:manage permission
	r.With(authorize.RequiresPermission(permissions.IOTManage)).Post("/detect-new", rs.detectNewDevices)
	r.With(authorize.RequiresPermission(permissions.IOTManage)).Post("/scan-network", rs.scanNetwork)

	return r
}

// =============================================================================
// EXPORTED HANDLERS FOR TESTING
// =============================================================================

// ListDevicesHandler returns the listDevices handler for testing.
func (rs *Resource) ListDevicesHandler() http.HandlerFunc { return rs.listDevices }

// GetDeviceHandler returns the getDevice handler for testing.
func (rs *Resource) GetDeviceHandler() http.HandlerFunc { return rs.getDevice }

// GetDeviceByDeviceIDHandler returns the getDeviceByDeviceID handler for testing.
func (rs *Resource) GetDeviceByDeviceIDHandler() http.HandlerFunc { return rs.getDeviceByDeviceID }

// CreateDeviceHandler returns the createDevice handler for testing.
func (rs *Resource) CreateDeviceHandler() http.HandlerFunc { return rs.createDevice }

// UpdateDeviceHandler returns the updateDevice handler for testing.
func (rs *Resource) UpdateDeviceHandler() http.HandlerFunc { return rs.updateDevice }

// DeleteDeviceHandler returns the deleteDevice handler for testing.
func (rs *Resource) DeleteDeviceHandler() http.HandlerFunc { return rs.deleteDevice }

// UpdateDeviceStatusHandler returns the updateDeviceStatus handler for testing.
func (rs *Resource) UpdateDeviceStatusHandler() http.HandlerFunc { return rs.updateDeviceStatus }

// PingDeviceHandler returns the pingDevice handler for testing.
func (rs *Resource) PingDeviceHandler() http.HandlerFunc { return rs.pingDevice }

// GetDevicesByTypeHandler returns the getDevicesByType handler for testing.
func (rs *Resource) GetDevicesByTypeHandler() http.HandlerFunc { return rs.getDevicesByType }

// GetDevicesByStatusHandler returns the getDevicesByStatus handler for testing.
func (rs *Resource) GetDevicesByStatusHandler() http.HandlerFunc { return rs.getDevicesByStatus }

// GetDevicesByRegisteredByHandler returns the getDevicesByRegisteredBy handler for testing.
func (rs *Resource) GetDevicesByRegisteredByHandler() http.HandlerFunc {
	return rs.getDevicesByRegisteredBy
}

// GetActiveDevicesHandler returns the getActiveDevices handler for testing.
func (rs *Resource) GetActiveDevicesHandler() http.HandlerFunc { return rs.getActiveDevices }

// GetDevicesRequiringMaintenanceHandler returns the getDevicesRequiringMaintenance handler for testing.
func (rs *Resource) GetDevicesRequiringMaintenanceHandler() http.HandlerFunc {
	return rs.getDevicesRequiringMaintenance
}

// GetOfflineDevicesHandler returns the getOfflineDevices handler for testing.
func (rs *Resource) GetOfflineDevicesHandler() http.HandlerFunc { return rs.getOfflineDevices }

// GetDeviceStatisticsHandler returns the getDeviceStatistics handler for testing.
func (rs *Resource) GetDeviceStatisticsHandler() http.HandlerFunc { return rs.getDeviceStatistics }

// DetectNewDevicesHandler returns the detectNewDevices handler for testing.
func (rs *Resource) DetectNewDevicesHandler() http.HandlerFunc { return rs.detectNewDevices }

// ScanNetworkHandler returns the scanNetwork handler for testing.
func (rs *Resource) ScanNetworkHandler() http.HandlerFunc { return rs.scanNetwork }
