package devices

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
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
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevice handles getting a device by ID
func (rs *Resource) getDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device retrieved successfully")
}

// getDeviceByDeviceID handles getting a device by its device ID
func (rs *Resource) getDeviceByDeviceID(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Get device
	device, err := rs.IoTService.GetDeviceByDeviceID(r.Context(), deviceID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device retrieved successfully")
}

// createDevice handles creating a new device
func (rs *Resource) createDevice(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
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
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusCreated, newDeviceCreationResponse(device), "Device created successfully")
}

// updateDevice handles updating an existing device
func (rs *Resource) updateDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Parse request
	req := &DeviceRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Get existing device
	device, err := rs.IoTService.GetDeviceByID(r.Context(), id)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
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
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newDeviceResponse(device), "Device updated successfully")
}

// deleteDevice handles deleting a device
func (rs *Resource) deleteDevice(w http.ResponseWriter, r *http.Request) {
	// Parse ID from URL
	id, err := common.ParseID(r)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceID)))
		return
	}

	// Delete device
	if err := rs.IoTService.DeleteDevice(r.Context(), id); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device deleted successfully")
}

// updateDeviceStatus handles updating the status of a device
func (rs *Resource) updateDeviceStatus(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Parse request
	req := &DeviceStatusRequest{}
	if err := render.Bind(r, req); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	// Update device status
	if err := rs.IoTService.UpdateDeviceStatus(r.Context(), deviceID, iot.DeviceStatus(req.Status)); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device status updated successfully")
}

// pingDevice handles pinging a device to update its last seen time
func (rs *Resource) pingDevice(w http.ResponseWriter, r *http.Request) {
	// Get device ID from URL
	deviceID := chi.URLParam(r, "deviceId")
	if deviceID == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgDeviceIDRequired)))
		return
	}

	// Ping device
	if err := rs.IoTService.PingDevice(r.Context(), deviceID); err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Device pinged successfully")
}

// getDevicesByType handles getting devices by type
func (rs *Resource) getDevicesByType(w http.ResponseWriter, r *http.Request) {
	// Get type from URL
	deviceType := chi.URLParam(r, "type")
	if deviceType == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgDeviceTypeRequired)))
		return
	}

	// Get devices by type
	devices, err := rs.IoTService.GetDevicesByType(r.Context(), deviceType)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevicesByStatus handles getting devices by status
func (rs *Resource) getDevicesByStatus(w http.ResponseWriter, r *http.Request) {
	// Get status from URL
	status := chi.URLParam(r, "status")
	if status == "" {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgStatusRequired)))
		return
	}

	// Validate status
	deviceStatus := iot.DeviceStatus(status)
	if !isValidDeviceStatus(deviceStatus) {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgInvalidDeviceStatus)))
		return
	}

	// Get devices by status
	devices, err := rs.IoTService.GetDevicesByStatus(r.Context(), deviceStatus)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getDevicesByRegisteredBy handles getting devices registered by a specific person
func (rs *Resource) getDevicesByRegisteredBy(w http.ResponseWriter, r *http.Request) {
	// Parse person ID from URL
	personID, err := common.ParseIDParam(r, "personId")
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(errors.New(errMsgInvalidPersonID)))
		return
	}

	// Get devices
	devices, err := rs.IoTService.GetDevicesByRegisteredBy(r.Context(), personID)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, msgDevicesRetrieved)
}

// getActiveDevices handles getting all active devices
func (rs *Resource) getActiveDevices(w http.ResponseWriter, r *http.Request) {
	// Get active devices
	devices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, "Active devices retrieved successfully")
}

// getDevicesRequiringMaintenance handles getting devices requiring maintenance
func (rs *Resource) getDevicesRequiringMaintenance(w http.ResponseWriter, r *http.Request) {
	// Get devices requiring maintenance
	devices, err := rs.IoTService.GetDevicesRequiringMaintenance(r.Context())
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

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
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, "Offline devices retrieved successfully")
}

// getDeviceStatistics handles getting device statistics
func (rs *Resource) getDeviceStatistics(w http.ResponseWriter, r *http.Request) {
	// Get device type statistics
	typeStats, err := rs.IoTService.GetDeviceTypeStatistics(r.Context())
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Get active devices count
	activeDevices, err := rs.IoTService.GetActiveDevices(r.Context())
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Get offline devices count (devices offline for more than 5 minutes)
	offlineDevices, err := rs.IoTService.GetOfflineDevices(r.Context(), 5*time.Minute)
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
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
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Build response
	responses := newDeviceResponses(devices)

	common.Respond(w, r, http.StatusOK, responses, "Device detection completed")
}

// scanNetwork handles scanning the network for IoT devices
func (rs *Resource) scanNetwork(w http.ResponseWriter, r *http.Request) {
	// Scan network
	scanResults, err := rs.IoTService.ScanNetwork(r.Context())
	if err != nil {
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
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
