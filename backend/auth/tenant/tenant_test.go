package tenant_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextHelpers tests all context helper functions from context.go
func TestContextHelpers(t *testing.T) {
	t.Run("SetTenantContext and TenantFromCtx", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			UserID:      "user-123",
			UserEmail:   "test@example.com",
			UserName:    "Test User",
			OrgID:       "org-456",
			OrgName:     "Test OGS",
			OrgSlug:     "test-ogs",
			Role:        "supervisor",
			Permissions: []string{"student:read", "location:read"},
			TraegerID:   "traeger-789",
			TraegerName: "Test Traeger",
		}

		newCtx := tenant.SetTenantContext(ctx, tc)
		retrieved := tenant.TenantFromCtx(newCtx)

		require.NotNil(t, retrieved)
		assert.Equal(t, tc.UserID, retrieved.UserID)
		assert.Equal(t, tc.OrgID, retrieved.OrgID)
		assert.Equal(t, tc.Role, retrieved.Role)
	})

	t.Run("TenantFromCtx returns nil for empty context", func(t *testing.T) {
		ctx := context.Background()
		retrieved := tenant.TenantFromCtx(ctx)
		assert.Nil(t, retrieved)
	})

	t.Run("PermissionsFromCtx", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"student:read", "group:read"},
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		perms := tenant.PermissionsFromCtx(newCtx)
		assert.Contains(t, perms, "student:read")
		assert.Contains(t, perms, "group:read")
	})

	t.Run("PermissionsFromCtx returns empty slice for empty context", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.PermissionsFromCtx(ctx)
		assert.Empty(t, perms)
	})

	t.Run("BueroIDFromCtx with Buero", func(t *testing.T) {
		ctx := context.Background()
		bueroID := "buero-123"
		tc := &tenant.TenantContext{
			BueroID:   &bueroID,
			BueroName: stringPtr("Test Buero"),
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		result := tenant.BueroIDFromCtx(newCtx)
		require.NotNil(t, result)
		assert.Equal(t, bueroID, *result)
	})

	t.Run("BueroIDFromCtx without Buero", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			BueroID: nil,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		result := tenant.BueroIDFromCtx(newCtx)
		assert.Nil(t, result)
	})

	t.Run("BueroIDFromCtx returns nil for empty context", func(t *testing.T) {
		ctx := context.Background()
		result := tenant.BueroIDFromCtx(ctx)
		assert.Nil(t, result)
	})
}

// TestRoleChecks tests role-based helper functions
func TestRoleChecks(t *testing.T) {
	testCases := []struct {
		name         string
		role         string
		isAdmin      bool
		isSupervisor bool
		canManageOGS bool
	}{
		{
			name:         "supervisor role",
			role:         "supervisor",
			isAdmin:      false,
			isSupervisor: true,
			canManageOGS: false,
		},
		{
			name:         "ogsAdmin role",
			role:         "ogsAdmin",
			isAdmin:      true,
			isSupervisor: false,
			canManageOGS: true,
		},
		{
			name:         "bueroAdmin role",
			role:         "bueroAdmin",
			isAdmin:      true,
			isSupervisor: false,
			canManageOGS: true,
		},
		{
			name:         "traegerAdmin role",
			role:         "traegerAdmin",
			isAdmin:      true,
			isSupervisor: false,
			canManageOGS: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			perms := tenant.GetPermissionsForRole(tc.role)
			tenantCtx := &tenant.TenantContext{
				Role:        tc.role,
				Permissions: perms,
			}
			newCtx := tenant.SetTenantContext(ctx, tenantCtx)

			assert.Equal(t, tc.isAdmin, tenant.IsAdmin(newCtx), "IsAdmin mismatch")
			assert.Equal(t, tc.isSupervisor, tenant.IsSupervisor(newCtx), "IsSupervisor mismatch")
			assert.Equal(t, tc.canManageOGS, tenant.CanManageOGS(newCtx), "CanManageOGS mismatch")
		})
	}

	t.Run("empty context returns false for all checks", func(t *testing.T) {
		ctx := context.Background()
		assert.False(t, tenant.IsAdmin(ctx))
		assert.False(t, tenant.IsSupervisor(ctx))
		assert.False(t, tenant.CanManageOGS(ctx))
		assert.False(t, tenant.CanManageStaff(ctx))
	})
}

