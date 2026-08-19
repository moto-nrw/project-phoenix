package phoenixapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNew(t *testing.T) {
	t.Parallel()

	a := New("http://localhost:8080/", false)
	assert.Equal(t, "http://localhost:8080", a.baseURL)
	assert.False(t, a.verbose)
	assert.NotNil(t, a.httpClient)
}

func TestNew_StripsTrailingSlash(t *testing.T) {
	t.Parallel()

	a := New("http://example.com///", false)
	assert.Equal(t, "http://example.com//", a.baseURL)
}

func TestNewWithHTTPClient_CustomClient(t *testing.T) {
	t.Parallel()

	custom := &http.Client{}
	a := NewWithHTTPClient("http://localhost", true, custom)
	assert.Same(t, custom, a.httpClient)
	assert.True(t, a.verbose)
}

func TestNewWithHTTPClient_NilClient(t *testing.T) {
	t.Parallel()

	a := NewWithHTTPClient("http://localhost", false, nil)
	assert.NotNil(t, a.httpClient)
}

func TestAdapter_BaseURL(t *testing.T) {
	t.Parallel()

	a := New("http://localhost:8080", false)
	assert.Equal(t, "http://localhost:8080", a.BaseURL())
}

func TestAdapter_HTTPClient(t *testing.T) {
	t.Parallel()

	a := New("http://localhost:8080", false)
	assert.NotNil(t, a.HTTPClient())
}

// =============================================================================
// CheckHealth Tests
// =============================================================================

func TestCheckHealth_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/health", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	err := a.CheckHealth(context.Background())
	assert.NoError(t, err)
}

func TestCheckHealth_NonOK(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	err := a.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
}

func TestCheckHealth_Unreachable(t *testing.T) {
	t.Parallel()

	a := New("http://localhost:1", false)
	err := a.CheckHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server not reachable")
}

// =============================================================================
// LoginOperator Tests
// =============================================================================

func TestLoginOperator_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/operator/auth/login", r.URL.Path)
		assert.Equal(t, http.MethodPost, r.Method)

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "op@test.de", payload["email"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]string{"access_token": "op-jwt-123"},
		})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "pass")
	require.NoError(t, err)
	assert.Equal(t, AuthBearer, auth.Kind)
	assert.Equal(t, "operator", auth.Label)
	assert.Equal(t, "op-jwt-123", auth.Token)
}

func TestLoginOperator_FallbackToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "flat-token"})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth, err := a.LoginOperator(context.Background(), "op@test.de", "pass")
	require.NoError(t, err)
	assert.Equal(t, "flat-token", auth.Token)
}

func TestLoginOperator_NoToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]string{}})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

func TestLoginOperator_ServerError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = fmt.Fprint(w, `{"error":"bad creds"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "wrong")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "operator login request failed")
}

func TestLoginOperator_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, err := a.LoginOperator(context.Background(), "op@test.de", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse operator login response")
}

// =============================================================================
// LoginTenant Tests
// =============================================================================

func TestLoginTenant_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/auth/login", r.URL.Path)

		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		require.NoError(t, json.Unmarshal(body, &payload))
		assert.Equal(t, "test-school", payload["tenant_slug"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "tenant-jwt"})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth, err := a.LoginTenant(context.Background(), "u@test.de", "pass", "test-school")
	require.NoError(t, err)
	assert.Equal(t, AuthBearer, auth.Kind)
	assert.Equal(t, "test-school", auth.Label)
	assert.Equal(t, "tenant-jwt", auth.Token)
}

func TestLoginTenant_EmptySlug(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]string
		_ = json.Unmarshal(body, &payload)
		assert.Empty(t, payload["tenant_slug"])

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth, err := a.LoginTenant(context.Background(), "u@test.de", "pass", "")
	require.NoError(t, err)
	assert.Equal(t, "tenant", auth.Label) // default label
}

func TestLoginTenant_WhitespaceSlug(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"access_token": "jwt"})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth, err := a.LoginTenant(context.Background(), "u@test.de", "pass", "  ")
	require.NoError(t, err)
	assert.Equal(t, "tenant", auth.Label)
}

func TestLoginTenant_NoToken(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, err := a.LoginTenant(context.Background(), "u@test.de", "pass", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no access token")
}

func TestLoginTenant_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{broken`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, err := a.LoginTenant(context.Background(), "u@test.de", "pass", "")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse login response")
}

// =============================================================================
// DeviceAuth Tests
// =============================================================================

func TestDeviceAuth(t *testing.T) {
	t.Parallel()

	auth := DeviceAuth("api-key-123", "1234", "scanner-1")
	assert.Equal(t, AuthDevice, auth.Kind)
	assert.Equal(t, "scanner-1", auth.Label)
	assert.Equal(t, "api-key-123", auth.APIKey)
	assert.Equal(t, "1234", auth.PIN)
}

// =============================================================================
// Raw Tests
// =============================================================================

