// Package internal_test tests the internal API handlers using the hermetic test pattern.
//
// These tests verify HTTP request/response handling, status codes, and error responses.
// For the sync user handler, we use mock services since the handler doesn't need
// database access - it delegates to the UserSyncService.
package internal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalAPI "github.com/moto-nrw/project-phoenix/api/internal"
	"github.com/moto-nrw/project-phoenix/email"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
)

// =============================================================================
// MOCK USER SYNC SERVICE
// =============================================================================

// mockUserSyncService implements authService.UserSyncService for testing.
type mockUserSyncService struct {
	syncUserFunc func(ctx context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error)
}

// SyncUser calls the mock function.
func (m *mockUserSyncService) SyncUser(ctx context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
	if m.syncUserFunc != nil {
		return m.syncUserFunc(ctx, params)
	}
	return nil, errors.New("mock not configured")
}

// =============================================================================
// TEST HELPERS
// =============================================================================

// createTestRouter creates a chi router with the internal API mounted.
func createTestRouter(userSyncService authService.UserSyncService) chi.Router {
	// Create resource with nil for email-related services since we don't test them here
	resource := internalAPI.NewResource(
		nil,           // mailer
		nil,           // dispatcher
		email.Email{}, // fromEmail - empty email.Email struct
		nil,           // accountRepo
		userSyncService,
	)

	router := chi.NewRouter()
	router.Mount("/api/internal", resource.Router())

	return router
}

// executeRequest runs a request against the router and returns the response.
func executeRequest(router chi.Router, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// createJSONRequest creates an HTTP request with JSON body.
func createJSONRequest(t *testing.T, method, target string, body interface{}) *http.Request {
	t.Helper()

	var reader io.Reader
	if body != nil {
		jsonBytes, err := json.Marshal(body)
		require.NoError(t, err, "Failed to marshal request body")
		reader = bytes.NewBuffer(jsonBytes)
	}

	req := httptest.NewRequest(method, target, reader)
	req.Header.Set("Content-Type", "application/json")
	return req
}

// parseSyncUserResponse parses the sync user response body.
func parseSyncUserResponse(t *testing.T, body []byte) internalAPI.SyncUserResponse {
	t.Helper()

	var response internalAPI.SyncUserResponse
	err := json.Unmarshal(body, &response)
	require.NoError(t, err, "Failed to parse response body: %s", string(body))
	return response
}

// =============================================================================
// TEST: SERVICE UNAVAILABLE (nil service)
// =============================================================================

func TestSyncUser_ServiceUnavailable(t *testing.T) {
	// Create router with nil userSyncService
	router := createTestRouter(nil)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		"name":               "Test User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code, "Expected 503 Service Unavailable")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "user sync service unavailable", response.Message)
}

// =============================================================================
// TEST: INVALID JSON BODY
// =============================================================================

func TestSyncUser_InvalidJSONBody(t *testing.T) {
	mockService := &mockUserSyncService{}
	router := createTestRouter(mockService)

	// Send invalid JSON
	req := httptest.NewRequest("POST", "/api/internal/sync-user", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code, "Expected 400 Bad Request")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "invalid JSON body", response.Message)
}

func TestSyncUser_EmptyBody(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			// The service should validate and return an error
			return nil, errors.New("betterauth_user_id is required")
		},
	}
	router := createTestRouter(mockService)

	// Send empty JSON object
	req := createJSONRequest(t, "POST", "/api/internal/sync-user", map[string]string{})
	rr := executeRequest(router, req)

	// Empty body is valid JSON, but service should reject it
	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Expected 500 Internal Server Error")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "required")
}

// =============================================================================
// TEST: SUCCESSFUL SYNC
// =============================================================================

func TestSyncUser_Success(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			// Verify params were passed correctly
			assert.Equal(t, "test-user-uuid", params.BetterAuthUserID)
			assert.Equal(t, "john.doe@example.com", params.Email)
			assert.Equal(t, "John Doe", params.Name)
			assert.Equal(t, "org-uuid-123", params.OrganizationID)
			assert.Equal(t, "admin", params.Role)

			return &authService.UserSyncResult{
				PersonID:  100,
				StaffID:   200,
				TeacherID: 300,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-user-uuid",
		"email":              "john.doe@example.com",
		"name":               "John Doe",
		"organization_id":    "org-uuid-123",
		"role":               "admin",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created. Body: %s", rr.Body.String())

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, "user synced successfully", response.Message)
	assert.Equal(t, int64(100), response.PersonID)
	assert.Equal(t, int64(200), response.StaffID)
	assert.Equal(t, int64(300), response.TeacherID)
}