// TestLocationPermission tests the GDPR-sensitive location permission check
func TestLocationPermission(t *testing.T) {
	t.Run("supervisor has location permission", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("supervisor")
		tc := &tenant.TenantContext{
			Role:        "supervisor",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.HasLocationPermission(newCtx))
	})

	t.Run("ogsAdmin has location permission", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("ogsAdmin")
		tc := &tenant.TenantContext{
			Role:        "ogsAdmin",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.HasLocationPermission(newCtx))
	})

	t.Run("bueroAdmin does NOT have location permission (GDPR)", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("bueroAdmin")
		tc := &tenant.TenantContext{
			Role:        "bueroAdmin",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.False(t, tenant.HasLocationPermission(newCtx), "GDPR violation: bueroAdmin should not have location permission")
	})

	t.Run("traegerAdmin does NOT have location permission (GDPR)", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("traegerAdmin")
		tc := &tenant.TenantContext{
			Role:        "traegerAdmin",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.False(t, tenant.HasLocationPermission(newCtx), "GDPR violation: traegerAdmin should not have location permission")
	})
}

// TestStaffManagementPermission tests the CanManageStaff helper
func TestStaffManagementPermission(t *testing.T) {
	t.Run("ogsAdmin can manage staff", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("ogsAdmin")
		tc := &tenant.TenantContext{
			Role:        "ogsAdmin",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.CanManageStaff(newCtx))
	})

	t.Run("supervisor cannot manage staff", func(t *testing.T) {
		ctx := context.Background()
		perms := tenant.GetPermissionsForRole("supervisor")
		tc := &tenant.TenantContext{
			Role:        "supervisor",
			Permissions: perms,
		}
		newCtx := tenant.SetTenantContext(ctx, tc)

		assert.False(t, tenant.CanManageStaff(newCtx))
	})
}

// TestRolesMap tests the roles.go utility functions
func TestRolesMap(t *testing.T) {
	t.Run("GetPermissionsForRole returns correct permissions", func(t *testing.T) {
		supervisorPerms := tenant.GetPermissionsForRole("supervisor")
		assert.Contains(t, supervisorPerms, "student:read")
		assert.Contains(t, supervisorPerms, "location:read")
		assert.NotContains(t, supervisorPerms, "student:delete")
	})

	t.Run("GetPermissionsForRole returns empty for unknown role", func(t *testing.T) {
		perms := tenant.GetPermissionsForRole("unknown_role")
		assert.Empty(t, perms)
	})

	t.Run("IsValidRole", func(t *testing.T) {
		assert.True(t, tenant.IsValidRole("supervisor"))
		assert.True(t, tenant.IsValidRole("ogsAdmin"))
		assert.True(t, tenant.IsValidRole("bueroAdmin"))
		assert.True(t, tenant.IsValidRole("traegerAdmin"))
		assert.False(t, tenant.IsValidRole("unknown"))
		assert.False(t, tenant.IsValidRole(""))
	})

	t.Run("AllRoles returns all valid roles", func(t *testing.T) {
		roles := tenant.AllRoles()
		assert.Len(t, roles, 4)
		assert.Contains(t, roles, "supervisor")
		assert.Contains(t, roles, "ogsAdmin")
		assert.Contains(t, roles, "bueroAdmin")
		assert.Contains(t, roles, "traegerAdmin")
	})

	t.Run("RolesWithPermission for location:read", func(t *testing.T) {
		roles := tenant.RolesWithPermission("location:read")
		assert.Contains(t, roles, "supervisor")
		assert.Contains(t, roles, "ogsAdmin")
		assert.NotContains(t, roles, "bueroAdmin")
		assert.NotContains(t, roles, "traegerAdmin")
	})

	t.Run("GetPermissionsForRole returns a copy", func(t *testing.T) {
		perms1 := tenant.GetPermissionsForRole("supervisor")
		perms2 := tenant.GetPermissionsForRole("supervisor")

		// Modify perms1
		if len(perms1) > 0 {
			perms1[0] = "modified"
		}

		// perms2 should be unchanged
		assert.NotEqual(t, perms1[0], perms2[0])
	})
}

// TestErrorResponses tests the error response constructors
func TestErrorResponses(t *testing.T) {
	t.Run("NewErrUnauthorized creates correct response", func(t *testing.T) {
		err := tenant.NewErrUnauthorized("custom unauthorized message")
		assert.Equal(t, http.StatusUnauthorized, err.HTTPStatusCode)
		assert.Equal(t, "error", err.StatusText)
		assert.Equal(t, "custom unauthorized message", err.ErrorText)
	})

	t.Run("NewErrForbidden creates correct response", func(t *testing.T) {
		err := tenant.NewErrForbidden("custom forbidden message")
		assert.Equal(t, http.StatusForbidden, err.HTTPStatusCode)
		assert.Equal(t, "error", err.StatusText)
		assert.Equal(t, "custom forbidden message", err.ErrorText)
	})

	t.Run("NewErrInternal creates correct response", func(t *testing.T) {
		err := tenant.NewErrInternal("custom internal error")
		assert.Equal(t, http.StatusInternalServerError, err.HTTPStatusCode)
		assert.Equal(t, "error", err.StatusText)
		assert.Equal(t, "custom internal error", err.ErrorText)
	})

	t.Run("ErrResponse Render sets status code", func(t *testing.T) {
		err := tenant.NewErrUnauthorized("test")
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		w := httptest.NewRecorder()

		renderErr := err.Render(w, req)
		assert.NoError(t, renderErr)
	})
}

