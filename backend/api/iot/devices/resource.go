package devices

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// Resource defines the Devices API resource
type Resource struct {
	IoTService devicefleet.Administration
	runtime    Runtime
}

// NewResource creates a new Devices resource
func NewResource(iotService devicefleet.Administration, runtime Runtime) *Resource {
	return &Resource{
		IoTService: iotService,
		runtime:    runtime,
	}
}

// Router returns a configured router for device management endpoints
// This router is mounted under /iot/ and handles all device CRUD operations
// All routes require JWT authentication with appropriate IOT permissions
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Read operations require iot:read permission
	r.With(rs.runtime.Permission("iot:read")).Get("/", rs.listDevices)
	r.With(rs.runtime.Permission("iot:read")).Get("/{id}", rs.getDevice)
	r.With(rs.runtime.Permission("iot:read")).Get("/device/{deviceId}", rs.getDeviceByDeviceID)
	r.With(rs.runtime.Permission("iot:read")).Get("/type/{type}", rs.getDevicesByType)
	r.With(rs.runtime.Permission("iot:read")).Get("/status/{status}", rs.getDevicesByStatus)
	r.With(rs.runtime.Permission("iot:read")).Get("/registered-by/{personId}", rs.getDevicesByRegisteredBy)
	r.With(rs.runtime.Permission("iot:read")).Get("/active", rs.getActiveDevices)
	r.With(rs.runtime.Permission("iot:read")).Get("/maintenance", rs.getDevicesRequiringMaintenance)
	r.With(rs.runtime.Permission("iot:read")).Get("/offline", rs.getOfflineDevices)
	r.With(rs.runtime.Permission("iot:read")).Get("/statistics", rs.getDeviceStatistics)

	// Write operations require iot:update or iot:manage permission
	r.With(rs.runtime.Permission("iot:manage")).Post("/", rs.createDevice)
	r.With(rs.runtime.Permission("iot:update")).Put("/{id}", rs.updateDevice)
	r.With(rs.runtime.Permission("iot:manage")).Delete("/{id}", rs.deleteDevice)
	r.With(rs.runtime.Permission("iot:update")).Patch("/{deviceId}/status", rs.updateDeviceStatus)
	r.With(rs.runtime.Permission("iot:update")).Post("/{deviceId}/ping", rs.pingDevice)

	// Network operations require iot:manage permission
	r.With(rs.runtime.Permission("iot:manage")).Post("/detect-new", rs.detectNewDevices)
	r.With(rs.runtime.Permission("iot:manage")).Post("/scan-network", rs.scanNetwork)

	return r
}
