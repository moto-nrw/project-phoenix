package auth_test

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/email"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// strongTestPassword is a password meeting the project's strength rules.
// Test-only constant, reused across guardian invitation tests.
const strongTestPassword = "GuardianTest!2026"

// guardianTestEnv bundles the real-DB dependencies tests need.
type guardianTestEnv struct {
	db      *bun.DB
	repos   *repositories.Factory
	service authService.GuardianInvitationService
	mailer  *email.MockMailer
	cleanup func()
}

// stubOutboxEnqueuer satisfies authService.OutboxEnqueuer so tests can
// capture what the service enqueues without wiring the full outbox stack.
type stubOutboxEnqueuer struct {
	requests []platformModels.OutboxEnqueueRequest
}

func (s *stubOutboxEnqueuer) EnqueueOutbox(_ context.Context, req platformModels.OutboxEnqueueRequest) error {
	s.requests = append(s.requests, req)
	return nil
}

func setupGuardianInvitationTest(t *testing.T, mutate ...func(*authService.GuardianInvitationServiceConfig)) *guardianTestEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	mailer := email.NewMockMailer()

	cfg := authService.GuardianInvitationServiceConfig{
		InvitationRepo:      repoFactory.GuardianInvitation,
		AccountRepo:         repoFactory.Account,
		AccountTenantRepo:   repoFactory.AccountTenant,
		AccountRoleRepo:     repoFactory.AccountRole,
		RoleRepo:            repoFactory.Role,
		PersonRepo:          repoFactory.Person,
		GuardianProfileRepo: repoFactory.GuardianProfile,
		StudentGuardianRepo: repoFactory.StudentGuardian,
		StudentRepo:         repoFactory.Student,
		SchoolRepo:          repoFactory.School,
		OutboxEnqueuer:      &stubOutboxEnqueuer{},
		FrontendURL:         "http://localhost:3000",
		FallbackExpiry:      48 * time.Hour,
		DB:                  db,
		Logger:              slog.Default(),
	}
	for _, m := range mutate {
		m(&cfg)
	}
	service := authService.NewGuardianInvitationService(cfg)

	cleanup := func() {
	}

	return &guardianTestEnv{
		db:      db,
		repos:   repoFactory,
		service: service,
		mailer:  mailer,
		cleanup: cleanup,
	}
}

// inviterAccountID creates an account to act as the invitation creator.
// Tests that need a real created_by FK target use this helper.
func (env *guardianTestEnv) inviterAccountID(t *testing.T) int64 {
	t.Helper()
	person, account := testpkg.CreateTestPersonWithAccount(t, env.db, "Inviter", "Tester")
	t.Cleanup(func() {
		// Best-effort cleanup, schema FKs cascade most rows.
		_, _ = env.db.NewDelete().
			TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("users.persons").
			Where("id = ?", person.ID).
			Exec(context.Background())
	})
	return account.ID
}

// cleanupInvitation removes a guardian invitation row plus its profile.
func (env *guardianTestEnv) cleanupInvitation(t *testing.T, invitationID, profileID int64) {
	t.Helper()
	_, _ = env.db.NewDelete().
		TableExpr("auth.guardian_invitations").
		Where("id = ?", invitationID).
		Exec(context.Background())
	_, _ = env.db.NewDelete().
		TableExpr("users.guardian_profiles").
		Where("id = ?", profileID).
		Exec(context.Background())
}

func TestGuardianInvitationService_Create_TokenAndExpiry(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "create-test")
	creatorID := env.inviterAccountID(t)
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("users.guardian_profiles").
			Where("id = ?", profile.ID).
			Exec(context.Background())
	}()

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	assert.NotEmpty(t, invitation.Token, "token must be set")
	assert.Equal(t, profile.ID, invitation.GuardianProfileID)
	assert.Equal(t, creatorID, invitation.CreatedBy)
	assert.True(t, invitation.ExpiresAt.After(time.Now().Add(46*time.Hour)),
		"expiry should be at least 46 hours in the future (default 48h)")
	assert.True(t, invitation.ExpiresAt.Before(time.Now().Add(50*time.Hour)),
		"expiry should be at most 50 hours in the future")
	assert.Nil(t, invitation.AcceptedAt, "fresh invitation must not be accepted")
}