func TestSyncUser_Success_MemberRole_NoTeacher(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "member", params.Role)

			// Member role doesn't create teacher
			return &authService.UserSyncResult{
				PersonID:  101,
				StaffID:   201,
				TeacherID: 0, // No teacher created
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "member-uuid",
		"email":              "member@example.com",
		"name":               "Member User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status)
	assert.Equal(t, int64(101), response.PersonID)
	assert.Equal(t, int64(201), response.StaffID)
	assert.Equal(t, int64(0), response.TeacherID, "Member role should not have teacher ID")
}

func TestSyncUser_Success_OwnerRole(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "owner", params.Role)

			return &authService.UserSyncResult{
				PersonID:  102,
				StaffID:   202,
				TeacherID: 302, // Owner gets teacher record
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "owner-uuid",
		"email":              "owner@example.com",
		"name":               "Owner User",
		"organization_id":    "org-uuid",
		"role":               "owner",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code, "Expected 201 Created")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "success", response.Status)
	assert.NotZero(t, response.TeacherID, "Owner role should have teacher ID")
}

// =============================================================================
// TEST: SERVICE ERRORS
// =============================================================================

func TestSyncUser_ServiceError(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("database connection failed")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		"name":               "Test User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code, "Expected 500 Internal Server Error")

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Equal(t, "database connection failed", response.Message)
}

func TestSyncUser_ValidationError_MissingBetterAuthUserID(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("betterauth_user_id is required")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		// Missing betterauth_user_id
		"email":           "test@example.com",
		"name":            "Test User",
		"organization_id": "org-uuid",
		"role":            "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "betterauth_user_id is required")
}

func TestSyncUser_ValidationError_MissingEmail(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("email is required")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		// Missing email
		"name":            "Test User",
		"organization_id": "org-uuid",
		"role":            "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "email is required")
}

func TestSyncUser_ValidationError_MissingName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("name is required")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		// Missing name
		"organization_id": "org-uuid",
		"role":            "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "name is required")
}

func TestSyncUser_ValidationError_MissingOrganizationID(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("organization_id is required")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		"name":               "Test User",
		// Missing organization_id
		"role": "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "organization_id is required")
}

// =============================================================================
// TEST: DIFFERENT ROLES
// =============================================================================

