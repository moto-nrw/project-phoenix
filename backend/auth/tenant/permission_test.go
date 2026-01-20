package tenant_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/tenant"
	"github.com/stretchr/testify/assert"
)

func TestHasPermission(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		required    string
		expected    bool
	}{
		{
			name:        "exact match",
			permissions: []string{"student:read"},
			required:    "student:read",
			expected:    true,
		},
		{
			name:        "no match",
			permissions: []string{"student:read"},
			required:    "student:write",
			expected:    false,
		},
		{
			name:        "wildcard action",
			permissions: []string{"student:*"},
			required:    "student:read",
			expected:    true,
		},
		{
			name:        "wildcard resource",
			permissions: []string{"*:read"},
			required:    "student:read",
			expected:    true,
		},
		{
			name:        "admin wildcard",
			permissions: []string{"admin:*"},
			required:    "student:read",
			expected:    true,
		},
		{
			name:        "superuser wildcard",
			permissions: []string{"*:*"},
			required:    "anything:here",
			expected:    true,
		},
		{
			name:        "empty required permission",
			permissions: []string{},
			required:    "",
			expected:    true,
		},
		{
			name:        "multiple permissions - match",
			permissions: []string{"group:read", "student:read", "room:read"},
			required:    "student:read",
			expected:    true,
		},
		{
			name:        "GDPR - supervisor has location:read",
			permissions: tenant.GetPermissionsForRole("supervisor"),
			required:    "location:read",
			expected:    true,
		},
		{
			name:        "GDPR - bueroAdmin has NO location:read",
			permissions: tenant.GetPermissionsForRole("bueroAdmin"),
			required:    "location:read",
			expected:    false,
		},
		{
			name:        "GDPR - traegerAdmin has NO location:read",
			permissions: tenant.GetPermissionsForRole("traegerAdmin"),
			required:    "location:read",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), tenant.CtxPermissions, tt.permissions)
			result := tenant.HasPermission(ctx, tt.required)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAnyPermission(t *testing.T) {
	ctx := context.WithValue(context.Background(), tenant.CtxPermissions, []string{"student:read", "group:read"})

	assert.True(t, tenant.HasAnyPermission(ctx, "student:read", "room:read"))
	assert.True(t, tenant.HasAnyPermission(ctx, "room:read", "group:read"))
	assert.False(t, tenant.HasAnyPermission(ctx, "room:read", "staff:read"))
}

func TestHasAllPermissions(t *testing.T) {
	ctx := context.WithValue(context.Background(), tenant.CtxPermissions, []string{"student:read", "group:read"})

	assert.True(t, tenant.HasAllPermissions(ctx, "student:read"))
	assert.True(t, tenant.HasAllPermissions(ctx, "student:read", "group:read"))
	assert.False(t, tenant.HasAllPermissions(ctx, "student:read", "room:read"))
}

func TestRequiresPermission_Allowed(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Setup context with permissions
	tc := &tenant.TenantContext{
		Permissions: []string{"student:read"},
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := tenant.SetTenantContext(req.Context(), tc)
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	middleware := tenant.RequiresPermission("student:read")
	middleware(handler).ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)
}

func TestRequiresPermission_Forbidden(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Setup context with different permissions
	tc := &tenant.TenantContext{
		Permissions: []string{"group:read"},
	}
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	ctx := tenant.SetTenantContext(req.Context(), tc)
	req = req.WithContext(ctx)

	recorder := httptest.NewRecorder()
	middleware := tenant.RequiresPermission("student:read")
	middleware(handler).ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusForbidden, recorder.Code)
}

func TestGetPermissionsForRole(t *testing.T) {
	// Test known roles return permissions
	assert.NotEmpty(t, tenant.GetPermissionsForRole("supervisor"))
	assert.NotEmpty(t, tenant.GetPermissionsForRole("ogsAdmin"))
	assert.NotEmpty(t, tenant.GetPermissionsForRole("bueroAdmin"))
	assert.NotEmpty(t, tenant.GetPermissionsForRole("traegerAdmin"))

	// Test unknown role returns empty
	assert.Empty(t, tenant.GetPermissionsForRole("unknown"))
}

func TestRoleGDPRCompliance(t *testing.T) {
	// Operational roles MUST have location:read
	supervisorPerms := tenant.GetPermissionsForRole("supervisor")
	assert.Contains(t, supervisorPerms, "location:read", "supervisor needs location:read for operations")

	ogsAdminPerms := tenant.GetPermissionsForRole("ogsAdmin")
	assert.Contains(t, ogsAdminPerms, "location:read", "ogsAdmin needs location:read for operations")

	// Administrative roles MUST NOT have location:read (GDPR)
	bueroAdminPerms := tenant.GetPermissionsForRole("bueroAdmin")
	assert.NotContains(t, bueroAdminPerms, "location:read", "bueroAdmin MUST NOT have location:read (GDPR)")

	traegerAdminPerms := tenant.GetPermissionsForRole("traegerAdmin")
	assert.NotContains(t, traegerAdminPerms, "location:read", "traegerAdmin MUST NOT have location:read (GDPR)")
}

func TestTenantContextHelpers(t *testing.T) {
	tc := &tenant.TenantContext{
		UserID:      "user-123",
		OrgID:       "org-456",
		Role:        "supervisor",
		TraegerID:   "traeger-789",
		Permissions: []string{"student:read"},
	}

	ctx := tenant.SetTenantContext(context.Background(), tc)

	assert.Equal(t, "user-123", tenant.UserIDFromCtx(ctx))
	assert.Equal(t, "org-456", tenant.OrgIDFromCtx(ctx))
	assert.Equal(t, "supervisor", tenant.RoleFromCtx(ctx))
	assert.Equal(t, "traeger-789", tenant.TraegerIDFromCtx(ctx))

	// Test nil context returns empty/nil
	assert.Equal(t, "", tenant.UserIDFromCtx(context.Background()))
	assert.Nil(t, tenant.TenantFromCtx(context.Background()))
}

func TestPermissionChecker(t *testing.T) {
	tc := &tenant.TenantContext{
		Permissions: []string{"student:read", "group:read", "location:read"},
	}
	ctx := tenant.SetTenantContext(context.Background(), tc)

	pc := tenant.NewPermissionChecker(ctx)

	assert.True(t, pc.Can("student:read"))
	assert.False(t, pc.Can("staff:read"))
	assert.True(t, pc.CanAny("staff:read", "student:read"))
	assert.False(t, pc.CanAll("student:read", "staff:read"))
	assert.True(t, pc.CanReadLocation())
}
