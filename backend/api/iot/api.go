package iot

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/iot"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
)

// Resource defines the IoT API resource
type Resource struct {
	IoTService iotSvc.Service
}

// NewResource creates a new IoT resource
func NewResource(iotService iotSvc.Service) *Resource {
	return &Resource{
		IoTService: iotService,
	}
}

// Router returns a configured router for IoT endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	// Create JWT auth instance for middleware
	tokenAuth, _ := jwt.NewTokenAuth()

	// Public routes (if any device endpoints should be public)
	r.Group(func(r chi.Router) {
		// Some basic device info might be public
		// Currently no public routes for IoT devices
	})

	// Protected routes that require authentication and permissions
	r.Group(func(r chi.Router) {
		r.Use(tokenAuth.Verifier())
		r.Use(jwt.Authenticator)

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
	})

	return r
}

// DeviceResponse represents a device API response
type DeviceResponse struct {
	ID             int64        `json:"id"`
	DeviceID       string       `json:"device_id"`
	DeviceType     string       `json:"device_type"`
	Name           *string      `json:"name,omitempty"`
	Status         string       `json:"status"`
	LastSeen       *common.Time `json:"last_seen,omitempty"`
	RegisteredByID *int64       `json:"registered_by_id,omitempty"`
	IsOnline       bool         `json:"is_online"`
	CreatedAt      common.Time  `json:"created_at"`
	UpdatedAt      common.Time  `json:"updated_at"`
}

// DeviceRequest represents a device creation/update request
type DeviceRequest struct {
	DeviceID       string  `json:"device_id"`
	DeviceType     string  `json:"device_type"`
	Name           *string `json:"name,omitempty"`
	Status         string  `json:"status,omitempty"`
	RegisteredByID *int64  `json:"registered_by_id,omitempty"`
}

// Bind validates the device request
func (req *DeviceRequest) Bind(r *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.DeviceID, validation.Required),
		validation.Field(&req.DeviceType, validation.Required),
	); err != nil {
		return err
	}

	// Validate status only if provided
	if req.Status != "" {
		if !isValidDeviceStatus(iot.DeviceStatus(req.Status)) {
			return errors.New("invalid device status")
		}
	}

	return nil
}

// DeviceStatusRequest represents a device status update request
type DeviceStatusRequest struct {
	Status string `json:"status"`
}

// Bind validates the device status request
func (req *DeviceStatusRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Status, validation.Required, validation.In(
			string(iot.DeviceStatusActive),
			string(iot.DeviceStatusInactive),
			string(iot.DeviceStatusMaintenance),
			string(iot.DeviceStatusOffline),
		)),
	)
}

// DeviceStatisticsResponse represents device statistics
type DeviceStatisticsResponse struct {
	DeviceTypeCount map[string]int `json:"device_type_count"`
	TotalDevices    int            `json:"total_devices"`
	ActiveDevices   int            `json:"active_devices"`
	OfflineDevices  int            `json:"offline_devices"`
	LastUpdated     time.Time      `json:"last_updated"`
}

// NetworkScanResponse represents network scan results
type NetworkScanResponse struct {
	Devices      map[string]string `json:"devices"`
	ScanTime     time.Time         `json:"scan_time"`
	DevicesFound int               `json:"devices_found"`
}

// newDeviceResponse converts a device model to a response object
func newDeviceResponse(device *iot.Device) DeviceResponse {
	response := DeviceResponse{
		ID:             device.ID,
		DeviceID:       device.DeviceID,
		DeviceType:     device.DeviceType,
		Name:           device.Name,
		Status:         string(device.Status),
		RegisteredByID: device.RegisteredByID,
		IsOnline:       device.IsOnline(),
		CreatedAt:      common.Time(device.CreatedAt),
		UpdatedAt:      common.Time(device.UpdatedAt),
	}

	if device.LastSeen != nil {
		lastSeen := common.Time(*device.LastSeen)
		response.LastSeen = &lastSeen
	}

	return response
}

// listDevices handles listing all devices with optional filtering
func (rs *Resource) listDevices(w http.ResponseWriter, r *http.Request) {
	// Get filter parameters
	deviceType := r.URL.Query().Get("device_type")
	status := r.URL.Query().Get("status")
	registeredByID := r.URL.Query().Get("registered_by_id")
	search := r.URL.Query().Get("search")

	// Create filters map
	filters := make(map[string]interface{})

	// Apply filters
	if deviceType != "" {
		filters["device_type"] = deviceType
	}

	if status != "" {
		filters["status"] = status
	}

	if registeredByID != "" {
		if id, err := strconv.ParseInt(registeredByID, 10, 64); err == nil {
			filters["registered_by_id"] = id
		}
	}

	if search != "" {
		filters["device_id_like"] = search
	}

	// Get devices
	devices, err := rs.IoTService.ListDevices(r.Context(), filters)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Devices retrieved successfully")
}

// getDevice handles getting a device by ID
func (rs *Resource) getDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid device ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device retrieved successfully")
}

// getDeviceByDeviceID handles getting a device by its device ID
func (rs *Resource) getDeviceByDeviceID(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("device ID is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByDeviceID(r.Context(), deviceID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device retrieved successfully")
}

// createDevice handles creating a new device
func (rs *Resource) createDevice(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Create device model
	device := &iot.Device{
		DeviceID:       req.DeviceID,
		DeviceType:     req.DeviceType,
		Name:           req.Name,
		RegisteredByID: req.RegisteredByID,
	}

	// Set status if provided, otherwise default to active
	if req.Status != "" {
		device.Status = iot.DeviceStatus(req.Status)
	} else {
		device.Status = iot.DeviceStatusActive
	}

	// Create device
	if err := rs.IoTService.CreateDevice(r.Context(), device); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusCreated, newDeviceResponse(device), "Device created successfully")
}

