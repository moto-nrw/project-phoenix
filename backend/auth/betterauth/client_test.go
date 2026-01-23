package betterauth_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/betterauth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSession_Success(t *testing.T) {
	// Setup mock BetterAuth server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/auth/get-session", r.URL.Path)
		assert.Equal(t, http.MethodGet, r.Method)

		// Return valid session
		response := betterauth.SessionResponse{
			User: betterauth.UserInfo{
				ID:    "user-123",
				Email: "test@example.com",
				Name:  "Test User",
			},
			Session: betterauth.SessionInfo{
				ID:                   "session-456",
				ActiveOrganizationID: "org-789",
				ExpiresAt:            time.Now().Add(24 * time.Hour),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Create client pointing to mock server
	client := betterauth.NewClientWithURL(server.URL)

	// Create request with session cookie
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "test-cookie"})

	// Get session
	session, err := client.GetSession(req.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, "user-123", session.User.ID)
	assert.Equal(t, "test@example.com", session.User.Email)
	assert.Equal(t, "org-789", session.Session.ActiveOrganizationID)
}

func TestGetSession_NoSession(t *testing.T) {
	// Setup mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, betterauth.ErrNoSession)
}

func TestGetSession_NoActiveOrg(t *testing.T) {
	// Setup mock server that returns session without active org
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := betterauth.SessionResponse{
			User: betterauth.UserInfo{
				ID:    "user-123",
				Email: "test@example.com",
			},
			Session: betterauth.SessionInfo{
				ID:                   "session-456",
				ActiveOrganizationID: "", // No active org
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.ErrorIs(t, err, betterauth.ErrNoActiveOrg)
}

func TestGetActiveMember_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/auth/organization/get-active-member", r.URL.Path)

		response := betterauth.MemberInfo{
			ID:             "member-123",
			OrganizationID: "org-789",
			UserID:         "user-123",
			Role:           "supervisor",
			CreatedAt:      "2025-01-20T12:00:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, "member-123", member.ID)
	assert.Equal(t, "supervisor", member.Role)
}

func TestGetActiveMember_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.ErrorIs(t, err, betterauth.ErrMemberNotFound)
}

func TestCookieForwarding(t *testing.T) {
	var receivedCookies []*http.Cookie

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "cookie-value"})
	req.AddCookie(&http.Cookie{Name: "other", Value: "another-value"})

	_, _ = client.GetSession(req.Context(), req)

	// Verify cookies were forwarded
	assert.Len(t, receivedCookies, 2)
	cookieNames := make([]string, len(receivedCookies))
	for i, c := range receivedCookies {
		cookieNames[i] = c.Name
	}
	assert.Contains(t, cookieNames, "session")
	assert.Contains(t, cookieNames, "other")
}

// =============================================================================
// NewClient Tests
// =============================================================================

func TestNewClient_WithEnvVar(t *testing.T) {
	// Save original env var
	originalURL := os.Getenv("BETTERAUTH_URL")
	defer func() {
		if originalURL == "" {
			_ = os.Unsetenv("BETTERAUTH_URL")
		} else {
			_ = os.Setenv("BETTERAUTH_URL", originalURL)
		}
	}()

	// Set custom URL
	customURL := "http://custom-auth.example.com:3002"
	err := os.Setenv("BETTERAUTH_URL", customURL)
	require.NoError(t, err)

	client := betterauth.NewClient()
	assert.Equal(t, customURL, client.BaseURL())
}

func TestNewClient_WithoutEnvVar(t *testing.T) {
	// Save original env var
	originalURL := os.Getenv("BETTERAUTH_URL")
	defer func() {
		if originalURL == "" {
			_ = os.Unsetenv("BETTERAUTH_URL")
		} else {
			_ = os.Setenv("BETTERAUTH_URL", originalURL)
		}
	}()

	// Unset the env var
	_ = os.Unsetenv("BETTERAUTH_URL")

	client := betterauth.NewClient()
	assert.Equal(t, "http://localhost:3001", client.BaseURL())
}

func TestNewClientWithURL(t *testing.T) {
	customURL := "http://test-auth:8888"
	client := betterauth.NewClientWithURL(customURL)
	assert.Equal(t, customURL, client.BaseURL())
}

// =============================================================================
// BaseURL Tests
// =============================================================================

func TestBaseURL(t *testing.T) {
	expectedURL := "http://auth-service.example.com:3001"
	client := betterauth.NewClientWithURL(expectedURL)
	assert.Equal(t, expectedURL, client.BaseURL())
}

// =============================================================================
// GetSession Additional Tests
// =============================================================================

func TestGetSession_NetworkError(t *testing.T) {
	// Create a client pointing to a non-existent server
	client := betterauth.NewClientWithURL("http://localhost:59999")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session request failed")
}

func TestGetSession_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected session status 500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

func TestGetSession_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode session response")
}

func TestGetSession_OriginHeaderSet(t *testing.T) {
	var receivedOrigin string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedOrigin = r.Header.Get("Origin")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _ = client.GetSession(req.Context(), req)

	// Origin header should be set to the server URL for CSRF protection
	assert.Equal(t, server.URL, receivedOrigin)
}