func TestGuardianInvitationService_Create_RejectsProfileWithoutEmail(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	// Build a profile with no email, must use a raw insert because the
	// fixture helper always sets one.
	profile := &users.GuardianProfile{
		FirstName:              "NoEmail",
		LastName:               "Guardian",
		PreferredContactMethod: "phone",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(testpkg.Tenant(t))
	_, err := env.db.NewInsert().
		Model(profile).
		ModelTableExpr(`users.guardian_profiles`).
		Returning("id").
		Exec(context.Background())
	require.NoError(t, err)
	defer func() {
		_, _ = env.db.NewDelete().
			TableExpr("users.guardian_profiles").
			Where("id = ?", profile.ID).
			Exec(context.Background())
	}()

	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	_, err = env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.Error(t, err, "Create must reject a profile with no email")
}

func TestGuardianInvitationService_Validate_ReturnsPublicInfo(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "validate-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	result, err := env.service.Validate(context.Background(), invitation.Token)
	require.NoError(t, err)
	assert.Equal(t, *profile.Email, result.Email)
	assert.Equal(t, profile.FirstName, result.FirstName)
	assert.Equal(t, profile.LastName, result.LastName)
}

func TestGuardianInvitationService_Validate_UnknownTokenReturns404Error(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	_, err := env.service.Validate(context.Background(), "nonexistent-token")
	require.Error(t, err)
	// Service wraps inner error in AuthError; the inner err is ErrInvitationNotFound.
}

func TestGuardianInvitationService_Validate_ExpiredTokenReturnsExpired(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "expired-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	// Expire the invitation by direct UPDATE.
	_, err = env.db.NewUpdate().
		TableExpr("auth.guardian_invitations").
		Set("expires_at = ?", time.Now().Add(-1*time.Hour)).
		Where("id = ?", invitation.ID).
		Exec(context.Background())
	require.NoError(t, err)

	_, err = env.service.Validate(context.Background(), invitation.Token)
	require.Error(t, err)
}

func TestGuardianInvitationService_Accept_HappyPath(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "accept-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	account, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	assert.Equal(t, *profile.Email, account.Email)
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
	})

	// Invitation must be marked accepted.
	updated, err := env.repos.GuardianInvitation.FindByID(context.Background(), invitation.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.AcceptedAt, "invitation should be marked accepted")

	// Profile must point at the new account.
	updatedProfile, err := env.repos.GuardianProfile.FindByID(context.Background(), profile.ID)
	require.NoError(t, err)
	require.NotNil(t, updatedProfile.AccountID, "guardian_profile.account_id should be linked")
	assert.Equal(t, account.ID, *updatedProfile.AccountID)
	assert.True(t, updatedProfile.HasAccount, "guardian_profile.has_account should flip true")

	// Account-tenant mapping must exist + be active.
	exists, err := env.repos.AccountTenant.ExistsByAccountAndTenant(context.Background(), account.ID, testpkg.Tenant(t))
	require.NoError(t, err)
	assert.True(t, exists, "account_tenant mapping should be created on accept")

	// Account must have the guardian role assigned.
	roles, err := env.repos.Role.FindByAccountID(testpkg.Ctx(t), account.ID)
	require.NoError(t, err)
	require.NotEmpty(t, roles, "account should have at least one role")
	hasGuardian := false
	for _, r := range roles {
		if r.Name == "guardian" {
			hasGuardian = true
		}
	}
	assert.True(t, hasGuardian, "guardian role should be assigned")
}

