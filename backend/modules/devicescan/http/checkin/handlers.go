package checkin

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// devicePing handles ping requests from RFID devices. The heartbeat keeps
// both the device and any running session alive.
func (rs *Resource) devicePing(w http.ResponseWriter, r *http.Request) {
	ping, err := rs.scans.Ping(r.Context())
	if err != nil {
		rs.failDevice(w, r, err)
		return
	}
	response := map[string]any{
		"device_id":      ping.Device.DeviceID,
		"device_name":    ping.Device.Name,
		"status":         ping.Device.Status,
		"last_seen":      ping.Device.LastSeen,
		"is_online":      ping.IsOnline,
		"ping_time":      ping.PingTime,
		"session_active": ping.SessionActive,
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Device ping successful")
}

// deviceStatus handles status requests from RFID devices.
func (rs *Resource) deviceStatus(w http.ResponseWriter, r *http.Request) {
	status, err := rs.scans.Status(r.Context())
	if err != nil {
		rs.failDevice(w, r, err)
		return
	}
	response := map[string]any{
		"device": map[string]any{
			"id":          status.Device.ID,
			"device_id":   status.Device.DeviceID,
			"device_type": status.Device.DeviceType,
			"name":        status.Device.Name,
			"status":      status.Device.Status,
			"last_seen":   status.Device.LastSeen,
			"is_online":   status.IsOnline,
			"is_active":   status.Device.Active,
		},
		"authenticated_at": status.AuthenticatedAt,
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Device status retrieved")
}

// failDevice renders a failure of a device-only endpoint, logging the
// missing credential the way the device middleware does.
func (rs *Resource) failDevice(w http.ResponseWriter, r *http.Request, err error) {
	if failure, ok := devicescan.IsFailure(err); ok && failure.Kind == devicescan.FailureUnauthorized {
		rs.logger.WarnContext(r.Context(), "device auth missing API key", slog.String("path", r.URL.Path))
	}
	rs.fail(w, r, err)
}

// devicePickupQuery handles read-only pickup information lookups.
func (rs *Resource) devicePickupQuery(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	device, err := rs.scans.Device(ctx)
	if err != nil {
		rs.fail(w, r, err)
		return
	}

	req := &PickupQueryRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.logger.ErrorContext(ctx, "invalid pickup query request",
			slog.String("device_id", device.DeviceID),
			slog.String("error", err.Error()),
		)
		invalidRequest(w, r, rs.runtime, err)
		return
	}

	info, err := rs.scans.PickupInfo(ctx, req.StudentRFID)
	if err != nil {
		rs.fail(w, r, err)
		return
	}
	response := map[string]any{
		"student_id":   info.StudentID,
		"student_name": info.StudentName,
		"action":       devicescan.ScanActionPickupInfo,
		"processed_at": info.ProcessedAt,
		"status":       "success",
	}
	if info.PickupTime != "" {
		response["pickup_time"] = info.PickupTime
	}
	if info.PickupNote != "" {
		response["pickup_note"] = info.PickupNote
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Pickup information retrieved successfully")
}

// deviceCheckin handles student check-in/check-out requests from RFID devices.
func (rs *Resource) deviceCheckin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	device, err := rs.scans.Device(ctx)
	if err != nil {
		rs.fail(w, r, err)
		return
	}

	req := &CheckinRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.logger.ErrorContext(ctx, "invalid checkin request",
			slog.String("device_id", device.DeviceID),
			slog.String("error", err.Error()),
		)
		invalidRequest(w, r, rs.runtime, err)
		return
	}

	result, err := rs.scans.Scan(ctx, devicescan.ScanCommand{RFIDTag: req.StudentRFID, RoomID: req.RoomID})
	if err != nil {
		rs.fail(w, r, err)
		return
	}

	switch result.Outcome {
	case devicescan.ScanOutcomeSupervisor:
		rs.runtime.Success(w, r, http.StatusOK, buildSupervisorResponse(result), "Supervisor authenticated")
	case devicescan.ScanOutcomeAttendance:
		sendCheckinResponse(w, r, rs.runtime, buildBinaryCheckinResponse(result), result.Action)
	default:
		sendCheckinResponse(w, r, rs.runtime, buildCheckinResponse(result), result.Action)
	}
}
