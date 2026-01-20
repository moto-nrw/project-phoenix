package betterauth_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