func TestGuardianInvitationService_PublicTokenRejectsUnapprovedStatuses(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	ctx := testpkg.Ctx(t)
	creatorID := env.inviterAccountID(t)
	statuses := []string{
		authModels.GuardianInvitationApprovalPending,
		authModels.GuardianInvitationApprovalRejected,
	}
	for _, status := range statuses {
		t.Run(status, func(t *testing.T) {
			profile := testpkg.CreateTestGuardianProfile(t, env.db, "unapproved-token-"+status)
			invitation := &authModels.GuardianInvitation{
				Token:             "unapproved-token-" + status + "-" + time.Now().Format("150405.000000000"),
				GuardianProfileID: profile.ID,
				CreatedBy:         creatorID,
				ExpiresAt:         time.Now().Add(time.Hour),
				ApprovalStatus:    status,
			}
			invitation.SetTenantID(testpkg.Tenant(t))
			require.NoError(t, env.repos.GuardianInvitation.Create(ctx, invitation))
			defer env.cleanupInvitation(t, invitation.ID, profile.ID)

			_, err := env.service.Validate(context.Background(), invitation.Token)
			require.Error(t, err)
			assert.True(t, errors.Is(err, authService.ErrInvitationNotFound))

			_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
				Password:        strongTestPassword,
				ConfirmPassword: strongTestPassword,
			})
			require.Error(t, err)
			assert.True(t, errors.Is(err, authService.ErrInvitationNotFound))
		})
	}
}

func TestGuardianInvitationService_Accept_ReusesExistingAccountWithoutPasswordChange(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "existing-guardian")
	creatorID := env.inviterAccountID(t)
	account := testpkg.CreateTestAccountWithPassword(t, env.db, *profile.Email, "Existing!2026")
	existingHash := *account.PasswordHash
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	acceptedAccount, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, acceptedAccount)
	assert.Equal(t, account.ID, acceptedAccount.ID)

	stored, err := env.repos.Account.FindByEmail(context.Background(), *profile.Email)
	require.NoError(t, err)
	require.NotNil(t, stored.PasswordHash)
	assert.Equal(t, existingHash, *stored.PasswordHash)
}

func TestGuardianInvitationService_Accept_ReactivatesExistingTenantMapping(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "inactive-guardian")
	creatorID := env.inviterAccountID(t)
	account := testpkg.CreateTestAccountWithPassword(t, env.db, *profile.Email, "Existing!2026")
	deactivatedAt := time.Now().Add(-time.Hour)
	require.NoError(t, env.repos.AccountTenant.Create(context.Background(), &authModels.AccountTenant{
		AccountID:     account.ID,
		TenantID:      testpkg.Tenant(t),
		Status:        authModels.AccountTenantStatusInactive,
		DeactivatedAt: &deactivatedAt,
	}))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", account.ID).Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("auth.accounts").Where("id = ?", account.ID).Exec(context.Background())
	})

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err)

	var mapping authModels.AccountTenant
	err = env.db.NewSelect().
		Model(&mapping).
		ModelTableExpr(`auth.account_tenants AS "account_tenant"`).
		Where(`"account_tenant".account_id = ?`, account.ID).
		Where(`"account_tenant".tenant_id = ?`, testpkg.Tenant(t)).
		Scan(context.Background())
	require.NoError(t, err)
	assert.Equal(t, authModels.AccountTenantStatusActive, mapping.Status)
	assert.Nil(t, mapping.DeactivatedAt)
	assert.NotNil(t, mapping.ActivatedAt)
}

func TestGuardianInvitationService_Accept_PasswordMismatch(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "mismatch-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: "DifferentP@ss!1",
	})
	require.Error(t, err)
}

func TestGuardianInvitationService_Accept_WeakPassword(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "weak-pw-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        "short",
		ConfirmPassword: "short",
	})
	require.Error(t, err, "weak password must be rejected")
}

