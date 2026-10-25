package account_test

import (
	"testing"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"

	"github.com/go-chi/chi/v5"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	authAPI "github.com/moto-nrw/project-phoenix/modules/identityaccess/inbound/account"
)

// setupGuardianInvitationRouter mounts the real auth Router() with the
// guardian invitation service wired, so these tests exercise the live
// public routes GET/POST /auth/guardian-invitations/{token}[/accept].
func setupGuardianInvitationRouter(t *testing.T) chi.Router {
	t.Helper()

	db, authRoute := setupAuthDependenciesRoute(t)
	resource := authAPI.NewResource(authRoute.AuthService, authRoute.Invitations, nil, authRoute.Sessions, identityaccessCompose.TenantUnitOfWork{}, transactionFailures.IsConstraintViolation, authRoute.SettingsService)

	router := testutil.NewTenantRouter(db)
	router.Mount("/auth", resource.Router(rootAPI.TenantSessionChain))
	return router
}

func TestValidateGuardianInvitation_NotFound(t *testing.T) {
	t.Parallel()
	router := setupGuardianInvitationRouter(t)

	req := testutil.NewJSONRequest(t, "GET", "/auth/guardian-invitations/invalid-token-12345", nil)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_NotFound(t *testing.T) {
	t.Parallel()
	router := setupGuardianInvitationRouter(t)

	body := map[string]any{
		"password":         "Test1234%",
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/guardian-invitations/invalid-token-12345/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertNotFound(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_MissingPassword(t *testing.T) {
	t.Parallel()
	router := setupGuardianInvitationRouter(t)

	body := map[string]any{
		"confirm_password": "Test1234%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/guardian-invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}

func TestAcceptGuardianInvitation_BadRequest_PasswordMismatch(t *testing.T) {
	t.Parallel()
	router := setupGuardianInvitationRouter(t)

	body := map[string]any{
		"password":         "Test1234%",
		"confirm_password": "DifferentPassword%",
	}

	req := testutil.NewJSONRequest(t, "POST", "/auth/guardian-invitations/some-token/accept", body)

	rr := testutil.ExecuteRequest(router, req)

	testutil.AssertBadRequest(t, rr)
}
