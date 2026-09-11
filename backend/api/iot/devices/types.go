package devices

import (
	"context"
	"errors"
	"net/http"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	devicefleet "github.com/moto-nrw/project-phoenix/modules/devicefleet"
)

// DeviceResponse represents a device API response
type DeviceResponse struct {
	ID             int64     `json:"id"`
	DeviceID       string    `json:"device_id"`
	DeviceType     string    `json:"device_type"`
	Name           *string   `json:"name,omitempty"`
	Status         string    `json:"status"`
	LastSeen       *wireTime `json:"last_seen,omitempty"`
	RegisteredByID *int64    `json:"registered_by_id,omitempty"`
	RoomName       *string   `json:"room_name,omitempty"`
	IsOnline       bool      `json:"is_online"`
	CreatedAt      wireTime  `json:"created_at"`
	UpdatedAt      wireTime  `json:"updated_at"`
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
func (req *DeviceRequest) Bind(_ *http.Request) error {
	if err := validation.ValidateStruct(req,
		validation.Field(&req.DeviceID, validation.Required),
		validation.Field(&req.DeviceType, validation.Required),
	); err != nil {
		return err
	}

	// Validate status only if provided
	if req.Status != "" {
		if !isValidDeviceStatus(devicefleet.DeviceStatus(req.Status)) {
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
func (req *DeviceStatusRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.Status, validation.Required, validation.In(
			string(devicefleet.DeviceStatusActive),
			string(devicefleet.DeviceStatusInactive),
			string(devicefleet.DeviceStatusMaintenance),
			string(devicefleet.DeviceStatusOffline),
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

// newDeviceResponse converts a device model to a response object. The
// online/offline decision is resolved by the IoT service (issue #586, Rule 12)
// rather than the model.
func newDeviceResponse(ctx context.Context, svc devicefleet.Administration, device *devicefleet.Device) DeviceResponse {
	response := DeviceResponse{
		ID:             device.ID,
		DeviceID:       device.DeviceID,
		DeviceType:     device.DeviceType,
		Name:           device.Name,
		Status:         string(device.Status),
		RegisteredByID: device.RegisteredByID,
		RoomName:       device.RoomName,
		IsOnline:       svc.IsDeviceOnline(ctx, device),
		CreatedAt:      wireTime(device.CreatedAt),
		UpdatedAt:      wireTime(device.UpdatedAt),
	}

	if device.LastSeen != nil {
		lastSeen := wireTime(*device.LastSeen)
		response.LastSeen = &lastSeen
	}

	return response
}

// newDeviceResponses converts a slice of device models to response objects
func newDeviceResponses(ctx context.Context, svc devicefleet.Administration, devices []*devicefleet.Device) []DeviceResponse {
	responses := make([]DeviceResponse, 0, len(devices))
	for _, device := range devices {
		responses = append(responses, newDeviceResponse(ctx, svc, device))
	}
	return responses
}

// newDeviceCreationResponse converts a device model to a creation response object with API key
func newDeviceCreationResponse(ctx context.Context, svc devicefleet.Administration, device *devicefleet.Device) DeviceCreationResponse {
	response := DeviceCreationResponse{
		DeviceResponse: newDeviceResponse(ctx, svc, device),
	}

	// Include API key only during creation
	if device.APIKey != nil {
		response.APIKey = *device.APIKey
	}

	return response
}

// isValidDeviceStatus validates a device status value
func isValidDeviceStatus(status devicefleet.DeviceStatus) bool {
	switch status {
	case devicefleet.DeviceStatusActive, devicefleet.DeviceStatusInactive, devicefleet.DeviceStatusMaintenance, devicefleet.DeviceStatusOffline:
		return true
	}
	return false
}
