package parent_test

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/parent"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// relAcctHandlerSettings is a configurable settings stub for the related-
// accounts gate at the HTTP layer.
type relAcctHandlerSettings struct {
	configService.SettingsService
	inviteMode string
	canRemove  bool
}

func (s relAcctHandlerSettings) ResolveStringForTenant(_ context.Context, _ int64, key string) (string, error) {
	if key == configModels.KeyGuardianParentInviteMode {
		return s.inviteMode, nil
	}
	return "", nil
}

func (s relAcctHandlerSettings) ResolveBoolForTenant(_ context.Context, _ int64, key string) (bool, error) {
	if key == configModels.KeyGuardianParentCanRemove {
		return s.canRemove, nil
	}
	return false, nil
}

// relAcctStubInvites is a no-op guardian invitation service: the related-
// accounts handlers only need it to succeed once the gate passes.
type relAcctStubInvites struct {
	authService.GuardianInvitationService
}

func (relAcctStubInvites) InviteToStudent(_ context.Context, req authService.InviteToStudentRequest) (*authService.InviteToStudentResult, error) {
	return &authService.InviteToStudentResult{
		Outcome:           authService.InviteOutcomeInvited,
		GuardianProfileID: req.StudentID,
	}, nil
}

func (relAcctStubInvites) RevokeAccess(_ context.Context, _ authService.RevokeAccessRequest) error {
	return nil
}

// relAcctSocialWorkerInvites refuses every invite with the social-worker
// sentinel, mirroring the service refusal for school-managed contacts (#2172).
type relAcctSocialWorkerInvites struct {
	authService.GuardianInvitationService
}

func (relAcctSocialWorkerInvites) InviteToStudent(_ context.Context, _ authService.InviteToStudentRequest) (*authService.InviteToStudentResult, error) {
	return nil, &authService.AuthError{Op: "invite guardian to student", Err: authService.ErrInviteSocialWorkerManaged}
}

func newRelAcctRouter(t *testing.T, db *bun.DB, inviteMode string, canRemove bool) http.Handler {
	t.Helper()
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		Settings:            relAcctHandlerSettings{inviteMode: inviteMode, canRemove: canRemove},
		GuardianInvites:     relAcctStubInvites{},
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	rs := parent.NewResource(nil, svc, nil, nil, nil, db)
	return testpkg.TenantRuntimeMiddleware(t, db)(rs.Router())
}

func TestRelatedAccountsEndpoint_List(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	router := newRelAcctRouter(t, db, configModels.ParentInviteModeDirect, false)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodGet, "/me/children/"+sid+"/related-accounts", token, nil)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"guardian_profile_id"`)
	assert.Contains(t, rr.Body.String(), `"guardian_role":"primary_guardian"`,
		"the per-child role must be exposed so the panel can gate social-worker rows")
}

func TestRelatedAccountsEndpoint_InviteGate(t *testing.T) {
	t.Parallel()

	sid := func(c testpkg.ParentChain) string { return strconv.FormatInt(c.StudentID, 10) }

	t.Run("disabled → 403", func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		router := newRelAcctRouter(t, db, configModels.ParentInviteModeDisabled, false)
		token := parentToken(t, chain.AccountID)

		rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid(chain)+"/related-accounts", token,
			map[string]any{"email": "x@example.test"})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("direct → 201", func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		router := newRelAcctRouter(t, db, configModels.ParentInviteModeDirect, false)
		token := parentToken(t, chain.AccountID)

		rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid(chain)+"/related-accounts", token,
			map[string]any{"email": "new@example.test"})
		assert.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	})
}

func TestRelatedAccountsEndpoint_RemoveGate(t *testing.T) {
	t.Parallel()

	t.Run("removal disabled → 403", func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		router := newRelAcctRouter(t, db, configModels.ParentInviteModeDirect, false)
		token := parentToken(t, chain.AccountID)
		sid := strconv.FormatInt(chain.StudentID, 10)
		gid := strconv.FormatInt(chain.GuardianProfileID, 10)

		rr := doRequest(t, router, http.MethodDelete, "/me/children/"+sid+"/related-accounts/"+gid, token, nil)
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("removal enabled → 200", func(t *testing.T) {
		db := testpkg.SetupTestDB(t)
		chain := testpkg.CreateTestParentGuardianChain(t, db)
		router := newRelAcctRouter(t, db, configModels.ParentInviteModeDirect, true)
		token := parentToken(t, chain.AccountID)
		sid := strconv.FormatInt(chain.StudentID, 10)
		gid := strconv.FormatInt(chain.GuardianProfileID, 10)

		rr := doRequest(t, router, http.MethodDelete, "/me/children/"+sid+"/related-accounts/"+gid, token, nil)
		assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

// relAcctCaptureInvites records the invite request and returns a canned result,
// so the handler's confirm_role_upgrade/existing_role passthrough is testable.
type relAcctCaptureInvites struct {
	authService.GuardianInvitationService
	lastReq *authService.InviteToStudentRequest
	result  *authService.InviteToStudentResult
}

func (s *relAcctCaptureInvites) InviteToStudent(_ context.Context, req authService.InviteToStudentRequest) (*authService.InviteToStudentResult, error) {
	s.lastReq = &req
	return s.result, nil
}

func newRelAcctRouterWithInvites(t *testing.T, db *bun.DB, invites authService.GuardianInvitationService) http.Handler {
	t.Helper()
	repos := repositories.NewFactory(db)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StatusDayRepo:       repos.StudentStatusDay,
		StudentRepo:         repos.Student,
		Settings:            relAcctHandlerSettings{inviteMode: configModels.ParentInviteModeDirect},
		GuardianInvites:     invites,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		DB:                  db,
		Logger:              slog.Default(),
	})
	rs := parent.NewResource(nil, svc, nil, nil, nil, db)
	return testpkg.TenantRuntimeMiddleware(t, db)(rs.Router())
}

func TestRelatedAccountsEndpoint_ConfirmRoleUpgradePassthrough(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	invites := &relAcctCaptureInvites{result: &authService.InviteToStudentResult{
		Outcome:           authService.InviteOutcomeExistingContactRestricted,
		GuardianProfileID: chain.GuardianProfileID,
		ExistingRole:      "emergency_contact",
	}}
	router := newRelAcctRouterWithInvites(t, db, invites)
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/related-accounts", token,
		map[string]any{"email": "contact@example.test", "confirm_role_upgrade": true})
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())

	require.NotNil(t, invites.lastReq)
	assert.True(t, invites.lastReq.ConfirmRoleUpgrade, "confirm_role_upgrade must reach the invite service")
	assert.Contains(t, rr.Body.String(), `"outcome":"existing_contact_restricted"`)
	assert.Contains(t, rr.Body.String(), `"existing_role":"emergency_contact"`)
}

func TestRelatedAccountsEndpoint_SocialWorkerRefusedWithCode(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	router := newRelAcctRouterWithInvites(t, db, relAcctSocialWorkerInvites{})
	token := parentToken(t, chain.AccountID)
	sid := strconv.FormatInt(chain.StudentID, 10)

	rr := doRequest(t, router, http.MethodPost, "/me/children/"+sid+"/related-accounts", token,
		map[string]any{"email": "sozialdienst@example.test", "confirm_role_upgrade": true})
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"code":"guardian_social_worker_managed"`)
}
