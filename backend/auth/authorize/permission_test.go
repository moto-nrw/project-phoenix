package authorize_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
)

func TestRequiresPermission(t *testing.T) {
	tests := []struct {
		name             string
		requiredPerm     string
		userPermissions  []string
		expectedStatus   int
		expectedResponse string
	}{
		{
			name:             "allows access with exact permission",
			requiredPerm:     "users:read",
			userPermissions:  []string{"users:read", "users:write"},
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success",
		},
		{
			name:             "allows access with admin wildcard",
			requiredPerm:     "users:read",
			userPermissions:  []string{"admin:*"},
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success",
		},
		{
			name:             "allows access with resource wildcard",
			requiredPerm:     "users:read",
			userPermissions:  []string{"users:*"},
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success",
		},
		{
			name:             "allows access with full wildcard",
			requiredPerm:     "users:read",
			userPermissions:  []string{"*:*"},
			expectedStatus:   http.StatusOK,
			expectedResponse: "Success",
		},
		{
			name:             "denies access without permission",
			requiredPerm:     "users:read",
			userPermissions:  []string{"posts:read", "posts:write"},
			expectedStatus:   http.StatusForbidden,
			expectedResponse: "",
		},
		{
			name:             "denies access with empty permissions",
			requiredPerm:     "users:read",
			userPermissions:  []string{},
			expectedStatus:   http.StatusForbidden,
			expectedResponse: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("Success"))
			})

			// Create middleware chain
			middleware := authorize.RequiresPermission(tt.requiredPerm)
			protectedHandler := middleware(handler)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			// Add permissions to context
			ctx := context.WithValue(req.Context(), ctxKeyPermissions, tt.userPermissions)
			req = req.WithContext(ctx)

			// Execute request
			protectedHandler.ServeHTTP(rr, req)

			// Assert results
			assert.Equal(t, tt.expectedStatus, rr.Code)
			if tt.expectedResponse != "" {
				assert.Contains(t, rr.Body.String(), tt.expectedResponse)
			}
		})
	}
}

func TestRequiresAnyPermission(t *testing.T) {
	tests := []struct {
		name            string
		requiredPerms   []string
		userPermissions []string
		expectedStatus  int
	}{
		{
			name:            "allows access with one matching permission",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"users:read"},
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "allows access with multiple matching permissions",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"users:read", "users:write", "posts:read"},
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "denies access without any matching permission",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"posts:read", "posts:write"},
			expectedStatus:  http.StatusForbidden,
		},
		{
			name:            "allows access with wildcard",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"admin:*"},
			expectedStatus:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := authorize.RequiresAnyPermission(tt.requiredPerms...)
			protectedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			ctx := context.WithValue(req.Context(), ctxKeyPermissions, tt.userPermissions)
			req = req.WithContext(ctx)

			protectedHandler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

func TestRequiresAllPermissions(t *testing.T) {
	tests := []struct {
		name            string
		requiredPerms   []string
		userPermissions []string
		expectedStatus  int
	}{
		{
			name:            "allows access with all permissions",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"users:read", "users:write", "posts:read"},
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "denies access with only some permissions",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"users:read"},
			expectedStatus:  http.StatusForbidden,
		},
		{
			name:            "allows access with admin wildcard",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{"admin:*"},
			expectedStatus:  http.StatusOK,
		},
		{
			name:            "denies access without permissions",
			requiredPerms:   []string{"users:read", "users:write"},
			userPermissions: []string{},
			expectedStatus:  http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := authorize.RequiresAllPermissions(tt.requiredPerms...)
			protectedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			ctx := context.WithValue(req.Context(), ctxKeyPermissions, tt.userPermissions)
			req = req.WithContext(ctx)

			protectedHandler.ServeHTTP(rr, req)

			assert.Equal(t, tt.expectedStatus, rr.Code)
		})
	}
}

// Context key for testing
type ctxKey int

const ctxKeyPermissions ctxKey = iota

// Helper function to setup test middleware that injects permissions into context
// This simulates what the JWT middleware would do
func setupTestMiddleware(permissions []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKeyPermissions, permissions)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Test permission validation with wildcard patterns
func TestHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		required    string
		permissions []string
		expected    bool
	}{
		{
			name:        "exact match",
			required:    "users:read",
			permissions: []string{"users:read"},
			expected:    true,
		},
		{
			name:        "no match",
			required:    "users:read",
			permissions: []string{"posts:read"},
			expected:    false,
		},
		{
			name:        "admin wildcard",
			required:    "users:read",
			permissions: []string{"admin:*"},
			expected:    true,
		},
		{
			name:        "resource wildcard",
			required:    "users:read",
			permissions: []string{"users:*"},
			expected:    true,
		},
		{
			name:        "action wildcard",
			required:    "users:read",
			permissions: []string{"*:read"},
			expected:    false, // This should not match based on action alone
		},
		{
			name:        "full wildcard",
			required:    "users:read",
			permissions: []string{"*:*"},
			expected:    true,
		},
		{
			name:        "empty required",
			required:    "",
			permissions: []string{"users:read"},
			expected:    true,
		},
		{
			name:        "invalid format",
			required:    "invalid",
			permissions: []string{"users:read"},
			expected:    false,
		},
		{
			name:        "partial prefix match",
			required:    "users:read",
			permissions: []string{"user*"},
			expected:    false, // Should not match - need proper format
		},
	}

	// Note: These tests assume you have access to the hasPermission function
	// If it's not exported, you'll need to test it indirectly through the middleware
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Since hasPermission is private, we test it through the middleware
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			middleware := authorize.RequiresPermission(tt.required)
			protectedHandler := middleware(handler)

			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			ctx := context.WithValue(req.Context(), ctxKeyPermissions, tt.permissions)
			req = req.WithContext(ctx)

			protectedHandler.ServeHTTP(rr, req)

			if tt.expected {
				assert.Equal(t, http.StatusOK, rr.Code)
			} else {
				assert.Equal(t, http.StatusForbidden, rr.Code)
			}
		})
	}
}