func TestGuardianInvitationService_Accept_AlreadyAccepted(t *testing.T) {
	t.Parallel()

	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "double-accept-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	account, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("auth.account_roles").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("auth.account_tenants").
			Where("account_id = ?", account.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().
			TableExpr("auth.accounts").
			Where("id = ?", account.ID).
			Exec(context.Background())
	})

	// Second accept must fail.
	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.Error(t, err, "second accept must reject already-accepted token")
}

func TestGuardianInvitationService_Resend_ResetsEmailColumns(t *testing.T) {
	t.Parallel()

	// Wire the outbox path (what production uses since PR 5). The legacy
	// dispatcher path would asynchronously re-populate email_sent_at after
	// delivery, racing the nil assertions below.
	outbox := &stubOutboxEnqueuer{}
	env := setupGuardianInvitationTest(t, func(cfg *authService.GuardianInvitationServiceConfig) {
		cfg.OutboxEnqueuer = outbox
	})
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "resend-test")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	// Stamp a fake error so we can verify Resend clears it.
	errMsg := "previous send failed"
	now := time.Now()
	require.NoError(t, env.repos.GuardianInvitation.UpdateEmailStatus(context.Background(), invitation.ID, &now, &errMsg, 2))

	require.NoError(t, env.service.Resend(ctx, invitation.ID, creatorID))

	updated, err := env.repos.GuardianInvitation.FindByID(context.Background(), invitation.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.EmailSentAt, "email_sent_at must be cleared on resend")
	assert.Nil(t, updated.EmailError, "email_error must be cleared on resend")
}

// --- Enrollment backfill on accept ---

// stubEnrollmentBackfiller records inputs to BackfillGuardianAccountID
// and lets the test choose what to return. The accept flow swallows
// backfill errors and only logs them, so we use this stub to confirm
// ordinary backfill errors and only logs them; savepoint-control errors
// remain fatal. This stub verifies calls without a real request row.
type stubEnrollmentBackfiller struct {
	calls       int
	gotAccount  int64
	gotEmail    string
	returnRows  int
	returnError error
}

type constraintFailingEnrollmentBackfiller struct {
	requestID int64
}

func (s *constraintFailingEnrollmentBackfiller) BackfillGuardianAccountID(ctx context.Context, _ int64, _ string) (int, error) {
	tx, ok := modelBase.TxFromContext(ctx)
	if !ok || tx == nil {
		return 0, errors.New("backfill transaction missing")
	}
	_, err := (*tx).NewRaw(`
		UPDATE enrollment.requests
		SET guardian_account_id = -1
		WHERE id = ?
	`, s.requestID).Exec(ctx)
	return 0, err
}

func (s *stubEnrollmentBackfiller) BackfillGuardianAccountID(_ context.Context, accountID int64, email string) (int, error) {
	s.calls++
	s.gotAccount = accountID
	s.gotEmail = email
	return s.returnRows, s.returnError
}

// setupGuardianInviteWithBackfiller wires the service exactly as
// setupGuardianInvitationTest but plugs in a custom EnrollmentBackfiller
// so the accept flow's backfill call can be observed.
func setupGuardianInviteWithBackfiller(t *testing.T, backfiller authService.EnrollmentBackfiller) *guardianTestEnv {
	t.Helper()
	db := testpkg.SetupTestDB(t)

	repoFactory := repositories.NewFactory(db)
	mailer := email.NewMockMailer()

	service := authService.NewGuardianInvitationService(authService.GuardianInvitationServiceConfig{
		InvitationRepo:       repoFactory.GuardianInvitation,
		AccountRepo:          repoFactory.Account,
		AccountTenantRepo:    repoFactory.AccountTenant,
		AccountRoleRepo:      repoFactory.AccountRole,
		RoleRepo:             repoFactory.Role,
		PersonRepo:           repoFactory.Person,
		GuardianProfileRepo:  repoFactory.GuardianProfile,
		SchoolRepo:           repoFactory.School,
		EnrollmentBackfiller: backfiller,
		OutboxEnqueuer:       &stubOutboxEnqueuer{},
		FrontendURL:          "http://localhost:3000",
		FallbackExpiry:       48 * time.Hour,
		DB:                   db,
		Logger:               slog.Default(),
	})

	cleanup := func() {}

	return &guardianTestEnv{
		db:      db,
		repos:   repoFactory,
		service: service,
		mailer:  mailer,
		cleanup: cleanup,
	}
}

