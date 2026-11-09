// Package auth internal tests for pure helper functions.
// These tests verify logic that doesn't require database access.
package account

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// normalizeBaseRole Tests
// =============================================================================

func TestNormalizeBaseRole(t *testing.T) {
	t.Parallel()

	t.Run("nil input returns nil", func(t *testing.T) {
		assert.Nil(t, trimPtrToNil(nil))
	})

	t.Run("empty string returns nil", func(t *testing.T) {
		empty := ""
		assert.Nil(t, trimPtrToNil(&empty))
	})

	t.Run("whitespace-only returns nil", func(t *testing.T) {
		spaces := "   "
		assert.Nil(t, trimPtrToNil(&spaces))
	})

	t.Run("trims whitespace", func(t *testing.T) {
		padded := "  admin  "
		result := trimPtrToNil(&padded)
		require.NotNil(t, result)
		assert.Equal(t, "admin", *result)
	})

	t.Run("passes through clean value", func(t *testing.T) {
		clean := "guardian"
		result := trimPtrToNil(&clean)
		require.NotNil(t, result)
		assert.Equal(t, "guardian", *result)
	})
}

// =============================================================================
// validateBaseRole / validateBaseRoleValue Tests
// =============================================================================

func TestValidateBaseRole(t *testing.T) {
	t.Parallel()

	t.Run("nil is rejected", func(t *testing.T) {
		err := validateBaseRole(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role is required")
	})

	t.Run("valid values accepted", func(t *testing.T) {
		for _, role := range []string{"admin", "user", "guardian"} {
			r := role
			assert.NoError(t, validateBaseRole(&r), "expected %q to be valid", role)
		}
	})

	t.Run("invalid value rejected", func(t *testing.T) {
		bad := "superadmin"
		err := validateBaseRole(&bad)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role must be one of")
	})

	t.Run("case sensitive", func(t *testing.T) {
		upper := "Admin"
		err := validateBaseRole(&upper)
		require.Error(t, err, "base_role validation must be case-sensitive")
	})
}

func TestValidateBaseRoleValue(t *testing.T) {
	t.Parallel()

	t.Run("empty string rejected", func(t *testing.T) {
		assert.Error(t, validateBaseRoleValue(""))
	})

	t.Run("unknown role rejected", func(t *testing.T) {
		assert.Error(t, validateBaseRoleValue("staff"))
	})

	t.Run("all valid roles accepted", func(t *testing.T) {
		for _, role := range identityaccess.ValidBaseRoles() {
			assert.NoError(t, validateBaseRoleValue(role))
		}
	})
}

// =============================================================================
// toRoleResponse Tests
// =============================================================================

func TestToRoleResponse(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC)
	baseRole := "admin"

	t.Run("maps all fields including base_role", func(t *testing.T) {
		role := identityaccess.Role{
			ID: 42, CreatedAt: now, UpdatedAt: now,
			Name:        "custom-admin",
			Description: "A custom admin role",
			IsSystem:    false,
			BaseRole:    &baseRole,
		}

		resp := toRoleResponse(role)

		assert.Equal(t, int64(42), resp.ID)
		assert.Equal(t, "custom-admin", resp.Name)
		assert.Equal(t, "A custom admin role", resp.Description)
		assert.False(t, resp.IsSystem)
		require.NotNil(t, resp.BaseRole)
		assert.Equal(t, "admin", *resp.BaseRole)
		assert.Equal(t, "2026-01-15T10:30:00Z", resp.CreatedAt)
		assert.Equal(t, "2026-01-15T10:30:00Z", resp.UpdatedAt)
		assert.Nil(t, resp.Permissions, "Permissions not set by toRoleResponse")
	})

	t.Run("nil base_role preserved", func(t *testing.T) {
		role := identityaccess.Role{
			ID: 1, CreatedAt: now, UpdatedAt: now,
			Name:     "legacy-role",
			BaseRole: nil,
		}

		resp := toRoleResponse(role)
		assert.Nil(t, resp.BaseRole)
	})

	t.Run("system role flag preserved", func(t *testing.T) {
		role := identityaccess.Role{
			ID: 1, CreatedAt: now, UpdatedAt: now,
			Name:     "admin",
			IsSystem: true,
			BaseRole: nil,
		}

		resp := toRoleResponse(role)
		assert.True(t, resp.IsSystem)
		assert.Nil(t, resp.BaseRole)
	})
}