func TestGetSession_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Server blocks, context cancellation should interrupt
		<-r.Context().Done()
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	session, err := client.GetSession(ctx, req)
	assert.Nil(t, session)
	assert.Error(t, err)
}

func TestGetSession_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	assert.Nil(t, session)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode session response")
}

func TestGetSession_NoCookies(t *testing.T) {
	var receivedCookies []*http.Cookie

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// No cookies added

	_, _ = client.GetSession(req.Context(), req)

	assert.Empty(t, receivedCookies)
}

func TestGetSession_VerifyRequestMethod(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _ = client.GetSession(req.Context(), req)

	assert.Equal(t, http.MethodGet, receivedMethod)
}

// =============================================================================
// GetActiveMember Additional Tests
// =============================================================================

func TestGetActiveMember_NoSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.ErrorIs(t, err, betterauth.ErrNoSession)
}

func TestGetActiveMember_NetworkError(t *testing.T) {
	// Create a client pointing to a non-existent server
	client := betterauth.NewClientWithURL("http://localhost:59999")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "member request failed")
}

func TestGetActiveMember_UnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "member lookup failed: status 500")
	assert.Contains(t, err.Error(), "Internal Server Error")
}

func TestGetActiveMember_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode member response")
}

func TestGetActiveMember_CookieForwarding(t *testing.T) {
	var receivedCookies []*http.Cookie

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCookies = r.Cookies()
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.AddCookie(&http.Cookie{Name: "better-auth.session_token", Value: "token-value"})
	req.AddCookie(&http.Cookie{Name: "another-cookie", Value: "another-value"})

	_, _ = client.GetActiveMember(req.Context(), req)

	// Verify cookies were forwarded
	assert.Len(t, receivedCookies, 2)
	cookieNames := make([]string, len(receivedCookies))
	for i, c := range receivedCookies {
		cookieNames[i] = c.Name
	}
	assert.Contains(t, cookieNames, "better-auth.session_token")
	assert.Contains(t, cookieNames, "another-cookie")
}

func TestGetActiveMember_OriginHeaderSet(t *testing.T) {
	var receivedOrigin string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedOrigin = r.Header.Get("Origin")
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _ = client.GetActiveMember(req.Context(), req)

	// Origin header should be set to the server URL for CSRF protection
	assert.Equal(t, server.URL, receivedOrigin)
}

func TestGetActiveMember_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	member, err := client.GetActiveMember(ctx, req)
	assert.Nil(t, member)
	assert.Error(t, err)
}

func TestGetActiveMember_VerifyEndpoint(t *testing.T) {
	var receivedPath string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _ = client.GetActiveMember(req.Context(), req)

	assert.Equal(t, "/api/auth/organization/get-active-member", receivedPath)
}

func TestGetActiveMember_VerifyRequestMethod(t *testing.T) {
	var receivedMethod string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedMethod = r.Method
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	_, _ = client.GetActiveMember(req.Context(), req)

	assert.Equal(t, http.MethodGet, receivedMethod)
}

func TestGetActiveMember_EmptyBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Empty body
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decode member response")
}

func TestGetActiveMember_PartialResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return partial response with only some fields
		response := betterauth.MemberInfo{
			ID:   "member-123",
			Role: "supervisor",
			// Missing other fields
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	require.NoError(t, err)
	assert.Equal(t, "member-123", member.ID)
	assert.Equal(t, "supervisor", member.Role)
	assert.Empty(t, member.OrganizationID) // Missing fields should be zero values
}

func TestGetActiveMember_BadRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Bad Request: missing required parameter"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "member lookup failed: status 400")
}

func TestGetActiveMember_Forbidden(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Access denied"))
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	assert.Nil(t, member)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "member lookup failed: status 403")
}

// =============================================================================
// Session Response Field Tests
// =============================================================================

func TestGetSession_FullResponse(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := betterauth.SessionResponse{
			User: betterauth.UserInfo{
				ID:    "user-abc123",
				Email: "admin@school.example.com",
				Name:  "Admin User",
			},
			Session: betterauth.SessionInfo{
				ID:                   "session-xyz789",
				ActiveOrganizationID: "org-12345",
				ExpiresAt:            expiresAt,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	session, err := client.GetSession(req.Context(), req)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, "user-abc123", session.User.ID)
	assert.Equal(t, "admin@school.example.com", session.User.Email)
	assert.Equal(t, "Admin User", session.User.Name)
	assert.Equal(t, "session-xyz789", session.Session.ID)
	assert.Equal(t, "org-12345", session.Session.ActiveOrganizationID)
	assert.False(t, session.Session.ExpiresAt.IsZero())
}

func TestGetActiveMember_FullResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := betterauth.MemberInfo{
			ID:             "member-m123",
			OrganizationID: "org-o456",
			UserID:         "user-u789",
			Role:           "ogsAdmin",
			CreatedAt:      "2025-01-15T10:30:00Z",
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := betterauth.NewClientWithURL(server.URL)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	member, err := client.GetActiveMember(req.Context(), req)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, "member-m123", member.ID)
	assert.Equal(t, "org-o456", member.OrganizationID)
	assert.Equal(t, "user-u789", member.UserID)
	assert.Equal(t, "ogsAdmin", member.Role)
	assert.Equal(t, "2025-01-15T10:30:00Z", member.CreatedAt)
}