// cleanupAcceptedAccount wipes the account + its derived rows created
// by a successful Accept. Pulled out so the backfill tests below don't
// each redeclare the same six-line block.
func cleanupAcceptedAccount(t *testing.T, db *bun.DB, accountID int64) {
	t.Helper()
	bg := context.Background()
	_, _ = db.NewDelete().TableExpr("auth.account_roles").Where("account_id = ?", accountID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("auth.account_tenants").Where("account_id = ?", accountID).Exec(bg)
	_, _ = db.NewDelete().TableExpr("auth.accounts").Where("id = ?", accountID).Exec(bg)
}

func createEnrollmentAwaitingGuardian(t *testing.T, env *guardianTestEnv, email string) *enrollmentModels.Request {
	t.Helper()
	phase := testpkg.CreateTestEnrollmentPhase(t, env.db)
	request := &enrollmentModels.Request{
		PhaseID:           phase.ID,
		GuardianFirstName: "Guardian",
		GuardianLastName:  "Test",
		GuardianEmail:     email,
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		SubmissionSource:  enrollmentModels.RequestSourcePublic,
		SourceMetadata:    map[string]any{},
		StatusToken:       "guardian-backfill-" + t.Name(),
		SubmittedAt:       time.Now(),
	}
	request.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.Request.Create(testpkg.Ctx(t), request))
	return request
}

func requireGuardianEnrollmentClaimed(t *testing.T, env *guardianTestEnv, requestID, accountID int64) {
	t.Helper()
	linkedRequest, err := env.repos.Request.FindByID(testpkg.Ctx(t), requestID)
	require.NoError(t, err)
	require.NotNil(t, linkedRequest.GuardianAccountID)
	assert.Equal(t, accountID, *linkedRequest.GuardianAccountID)

	var enrollments []*parentModels.EnrollmentRequestSummary
	require.NoError(t, tenant.WithAdminTx(context.Background(), env.db, func(ctx context.Context, _ bun.Tx) error {
		var listErr error
		enrollments, listErr = env.repos.ParentEnrollmentRequest.ListByAccount(ctx, accountID)
		return listErr
	}))
	require.Len(t, enrollments, 1)
	assert.Equal(t, requestID, enrollments[0].RequestID)
}

func TestGuardianInvitationService_Accept_InvokesBackfiller(t *testing.T) {
	t.Parallel()

	bf := &stubEnrollmentBackfiller{returnRows: 0}
	env := setupGuardianInviteWithBackfiller(t, bf)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "backfill-call")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	account, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err)
	t.Cleanup(func() { cleanupAcceptedAccount(t, env.db, account.ID) })

	require.Equal(t, 1, bf.calls, "backfiller must be invoked exactly once on successful accept")
	assert.Equal(t, account.ID, bf.gotAccount, "backfiller must receive the new account's id")
	assert.Equal(t, *profile.Email, bf.gotEmail, "backfiller must receive the invitation email")
}

