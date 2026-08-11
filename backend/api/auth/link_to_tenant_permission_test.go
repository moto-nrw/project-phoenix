package auth_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	jwtPkg "github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// POST /auth/link-to-tenant assigns a role to an existing account inside the
// caller's tenant. Guarded by users:create it was reachable by every account
// holding the base "user" role (which migration 1.5.3 grants users:create),
// letting a caregiver assign themselves the admin role. The route requires
// users:manage — the same permission /auth/register uses for exactly this
// reason (issue #1021).
func TestLinkToTenant_RequiresUsersManage(t *testing.T) {
	tc := setupTestContext(t)
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	email := fmt.Sprintf("link-perm-%d@example.com", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, tc.db, email, "SecurePass123!")
	defer testpkg.CleanupAccount(t, tc.db, account.ID)

	adminRole := testpkg.GetOrCreateTestRole(t, tc.db, "admin")

	// first_name/last_name are required for a staff-tier role (#2222); without
	// them the request is refused before the permission check is reached, which
	// would make this test pass for the wrong reason.
	body := map[string]interface{}{
		"email":      email,
		"role_id":    adminRole.ID,
		"first_name": "Link",
		"last_name":  "Permission",
	}
	claims := jwtPkg.AppClaims{
		ID:       int(account.ID),
		TenantID: 1,
		Sub:      account.Email,
		Username: "link-perm-test",
		Roles:    []string{"user"},
	}

	t.Run("users:create alone is rejected", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:create"})

		require.Equal(t, http.StatusForbidden, rr.Code, "Body: %s", rr.Body.String())

		// And nothing was written — no self-granted admin role.
		count, err := tc.db.NewSelect().
			TableExpr("auth.account_roles").
			Where("account_id = ?", account.ID).
			Count(t.Context())
		require.NoError(t, err)
		assert.Zero(t, count, "a rejected request must not assign a role")
	})

	t.Run("users:manage is accepted", func(t *testing.T) {
		req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", body)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:manage"})

		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

		_, err := tc.db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id = ?", account.ID).
			Exec(t.Context())
		require.NoError(t, err)
		_, err = tc.db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ?", account.ID).
			Exec(t.Context())
		require.NoError(t, err)
	})
}
