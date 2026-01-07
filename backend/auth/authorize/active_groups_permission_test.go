package authorize_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/stretchr/testify/assert"
)

// TestActiveGroupsPermissions verifies that the permission middleware correctly
// handles the specific permission strings used by the active groups API.
// This complements the generic permission_test.go by testing real permission constants.
func TestActiveGroupsPermissions(t *testing.T) {
	tests := []struct {
		name            string
		requiredPerm    string
		userPermissions []string
		expectedStatus  int
		description     string
	}{
		// GroupsRead permission tests
		{
			name:            "user with groups:read can list active groups",
			requiredPerm:    permissions.GroupsRead,
			userPermissions: []string{permissions.GroupsRead},
			expectedStatus:  http.StatusOK,
			description:     "GET /api/active/groups requires groups:read",
		},
		{
			name:            "user without groups:read cannot list active groups",
			requiredPerm:    permissions.GroupsRead,
			userPermissions: []string{permissions.ActivitiesRead}, // wrong permission
			expectedStatus:  http.StatusForbidden,
			description:     "GET /api/active/groups denies users without groups:read",
		},

		// GroupsCreate permission tests
		{
			name:            "user with groups:create can create active groups",
			requiredPerm:    permissions.GroupsCreate,
			userPermissions: []string{permissions.GroupsCreate},
			expectedStatus:  http.StatusOK,
			description:     "POST /api/active/groups requires groups:create",
		},
		{
			name:            "user with only groups:read cannot create active groups",
			requiredPerm:    permissions.GroupsCreate,
			userPermissions: []string{permissions.GroupsRead},
			expectedStatus:  http.StatusForbidden,
			description:     "POST /api/active/groups denies users with only groups:read",
		},

		// GroupsUpdate permission tests
		{
			name:            "user with groups:update can update active groups",
			requiredPerm:    permissions.GroupsUpdate,
			userPermissions: []string{permissions.GroupsUpdate},
			expectedStatus:  http.StatusOK,
			description:     "PUT /api/active/groups/{id} requires groups:update",
		},
		{
			name:            "user with groups:update can end active group session",
			requiredPerm:    permissions.GroupsUpdate,
			userPermissions: []string{permissions.GroupsUpdate},
			expectedStatus:  http.StatusOK,
			description:     "POST /api/active/groups/{id}/end requires groups:update",
		},
		{
			name:            "user with groups:update can claim group",
			requiredPerm:    permissions.GroupsUpdate,
			userPermissions: []string{permissions.GroupsUpdate},
			expectedStatus:  http.StatusOK,
			description:     "POST /api/active/groups/{id}/claim requires groups:update",
		},

		// GroupsDelete permission tests
		{
			name:            "user with groups:delete can delete active groups",
			requiredPerm:    permissions.GroupsDelete,
			userPermissions: []string{permissions.GroupsDelete},
			expectedStatus:  http.StatusOK,
			description:     "DELETE /api/active/groups/{id} requires groups:delete",
		},
		{
			name:            "user without groups:delete cannot delete active groups",
			requiredPerm:    permissions.GroupsDelete,
			userPermissions: []string{permissions.GroupsRead, permissions.GroupsCreate, permissions.GroupsUpdate},
			expectedStatus:  http.StatusForbidden,
			description:     "DELETE requires explicit groups:delete permission",
		},

		// GroupsAssign permission tests (for supervisors)
		{
			name:            "user with groups:assign can create supervisors",
			requiredPerm:    permissions.GroupsAssign,
			userPermissions: []string{permissions.GroupsAssign},
			expectedStatus:  http.StatusOK,
			description:     "POST /api/active/supervisors requires groups:assign",
		},
		{
			name:            "user without groups:assign cannot create supervisors",
			requiredPerm:    permissions.GroupsAssign,
			userPermissions: []string{permissions.GroupsCreate, permissions.GroupsUpdate},
			expectedStatus:  http.StatusForbidden,
			description:     "Supervisor management requires explicit groups:assign",
		},

		// Wildcard permission tests
		{
			name:            "admin:* grants access to groups:read",
			requiredPerm:    permissions.GroupsRead,
			userPermissions: []string{"admin:*"},
			expectedStatus:  http.StatusOK,
			description:     "Admin wildcard grants all permissions",
		},
		{
			name:            "groups:* grants access to groups:create",
			requiredPerm:    permissions.GroupsCreate,
			userPermissions: []string{"groups:*"},
			expectedStatus:  http.StatusOK,
			description:     "Resource wildcard grants all actions on resource",
		},
		{
			name:            "groups:* grants access to groups:delete",
			requiredPerm:    permissions.GroupsDelete,
			userPermissions: []string{"groups:*"},
			expectedStatus:  http.StatusOK,
			description:     "Resource wildcard includes delete permission",
		},

		// Empty permissions
		{
			name:            "user with no permissions cannot access groups",
			requiredPerm:    permissions.GroupsRead,
			userPermissions: []string{},
			expectedStatus:  http.StatusForbidden,
			description:     "Empty permissions array denies all access",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler that returns 200 OK if middleware passes
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("Success"))
			})

			// Create middleware chain
			middleware := authorize.RequiresPermission(tt.requiredPerm)
			protectedHandler := middleware(handler)

			// Create test request
			req := httptest.NewRequest("GET", "/test", nil)
			rr := httptest.NewRecorder()

			// Set permissions using the JWT context key
			ctx := context.WithValue(req.Context(), jwt.CtxPermissions, tt.userPermissions)
			req = req.WithContext(ctx)

			// Execute request
			protectedHandler.ServeHTTP(rr, req)

			// Assert results
			assert.Equal(t, tt.expectedStatus, rr.Code, tt.description)
		})
	}
}

// TestActiveGroupsPermissionConstants verifies that the permission constants
// used by the active groups API are correctly defined and distinct.
func TestActiveGroupsPermissionConstants(t *testing.T) {
	// Verify permission constants are non-empty and follow expected format
	groupPermissions := []string{
		permissions.GroupsRead,
		permissions.GroupsCreate,
		permissions.GroupsUpdate,
		permissions.GroupsDelete,
		permissions.GroupsAssign,
	}

	for _, perm := range groupPermissions {
		t.Run(perm, func(t *testing.T) {
			assert.NotEmpty(t, perm, "Permission constant should not be empty")
			assert.Contains(t, perm, ":", "Permission should follow resource:action format")
			assert.Contains(t, perm, "groups:", "Group permissions should have 'groups:' prefix")
		})
	}

	// Verify all permissions are distinct
	seen := make(map[string]bool)
	for _, perm := range groupPermissions {
		assert.False(t, seen[perm], "Permission %s should be unique", perm)
		seen[perm] = true
	}
}
