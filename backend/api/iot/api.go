package iot

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/facilities"
	"github.com/moto-nrw/project-phoenix/models/iot"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	facilitiesSvc "github.com/moto-nrw/project-phoenix/services/facilities"
	iotSvc "github.com/moto-nrw/project-phoenix/services/iot"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
)

// Resource defines the IoT API resource
type Resource struct {
	IoTService        iotSvc.Service
	UsersService      usersSvc.PersonService
	ActiveService     activeSvc.Service
	ActivitiesService activitiesSvc.ActivityService
	ConfigService     configSvc.Service
	FacilityService   facilitiesSvc.Service
}

// NewResource creates a new IoT resource
func NewResource(iotService iotSvc.Service, usersService usersSvc.PersonService, activeService activeSvc.Service, activitiesService activitiesSvc.ActivityService, configService configSvc.Service, facilityService facilitiesSvc.Service) *Resource {
	return &Resource{
		IoTService:        iotService,
		UsersService:      usersService,
		ActiveService:     activeService,
		ActivitiesService: activitiesService,
		ConfigService:     configService,
		FacilityService:   facilityService,
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

	// Device-only authenticated routes (API key only, no PIN required)
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceOnlyAuthenticator(rs.IoTService))

		// Get available teachers for device login selection
		r.Get("/teachers", rs.getAvailableTeachers)
	})

	// Device-authenticated routes for RFID devices
	r.Group(func(r chi.Router) {
		r.Use(device.DeviceAuthenticator(rs.IoTService, rs.UsersService))

		// Device endpoints that require device API key + staff PIN authentication
		r.Post("/ping", rs.devicePing)
		r.Post("/checkin", rs.deviceCheckin)
		r.Get("/status", rs.deviceStatus)
		r.Get("/students", rs.getTeacherStudents)
		r.Get("/activities", rs.getTeacherActivities)
		r.Get("/rooms/available", rs.getAvailableRoomsForDevice)
		r.Get("/rfid/{tagId}", rs.checkRFIDTagAssignment)

		// Activity session management
		r.Post("/session/start", rs.startActivitySession)
		r.Post("/session/end", rs.endActivitySession)
		r.Get("/session/current", rs.getCurrentSession)
		r.Post("/session/check-conflict", rs.checkSessionConflict)

		// Session timeout management
		r.Post("/session/timeout", rs.processSessionTimeout)
		r.Get("/session/timeout-config", rs.getSessionTimeoutConfig)
		r.Post("/session/activity", rs.updateSessionActivity)
		r.Post("/session/validate-timeout", rs.validateSessionTimeout)
		r.Get("/session/timeout-info", rs.getSessionTimeoutInfo)
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

// DeviceCreationResponse represents a device creation response with API key
type DeviceCreationResponse struct {
	DeviceResponse
	APIKey string `json:"api_key"` // Only included during creation
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

// newDeviceCreationResponse converts a device model to a creation response object with API key
func newDeviceCreationResponse(device *iot.Device) DeviceCreationResponse {
	response := DeviceCreationResponse{
		DeviceResponse: newDeviceResponse(device),
	}

	// Include API key only during creation
	if device.APIKey != nil {
		response.APIKey = *device.APIKey
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

	common.Respond(w, r, http.StatusCreated, newDeviceCreationResponse(device), "Device created successfully")
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

// Device-only authenticated handlers (API key only, no PIN required)

// getAvailableTeachers handles getting the list of teachers available for device login selection
func (rs *Resource) getAvailableTeachers(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context required)
	deviceCtx := device.DeviceFromCtx(r.Context())
	if deviceCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get all staff members who are teachers
	staffMembers, err := rs.UsersService.StaffRepository().List(r.Context(), nil)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Build response with teachers who have PINs set
	responses := make([]DeviceTeacherResponse, 0)
	teacherRepo := rs.UsersService.TeacherRepository()

	for _, staff := range staffMembers {
		// Check if this staff member is a teacher
		teacher, err := teacherRepo.FindByStaffID(r.Context(), staff.ID)
		if err != nil || teacher == nil {
			continue // Skip non-teachers
		}

		// Get person details
		person, err := rs.UsersService.Get(r.Context(), staff.PersonID)
		if err != nil || person == nil {
			continue // Skip if person not found
		}

		// Check if person has an account with a PIN set
		if person.Account == nil || !person.Account.HasPIN() {
			continue // Skip teachers without PINs
		}

		// Create teacher response
		response := DeviceTeacherResponse{
			StaffID:     staff.ID,
			PersonID:    person.ID,
			FirstName:   person.FirstName,
			LastName:    person.LastName,
			DisplayName: fmt.Sprintf("%s %s", person.FirstName, person.LastName),
		}

		responses = append(responses, response)
	}

	// Log device access for audit trail
	log.Printf("Device %s requested teacher list, returned %d teachers", deviceCtx.DeviceID, len(responses))

	common.Respond(w, r, http.StatusOK, responses, "Available teachers retrieved successfully")
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
		"device_id":   deviceCtx.DeviceID,
		"device_name": deviceCtx.Name,
		"status":      deviceCtx.Status,
		"staff_id":    staffCtx.ID,
		"person_id":   staffCtx.PersonID,
		"last_seen":   deviceCtx.LastSeen,
		"is_online":   deviceCtx.IsOnline(),
		"ping_time":   time.Now(),
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

// DeviceTeacherResponse represents a teacher available for device login selection
type DeviceTeacherResponse struct {
	StaffID     int64  `json:"staff_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DisplayName string `json:"display_name"`
}

// TeacherStudentResponse represents a student supervised by a teacher for RFID devices
type TeacherStudentResponse struct {
	StudentID   int64  `json:"student_id"`
	PersonID    int64  `json:"person_id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GroupName   string `json:"group_name"`
	RFIDTag     string `json:"rfid_tag,omitempty"`
}

// DeviceActivityResponse represents an activity available for teacher selection on RFID devices
type DeviceActivityResponse struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	CategoryName    string `json:"category_name"`
	CategoryColor   string `json:"category_color,omitempty"`
	RoomName        string `json:"room_name,omitempty"`
	EnrollmentCount int    `json:"enrollment_count"`
	MaxParticipants int    `json:"max_participants"`
	HasSpots        bool   `json:"has_spots"`
	SupervisorName  string `json:"supervisor_name"`
	IsActive        bool   `json:"is_active"`
}

// DeviceRoomResponse represents a room available for RFID device selection
type DeviceRoomResponse struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Building string `json:"building,omitempty"`
	Floor    int    `json:"floor"`
	Capacity int    `json:"capacity"`
	Category string `json:"category"`
	Color    string `json:"color"`
}

// RFIDTagAssignmentResponse represents RFID tag assignment status
type RFIDTagAssignmentResponse struct {
	Assigned bool                          `json:"assigned"`
	Student  *RFIDTagAssignedStudent       `json:"student,omitempty"`
}

// RFIDTagAssignedStudent represents student info for assigned RFID tag
type RFIDTagAssignedStudent struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Group string `json:"group"`
}

// Bind validates the checkin request
func (req *CheckinRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.StudentRFID, validation.Required),
		// Note: Action field is ignored in logic but still required for API compatibility
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

	// Debug logging to track the action field
	log.Printf("RFID Request: action='%s', student_rfid='%s', room_id=%v", req.Action, req.StudentRFID, req.RoomID)

	// Auto-determine action based on student's current status (ignore req.Action)
	// If student has an active visit, perform checkout; otherwise perform checkin
	if currentVisit != nil && currentVisit.ExitTime == nil {
		// Student is currently checked in - perform CHECKOUT
		log.Printf("Student %d has active visit %d - performing checkout", student.ID, currentVisit.ID)
		
		// End current visit
		if err := rs.ActiveService.EndVisit(r.Context(), currentVisit.ID); err != nil {
			if err := render.Render(w, r, ErrorInternalServer(errors.New("failed to end visit record"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}

		visitID = &currentVisit.ID
		actionMsg = "checked_out"
	} else {
		// Student is not currently checked in - perform CHECKIN
		log.Printf("Student %d has no active visit - performing checkin", student.ID)
		
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
	}

	// Generate German greeting message based on actual action performed
	studentName := person.FirstName + " " + person.LastName
	var greetingMsg string
	switch actionMsg {
	case "checked_in":
		greetingMsg = "Hallo " + person.FirstName + "!"
	case "checked_out":
		greetingMsg = "Tschüss " + person.FirstName + "!"
	}

	// Update session activity when student scans (for monitoring only)
	if req.RoomID != nil {
		if activeGroups, err := rs.ActiveService.FindActiveGroupsByRoomID(r.Context(), *req.RoomID); err == nil {
			for _, group := range activeGroups {
				// Only update activity for device-managed sessions
				if group.DeviceID != nil && *group.DeviceID == deviceCtx.ID {
					if updateErr := rs.ActiveService.UpdateSessionActivity(r.Context(), group.ID); updateErr != nil {
						log.Printf("Warning: Failed to update session activity for group %d: %v", group.ID, updateErr)
						// Don't fail the request - this is just for monitoring
					}
					break // Only update the matching device session
				}
			}
		}
	}

	// Debug logging before sending response
	log.Printf("RFID Response: action='%s', student='%s', message='%s'", actionMsg, studentName, greetingMsg)

	// Prepare response
	response := map[string]interface{}{
		"student_id":   student.ID,
		"student_name": studentName,
		"action":       actionMsg,
		"visit_id":     visitID,
		"room_name":    roomName,
		"processed_at": now,
		"message":      greetingMsg,
		"status":       "success",
	}

	common.Respond(w, r, http.StatusOK, response, "Student "+actionMsg+" successfully")
}

// getTeacherStudents handles getting students supervised by the authenticated teacher (for RFID devices)
func (rs *Resource) getTeacherStudents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Find teacher for authenticated staff
	teacherRepo := rs.UsersService.TeacherRepository()
	teacher, err := teacherRepo.FindByStaffID(r.Context(), staffCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("teacher not found for authenticated staff"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	if teacher == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("teacher not found for authenticated staff"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get students supervised by this teacher with group information
	studentsWithGroups, err := rs.UsersService.GetStudentsWithGroupsByTeacher(r.Context(), teacher.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert students to device response format
	responses := make([]TeacherStudentResponse, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		student := swg.Student
		response := TeacherStudentResponse{
			StudentID:   student.ID,
			PersonID:    student.PersonID,
			SchoolClass: student.SchoolClass,
			GroupName:   swg.GroupName,
		}

		// Add person details if available
		if student.Person != nil {
			response.FirstName = student.Person.FirstName
			response.LastName = student.Person.LastName

			// Add RFID tag if available
			if student.Person.TagID != nil {
				response.RFIDTag = *student.Person.TagID
			}
		}

		responses = append(responses, response)
	}

	common.Respond(w, r, http.StatusOK, responses, "Teacher students retrieved successfully")
}

// convertToDeviceActivityResponse converts an activity group to device response format
func convertToDeviceActivityResponse(group *activities.Group, enrollmentCount int, supervisorName string) DeviceActivityResponse {
	response := DeviceActivityResponse{
		ID:              group.ID,
		Name:            group.Name,
		EnrollmentCount: enrollmentCount,
		MaxParticipants: group.MaxParticipants,
		HasSpots:        enrollmentCount < group.MaxParticipants,
		SupervisorName:  supervisorName,
		IsActive:        group.IsOpen,
	}

	if group.Category != nil {
		response.CategoryName = group.Category.Name
		response.CategoryColor = group.Category.Color
	}

	if group.PlannedRoom != nil {
		response.RoomName = group.PlannedRoom.Name
	}

	return response
}

// newDeviceRoomResponse converts a facilities.Room to DeviceRoomResponse format
func newDeviceRoomResponse(room *facilities.Room) DeviceRoomResponse {
	return DeviceRoomResponse{
		ID:       room.ID,
		Name:     room.Name,
		Building: room.Building,
		Floor:    room.Floor,
		Capacity: room.Capacity,
		Category: room.Category,
		Color:    room.Color,
	}
}

// getTeacherActivities handles getting activities supervised by the authenticated teacher (for RFID devices)
func (rs *Resource) getTeacherActivities(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Find teacher for authenticated staff
	teacherRepo := rs.UsersService.TeacherRepository()
	teacher, err := teacherRepo.FindByStaffID(r.Context(), staffCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("teacher not found for authenticated staff"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}
	if teacher == nil {
		if err := render.Render(w, r, ErrorNotFound(errors.New("teacher not found for authenticated staff"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get teacher's activities for today
	activityGroups, err := rs.ActivitiesService.GetTeacherTodaysActivities(r.Context(), staffCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to device response format
	responses := make([]DeviceActivityResponse, 0, len(activityGroups))
	for _, activityGroup := range activityGroups {
		// Get enrollment count - for now using 0, this could be optimized with batch query
		enrollmentCount := 0 // TODO: Implement enrollment counting if needed

		// Get supervisor name from teacher context
		supervisorName := ""
		if teacher.Staff != nil && teacher.Staff.Person != nil {
			supervisorName = fmt.Sprintf("%s %s", teacher.Staff.Person.FirstName, teacher.Staff.Person.LastName)
		}

		responses = append(responses, convertToDeviceActivityResponse(activityGroup, enrollmentCount, supervisorName))
	}

	common.Respond(w, r, http.StatusOK, responses, "Teacher activities retrieved successfully")
}

// getAvailableRoomsForDevice handles getting available rooms for RFID devices
func (rs *Resource) getAvailableRoomsForDevice(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Parse capacity parameter if provided
	capacity := 0
	if capacityStr := r.URL.Query().Get("capacity"); capacityStr != "" {
		if cap, err := strconv.Atoi(capacityStr); err == nil && cap > 0 {
			capacity = cap
		}
	}

	// Get available rooms from facility service
	rooms, err := rs.FacilityService.GetAvailableRooms(r.Context(), capacity)
	if err != nil {
		if err := render.Render(w, r, ErrorInternalServer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Convert to device response format
	responses := make([]DeviceRoomResponse, 0, len(rooms))
	for _, room := range rooms {
		responses = append(responses, newDeviceRoomResponse(room))
	}

	common.Respond(w, r, http.StatusOK, responses, "Available rooms retrieved successfully")
}

// Activity Session Management Handlers

// SessionStartRequest represents a request to start an activity session
type SessionStartRequest struct {
	ActivityID int64 `json:"activity_id"`
	Force      bool  `json:"force,omitempty"`
}

// SessionStartResponse represents the response when starting an activity session
type SessionStartResponse struct {
	ActiveGroupID int64                 `json:"active_group_id"`
	ActivityID    int64                 `json:"activity_id"`
	DeviceID      int64                 `json:"device_id"`
	StartTime     time.Time             `json:"start_time"`
	ConflictInfo  *ConflictInfoResponse `json:"conflict_info,omitempty"`
	Status        string                `json:"status"`
	Message       string                `json:"message"`
}

// ConflictInfoResponse represents conflict information for API responses
type ConflictInfoResponse struct {
	HasConflict       bool   `json:"has_conflict"`
	ConflictingDevice *int64 `json:"conflicting_device,omitempty"`
	ConflictMessage   string `json:"conflict_message"`
	CanOverride       bool   `json:"can_override"`
}

// SessionTimeoutResponse represents the result of processing a session timeout
type SessionTimeoutResponse struct {
	SessionID          int64     `json:"session_id"`
	ActivityID         int64     `json:"activity_id"`
	StudentsCheckedOut int       `json:"students_checked_out"`
	TimeoutAt          time.Time `json:"timeout_at"`
	Status             string    `json:"status"`
	Message            string    `json:"message"`
}

// SessionTimeoutConfig represents timeout configuration for devices
type SessionTimeoutConfig struct {
	TimeoutMinutes       int `json:"timeout_minutes"`
	WarningMinutes       int `json:"warning_minutes"`
	CheckIntervalSeconds int `json:"check_interval_seconds"`
}

// SessionActivityRequest represents a session activity update request
type SessionActivityRequest struct {
	ActivityType string    `json:"activity_type"` // "rfid_scan", "button_press", "ui_interaction"
	Timestamp    time.Time `json:"timestamp"`
}

// Bind validates the session activity request
func (req *SessionActivityRequest) Bind(r *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.ActivityType, validation.Required, validation.In("rfid_scan", "button_press", "ui_interaction")),
	); err != nil {
		return err
	}

	// Set timestamp to now if not provided
	if req.Timestamp.IsZero() {
		req.Timestamp = time.Now()
	}

	return nil
}

// TimeoutValidationRequest represents a timeout validation request
type TimeoutValidationRequest struct {
	TimeoutMinutes int       `json:"timeout_minutes"`
	LastActivity   time.Time `json:"last_activity"`
}

// Bind validates the timeout validation request
func (req *TimeoutValidationRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.TimeoutMinutes, validation.Required, validation.Min(1), validation.Max(480)),
		validation.Field(&req.LastActivity, validation.Required),
	)
}

// SessionTimeoutInfoResponse provides comprehensive timeout information
type SessionTimeoutInfoResponse struct {
	SessionID               int64     `json:"session_id"`
	ActivityID              int64     `json:"activity_id"`
	StartTime               time.Time `json:"start_time"`
	LastActivity            time.Time `json:"last_activity"`
	TimeoutMinutes          int       `json:"timeout_minutes"`
	InactivitySeconds       int       `json:"inactivity_seconds"`
	TimeUntilTimeoutSeconds int       `json:"time_until_timeout_seconds"`
	IsTimedOut              bool      `json:"is_timed_out"`
	ActiveStudentCount      int       `json:"active_student_count"`
}

// SessionCurrentResponse represents the current session information
type SessionCurrentResponse struct {
	ActiveGroupID   *int64     `json:"active_group_id,omitempty"`
	ActivityID      *int64     `json:"activity_id,omitempty"`
	ActivityName    *string    `json:"activity_name,omitempty"`
	RoomID          *int64     `json:"room_id,omitempty"`
	RoomName        *string    `json:"room_name,omitempty"`
	DeviceID        int64      `json:"device_id"`
	StartTime       *time.Time `json:"start_time,omitempty"`
	Duration        *string    `json:"duration,omitempty"`
	IsActive        bool       `json:"is_active"`
	ActiveStudents  *int       `json:"active_students,omitempty"`
}

// Bind validates the session start request
func (req *SessionStartRequest) Bind(r *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.ActivityID, validation.Required, validation.Min(int64(1))),
	)
}

// startActivitySession handles starting an activity session on a device
func (rs *Resource) startActivitySession(w http.ResponseWriter, r *http.Request) {
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
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	var activeGroup *active.Group
	var err error

	if req.Force {
		// Force start with override
		activeGroup, err = rs.ActiveService.ForceStartActivitySession(r.Context(), req.ActivityID, deviceCtx.ID, staffCtx.ID)
	} else {
		// Normal start with conflict detection
		activeGroup, err = rs.ActiveService.StartActivitySession(r.Context(), req.ActivityID, deviceCtx.ID, staffCtx.ID)
	}

	if err != nil {
		// Check if this is a conflict error and provide conflict info
		if errors.Is(err, activeSvc.ErrSessionConflict) || errors.Is(err, activeSvc.ErrActivityAlreadyActive) || errors.Is(err, activeSvc.ErrDeviceAlreadyActive) {
			// Get conflict details
			conflictInfo, conflictErr := rs.ActiveService.CheckActivityConflict(r.Context(), req.ActivityID, deviceCtx.ID)
			if conflictErr == nil && conflictInfo.HasConflict {
				response := SessionStartResponse{
					Status:  "conflict",
					Message: conflictInfo.ConflictMessage,
					ConflictInfo: &ConflictInfoResponse{
						HasConflict:     conflictInfo.HasConflict,
						ConflictMessage: conflictInfo.ConflictMessage,
						CanOverride:     conflictInfo.CanOverride,
					},
				}
				if conflictInfo.ConflictingDevice != nil {
					if deviceID, parseErr := strconv.ParseInt(*conflictInfo.ConflictingDevice, 10, 64); parseErr == nil {
						response.ConflictInfo.ConflictingDevice = &deviceID
					}
				}
				common.Respond(w, r, http.StatusConflict, response, "Session conflict detected")
				return
			}
		}

		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Success response
	response := SessionStartResponse{
		ActiveGroupID: activeGroup.ID,
		ActivityID:    activeGroup.GroupID,
		DeviceID:      deviceCtx.ID,
		StartTime:     activeGroup.StartTime,
		Status:        "started",
		Message:       "Activity session started successfully",
	}

	common.Respond(w, r, http.StatusOK, response, "Activity session started successfully")
}

// endActivitySession handles ending the current activity session on a device
func (rs *Resource) endActivitySession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get current session for this device
	currentSession, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)
	if err != nil {
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
			if err := render.Render(w, r, ErrorInvalidRequest(errors.New("no active session to end"))); err != nil {
				log.Printf("Render error: %v", err)
			}
			return
		}
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// End the session
	if err := rs.ActiveService.EndActivitySession(r.Context(), currentSession.ID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := map[string]interface{}{
		"active_group_id": currentSession.ID,
		"activity_id":     currentSession.GroupID,
		"device_id":       deviceCtx.ID,
		"ended_at":        time.Now(),
		"duration":        time.Since(currentSession.StartTime).String(),
		"status":          "ended",
		"message":         "Activity session ended successfully",
	}

	common.Respond(w, r, http.StatusOK, response, "Activity session ended successfully")
}

// getCurrentSession handles getting the current session information for a device
func (rs *Resource) getCurrentSession(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get current session for this device
	currentSession, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)

	response := SessionCurrentResponse{
		DeviceID: deviceCtx.ID,
		IsActive: false,
	}

	if err != nil {
		if errors.Is(err, activeSvc.ErrNoActiveSession) {
			// No active session - return empty response with IsActive: false
			common.Respond(w, r, http.StatusOK, response, "No active session")
			return
		}
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Session found - populate response
	response.IsActive = true
	response.ActiveGroupID = &currentSession.ID
	response.ActivityID = &currentSession.GroupID
	response.RoomID = &currentSession.RoomID
	response.StartTime = &currentSession.StartTime
	duration := time.Since(currentSession.StartTime).String()
	response.Duration = &duration

	// Add activity name if available
	if currentSession.ActualGroup != nil {
		response.ActivityName = &currentSession.ActualGroup.Name
	}

	// Add room name if available
	if currentSession.Room != nil {
		response.RoomName = &currentSession.Room.Name
	}

	// Get active student count for this session
	activeVisits, err := rs.ActiveService.FindVisitsByActiveGroupID(r.Context(), currentSession.ID)
	if err != nil {
		// Log error but don't fail the request - student count is optional info
		log.Printf("Warning: Failed to get active student count for session %d: %v", currentSession.ID, err)
	} else {
		// Count visits without exit_time (active students)
		activeCount := 0
		for _, visit := range activeVisits {
			if visit.ExitTime == nil {
				activeCount++
			}
		}
		response.ActiveStudents = &activeCount
	}

	common.Respond(w, r, http.StatusOK, response, "Current session retrieved successfully")
}

// checkSessionConflict handles checking for conflicts before starting a session
func (rs *Resource) checkSessionConflict(w http.ResponseWriter, r *http.Request) {
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
	req := &SessionStartRequest{}
	if err := render.Bind(r, req); err != nil {
		if err := render.Render(w, r, ErrorInvalidRequest(err)); err != nil {
			log.Printf("Render error: %v", err)
		}
		return
	}

	// Check for conflicts
	conflictInfo, err := rs.ActiveService.CheckActivityConflict(r.Context(), req.ActivityID, deviceCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := ConflictInfoResponse{
		HasConflict:     conflictInfo.HasConflict,
		ConflictMessage: conflictInfo.ConflictMessage,
		CanOverride:     conflictInfo.CanOverride,
	}

	if conflictInfo.ConflictingDevice != nil {
		if deviceID, parseErr := strconv.ParseInt(*conflictInfo.ConflictingDevice, 10, 64); parseErr == nil {
			response.ConflictingDevice = &deviceID
		}
	}

	statusCode := http.StatusOK
	message := "No conflicts detected"
	if conflictInfo.HasConflict {
		statusCode = http.StatusConflict
		message = "Conflict detected"
	}

	common.Respond(w, r, statusCode, response, message)
}

// processSessionTimeout handles device timeout notification
func (rs *Resource) processSessionTimeout(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	// Process timeout via device ID
	result, err := rs.ActiveService.ProcessSessionTimeout(r.Context(), deviceCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := SessionTimeoutResponse{
		SessionID:          result.SessionID,
		ActivityID:         result.ActivityID,
		StudentsCheckedOut: result.StudentsCheckedOut,
		TimeoutAt:          result.TimeoutAt,
		Status:             "completed",
		Message:            fmt.Sprintf("Session ended due to timeout. %d students checked out.", result.StudentsCheckedOut),
	}

	common.Respond(w, r, http.StatusOK, response, "Session timeout processed successfully")
}

// getSessionTimeoutConfig returns timeout configuration for the requesting device
func (rs *Resource) getSessionTimeoutConfig(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	settings, err := rs.ConfigService.GetDeviceTimeoutSettings(r.Context(), deviceCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	config := SessionTimeoutConfig{
		TimeoutMinutes:       settings.GetEffectiveTimeoutMinutes(),
		WarningMinutes:       settings.WarningThresholdMinutes,
		CheckIntervalSeconds: settings.CheckIntervalSeconds,
	}

	common.Respond(w, r, http.StatusOK, config, "Timeout configuration retrieved")
}

// updateSessionActivity handles activity updates for timeout tracking
func (rs *Resource) updateSessionActivity(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	var req SessionActivityRequest
	if err := render.Bind(r, &req); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Get current session for this device
	session, err := rs.ActiveService.GetDeviceCurrentSession(r.Context(), deviceCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Update session activity
	if err := rs.ActiveService.UpdateSessionActivity(r.Context(), session.ID); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := map[string]interface{}{
		"session_id":    session.ID,
		"activity_type": req.ActivityType,
		"updated_at":    time.Now(),
		"last_activity": time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Session activity updated")
}

// validateSessionTimeout validates if a timeout request is legitimate
func (rs *Resource) validateSessionTimeout(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	var req TimeoutValidationRequest
	if err := render.Bind(r, &req); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Validate the timeout request
	if err := rs.ActiveService.ValidateSessionTimeout(r.Context(), deviceCtx.ID, req.TimeoutMinutes); err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := map[string]interface{}{
		"valid":           true,
		"timeout_minutes": req.TimeoutMinutes,
		"last_activity":   req.LastActivity,
		"validated_at":    time.Now(),
	}

	common.Respond(w, r, http.StatusOK, response, "Timeout validation successful")
}

// getSessionTimeoutInfo provides comprehensive timeout information
func (rs *Resource) getSessionTimeoutInfo(w http.ResponseWriter, r *http.Request) {
	deviceCtx := device.DeviceFromCtx(r.Context())

	info, err := rs.ActiveService.GetSessionTimeoutInfo(r.Context(), deviceCtx.ID)
	if err != nil {
		if err := render.Render(w, r, ErrorRenderer(err)); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	response := SessionTimeoutInfoResponse{
		SessionID:               info.SessionID,
		ActivityID:              info.ActivityID,
		StartTime:               info.StartTime,
		LastActivity:            info.LastActivity,
		TimeoutMinutes:          info.TimeoutMinutes,
		InactivitySeconds:       int(info.InactivityDuration.Seconds()),
		TimeUntilTimeoutSeconds: int(info.TimeUntilTimeout.Seconds()),
		IsTimedOut:              info.IsTimedOut,
		ActiveStudentCount:      info.ActiveStudentCount,
	}

	common.Respond(w, r, http.StatusOK, response, "Session timeout information retrieved")
}

// checkRFIDTagAssignment handles checking RFID tag assignment status
func (rs *Resource) checkRFIDTagAssignment(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	deviceCtx := device.DeviceFromCtx(r.Context())
	staffCtx := device.StaffFromCtx(r.Context())

	if deviceCtx == nil || staffCtx == nil {
		if err := render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)); err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	// Get tagId from URL parameter
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		if err := render.Render(w, r, ErrorInvalidRequest(errors.New("tagId parameter is required"))); err != nil {
			log.Printf("Error rendering error response: %v", err)
		}
		return
	}

	// Normalize the tag ID to match the stored format (same logic as in person repository)
	normalizedTagID := normalizeTagID(tagID)

	// Find person by RFID tag using existing service method
	person, err := rs.UsersService.FindByTagID(r.Context(), normalizedTagID)
	if err != nil {
		// Handle case where tag is not assigned to anyone (no person found)
		log.Printf("Warning: No person found for RFID tag %s: %v", tagID, err)
		// Continue with response.Assigned = false (tag not assigned to anyone)
		person = nil
	}

	// Prepare response for unassigned tag
	response := RFIDTagAssignmentResponse{
		Assigned: false,
	}

	// If person found and has this tag, check if they're a student
	if person != nil && person.TagID != nil && *person.TagID == normalizedTagID {
		// Get student details using existing repository
		studentRepo := rs.UsersService.StudentRepository()
		student, err := studentRepo.FindByPersonID(r.Context(), person.ID)
		
		// Handle case where person exists but is not a student (no error, just nil result)
		if err != nil {
			// Only treat as error if it's not a "no rows found" situation
			log.Printf("Warning: Error finding student for person %d: %v", person.ID, err)
			// Continue with response.Assigned = false (person exists but not a student)
		} else if student != nil {
			// Person is a student, return assignment info
			response.Assigned = true
			response.Student = &RFIDTagAssignedStudent{
				ID:    student.ID,
				Name:  person.FirstName + " " + person.LastName,
				Group: student.SchoolClass, // Use school class as group identifier
			}
		}
		// If student == nil, the person exists but is not a student (keep response.Assigned = false)
	}

	common.Respond(w, r, http.StatusOK, response, "RFID tag assignment status retrieved")
}

// normalizeTagID normalizes RFID tag ID format (same logic as in person repository)
func normalizeTagID(tagID string) string {
	// Trim spaces
	tagID = strings.TrimSpace(tagID)
	
	// Remove common separators
	tagID = strings.ReplaceAll(tagID, ":", "")
	tagID = strings.ReplaceAll(tagID, "-", "")
	tagID = strings.ReplaceAll(tagID, " ", "")
	
	// Convert to uppercase
	return strings.ToUpper(tagID)
}
