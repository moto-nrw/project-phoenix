package iot

import (
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
