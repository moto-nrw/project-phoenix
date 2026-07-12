package iot

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	shared "github.com/moto-nrw/project-phoenix/api/iot/internal/shared"
	"github.com/moto-nrw/project-phoenix/models/iot"
)

const (
	msgDevicesRetrieved       = "Devices retrieved successfully"
	errMsgInvalidDeviceID     = "invalid device ID"
	errMsgDeviceIDRequired    = "device ID is required"
	errMsgDeviceTypeRequired  = "device type is required"
	errMsgStatusRequired      = "status is required"
	errMsgInvalidPersonID     = "invalid person ID"
	errMsgInvalidDeviceStatus = "invalid device status"
)

// listDevices handles listing all devices with optional filtering
func (rs *DevicesResource) listDevices(w http.ResponseWriter, r *http.Request) {
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
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevice handles getting a device by ID
func (rs *DevicesResource) getDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(r.Context(), rs.IoTService, device), "Device retrieved successfully")
}

// getDeviceByDeviceID handles getting a device by its device ID
func (rs *DevicesResource) getDeviceByDeviceID(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByDeviceID(r.Context(), deviceID)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(r.Context(), rs.IoTService, device), "Device retrieved successfully")
}

// createDevice handles creating a new device
func (rs *DevicesResource) createDevice(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
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
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	createdDevice, err := rs.IoTService.GetDeviceByID(r.Context(), device.ID)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newDeviceCreationResponse(r.Context(), rs.IoTService, createdDevice), "Device created successfully")
}

// updateDevice handles updating an existing device
func (rs *DevicesResource) updateDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Get existing device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
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
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	updatedDevice, err := rs.IoTService.GetDeviceByID(r.Context(), device.ID)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(r.Context(), rs.IoTService, updatedDevice), "Device updated successfully")
}

// deleteDevice handles deleting a device
func (rs *DevicesResource) deleteDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Delete device
	if err := rs.IoTService.DeleteDevice(r.Context(), id); err != nil {
		if common.IsConstraintViolation(err) {
			common.RenderError(w, r, common.ErrorConflictMessage("Gerät kann nicht gelöscht werden: Gerät wird aktuell von einer aktiven Gruppe verwendet"))
			return
		}
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device deleted successfully")
}

// updateDeviceStatus handles updating the status of a device
func (rs *DevicesResource) updateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Parse request
	req := &DeviceStatusRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	// Update device status
	if err := rs.IoTService.UpdateDeviceStatus(r.Context(), deviceID, iot.DeviceStatus(req.Status)); err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device status updated successfully")
}

// pingDevice handles pinging a device to update its last seen time
func (rs *DevicesResource) pingDevice(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Ping device
	if err := rs.IoTService.PingDevice(r.Context(), deviceID); err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device pinged successfully")
}

// getDevicesByType handles getting devices by type
func (rs *DevicesResource) getDevicesByType(w http.ResponseWriter, r *http.Request) {
	// Get type from URL
	deviceType := chi.URLParam(r, "type")
	if deviceType == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgDeviceTypeRequired)))
		return
	}

	// Get devices by type
	devices, err := rs.IoTService.GetDevicesByType(r.Context(), deviceType)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevicesByStatus handles getting devices by status
func (rs *DevicesResource) getDevicesByStatus(w http.ResponseWriter, r *http.Request) {
	// Get status from URL
	status := chi.URLParam(r, "status")
	if status == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgStatusRequired)))
		return
	}

	// Validate status
	deviceStatus := iot.DeviceStatus(status)
	if !isValidDeviceStatus(deviceStatus) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceStatus)))
		return
	}

	// Get devices by status
	devices, err := rs.IoTService.GetDevicesByStatus(r.Context(), deviceStatus)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevicesByRegisteredBy handles getting devices registered by a specific person
func (rs *DevicesResource) getDevicesByRegisteredBy(w http.ResponseWriter, r *http.Request) {
	// Parse person ID from URL
	personID, err := common.ParseIDParam(r, "personId")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New(errMsgInvalidPersonID)))
		return
	}

	// Get devices
	devices, err := rs.IoTService.GetDevicesByRegisteredBy(r.Context(), personID)
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getActiveDevices handles getting all active devices
func (rs *DevicesResource) getActiveDevices(w http.ResponseWriter, r *http.Request) {
	// Get active devices
	devices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, "Active devices retrieved successfully")
}

// getDevicesRequiringMaintenance handles getting devices requiring maintenance
func (rs *DevicesResource) getDevicesRequiringMaintenance(w http.ResponseWriter, r *http.Request) {
	// Get devices requiring maintenance
	devices, err := rs.IoTService.GetDevicesRequiringMaintenance(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, "Devices requiring maintenance retrieved successfully")
}

// getOfflineDevices handles getting offline devices
func (rs *DevicesResource) getOfflineDevices(w http.ResponseWriter, r *http.Request) {
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
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, "Offline devices retrieved successfully")
}

// getDeviceStatistics handles getting device statistics
func (rs *DevicesResource) getDeviceStatistics(w http.ResponseWriter, r *http.Request) {
	// Get device type statistics
	typeStats, err := rs.IoTService.GetDeviceTypeStatistics(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Get active devices count
	activeDevices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Get offline devices count using the same tenant-configured online window
	// used by per-device is_online responses.
	offlineDevices, err := rs.IoTService.GetOfflineDevices(r.Context(), rs.IoTService.DeviceOnlineWindow(r.Context()))
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
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
func (rs *DevicesResource) detectNewDevices(w http.ResponseWriter, r *http.Request) {
	// Detect new devices
	devices, err := rs.IoTService.DetectNewDevices(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(r.Context(), rs.IoTService, devices)

	common.Respond(w, r, http.StatusOK, responses, "Device detection completed")
}

// scanNetwork handles scanning the network for IoT devices
func (rs *DevicesResource) scanNetwork(w http.ResponseWriter, r *http.Request) {
	// Scan network
	scanResults, err := rs.IoTService.ScanNetwork(r.Context())
	if err != nil {
		common.RenderError(w, r, shared.ErrorRenderer(err))
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
