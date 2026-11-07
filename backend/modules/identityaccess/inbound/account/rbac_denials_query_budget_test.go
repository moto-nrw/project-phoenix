//go:build testhooks

package account_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestRBACListRequestDenials(t *testing.T) {
	t.Parallel()
	f := newRBACBudgetFixture(t)
	role := testpkg.CreateTestRole(t, f.db, "rbac-denials-role")
	none, _ := f.accountToken(t, "rbac-no-permissions")
	reader, _ := f.accountToken(t, "rbac-role-reader", "roles:read")
	paths := []string{"/auth/roles/", "/auth/permissions/", fmt.Sprintf("/auth/roles/%d/permissions/", role.ID)}
	for _, path := range paths {
		rr, _ := f.get(t, path, none)
		rbacBudgetProblem(t, rr, 403, "general.permission", "Forbidden", "anleitung-zugriff-pruefen")
		rr, _ = f.get(t, path, "")
		rbacBudgetUnauthorized(t, rr)
		for _, scope := range []string{"parent", "school", "org", "caterer"} {
			// The same actual account and persisted rights cannot cross portals.
			claims, err := testutil.TestTokenAuth(t).ParseAccessJWT(f.token)
			require.NoError(t, err)
			claims.Scope = scope
			token := testutil.MintTestJWT(t, *claims)
			verified, err := testutil.TestTokenAuth(t).ParseAccessJWT(token)
			require.NoError(t, err, "portal refusal must not merely be an invalid signature")
			require.Equal(t, scope, verified.Scope)
			t.Logf("wrong_portal=%s", scope)
			rr, _ := f.get(t, path, token)
			rbacBudgetUnauthorized(t, rr)
			require.Empty(t, f.counter.Queries(), "wrong portal must be refused before opening the tenant transaction")
		}
	}
	// Check both executed forms, never equate the legacy {id} declaration to
	// the canonical roleId route from a source-string comparison alone.
	for _, path := range []string{paths[2], paths[2][:len(paths[2])-1]} {
		rr, _ := f.get(t, path, reader)
		rbacBudgetProblem(t, rr, 403, "general.permission", "Forbidden", "anleitung-zugriff-pruefen")
	}
}
