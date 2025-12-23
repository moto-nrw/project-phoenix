package authorize

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRolesTestAuth(t *testing.T) *jwt.TokenAuth {
	t.Helper()
	viper.Set("auth_jwt_expiry", 15*time.Minute)
	viper.Set("auth_jwt_refresh_expiry", 24*time.Hour)

	auth, err := jwt.NewTokenAuthWithSecret("test-secret-for-roles-testing!!")
	require.NoError(t, err)
	return auth
}

func TestRequiresRole_AllowsWithMatchingRole(t *testing.T) {
	auth := setupRolesTestAuth(t)

	// Create a token with the required role
	claims := jwt.AppClaims{
		ID:          1,
		Sub:         "user@example.com",
		Roles:       []string{"admin", "teacher"},
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	})

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(jwt.Authenticator)
	r.With(RequiresRole("admin")).Get("/admin", handler)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "success", rr.Body.String())
}

func TestRequiresRole_DeniesWithMissingRole(t *testing.T) {
	auth := setupRolesTestAuth(t)

	// Create a token without the required role
	claims := jwt.AppClaims{
		ID:          1,
		Sub:         "user@example.com",
		Roles:       []string{"user"}, // Not "admin"
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(jwt.Authenticator)
	r.With(RequiresRole("admin")).Get("/admin", handler)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequiresRole_DeniesWithEmptyRoles(t *testing.T) {
	auth := setupRolesTestAuth(t)

	// Create a token with no roles
	claims := jwt.AppClaims{
		ID:          1,
		Sub:         "user@example.com",
		Roles:       []string{},
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(jwt.Authenticator)
	r.With(RequiresRole("admin")).Get("/admin", handler)

	req := httptest.NewRequest("GET", "/admin", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestRequiresRole_MultipleRolesAllowsMatch(t *testing.T) {
	auth := setupRolesTestAuth(t)

	// Create a token with multiple roles
	claims := jwt.AppClaims{
		ID:          1,
		Sub:         "user@example.com",
		Roles:       []string{"user", "moderator", "editor"},
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(jwt.Authenticator)
	r.With(RequiresRole("editor")).Get("/editor", handler)

	req := httptest.NewRequest("GET", "/editor", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestHasRole(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		roles    []string
		expected bool
	}{
		{
			name:     "role exists",
			role:     "admin",
			roles:    []string{"user", "admin", "moderator"},
			expected: true,
		},
		{
			name:     "role does not exist",
			role:     "admin",
			roles:    []string{"user", "moderator"},
			expected: false,
		},
		{
			name:     "empty roles slice",
			role:     "admin",
			roles:    []string{},
			expected: false,
		},
		{
			name:     "nil roles slice",
			role:     "admin",
			roles:    nil,
			expected: false,
		},
		{
			name:     "single role match",
			role:     "admin",
			roles:    []string{"admin"},
			expected: true,
		},
		{
			name:     "case sensitive - no match",
			role:     "Admin",
			roles:    []string{"admin"},
			expected: false,
		},
		{
			name:     "empty role search",
			role:     "",
			roles:    []string{"admin", "user"},
			expected: false,
		},
		{
			name:     "empty role in list",
			role:     "",
			roles:    []string{"admin", "", "user"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hasRole(tt.role, tt.roles)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test that claims are properly retrieved from context
func TestRequiresRole_UsesClaimsFromContext(t *testing.T) {
	auth := setupRolesTestAuth(t)

	// Create a token
	claims := jwt.AppClaims{
		ID:          42,
		Sub:         "specific@example.com",
		Username:    "specificuser",
		Roles:       []string{"specific-role"},
		Permissions: []string{},
	}

	token, err := auth.CreateJWT(claims)
	require.NoError(t, err)

	var capturedClaims jwt.AppClaims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedClaims = jwt.ClaimsFromCtx(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	r := chi.NewRouter()
	r.Use(auth.Verifier())
	r.Use(jwt.Authenticator)
	r.With(RequiresRole("specific-role")).Get("/specific", handler)

	req := httptest.NewRequest("GET", "/specific", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 42, capturedClaims.ID)
	assert.Equal(t, "specificuser", capturedClaims.Username)
}

// Test RequiresRole with context that has claims directly (without going through full auth flow)
func TestRequiresRole_DirectContextClaims(t *testing.T) {
	// This tests the middleware when claims are already in context
	claims := jwt.AppClaims{
		ID:          1,
		Sub:         "user@example.com",
		Roles:       []string{"supervisor"},
		Permissions: []string{},
	}

	// Set up the context with claims
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, claims)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("passed"))
	})

	middleware := RequiresRole("supervisor")
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "passed", rr.Body.String())
}
