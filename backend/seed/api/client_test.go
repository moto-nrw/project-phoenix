package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/integration/phoenixapi"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// extractResponseSummary Tests
// =============================================================================

func TestExtractResponseSummary_WithStatus(t *testing.T) {
	resp := map[string]any{"status": "success"}
	result := extractResponseSummary(resp)
	assert.Equal(t, "{status=success}", result)
}

func TestExtractResponseSummary_WithDataID(t *testing.T) {
	resp := map[string]any{
		"status": "success",
		"data":   map[string]any{"id": float64(42)},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "status=success")
	assert.Contains(t, result, "id=42")
}

func TestExtractResponseSummary_WithDataFields(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{
			"id":    float64(1),
			"name":  "Test Room",
			"email": "test@mail.de",
		},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "id=1")
	assert.Contains(t, result, "name=Test Room")
	assert.Contains(t, result, "email=test@mail.de")
}

func TestExtractResponseSummary_WithTagID(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{"tag_id": "ABC123"},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "tag_id=ABC123")
}

func TestExtractResponseSummary_WithStudentID(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{"student_id": float64(99)},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "student_id=99")
}

func TestExtractResponseSummary_WithActiveGroupID(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{"active_group_id": float64(7)},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "active_group_id=7")
}

func TestExtractResponseSummary_WithActivityName(t *testing.T) {
	resp := map[string]any{
		"data": map[string]any{"activity_name": "Fußball"},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "activity=Fußball")
}

func TestExtractResponseSummary_WithArrayData(t *testing.T) {
	resp := map[string]any{
		"status": "success",
		"data":   []any{1, 2, 3},
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "count=3")
}

func TestExtractResponseSummary_WithMessage(t *testing.T) {
	resp := map[string]any{
		"message": "Student enrolled successfully",
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "msg=Student enrolled successfully")
}

func TestExtractResponseSummary_WithLongMessage(t *testing.T) {
	resp := map[string]any{
		"message": "This is a very long message that should be truncated to avoid excessive log output",
	}
	result := extractResponseSummary(resp)
	assert.Contains(t, result, "...")
}

func TestExtractResponseSummary_EmptyResponse(t *testing.T) {
	resp := map[string]any{}
	result := extractResponseSummary(resp)
	assert.Equal(t, "ok", result)
}

func TestExtractResponseSummary_MessageNotShownWhenSpecificFieldsPresent(t *testing.T) {
	resp := map[string]any{
		"status":  "success",
		"data":    map[string]any{"id": float64(1), "name": "Test"},
		"message": "Some message",
	}
	result := extractResponseSummary(resp)
	// status + id + name = 3 parts > 1, so message is not shown
	assert.NotContains(t, result, "msg=")
}

// =============================================================================
// joinStrings Tests
// =============================================================================

func TestJoinStrings_Empty(t *testing.T) {
	assert.Equal(t, "", strings.Join([]string{}, ", "))
}

func TestJoinStrings_Single(t *testing.T) {
	assert.Equal(t, "hello", strings.Join([]string{"hello"}, ", "))
}

func TestJoinStrings_Multiple(t *testing.T) {
	assert.Equal(t, "a, b, c", strings.Join([]string{"a", "b", "c"}, ", "))
}

// =============================================================================
// Client Tests (with httptest)
// =============================================================================

// newTestClient builds a Client the way the seeder does (NewClientWithAdapter
// over a fresh adapter); the direct constructor was deleted as dead code.
func newTestClient(baseURL string, verbose bool) *Client {
	return NewClientWithAdapter(phoenixapi.New(baseURL, verbose), verbose)
}

func TestNewClient(t *testing.T) {
	c := newTestClient("http://localhost:8080", true)
	assert.Equal(t, "http://localhost:8080", c.baseURL)
	assert.True(t, c.verbose)
	assert.NotNil(t, c.httpClient)
}

func TestClient_CheckHealth_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.CheckHealth()
	assert.NoError(t, err)
}

func TestClient_CheckHealth_ServerDown(t *testing.T) {
	c := newTestClient("http://localhost:1", false)
	err := c.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not reachable")
}

func TestClient_CheckHealth_NonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestClient_Login_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/login", r.URL.Path)
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		resp := map[string]string{
			"access_token":  "test-token-123",
			"refresh_token": "refresh-456",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.Login("demo1@mail.de", "pass123")
	require.NoError(t, err)
	assert.Equal(t, "test-token-123", c.token)
}

