package rfid

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// performRequest is a test helper for making HTTP requests to the test server
func performRequest(t *testing.T, ts *httptest.Server, method, path string, body io.Reader) *http.Response {
	req, err := http.NewRequest(method, ts.URL+path, body)
	require.NoError(t, err)

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add API key authentication header for tests
	req.Header.Set("Authorization", "Bearer test_api_key")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

// Helper function for parsing JSON responses
func parseJSONResponse(t *testing.T, resp *http.Response, target interface{}) {
	defer resp.Body.Close()
	require.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

// Helper function to replace path parameters in a URL
func replacePathParams(path string, params map[string]string) string {
	result := path
	for k, v := range params {
		result = strings.Replace(result, "{"+k+"}", v, -1)
	}
	return result
}

// mockAPIKeyAuthMiddleware is a test middleware that mocks API key authentication
func mockAPIKeyAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create a mock device and add it to context
		device := &TauriDevice{
			ID:          1,
			DeviceID:    "test-device-id",
			Name:        "Test Device",
			Description: "Test device for unit tests",
			LastSyncAt:  nil,
			LastIP:      "127.0.0.1",
			Status:      "active",
			APIKey:      "test_api_key",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// Add device to context
		ctx := context.WithValue(r.Context(), "device", device)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// setupTestRouter creates a test router that bypasses API key authentication
func setupTestRouter(api *API) *chi.Mux {
	r := chi.NewRouter()

	// All endpoints without authentication - for testing
	r.Post("/devices", api.handleRegisterDevice)
	r.Get("/app/status", api.handleTauriStatus)
	r.Post("/tag", api.handleTagRead)
	r.Get("/tags", api.handleGetAllTags)
	r.Post("/track-student", api.handleStudentTracking)
	r.Post("/room-entry", api.handleRoomEntry)
	r.Post("/room-exit", api.handleRoomExit)
	r.Get("/room-occupancy", api.handleGetRoomOccupancy)
	r.Get("/student/{id}/visits", api.handleGetStudentVisits)
	r.Get("/room/{id}/visits", api.handleGetRoomVisits)
	r.Get("/visits/today", api.handleGetTodayVisits)
	r.Post("/app/sync", api.handleTauriSync)

	r.Route("/devices", func(r chi.Router) {
		r.Get("/", api.handleListDevices)
		r.Get("/{device_id}", api.handleGetDevice)
		r.Put("/{device_id}", api.handleUpdateDevice)
		r.Get("/{device_id}/sync-history", api.handleGetDeviceSyncHistory)
	})

	return r
}

// setupTestAPI sets up a test API with mock stores and authentication
func setupTestAPI(t *testing.T) (*API, *MockRFIDStore) {
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	// Mock getting device by API key
	mockRFIDStore.On("GetDeviceByAPIKey", mock.Anything, "test_api_key").Return(&TauriDevice{
		ID:          1,
		DeviceID:    "test-device-id",
		Name:        "Test Device",
		Description: "Test device for unit tests",
		LastSyncAt:  nil,
		LastIP:      "127.0.0.1",
		Status:      "active",
		APIKey:      "test_api_key",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}, nil)

	// Mock updating device
	mockRFIDStore.On("UpdateDevice", mock.Anything, "test-device-id", mock.Anything).Return(nil)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	return api, mockRFIDStore
}
