package auth_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	authAPI "github.com/moto-nrw/project-phoenix/api/auth"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestInvitationHTTPRequiresVerifiedOwner(t *testing.T) {
	t.Parallel()
	db, module := testutil.SetupInvitationModule(t)
	repos, service := module.Persistence, module.Invitation
	// Unrelated route registrations capture method values but never call this auth service.
	resource := authAPI.NewResource(&authService.Service{}, service, nil, db)
	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router())
	owner := testpkg.CreateTestAccountWithPassword(t, db, fmt.Sprintf("http-owner-%d@example.com", testpkg.Tenant(t)), "OwnerPass123!")
	other := testpkg.CreateTestAccount(t, db, "wrong-http-owner")
	schoolA := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, schoolA)
	role := testpkg.CreateTestRoleForTenant(t, db, "invited-http-staff", schoolA)
	invitation := &authModels.InvitationToken{
		Email: owner.Email, RoleID: role.ID, Token: fmt.Sprintf("http-owner-%d", schoolA),
		FirstName: testpkg.StrPtr("Invited"), LastName: testpkg.StrPtr("Owner"), ExpiresAt: time.Now().Add(time.Hour),
	}
	invitation.SetTenantID(schoolA)
	require.NoError(t, repos.InvitationToken.Create(testpkg.TenantContext(schoolA), invitation))
	path := "/auth/invitations/" + invitation.Token + "/accept"
	// Neither a client account ID nor a client email is proof of ownership.
	body := map[string]any{"account_id": owner.ID, "email": owner.Email, "owner_access_token": "unverified"}
	req := testutil.NewJSONRequest(t, http.MethodPost, path, body)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), "INVITATION_ACCOUNT_LOGIN_REQUIRED")
	for _, scope := range []string{"", "org", "parent", "school", "platform"} {
		claims := jwt.AppClaims{ID: int(other.ID), Sub: owner.Email, Roles: []string{}, Scope: scope, TenantID: testpkg.Tenant(t)}
		req = testutil.NewJSONRequest(t, http.MethodPost, path, body)
		req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
		rr = testutil.ExecuteRequest(router, req)
		if scope == "platform" {
			require.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
		} else {
			require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
			require.Contains(t, rr.Body.String(), "INVITATION_ACCOUNT_MISMATCH")
		}
	}
	stored, err := repos.Account.FindByID(context.Background(), owner.ID)
	require.NoError(t, err)
	require.Equal(t, owner.PasswordHash, stored.PasswordHash)
	joined, err := repos.AccountTenant.ExistsByAccountAndTenant(context.Background(), owner.ID, schoolA)
	require.NoError(t, err)
	require.False(t, joined)
	req = testutil.NewJSONRequest(t, http.MethodPost, path, map[string]string{})
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, jwt.AppClaims{ID: int(owner.ID), Sub: owner.Email, Roles: []string{}, Scope: "school", TenantID: testpkg.Tenant(t)}))
	rr = testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	rr = testutil.ExecuteRequest(router, testutil.NewJSONRequest(t, http.MethodPost, path, body))
	require.Equal(t, http.StatusGone, rr.Code, rr.Body.String())
}
