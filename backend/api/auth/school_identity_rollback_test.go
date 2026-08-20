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

// A refused provisioning must leave nothing behind.
//
// The account, its school mapping and its role are all written before the
// identity step runs, in the request's tenant transaction — and
// TenantTxMiddleware commits that transaction for every response below 500. A
// 400 therefore used to keep exactly what it claimed to refuse: an account that
// holds a school role, logs in, and is not staff as far as the database is
// concerned, which is the state #2222 is about. The handler marks the
// transaction for rollback so the refusal is real.
func TestRegisterRollsBackRefusedProvisioning(t *testing.T) {
	t.Parallel()
	db, router := setupPublicRouterWithDB(t)
	adminToken, validRoleID := loginAsAdmin(t, db, router)

	email := fmt.Sprintf("rollback_%d@example.com", time.Now().UnixNano())
	username := fmt.Sprintf("rollback_%d", time.Now().UnixNano())

	// A staff-tier role and no name: there is nowhere to take one from, so the
	// identity cannot be built and the whole request has to fall away.
	body := map[string]interface{}{
		"email":            email,
		"username":         username,
		"password":         "SecurePass123!",
		"confirm_password": "SecurePass123!",
		"role_id":          validRoleID,
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/register", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rr := testutil.ExecuteRequest(router, req)

	require.Equal(t, http.StatusBadRequest, rr.Code, "Body: %s", rr.Body.String())

	count, err := db.NewSelect().
		TableExpr("auth.accounts").
		Where("email = ?", email).
		Count(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count, "a refused registration must not leave the account behind")
}

// The link endpoint completes an account that already carries a person at this
// school, and reusing that person is the whole point — it is the only person
// the partial unique index on (tenant_id, account_id) allows. Requiring a name
// for it refused the request the endpoint exists for; the name is needed only
// where a person has to be created.
func TestLinkToTenantReusesExistingPersonWithoutNames(t *testing.T) {
	t.Parallel()
	tc := setupTestContext(t)
	router := testutil.NewTenantRouter(tc.db)
	router.Mount("/auth", tc.resource.Router())

	email := fmt.Sprintf("link-reuse-%d@example.com", time.Now().UnixNano())
	account := testpkg.CreateTestAccountWithPassword(t, tc.db, email, "SecurePass123!")

	// The identity the account already has here, exactly as an earlier grant or
	// a staff record would have left it.
	person := testpkg.CreateTestPerson(t, tc.db, "Bestehend", "Person")
	_, err := tc.db.NewUpdate().
		TableExpr("users.persons").
		Set("account_id = ?", account.ID).
		Where("id = ?", person.ID).
		Exec(t.Context())
	require.NoError(t, err)

	adminRole := testpkg.GetOrCreateTestRole(t, tc.db, "admin")
	claims := jwtPkg.AppClaims{
		ID:       int(account.ID),
		TenantID: testpkg.Tenant(t),
		Sub:      account.Email,
		Username: "link-reuse-test",
		Roles:    []string{"user"},
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/link-to-tenant", map[string]interface{}{
		"email":   email,
		"role_id": adminRole.ID,
	})
	rr := testutil.ExecuteWithAuthPermissions(t, router, req, claims, []string{"users:manage"})
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())

	// The person that was already there carries the new staff record — no
	// second person, and no name invented for one.
	staffCount, err := tc.db.NewSelect().
		TableExpr("users.staff").
		Where("person_id = ?", person.ID).
		Where("deleted_at IS NULL").
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, staffCount, "the existing person is completed, not duplicated")

	personCount, err := tc.db.NewSelect().
		TableExpr("users.persons").
		Where("account_id = ?", account.ID).
		Where("deleted_at IS NULL").
		Count(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, personCount)
}