func TestGuardianInvitationService_Accept_ClaimsEnrollmentInsideOuterAdminTransaction(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	realBackfiller := repositories.NewFactory(db).ParentEnrollmentRequest
	env := setupGuardianInviteWithBackfiller(t, realBackfiller)

	profile := testpkg.CreateTestGuardianProfile(t, db, "backfill-outer-admin")
	request := createEnrollmentAwaitingGuardian(t, env, "  "+strings.ToUpper(*profile.Email)+"  ")

	creatorID := env.inviterAccountID(t)
	invitation, err := env.service.Create(testpkg.Ctx(t), authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)

	var account *authModels.Account
	err = tenant.WithAdminTx(context.Background(), db, func(adminCtx context.Context, _ bun.Tx) error {
		var acceptErr error
		account, acceptErr = env.service.Accept(adminCtx, invitation.Token, authService.GuardianInvitationAcceptData{
			Password:        strongTestPassword,
			ConfirmPassword: strongTestPassword,
		})
		return acceptErr
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	requireGuardianEnrollmentClaimed(t, env, request.ID, account.ID)
}

func TestGuardianInvitationService_Accept_NotInvokedOnPasswordMismatch(t *testing.T) {
	t.Parallel()

	bf := &stubEnrollmentBackfiller{}
	env := setupGuardianInviteWithBackfiller(t, bf)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "backfill-mismatch")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: "DifferentP@ss!1",
	})
	require.Error(t, err)
	assert.Equal(t, 0, bf.calls, "backfiller must not run when accept fails")
}

func TestGuardianInvitationService_Accept_BackfillErrorDoesNotBreakAccept(t *testing.T) {
	t.Parallel()

	bf := &constraintFailingEnrollmentBackfiller{}
	env := setupGuardianInviteWithBackfiller(t, bf)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "backfill-error")
	request := createEnrollmentAwaitingGuardian(t, env, *profile.Email)
	bf.requestID = request.ID
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	account, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err, "accept must not fail when backfill errors out, backfill is best-effort")
	require.NotNil(t, account)
	t.Cleanup(func() { cleanupAcceptedAccount(t, env.db, account.ID) })

	updated, err := env.repos.GuardianInvitation.FindByID(context.Background(), invitation.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.AcceptedAt, "invitation must remain accepted after backfill error")
	unlinked, err := env.repos.Request.FindByID(testpkg.Ctx(t), request.ID)
	require.NoError(t, err)
	assert.Nil(t, unlinked.GuardianAccountID, "failed backfill write must be rolled back")
}

func TestGuardianInvitationService_Accept_SavepointControlFailureRollsBackAccept(t *testing.T) {
	t.Parallel()

	bf := &stubEnrollmentBackfiller{returnError: tenant.ErrSavepointControl}
	env := setupGuardianInviteWithBackfiller(t, bf)
	profile := testpkg.CreateTestGuardianProfile(t, env.db, "backfill-savepoint-control")
	creatorID := env.inviterAccountID(t)

	invitation, err := env.service.Create(testpkg.Ctx(t), authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)

	_, err = env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.ErrorIs(t, err, tenant.ErrSavepointControl)

	updated, err := env.repos.GuardianInvitation.FindByID(context.Background(), invitation.ID)
	require.NoError(t, err)
	assert.Nil(t, updated.AcceptedAt, "untrusted transaction state must roll back invitation acceptance")
}

func TestGuardianInvitationService_Accept_NilBackfillerIsSafe(t *testing.T) {
	t.Parallel()

	// Same setup as the production wiring used to be, no backfiller.
	// The accept flow checks `if s.enrollmentBackfiller != nil` so this
	// must not panic.
	env := setupGuardianInvitationTest(t)
	defer env.cleanup()

	profile := testpkg.CreateTestGuardianProfile(t, env.db, "backfill-nil")
	creatorID := env.inviterAccountID(t)

	ctx := testpkg.Ctx(t)
	invitation, err := env.service.Create(ctx, authService.GuardianInvitationCreateRequest{
		GuardianProfileID: profile.ID,
		CreatedBy:         creatorID,
	})
	require.NoError(t, err)
	defer env.cleanupInvitation(t, invitation.ID, profile.ID)

	account, err := env.service.Accept(context.Background(), invitation.Token, authService.GuardianInvitationAcceptData{
		Password:        strongTestPassword,
		ConfirmPassword: strongTestPassword,
	})
	require.NoError(t, err, "accept must succeed when backfiller is nil")
	require.NotNil(t, account)
	t.Cleanup(func() { cleanupAcceptedAccount(t, env.db, account.ID) })
}