func TestClient_Login_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"access_token":""}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.Login("demo1@mail.de", "pass123")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

func TestClient_Login_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"invalid credentials"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.Login("wrong@mail.de", "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "login request failed")
}

func TestClient_Post_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/rooms", r.URL.Path)
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":1}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	c.token = "test-token"

	resp, err := c.Post("/api/rooms", map[string]string{"name": "Room 1"})
	require.NoError(t, err)
	assert.Contains(t, string(resp), `"id":1`)
}

func TestClient_Get_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/active/visits", r.URL.Path)
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":[]}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	c.token = "my-token"

	resp, err := c.Get("/api/active/visits")
	require.NoError(t, err)
	assert.Contains(t, string(resp), `"data":[]`)
}

func TestClient_Put_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/api/students/1", r.URL.Path)

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	c.token = "tok"

	resp, err := c.Put("/api/students/1", map[string]bool{"sick": true})
	require.NoError(t, err)
	assert.Contains(t, string(resp), "success")
}

func TestClient_Post_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"internal"}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	c.token = "tok"

	_, err := c.Post("/api/rooms", map[string]string{"name": "Room"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestClient_Post_NoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// When no token is set and auth=true, Authorization header should be empty
		assert.Empty(t, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	// token is empty
	_, err := c.Post("/api/rooms", nil)
	assert.NoError(t, err)
}

func TestClient_Get_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		// GET requests should have no Content-Type
		assert.Empty(t, r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	c.token = "tok"
	resp, err := c.Get("/api/test")
	require.NoError(t, err)
	assert.Contains(t, string(resp), "ok")
}

func TestClient_DeviceGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/iot/session/current", r.URL.Path)
		assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
		assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"is_active":true}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)

	resp, err := c.DeviceGet("/api/iot/session/current", "device-key-123", "1234")
	require.NoError(t, err)
	assert.Contains(t, string(resp), "is_active")
}

func TestClient_DevicePut_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "/api/iot/session/5/supervisors", r.URL.Path)
		assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
		assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"active_group_id":5}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)

	body := map[string]any{"supervisor_ids": []int64{7}}
	resp, err := c.DevicePut("/api/iot/session/5/supervisors", body, "device-key-123", "1234")
	require.NoError(t, err)
	assert.Contains(t, string(resp), "active_group_id")
}

func TestClient_Verbose_Logging(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":1}}`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, true) // verbose=true
	c.token = "tok"

	// Just verify it doesn't panic with verbose logging
	_, err := c.Post("/api/rooms", map[string]string{"name": "Room"})
	assert.NoError(t, err)
}

func TestClient_Login_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	c := newTestClient(srv.URL, false)
	err := c.Login("test@mail.de", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse login response")
}

// =============================================================================
// logAPIResponse Tests (covers the 54.5% gap)
// =============================================================================

func TestLogAPIResponse_SuccessJSON(t *testing.T) {
	body := []byte(`{"status":"success","data":{"id":1}}`)
	// Should not panic
	logAPIResponse(200, body)
}

func TestLogAPIResponse_ErrorStatus(t *testing.T) {
	body := []byte(`{"status":"error","message":"not found"}`)
	logAPIResponse(404, body)
}

func TestLogAPIResponse_InvalidJSON(t *testing.T) {
	body := []byte(`not json at all`)
	logAPIResponse(200, body)
}

func TestLogAPIResponse_LongNonJSON(t *testing.T) {
	body := make([]byte, 200)
	for i := range body {
		body[i] = 'x'
	}
	logAPIResponse(500, body)
}

func TestLogAPIResponse_EmptyBody(t *testing.T) {
	logAPIResponse(200, []byte{})
}

// =============================================================================
// logAPIRequest Tests
// =============================================================================

func TestLogAPIRequest_WithBody(t *testing.T) {
	logAPIRequest("POST", "/api/rooms", map[string]string{"name": "Room"}, "")
}

func TestLogAPIRequest_WithAuthContext(t *testing.T) {
	logAPIRequest("POST", "/api/rooms", map[string]string{"name": "Room"}, "admin")
}

func TestLogAPIRequest_NilBody(t *testing.T) {
	logAPIRequest("GET", "/api/rooms", nil, "")
}

func TestLogAPIRequest_WithAuthContextNoBody(t *testing.T) {
	logAPIRequest("GET", "/api/roles", nil, "jwt")
}