// =============================================================================
// CreateRoleRequest.Bind / UpdateRoleRequest.Bind Tests
// =============================================================================

func TestCreateRoleRequest_Bind_BaseRoleValidation(t *testing.T) {
	t.Parallel()

	t.Run("missing base_role rejected", func(t *testing.T) {
		req := &CreateRoleRequest{Name: "test", Description: "desc"}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role is required")
	})

	t.Run("invalid base_role rejected", func(t *testing.T) {
		bad := "manager"
		req := &CreateRoleRequest{Name: "test", Description: "desc", BaseRole: &bad}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role must be one of")
	})

	t.Run("valid base_role accepted", func(t *testing.T) {
		valid := "guardian"
		req := &CreateRoleRequest{Name: "test", Description: "desc", BaseRole: &valid}
		err := req.Bind(nil)
		assert.NoError(t, err)
	})

	t.Run("whitespace base_role normalized then rejected as nil", func(t *testing.T) {
		spaces := "   "
		req := &CreateRoleRequest{Name: "test", Description: "desc", BaseRole: &spaces}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role is required")
	})
}

func TestUpdateRoleRequest_Bind_BaseRoleValidation(t *testing.T) {
	t.Parallel()

	t.Run("nil base_role accepted on update", func(t *testing.T) {
		req := &UpdateRoleRequest{Name: "test", Description: "desc"}
		err := req.Bind(nil)
		assert.NoError(t, err)
	})

	t.Run("valid base_role accepted on update", func(t *testing.T) {
		valid := "admin"
		req := &UpdateRoleRequest{Name: "test", Description: "desc", BaseRole: &valid}
		err := req.Bind(nil)
		assert.NoError(t, err)
	})

	t.Run("invalid base_role rejected on update", func(t *testing.T) {
		bad := "owner"
		req := &UpdateRoleRequest{Name: "test", Description: "desc", BaseRole: &bad}
		err := req.Bind(nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "base_role must be one of")
	})
}

// TestRouterWithAuthRateLimiter pins which routes the production composition
// throttles: the public credential and second-factor exchanges, and nothing
// else. The route golden builds the router without a limiter, so this is the
// only check on the limiter's placement.
func TestRouterWithAuthRateLimiter(t *testing.T) {
	t.Parallel()

	throttled := func(http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
	router := NewResource(nil, nil, nil, nil, nil, transactionFailures.IsConstraintViolation, unusedTenantSettings{}).RouterWithAuthRateLimiter(throttled, scopeTestTenantSession(testpkg.TenantSessionGates()))

	for _, path := range []string{
		"/login",
		"/password-reset",
		"/password-reset/confirm",
		"/mfa/verify",
		"/mfa/resend",
		"/passkeys/login/options",
		"/passkeys/login/verify",
	} {
		t.Run("throttled POST "+path, func(t *testing.T) {
			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, path, nil))
			assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		})
	}

	t.Run("unthrottled GET /tenant/resolve", func(t *testing.T) {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/tenant/resolve", nil))
		assert.Equal(t, http.StatusBadRequest, rr.Code, "missing slug is rejected by the handler, not the limiter")
	})

	t.Run("unthrottled POST /register", func(t *testing.T) {
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/register", nil))
		assert.Equal(t, http.StatusUnauthorized, rr.Code, "register sits behind the access-token group, not the limiter")
	})
}
