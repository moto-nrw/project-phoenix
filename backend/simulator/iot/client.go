package iot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	iotapi "github.com/moto-nrw/project-phoenix/api/iot"
)

// Client wraps HTTP interactions with the IoT API on behalf of devices.
type Client struct {
	baseURL    string
	pin        string
	httpClient *http.Client
}

// NewClient creates a new API client.
func NewClient(baseURL, pin string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		pin:        pin,
		httpClient: httpClient,
	}
}

// Authenticate validates a device's API key + PIN combination.
func (c *Client) Authenticate(ctx context.Context, device DeviceConfig) error {
	req, err := c.newRequest(ctx, device, http.MethodGet, "/api/iot/status", nil, nil)
	if err != nil {
		return fmt.Errorf("build status request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call status endpoint: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d from /api/iot/status", resp.StatusCode)
	}

	return nil
}

// FetchSession retrieves the current session for a device.
func (c *Client) FetchSession(ctx context.Context, device DeviceConfig) (*iotapi.SessionCurrentResponse, error) {
	var result iotapi.SessionCurrentResponse
	if err := c.get(ctx, device, "/api/iot/session/current", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// FetchStudents retrieves the student roster for the provided teacher IDs.
func (c *Client) FetchStudents(ctx context.Context, device DeviceConfig) ([]iotapi.TeacherStudentResponse, error) {
	if device.TeacherIDsParam() == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("teacher_ids", device.TeacherIDsParam())

	var result []iotapi.TeacherStudentResponse
	if err := c.get(ctx, device, "/api/iot/students", query, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchRooms retrieves the available rooms for the device.
func (c *Client) FetchRooms(ctx context.Context, device DeviceConfig) ([]iotapi.DeviceRoomResponse, error) {
	var result []iotapi.DeviceRoomResponse
	if err := c.get(ctx, device, "/api/iot/rooms/available", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchActivities retrieves the available activities for the device.
func (c *Client) FetchActivities(ctx context.Context, device DeviceConfig) ([]iotapi.TeacherActivityResponse, error) {
	var result []iotapi.TeacherActivityResponse
	if err := c.get(ctx, device, "/api/iot/activities", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// FetchTeachers retrieves the available staff roster for the device.
func (c *Client) FetchTeachers(ctx context.Context, device DeviceConfig) ([]iotapi.DeviceTeacherResponse, error) {
	var result []iotapi.DeviceTeacherResponse
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
func (c *Client) PerformCheckAction(ctx context.Context, device DeviceConfig, payload CheckActionPayload) (*iotapi.CheckinResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal check action payload: %w", err)
	}

	req, err := c.newRequest(ctx, device, http.MethodPost, "/api/iot/checkin", nil, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build checkin request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call checkin endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d from /api/iot/checkin: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response from /api/iot/checkin: %w", err)
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("checkin action failed: %s", envelope.Message)
	}

	var result iotapi.CheckinResponse
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &result); err != nil {
			return nil, fmt.Errorf("decode checkin payload: %w", err)
		}
	}

	return &result, nil
}

// AttendanceTogglePayload wraps attendance toggle requests.
type AttendanceTogglePayload struct {
	RFID   string `json:"rfid"`
	Action string `json:"action"`
}

// ToggleAttendance toggles a student's attendance state.
func (c *Client) ToggleAttendance(ctx context.Context, device DeviceConfig, payload AttendanceTogglePayload) (*iotapi.AttendanceToggleResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal attendance payload: %w", err)
	}

	req, err := c.newRequest(ctx, device, http.MethodPost, "/api/iot/attendance/toggle", nil, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build attendance toggle request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call attendance toggle endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d from /api/iot/attendance/toggle: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode attendance toggle response: %w", err)
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("attendance toggle failed: %s", envelope.Message)
	}

	var result iotapi.AttendanceToggleResponse
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &result); err != nil {
			return nil, fmt.Errorf("decode attendance payload: %w", err)
		}
	}

	return &result, nil
}

// UpdateSessionSupervisors updates the supervisors assigned to a session.
func (c *Client) UpdateSessionSupervisors(ctx context.Context, device DeviceConfig, sessionID int64, supervisorIDs []int64) (*iotapi.UpdateSupervisorsResponse, error) {
	payload := &iotapi.UpdateSupervisorsRequest{SupervisorIDs: supervisorIDs}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal supervisor payload: %w", err)
	}

	path := fmt.Sprintf("/api/iot/session/%d/supervisors", sessionID)
	req, err := c.newRequest(ctx, device, http.MethodPut, path, nil, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build supervisor update request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call supervisor update endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, strings.TrimSpace(string(body)))
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode supervisor update response: %w", err)
	}
	if envelope.Status != "success" {
		return nil, fmt.Errorf("supervisor update failed: %s", envelope.Message)
	}

	var result iotapi.UpdateSupervisorsResponse
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &result); err != nil {
			return nil, fmt.Errorf("decode supervisor payload: %w", err)
		}
	}

	return &result, nil
}

// StartSession starts a default session for the device.
func (c *Client) StartSession(ctx context.Context, device DeviceConfig, session *SessionConfig) (*iotapi.SessionStartResponse, error) {
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

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal session payload: %w", err)
	}

	req, err := c.newRequest(ctx, device, http.MethodPost, "/api/iot/session/start", nil, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build session start request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call session start endpoint: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("unexpected status %d from /api/iot/session/start: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("decode response from /api/iot/session/start: %w", err)
	}

	if envelope.Status != "success" {
		return nil, fmt.Errorf("session start failed: %s", envelope.Message)
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil, fmt.Errorf("session start returned empty payload")
	}

	var result iotapi.SessionStartResponse
	if err := json.Unmarshal(envelope.Data, &result); err != nil {
		return nil, fmt.Errorf("decode session start payload: %w", err)
	}

	return &result, nil
}

func (c *Client) get(ctx context.Context, device DeviceConfig, path string, query url.Values, out interface{}) error {
	req, err := c.newRequest(ctx, device, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("unexpected status %d from %s: %s", resp.StatusCode, path, strings.TrimSpace(string(body)))
	}

	var envelope apiResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode response from %s: %w", path, err)
	}

	if envelope.Status != "success" {
		return fmt.Errorf("api returned status %q for %s: %s", envelope.Status, path, envelope.Message)
	}

	if out == nil {
		return nil
	}

	if len(envelope.Data) == 0 || string(envelope.Data) == "null" {
		return nil
	}

	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode data payload from %s: %w", path, err)
	}
	return nil
}

func (c *Client) newRequest(ctx context.Context, device DeviceConfig, method, path string, query url.Values, body io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	fullURL := c.baseURL + path
	if query != nil {
		fullURL = fullURL + "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+device.APIKey)
	req.Header.Set("X-Staff-PIN", c.pin)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "project-phoenix-simulator/0.1")

	return req, nil
}

type apiResponse struct {
	Status  string          `json:"status"`
	Data    json.RawMessage `json:"data"`
	Message string          `json:"message"`
}
