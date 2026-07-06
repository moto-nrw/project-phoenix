package simulate

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// truncate Tests
// =============================================================================

func TestTruncate_ShortString(t *testing.T) {
	assert.Equal(t, "hello", strutil.TruncateBytes("hello", 10, "..."))
}

func TestTruncate_ExactLength(t *testing.T) {
	assert.Equal(t, "hello", strutil.TruncateBytes("hello", 5, "..."))
}

func TestTruncate_LongString(t *testing.T) {
	assert.Equal(t, "hel...", strutil.TruncateBytes("hello world", 3, "..."))
}

func TestTruncate_EmptyString(t *testing.T) {
	assert.Equal(t, "", strutil.TruncateBytes("", 10, "..."))
}

// =============================================================================
// NewClient Tests
// =============================================================================

func TestNewClient(t *testing.T) {
	c := NewClient("http://localhost:8080/", true)
	assert.Equal(t, "http://localhost:8080", c.baseURL) // trailing slash stripped
	assert.True(t, c.verbose)
	assert.NotNil(t, c.httpClient)
	assert.Empty(t, c.jwtToken)
}

func TestNewClient_NoTrailingSlash(t *testing.T) {
	c := NewClient("http://localhost:8080", false)
	assert.Equal(t, "http://localhost:8080", c.baseURL)
}

// =============================================================================
// CheckHealth Tests
// =============================================================================

func TestClient_CheckHealth_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.CheckHealth()
	assert.NoError(t, err)
}

func TestClient_CheckHealth_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestClient_CheckHealth_Unreachable(t *testing.T) {
	c := NewClient("http://localhost:1", false)
	err := c.CheckHealth()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not reachable")
}

// =============================================================================
// Login Tests
// =============================================================================

func TestClient_Login_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/auth/login", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt-token-xyz"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.Login("demo1@mail.de", "pass123")
	require.NoError(t, err)
	assert.Equal(t, "jwt-token-xyz", c.jwtToken)
}

func TestClient_Login_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"access_token":""}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.Login("demo1@mail.de", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

func TestClient_Login_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.Login("demo1@mail.de", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse login response")
}

func TestClient_Login_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"bad creds"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	err := c.Login("wrong@mail.de", "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "login failed")
}

// =============================================================================
// JWT-authenticated request Tests
// =============================================================================

func TestClient_AdminGet_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Equal(t, "/api/active/groups", r.URL.Path)
		assert.Equal(t, "Bearer jwt-123", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	c.jwtToken = "jwt-123"

	resp, err := c.AdminGet("/api/active/groups")
	require.NoError(t, err)
	assert.Contains(t, string(resp), "data")
}

func TestClient_AdminGet_NotLoggedIn(t *testing.T) {
	c := NewClient("http://localhost:8080", false)
	_, err := c.AdminGet("/api/test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not logged in")
}

func TestClient_AdminPost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer tok", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	c.jwtToken = "tok"

	resp, err := c.AdminPost("/api/students/1/rfid-tag", map[string]string{"rfid_tag": "ABC"})
	require.NoError(t, err)
	assert.Contains(t, string(resp), "success")
}

func TestClient_AdminPut_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "PUT", r.Method)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	c.jwtToken = "tok"

	resp, err := c.AdminPut("/api/students/1", map[string]bool{"sick": true})
	require.NoError(t, err)
	assert.Contains(t, string(resp), "success")
}

// =============================================================================
// Device-authenticated request Tests
// =============================================================================

func TestClient_DevicePost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/iot/checkin", r.URL.Path)
		assert.Equal(t, "Bearer device-key-123", r.Header.Get("Authorization"))
		assert.Equal(t, "1234", r.Header.Get("X-Staff-PIN"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "project-phoenix-simulate/0.1", r.Header.Get("User-Agent"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"student_id":1}}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)

	body := map[string]string{"student_rfid": "DEABC", "action": "checkin"}
	resp, err := c.DevicePost("/api/iot/checkin", body, "device-key-123", "1234")
	require.NoError(t, err)
	assert.Contains(t, string(resp), "student_id")
}

func TestClient_DevicePost_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"invalid rfid"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)

	_, err := c.DevicePost("/api/iot/checkin", nil, "key", "pin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// =============================================================================
// Verbose logging Tests
// =============================================================================

func TestClient_Verbose_JWTRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true) // verbose
	c.jwtToken = "tok"

	// Should not panic with verbose logging
	_, err := c.AdminGet("/api/test")
	assert.NoError(t, err)
}

func TestClient_Verbose_DeviceRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true) // verbose
	_, err := c.DevicePost("/api/iot/test", nil, "key", "pin")
	assert.NoError(t, err)
}

func TestClient_Verbose_ErrorResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"server error"}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, true)
	c.jwtToken = "tok"
	_, err := c.AdminGet("/api/test")
	assert.Error(t, err)
}

// =============================================================================
// Edge cases
// =============================================================================

func TestClient_DoRequest_NilBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	// Direct call with no auth - POST with nil body
	resp, err := c.DevicePost("/api/iot/session/end", nil, "key", "pin")
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestClient_ErrorResponseTruncated(t *testing.T) {
	longError := ""
	for i := 0; i < 300; i++ {
		longError += "x"
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, longError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, false)
	_, err := c.DevicePost("/api/test", nil, "key", "pin")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "...")
	// Error message should be truncated to 200 chars
	assert.Less(t, len(err.Error()), 300)
}