func TestRaw_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/test", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
		assert.Equal(t, "application/json", r.Header.Get("Accept"))
		assert.Equal(t, "project-phoenix-integration/0.1", r.Header.Get("User-Agent"))

		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	body, status, err := a.Raw(context.Background(), AuthRef{}, http.MethodPost, "/api/test", map[string]string{"key": "val"}, nil)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, string(body), "ok")
}

func TestRaw_WithBearerAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer my-token", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth := AuthRef{Kind: AuthBearer, Token: "my-token"}
	_, _, err := a.Raw(context.Background(), auth, http.MethodGet, "/test", nil, nil)
	assert.NoError(t, err)
}

func TestRaw_WithDeviceAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer dev-key", r.Header.Get("Authorization"))
		assert.Equal(t, "9999", r.Header.Get("X-Staff-PIN"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	auth := DeviceAuth("dev-key", "9999", "scanner")
	_, _, err := a.Raw(context.Background(), auth, http.MethodPost, "/api/iot/checkin", nil, nil)
	assert.NoError(t, err)
}

func TestRaw_CustomHeaders(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "custom-val", r.Header.Get("X-Custom"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	headers := map[string]string{"X-Custom": "custom-val"}
	_, _, err := a.Raw(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, headers)
	assert.NoError(t, err)
}

func TestRaw_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"message":"bad input"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, status, err := a.Raw(context.Background(), AuthRef{}, http.MethodPost, "/api/test", nil, nil)
	require.Error(t, err)
	assert.Equal(t, http.StatusBadRequest, status)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "bad input", apiErr.Message)
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

func TestRaw_Verbose(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, true) // verbose
	_, _, err := a.Raw(context.Background(), AuthRef{Kind: AuthBearer, Label: "admin"}, http.MethodGet, "/test", nil, nil)
	assert.NoError(t, err)
}

func TestRaw_VerboseError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprint(w, `{"error":"fail"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, true) // verbose
	_, _, err := a.Raw(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.Error(t, err)
}

func TestRaw_NilBody(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No Content-Type when body is nil
		assert.Empty(t, r.Header.Get("Content-Type"))
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, _, err := a.Raw(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.NoError(t, err)
}

func TestRaw_PathWithoutLeadingSlash(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/test", r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	_, _, err := a.Raw(context.Background(), AuthRef{}, http.MethodGet, "api/test", nil, nil)
	assert.NoError(t, err)
}

func TestRaw_Unreachable(t *testing.T) {
	t.Parallel()

	a := New("http://localhost:1", false)
	_, _, err := a.Raw(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execute request")
}

// =============================================================================
// JSON Tests
// =============================================================================

func TestJSON_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "test"})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.JSON(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	require.NoError(t, err)
	assert.Equal(t, "test", out["name"])
}

func TestJSON_NilOut(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"ignored": true}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	err := a.JSON(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.NoError(t, err)
}

func TestJSON_InvalidJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.JSON(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestJSON_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":"forbidden"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.JSON(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.Error(t, err)
}

// =============================================================================
// Envelope Tests
// =============================================================================

func TestEnvelope_Success(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]string{"id": "42"},
		})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	require.NoError(t, err)
	assert.Equal(t, "42", out["id"])
}

func TestEnvelope_NilOut(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "success",
			"data":   map[string]string{"id": "42"},
		})
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.NoError(t, err)
}

func TestEnvelope_NullData(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.NoError(t, err)
}

func TestEnvelope_EmptyData(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"success"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.NoError(t, err)
}

func TestEnvelope_ErrorStatus(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `{"status":"error","message":"something went wrong"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	require.Error(t, err)
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "something went wrong", apiErr.Message)
}

func TestEnvelope_InvalidEnvelopeJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, `not json`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]string
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestEnvelope_InvalidDataJSON(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		// data is a string, but we try to unmarshal into a map
		_, _ = fmt.Fprint(w, `{"status":"success","data":"not a map"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	var out map[string]int
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, &out)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode")
}

func TestEnvelope_HTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":"not found"}`)
	}))
	defer srv.Close()

	a := New(srv.URL, false)
	err := a.Envelope(context.Background(), AuthRef{}, http.MethodGet, "/test", nil, nil)
	assert.Error(t, err)
}

// =============================================================================
// APIError Tests
// =============================================================================

func TestAPIError_Error_WithMessage(t *testing.T) {
	t.Parallel()

	e := &APIError{Method: "GET", Path: "/test", StatusCode: 400, Message: "bad input"}
	assert.Contains(t, e.Error(), "GET /test failed: 400 - bad input")
}

func TestAPIError_Error_WithBody(t *testing.T) {
	t.Parallel()

	e := &APIError{Method: "POST", Path: "/api", StatusCode: 500, Body: "raw body"}
	assert.Contains(t, e.Error(), "POST /api failed: 500 - raw body")
}

