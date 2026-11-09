package account_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestChangePasswordKeepsOnlyTheCallingSession drives POST /auth/password
// through the production chain (#6289): the family of the access token the
// change is made with stays signed in, the account's other login ends.
func TestChangePasswordKeepsOnlyTheCallingSession(t *testing.T) {
	t.Parallel()
	tc, router := setupProtectedRouter(t)
	email := fmt.Sprintf("change-password-sessions-%d@example.com", time.Now().UnixNano())
	password := testpkg.NewTestPassword()
	account := testpkg.CreateTestAccountWithPassword(t, tc.db, email, password)
	testpkg.EnsureAccountTenant(t, tc.db, account.ID, testpkg.Tenant(t))

	login := func() string {
		rr := testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, "POST", "/auth/login", map[string]string{"email": email, "password": password}))
		require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
		access, ok := testutil.ParseJSONResponse(t, rr.Body.Bytes())["access_token"].(string)
		require.True(t, ok)
		return access
	}
	currentAccess := login()
	current := accountFamilies(t, tc.db, account.ID)
	require.Len(t, current, 1)
	login()
	families := accountFamilies(t, tc.db, account.ID)
	require.Len(t, families, 2)

	newPassword := testpkg.NewTestPassword()
	req := testutil.NewJSONRequest(t, "POST", "/auth/password", map[string]string{
		"current_password": password, "new_password": newPassword, "confirm_password": newPassword,
	})
	req.Header.Set("Authorization", "Bearer "+currentAccess)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusNoContent, rr.Code, "Body: %s", rr.Body.String())

	assert.Equal(t, current, accountFamilies(t, tc.db, account.ID), "only the calling session's family remains")
}

// accountFamilies lists the account's session families, oldest first.
func accountFamilies(t *testing.T, db *bun.DB, accountID int64) []string {
	t.Helper()
	var families []string
	require.NoError(t, db.NewRaw(`SELECT family_id FROM auth.tokens WHERE account_id = ? GROUP BY family_id ORDER BY MIN(id)`, accountID).
		Scan(context.Background(), &families))
	return families
}
