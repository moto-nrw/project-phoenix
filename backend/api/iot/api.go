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
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/iot"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the IoT API resource
type Resource struct {
	IoTService    iotSvc.Service
	UsersService  usersSvc.PersonService
	ActiveService activeSvc.Service
}

// NewResource creates a new IoT resource
func NewResource(iotService iotSvc.Service, usersService usersSvc.PersonService, activeService activeSvc.Service) *Resource {
	return &Resource{
		IoTService:    iotService,
		UsersService:  usersService,
		ActiveService: activeService,
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

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService))

		// Device endpoints that require device API key + staff PIN authentication
		r.Post("/ping", rs.devicePing)
		r.Post("/checkin", rs.deviceCheckin)
		r.Get("/status", rs.deviceStatus)
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

// Device-authenticated handlers for RFID devices

// devicePing handles ping requests from RFID devices
func (rs *Resource) devicePing(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Update device last seen time (already done in middleware, but let's be explicit)
	if err := rs.IoTService.PingDevice(r.Context(), deviceCtx.DeviceID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Return device status and staff info
	response := map[string]interface{}{
		"device_id":    deviceCtx.DeviceID,
		"device_name":  deviceCtx.Name,
		"status":       deviceCtx.Status,
		"staff_id":     staffCtx.ID,
		"person_id":    staffCtx.PersonID,
		"last_seen":    deviceCtx.LastSeen,
		"is_online":    deviceCtx.IsOnline(),
		"ping_time":    time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Device ping successful")
}

// deviceStatus handles status requests from RFID devices
func (rs *Resource) deviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Return detailed device and staff status
	response := map[string]interface{}{
		"device": map[string]interface{}{
			"id":          deviceCtx.ID,
			"device_id":   deviceCtx.DeviceID,
			"device_type": deviceCtx.DeviceType,
			"name":        deviceCtx.Name,
			"status":      deviceCtx.Status,
			"last_seen":   deviceCtx.LastSeen,
			"is_online":   deviceCtx.IsOnline(),
			"is_active":   deviceCtx.IsActive(),
		},
		"staff": map[string]interface{}{
			"id":        staffCtx.ID,
			"person_id": staffCtx.PersonID,
			"is_locked": staffCtx.IsLocked(),
		},
		"authenticated_at": time.Now(),
	}

	// Add person info if available
	if staffCtx.Person != nil {
		response["person"] = map[string]interface{}{
			"id":         staffCtx.Person.ID,
			"first_name": staffCtx.Person.FirstName,
			"last_name":  staffCtx.Person.LastName,
		}
	}

	common.Respond(w, r, http.StatusOK, response, "Device status retrieved")
}

// CheckinRequest represents a student check-in request from RFID devices
type CheckinRequest struct {
	StudentRFID string `json:"student_rfid"`
	Action      string `json:"action"` // "checkin" or "checkout"
	RoomID      *int64 `json:"room_id,omitempty"`
}

// CheckinResponse represents the response to a student check-in request
type CheckinResponse struct {
	StudentID   int64     `json:"student_id"`
	StudentName string    `json:"student_name"`
	Action      string    `json:"action"`
	VisitID     *int64    `json:"visit_id,omitempty"`
	RoomName    string    `json:"room_name,omitempty"`
	ProcessedAt time.Time `json:"processed_at"`
	Message     string    `json:"message"`
	Status      string    `json:"status"`
}

// Bind validates the checkin request
func (req *CheckinRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StudentRFID, validation.Required),
		validation.Field(&req.Action, validation.Required, validation.In("checkin", "checkout")),
	)
}

// deviceCheckin handles student check-in/check-out requests from RFID devices
func (rs *Resource) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse request
	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Find student by RFID tag
	person, err := rs.UsersService.FindByTagID(r.Context(), req.StudentRFID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not found"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if person == nil || person.TagID == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("RFID tag not assigned to any person"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Get student details from person
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByPersonID(r.Context(), person.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("person is not a student"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	if student == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("person is not a student"))); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Load person details for student name
	student.Person = person

	// Check for existing active visit
	currentVisit, err := rs.ActiveService.GetStudentCurrentVisit(r.Context(), student.ID)
	if err != nil {
		// Log error but don't fail - student might not have any visits
		log.Printf("Error checking current visit: %v", err)
	}

	now := time.Now()
	var visitID *int64
	var actionMsg string
	var roomName string

	switch req.Action {
	case "checkin":
		// End previous visit if exists (auto-checkout)
		if currentVisit != nil && currentVisit.ExitTime == nil {
			if err := rs.ActiveService.EndVisit(r.Context(), currentVisit.ID); err != nil {
				log.Printf("Error ending previous visit: %v", err)
				// Continue anyway - we'll create a new visit
			}
		}

		// Determine which active group to associate with
		var activeGroupID int64
		if req.RoomID != nil {
			// Find active groups in the specified room
			activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(r.Context(), *req.RoomID)
			if err != nil {
				if err := render.Render(w, r, ErrorInternalServer(errors.New("error finding active groups in room"))); err != nil {
					log.Printf("Render error: %v", err)
				}
				return
			}

			if len(activeGroups) == 0 {
				if err := render.Render(w, r, ErrorNotFound(errors.New("no active groups in specified room"))); err != nil {
					log.Printf("Render error: %v", err)
				}
				return
			}

			// Use the first active group (in practice, there should typically be only one per room)
			activeGroupID = activeGroups[0].ID
			roomName = "Room " + string(rune(*req.RoomID)) // Simple room name - could be enhanced
		} else {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("room_id is required for check-in"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}

		// Create new visit
		newVisit := &active.Visit{
			StudentID:     student.ID,
			ActiveGroupID: activeGroupID,
			EntryTime:     now,
		}

		if err := rs.ActiveService.CreateVisit(r.Context(), newVisit); err != nil {
			if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to create visit record"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}

		visitID = &newVisit.ID
		actionMsg = "checked_in"

	case "checkout":
		// Check if student has an active visit to end
		if currentVisit == nil || currentVisit.ExitTime != nil {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("student is not currently checked in"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}

		// End current visit
		if err := rs.ActiveService.EndVisit(r.Context(), currentVisit.ID); err != nil {
			if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to end visit record"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}

		visitID = &currentVisit.ID
		actionMsg = "checked_out"
	}

	// Generate German greeting message
	studentName := person.FirstName + " " + person.LastName
	var greetingMsg string
	switch req.Action {
	case "checkin":
		greetingMsg = "Hallo " + person.FirstName + "!"
	case "checkout":
		greetingMsg = "Tschüss " + person.FirstName + "!"
	}

	// Prepare response
	response := map[string]interface{}{
		"student_id":    student.ID,
		"student_name":  studentName,
		"action":        actionMsg,
		"visit_id":      visitID,
		"room_name":     roomName,
		"processed_at":  now,
		"message":       greetingMsg,
		"status":        "success",
	}

	common.Respond(w, r, http.StatusOK, response, "Student "+actionMsg+" successfully")
}
