package iot

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/moto-nrw/project-phoenix/api/iot/attendance"
	"github.com/moto-nrw/project-phoenix/api/iot/checkin"
	"github.com/moto-nrw/project-phoenix/api/iot/data"
	sessionsapi "github.com/moto-nrw/project-phoenix/api/iot/sessions"
	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"
)

// Client wraps HTTP interactions with the IoT API on behalf of devices.
type Client struct {
	adapter *phoenixapi.Adapter
	pin     string
}

// NewClient creates a new API client.
func NewClient(baseURL, pin string, httpClient *http.Client) *Client {
	return &Client{
		adapter: phoenixapi.NewWithHTTPClient(baseURL, false, httpClient),
		pin:     pin,
	}
}

// Authenticate validates a device's API key + PIN combination.
func (c *Client) Authenticate(ctx context.Context, device DeviceConfig) error {
	if _, _, err := c.adapter.Raw(ctx, c.deviceAuth(device), http.MethodGet, "/api/iot/status", nil, nil); err != nil {
		return fmt.Errorf("call status endpoint: %w", err)
	}
	return nil
}

// FetchSession retrieves the current session for a device.
func (c *Client) FetchSession(ctx context.Context, device DeviceConfig) (*sessionsapi.SessionCurrentResponse, error) {
	var result sessionsapi.SessionCurrentResponse
	if err := c.get(ctx, device, "/api/iot/session/current", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FetchStudents retrieves the student roster for the provided teacher IDs.
func (c *Client) FetchStudents(ctx context.Context, device DeviceConfig) ([]data.TeacherStudentResponse, error) {
	if device.TeacherIDsParam() == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("teacher_ids", device.TeacherIDsParam())

	var result []data.TeacherStudentResponse
	if err := c.get(ctx, device, "/api/iot/students", query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchRooms retrieves the available rooms for the device.
func (c *Client) FetchRooms(ctx context.Context, device DeviceConfig) ([]data.DeviceRoomResponse, error) {
	var result []data.DeviceRoomResponse
	if err := c.get(ctx, device, "/api/iot/rooms/available", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchActivities retrieves the available activities for the device.
func (c *Client) FetchActivities(ctx context.Context, device DeviceConfig) ([]data.TeacherActivityResponse, error) {
	var result []data.TeacherActivityResponse
	if err := c.get(ctx, device, "/api/iot/activities", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchTeachers retrieves the available staff roster for the device.
func (c *Client) FetchTeachers(ctx context.Context, device DeviceConfig) ([]data.DeviceTeacherResponse, error) {
	var result []data.DeviceTeacherResponse
	if err := c.get(ctx, device, "/api/iot/teachers", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CheckActionPayload wraps checkin/checkout requests.
type CheckActionPayload struct {
	StudentRFID string `json:"student_rfid"`
	Action      string `json:"action"`
	RoomID      *int64 `json:"room_id,omitempty"`
}

// PerformCheckAction submits a checkin/checkout action for a student.
func (c *Client) PerformCheckAction(ctx context.Context, device DeviceConfig, payload CheckActionPayload) (*checkin.CheckinResponse, error) {
	var result checkin.CheckinResponse
	if err := c.adapter.Envelope(ctx, c.deviceAuth(device), http.MethodPost, "/api/iot/checkin", payload, &result); err != nil {
		return nil, fmt.Errorf("call checkin endpoint: %w", err)
	}
	return &result, nil
}

// AttendanceTogglePayload wraps attendance toggle requests.
type AttendanceTogglePayload struct {
	RFID   string `json:"rfid"`
	Action string `json:"action"`
}

// ToggleAttendance toggles a student's attendance state.
func (c *Client) ToggleAttendance(ctx context.Context, device DeviceConfig, payload AttendanceTogglePayload) (*attendance.AttendanceToggleResponse, error) {
	var result attendance.AttendanceToggleResponse
	if err := c.adapter.Envelope(ctx, c.deviceAuth(device), http.MethodPost, "/api/iot/attendance/toggle", payload, &result); err != nil {
		return nil, fmt.Errorf("call attendance toggle endpoint: %w", err)
	}
	return &result, nil
}

// UpdateSessionSupervisors updates the supervisors assigned to a session.
func (c *Client) UpdateSessionSupervisors(ctx context.Context, device DeviceConfig, sessionID int64, supervisorIDs []int64) (*sessionsapi.UpdateSupervisorsResponse, error) {
	payload := &sessionsapi.UpdateSupervisorsRequest{SupervisorIDs: supervisorIDs}
	path := fmt.Sprintf("/api/iot/session/%d/supervisors", sessionID)
	var result sessionsapi.UpdateSupervisorsResponse
	if err := c.adapter.Envelope(ctx, c.deviceAuth(device), http.MethodPut, path, payload, &result); err != nil {
		return nil, fmt.Errorf("call supervisor update endpoint: %w", err)
	}
	return &result, nil
}

// StartSession starts a default session for the device.
func (c *Client) StartSession(ctx context.Context, device DeviceConfig, session *SessionConfig) (*sessionsapi.SessionStartResponse, error) {
	if session == nil {
		return nil, fmt.Errorf("session config is required")
	}

	payload := map[string]interface{}{
		"activity_id": session.ActivityID,
		"room_id":     session.RoomID,
	}
	if len(session.SupervisorIDs) > 0 {
		payload["supervisor_ids"] = session.SupervisorIDs
	}

	var result sessionsapi.SessionStartResponse
	if err := c.adapter.Envelope(ctx, c.deviceAuth(device), http.MethodPost, "/api/iot/session/start", payload, &result); err != nil {
		return nil, fmt.Errorf("call session start endpoint: %w", err)
	}

	return &result, nil
}

func (c *Client) get(ctx context.Context, device DeviceConfig, path string, query url.Values, out interface{}) error {
	if query != nil {
		path = path + "?" + query.Encode()
	}
	return c.adapter.Envelope(ctx, c.deviceAuth(device), http.MethodGet, path, nil, out)
}

func (c *Client) deviceAuth(device DeviceConfig) phoenixapi.AuthRef {
	return phoenixapi.DeviceAuth(device.APIKey, c.pin, device.DeviceID)
}