func TestSyncUser_OGSAdminRole(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "ogsadmin", params.Role)
			return &authService.UserSyncResult{
				PersonID:  103,
				StaffID:   203,
				TeacherID: 303,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "ogsadmin-uuid",
		"email":              "ogsadmin@example.com",
		"name":               "OGS Admin",
		"organization_id":    "org-uuid",
		"role":               "ogsadmin",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_TraegerAdminRole(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "traegeradmin", params.Role)
			return &authService.UserSyncResult{
				PersonID:  104,
				StaffID:   204,
				TeacherID: 304,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "traegeradmin-uuid",
		"email":              "traegeradmin@example.com",
		"name":               "Traeger Admin",
		"organization_id":    "org-uuid",
		"role":               "traegeradmin",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_BueroAdminRole(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "bueroadmin", params.Role)
			return &authService.UserSyncResult{
				PersonID:  105,
				StaffID:   205,
				TeacherID: 305,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "bueroadmin-uuid",
		"email":              "bueroadmin@example.com",
		"name":               "Buero Admin",
		"organization_id":    "org-uuid",
		"role":               "bueroadmin",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

// =============================================================================
// TEST: NAME PARSING (via params verification)
// =============================================================================

func TestSyncUser_SingleName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			// Verify that single name is passed through
			assert.Equal(t, "SingleName", params.Name)
			return &authService.UserSyncResult{
				PersonID: 106,
				StaffID:  206,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "single-uuid",
		"email":              "single@example.com",
		"name":               "SingleName",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_MultipartName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			// Verify that multipart name is passed through
			assert.Equal(t, "Johann Wolfgang von Goethe", params.Name)
			return &authService.UserSyncResult{
				PersonID: 107,
				StaffID:  207,
			}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "multipart-uuid",
		"email":              "goethe@example.com",
		"name":               "Johann Wolfgang von Goethe",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

// =============================================================================
// TEST: CONTENT TYPE HANDLING
// =============================================================================

func TestSyncUser_ResponseContentType(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return &authService.UserSyncResult{PersonID: 1, StaffID: 1}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		"name":               "Test User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "Response should have JSON content type")
}

func TestSyncUser_ErrorResponseContentType(t *testing.T) {
	// Test that error responses also have JSON content type
	router := createTestRouter(nil) // nil service triggers 503

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
		"email":              "test@example.com",
		"name":               "Test User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	contentType := rr.Header().Get("Content-Type")
	assert.Equal(t, "application/json", contentType, "Error response should have JSON content type")
}

// =============================================================================
// TEST: EDGE CASES
// =============================================================================

func TestSyncUser_WhitespaceInName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			// Name with leading/trailing whitespace
			assert.Equal(t, "  Test User  ", params.Name)
			return &authService.UserSyncResult{PersonID: 108, StaffID: 208}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "whitespace-uuid",
		"email":              "whitespace@example.com",
		"name":               "  Test User  ",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_SpecialCharactersInName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "José María García-López", params.Name)
			return &authService.UserSyncResult{PersonID: 109, StaffID: 209}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "special-uuid",
		"email":              "special@example.com",
		"name":               "José María García-López",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_UnicodeCharactersInName(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, "田中太郎", params.Name)
			return &authService.UserSyncResult{PersonID: 110, StaffID: 210}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "unicode-uuid",
		"email":              "tanaka@example.com",
		"name":               "田中太郎",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestSyncUser_LongValues(t *testing.T) {
	longName := strings.Repeat("A", 500)
	longEmail := strings.Repeat("a", 200) + "@example.com"

	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			assert.Equal(t, longName, params.Name)
			assert.Equal(t, longEmail, params.Email)
			return &authService.UserSyncResult{PersonID: 111, StaffID: 211}, nil
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "long-uuid",
		"email":              longEmail,
		"name":               longName,
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

// =============================================================================
// TEST: CONCURRENT REQUESTS (race condition safety)
// =============================================================================

func TestSyncUser_ConcurrentRequests(t *testing.T) {
	callCount := 0
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, params authService.UserSyncParams) (*authService.UserSyncResult, error) {
			callCount++
			return &authService.UserSyncResult{
				PersonID: int64(callCount * 100),
				StaffID:  int64(callCount * 200),
			}, nil
		},
	}
	router := createTestRouter(mockService)

	// Run multiple requests concurrently
	done := make(chan bool, 3)

	for i := 0; i < 3; i++ {
		go func(idx int) {
			body := map[string]string{
				"betterauth_user_id": "concurrent-uuid-" + string(rune('A'+idx)),
				"email":              "concurrent" + string(rune('A'+idx)) + "@example.com",
				"name":               "Concurrent User " + string(rune('A'+idx)),
				"organization_id":    "org-uuid",
				"role":               "member",
			}

			req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
			rr := executeRequest(router, req)

			assert.Equal(t, http.StatusCreated, rr.Code)
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < 3; i++ {
		<-done
	}

	assert.Equal(t, 3, callCount, "Should have processed 3 requests")
}

// =============================================================================
// TEST: METHOD NOT ALLOWED (only POST is valid)
// =============================================================================

func TestSyncUser_MethodNotAllowed_GET(t *testing.T) {
	mockService := &mockUserSyncService{}
	router := createTestRouter(mockService)

	req := httptest.NewRequest("GET", "/api/internal/sync-user", nil)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "GET should not be allowed")
}

func TestSyncUser_MethodNotAllowed_PUT(t *testing.T) {
	mockService := &mockUserSyncService{}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "test-uuid",
	}
	req := createJSONRequest(t, "PUT", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "PUT should not be allowed")
}

func TestSyncUser_MethodNotAllowed_DELETE(t *testing.T) {
	mockService := &mockUserSyncService{}
	router := createTestRouter(mockService)

	req := httptest.NewRequest("DELETE", "/api/internal/sync-user", nil)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code, "DELETE should not be allowed")
}

// =============================================================================
// TEST: DIFFERENT ERROR SCENARIOS
// =============================================================================

func TestSyncUser_TransactionError(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("failed to set RLS context: pq: syntax error")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "tx-error-uuid",
		"email":              "txerror@example.com",
		"name":               "Tx Error User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "RLS context")
}

func TestSyncUser_PersonCreationError(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("failed to create person: duplicate key value")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "person-error-uuid",
		"email":              "personerror@example.com",
		"name":               "Person Error User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "failed to create person")
}

func TestSyncUser_StaffCreationError(t *testing.T) {
	mockService := &mockUserSyncService{
		syncUserFunc: func(_ context.Context, _ authService.UserSyncParams) (*authService.UserSyncResult, error) {
			return nil, errors.New("failed to create staff: foreign key violation")
		},
	}
	router := createTestRouter(mockService)

	body := map[string]string{
		"betterauth_user_id": "staff-error-uuid",
		"email":              "stafferror@example.com",
		"name":               "Staff Error User",
		"organization_id":    "org-uuid",
		"role":               "member",
	}

	req := createJSONRequest(t, "POST", "/api/internal/sync-user", body)
	rr := executeRequest(router, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)

	response := parseSyncUserResponse(t, rr.Body.Bytes())
	assert.Equal(t, "error", response.Status)
	assert.Contains(t, response.Message, "failed to create staff")
}