// TestPermissionMiddleware tests the permission middleware functions
func TestPermissionMiddleware(t *testing.T) {
	t.Run("RequiresPermission allows with permission", func(t *testing.T) {
		handler := tenant.RequiresPermission("student:read")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"student:read", "student:update"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RequiresPermission blocks without permission", func(t *testing.T) {
		handler := tenant.RequiresPermission("student:delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"student:read"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("RequiresAnyPermission allows with one matching permission", func(t *testing.T) {
		handler := tenant.RequiresAnyPermission("staff:create", "staff:update")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"staff:update"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RequiresAnyPermission blocks without any matching permission", func(t *testing.T) {
		handler := tenant.RequiresAnyPermission("staff:create", "staff:delete")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"student:read"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("RequiresAllPermissions allows with all permissions", func(t *testing.T) {
		handler := tenant.RequiresAllPermissions("group:update", "group:assign")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"group:update", "group:assign", "group:read"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("RequiresAllPermissions blocks with partial permissions", func(t *testing.T) {
		handler := tenant.RequiresAllPermissions("group:update", "group:assign")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))

		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"group:update"}, // Missing group:assign
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		req := httptest.NewRequest(http.MethodGet, "/test", nil).WithContext(ctx)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}

// TestPermissionWildcards tests wildcard permission matching
func TestPermissionWildcards(t *testing.T) {
	t.Run("admin:* matches any admin action", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"admin:*"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.HasPermission(ctx, "student:read"))
		assert.True(t, tenant.HasPermission(ctx, "staff:delete"))
	})

	t.Run("*:* matches everything (superuser)", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"*:*"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.HasPermission(ctx, "anything:here"))
	})

	t.Run("student:* matches any student action", func(t *testing.T) {
		ctx := context.Background()
		tc := &tenant.TenantContext{
			Permissions: []string{"student:*"},
		}
		ctx = tenant.SetTenantContext(ctx, tc)

		assert.True(t, tenant.HasPermission(ctx, "student:read"))
		assert.True(t, tenant.HasPermission(ctx, "student:delete"))
		assert.False(t, tenant.HasPermission(ctx, "staff:read"))
	})
}

// TestNewPermissionsAddedInWP6Prep tests the new permissions added for WP6
func TestNewPermissionsAddedInWP6Prep(t *testing.T) {
	t.Run("supervisor has schedule, feedback, substitution read permissions", func(t *testing.T) {
		perms := tenant.GetPermissionsForRole("supervisor")
		assert.Contains(t, perms, "schedule:read")
		assert.Contains(t, perms, "feedback:read")
		assert.Contains(t, perms, "feedback:create")
		assert.Contains(t, perms, "substitution:read")
	})

	t.Run("ogsAdmin has full config, schedule, feedback, substitution permissions", func(t *testing.T) {
		perms := tenant.GetPermissionsForRole("ogsAdmin")
		// Config
		assert.Contains(t, perms, "config:read")
		assert.Contains(t, perms, "config:update")
		// Schedule
		assert.Contains(t, perms, "schedule:read")
		assert.Contains(t, perms, "schedule:create")
		assert.Contains(t, perms, "schedule:update")
		assert.Contains(t, perms, "schedule:delete")
		// Feedback
		assert.Contains(t, perms, "feedback:read")
		assert.Contains(t, perms, "feedback:create")
		assert.Contains(t, perms, "feedback:delete")
		// Substitution
		assert.Contains(t, perms, "substitution:read")
		assert.Contains(t, perms, "substitution:create")
		assert.Contains(t, perms, "substitution:update")
		assert.Contains(t, perms, "substitution:delete")
	})

	t.Run("bueroAdmin has same new permissions as ogsAdmin", func(t *testing.T) {
		perms := tenant.GetPermissionsForRole("bueroAdmin")
		assert.Contains(t, perms, "config:read")
		assert.Contains(t, perms, "schedule:create")
		assert.Contains(t, perms, "feedback:delete")
		assert.Contains(t, perms, "substitution:update")
	})

	t.Run("traegerAdmin has same new permissions as ogsAdmin", func(t *testing.T) {
		perms := tenant.GetPermissionsForRole("traegerAdmin")
		assert.Contains(t, perms, "config:update")
		assert.Contains(t, perms, "schedule:delete")
		assert.Contains(t, perms, "feedback:create")
		assert.Contains(t, perms, "substitution:read")
	})
}

func stringPtr(s string) *string {
	return &s
}