// updateDevice handles updating an existing device
func (rs *Resource) updateDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid device ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get existing device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update fields
	device.DeviceID = req.DeviceID
	device.DeviceType = req.DeviceType
	device.Name = req.Name
	device.RegisteredByID = req.RegisteredByID

	if req.Status != "" {
		device.Status = iot.DeviceStatus(req.Status)
	}

	// Update device
	if err := rs.IoTService.UpdateDevice(r.Context(), device); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device updated successfully")
}

// deleteDevice handles deleting a device
func (rs *Resource) deleteDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid device ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Delete device
	if err := rs.IoTService.DeleteDevice(r.Context(), id); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device deleted successfully")
}

// updateDeviceStatus handles updating the status of a device
func (rs *Resource) updateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("device ID is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Parse request
	req := &DeviceStatusRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Update device status
	if err := rs.IoTService.UpdateDeviceStatus(r.Context(), deviceID, iot.DeviceStatus(req.Status)); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device status updated successfully")
}

// pingDevice handles pinging a device to update its last seen time
func (rs *Resource) pingDevice(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("device ID is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Ping device
	if err := rs.IoTService.PingDevice(r.Context(), deviceID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device pinged successfully")
}

// getDevicesByType handles getting devices by type
func (rs *Resource) getDevicesByType(w http.ResponseWriter, r *http.Request) {
	// Get type from URL
	deviceType := chi.URLParam(r, "type")
	if deviceType == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("device type is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get devices by type
	devices, err := rs.IoTService.GetDevicesByType(r.Context(), deviceType)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Devices retrieved successfully")
}

// getDevicesByStatus handles getting devices by status
func (rs *Resource) getDevicesByStatus(w http.ResponseWriter, r *http.Request) {
	// Get status from URL
	status := chi.URLParam(r, "status")
	if status == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("status is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate status
	deviceStatus := iot.DeviceStatus(status)
	if !isValidDeviceStatus(deviceStatus) {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid device status"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get devices by status
	devices, err := rs.IoTService.GetDevicesByStatus(r.Context(), deviceStatus)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Devices retrieved successfully")
}

// getDevicesByRegisteredBy handles getting devices registered by a specific person
func (rs *Resource) getDevicesByRegisteredBy(w http.ResponseWriter, r *http.Request) {
	// Parse person ID from URL
	personID, err := strconv.ParseInt(chi.URLParam(r, "personId"), 10, 64)
	if err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("invalid person ID"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get devices
	devices, err := rs.IoTService.GetDevicesByRegisteredBy(r.Context(), personID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Devices retrieved successfully")
}

// getActiveDevices handles getting all active devices
func (rs *Resource) getActiveDevices(w http.ResponseWriter, r *http.Request) {
	// Get active devices
	devices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Active devices retrieved successfully")
}

// getDevicesRequiringMaintenance handles getting devices requiring maintenance
func (rs *Resource) getDevicesRequiringMaintenance(w http.ResponseWriter, r *http.Request) {
	// Get devices requiring maintenance
	devices, err := rs.IoTService.GetDevicesRequiringMaintenance(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Devices requiring maintenance retrieved successfully")
}

// getOfflineDevices handles getting offline devices
func (rs *Resource) getOfflineDevices(w http.ResponseWriter, r *http.Request) {
	// Get duration parameter (default to 1 hour)
	durationStr := r.URL.Query().Get("duration")
	duration := time.Hour // default

	if durationStr != "" {
		if parsedDuration, err := time.ParseDuration(durationStr); err == nil {
			duration = parsedDuration
		}
	}

	// Get offline devices
	devices, err := rs.IoTService.GetOfflineDevices(r.Context(), duration)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Offline devices retrieved successfully")
}

// getDeviceStatistics handles getting device statistics
func (rs *Resource) getDeviceStatistics(w http.ResponseWriter, r *http.Request) {
	// Get device type statistics
	typeStats, err := rs.IoTService.GetDeviceTypeStatistics(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get active devices count
	activeDevices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get offline devices count (devices offline for more than 5 minutes)
	offlineDevices, err := rs.IoTService.GetOfflineDevices(r.Context(), 5*time.Minute)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Calculate total devices
	total := 0
	for _, count := range typeStats {
		total += count
	}

	// Build response
	response := DeviceStatisticsResponse{
		DeviceTypeCount: typeStats,
		TotalDevices:    total,
		ActiveDevices:   len(activeDevices),
		OfflineDevices:  len(offlineDevices),
		LastUpdated:     time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Device statistics retrieved successfully")
}

// detectNewDevices handles detecting new devices on the network
func (rs *Resource) detectNewDevices(w http.ResponseWriter, r *http.Request) {
	// Detect new devices
	devices, err := rs.IoTService.DetectNewDevices(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(device))
	}

	common.Respond(w, r, http.StatusOK, responses, "Device detection completed")
}

// scanNetwork handles scanning the network for IoT devices
func (rs *Resource) scanNetwork(w http.ResponseWriter, r *http.Request) {
	// Scan network
	scanResults, err := rs.IoTService.ScanNetwork(r.Context())
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response
	response := NetworkScanResponse{
		Devices:      scanResults,
		ScanTime:     time.Now(),
		DevicesFound: len(scanResults),
	}

	common.Respond(w, r, http.StatusOK, response, "Network scan completed")
}

// Helper function to validate device status
func isValidDeviceStatus(status iot.DeviceStatus) bool {
	switch status {
	case iot.DeviceStatusActive, iot.DeviceStatusInactive, iot.DeviceStatusMaintenance, iot.DeviceStatusOffline:
		return true
	}
	return false
}