func TestAPIError_Error_StatusOnly(t *testing.T) {
	t.Parallel()

	e := &APIError{Method: "DELETE", Path: "/item", StatusCode: 404}
	assert.Equal(t, "DELETE /item failed: 404", e.Error())
}

func TestAPIError_Error_Nil(t *testing.T) {
	t.Parallel()

	var e *APIError
	assert.Equal(t, "", e.Error())
}

func TestAPIError_Error_MessagePrecedence(t *testing.T) {
	t.Parallel()

	e := &APIError{Method: "GET", Path: "/", StatusCode: 400, Message: "msg", Body: "body"}
	// Message takes precedence over Body
	assert.Contains(t, e.Error(), "msg")
	assert.NotContains(t, e.Error(), "body")
}

// =============================================================================
// parseHTTPError Tests
// =============================================================================

func TestParseHTTPError_WithMessage(t *testing.T) {
	t.Parallel()

	body := `{"status":"error","message":"not found"}`
	err := parseHTTPError("GET", "/test", 404, []byte(body))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "not found", apiErr.Message)
	assert.Equal(t, 404, apiErr.StatusCode)
}

func TestParseHTTPError_WithErrorField(t *testing.T) {
	t.Parallel()

	body := `{"error":"unauthorized"}`
	err := parseHTTPError("POST", "/login", 401, []byte(body))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "unauthorized", apiErr.Message)
}

func TestParseHTTPError_InvalidJSON(t *testing.T) {
	t.Parallel()

	err := parseHTTPError("GET", "/test", 500, []byte("not json"))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "", apiErr.Message)
	assert.Contains(t, apiErr.Body, "not json")
}

func TestParseHTTPError_EmptyMessageAndError(t *testing.T) {
	t.Parallel()

	body := `{"status":"error"}`
	err := parseHTTPError("GET", "/test", 500, []byte(body))
	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "", apiErr.Message)
}

// =============================================================================
// truncateBody Tests
// =============================================================================

func TestTruncateBody_Short(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncateBody("hello"))
}

func TestTruncateBody_Exact200(t *testing.T) {
	t.Parallel()

	s := ""
	for i := 0; i < 200; i++ {
		s += "x"
	}
	assert.Equal(t, s, truncateBody(s))
}

func TestTruncateBody_Over200(t *testing.T) {
	t.Parallel()

	s := ""
	for i := 0; i < 250; i++ {
		s += "x"
	}
	result := truncateBody(s)
	assert.Len(t, result, 203) // 200 + "..."
	assert.True(t, len(result) < len(s))
}

func TestTruncateBody_WithWhitespace(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "hello", truncateBody("  hello  "))
}

func TestTruncateBody_Empty(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", truncateBody(""))
}

// =============================================================================
// authModeLabel Tests
// =============================================================================

func TestAuthModeLabel_BearerWithLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "admin", authModeLabel(AuthRef{Kind: AuthBearer, Label: "admin"}))
}

func TestAuthModeLabel_BearerNoLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "bearer", authModeLabel(AuthRef{Kind: AuthBearer}))
}

func TestAuthModeLabel_DeviceWithLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "device:scanner-1", authModeLabel(AuthRef{Kind: AuthDevice, Label: "scanner-1"}))
}

func TestAuthModeLabel_DeviceNoLabel(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "device", authModeLabel(AuthRef{Kind: AuthDevice}))
}

func TestAuthModeLabel_None(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "public", authModeLabel(AuthRef{Kind: AuthKind("none")}))
}

func TestAuthModeLabel_Default(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "public", authModeLabel(AuthRef{}))
}

// =============================================================================
// applyAuth Tests
// =============================================================================

func TestApplyAuth_Bearer(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthBearer, Token: "tok123"})
	assert.Equal(t, "Bearer tok123", req.Header.Get("Authorization"))
}

func TestApplyAuth_BearerEmptyToken(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthBearer, Token: ""})
	assert.Empty(t, req.Header.Get("Authorization"))
}

func TestApplyAuth_Device(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthDevice, APIKey: "key", PIN: "1234"})
	assert.Equal(t, "Bearer key", req.Header.Get("Authorization"))
	assert.Equal(t, "1234", req.Header.Get("X-Staff-PIN"))
}

func TestApplyAuth_DeviceEmptyKey(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthDevice, APIKey: "", PIN: "1234"})
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Equal(t, "1234", req.Header.Get("X-Staff-PIN"))
}

func TestApplyAuth_DeviceEmptyPIN(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthDevice, APIKey: "key", PIN: ""})
	assert.Equal(t, "Bearer key", req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("X-Staff-PIN"))
}

func TestApplyAuth_None(t *testing.T) {
	t.Parallel()

	req, _ := http.NewRequest(http.MethodGet, "http://test", nil)
	applyAuth(req, AuthRef{Kind: AuthKind("none")})
	assert.Empty(t, req.Header.Get("Authorization"))
	assert.Empty(t, req.Header.Get("X-Staff-PIN"))
}
