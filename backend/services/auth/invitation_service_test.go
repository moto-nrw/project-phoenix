package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	platformModel "github.com/moto-nrw/project-phoenix/models/platform"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// testStrongCredential is a valid credential for unit tests that meets strength requirements.
// This is NOT a real secret - it's only used with mocked services in tests.
const testStrongCredential = "Str0ngP@ssword!" //nolint:gosec // Test-only constant, not a real credential

func newInvitationTestEnv(t *testing.T) (InvitationService, *stubInvitationTokenRepository, *stubAccountRepository, *stubRoleRepository, *stubAccountRoleRepository, *stubPersonRepository, *testpkg.CapturingMailer, sqlmock.Sqlmock, func()) {
	service, invitations, accounts, roles, accountRoles, persons, _, mock, cleanup := newInvitationTestEnvWithMailer(t, nil)
	return service, invitations, accounts, roles, accountRoles, persons, nil, mock, cleanup
}

func newInvitationTestEnvWithMailer(t *testing.T, mailer email.Mailer) (InvitationService, *stubInvitationTokenRepository, *stubAccountRepository, *stubRoleRepository, *stubAccountRoleRepository, *stubPersonRepository, email.Mailer, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	// Shaped like the real seeded roles: system roles carry is_system and a NULL
	// tenant_id. A role with neither is the orphan shape migration 1.15.17
	// removed, and role-grant checks reject it.
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "admin", IsSystem: true},
		&authModel.Role{Model: baseModel.Model{ID: 2}, Name: "user", IsSystem: true},
	)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo, _ := newStubStaffRepository()
	teacherRepo := newStubTeacherRepository()
	studentRepo := newStubStudentRepository()

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitationRepo,
		AccountRepo:       accountRepo,
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo:          roleRepo,
		AccountRoleRepo:   accountRoleRepo,
		PersonRepo:        personRepo,
		StaffRepo:         staffRepo,
		TeacherRepo:       teacherRepo,
		StudentRepo:       studentRepo,
		SchoolRepo:        newStubSchoolRepository(nil),
		Mailer:            mailer,
		Dispatcher:        dispatcher,
		FrontendURL:       "http://localhost:3000",
		SchoolURL:         "http://schule.localhost:3000",
		DefaultFrom:       newDefaultFromEmail(),
		InvitationExpiry:  48 * time.Hour,
		DB:                bunDB,
	})

	cleanup := func() {
		require.Eventually(t, func() bool {
			return sqlDB.Stats().InUse == 0
		}, time.Second, time.Millisecond)
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, invitationRepo, accountRepo, roleRepo, accountRoleRepo, personRepo, mailer, mock, cleanup
}

func expectAdminTx(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()
}

func expectAdminTxRollback(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
}

func TestCreateInvitationSuccess(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, rawMailer, mock, cleanup := newInvitationTestEnvWithMailer(t, testpkg.NewCapturingMailer())
	t.Cleanup(cleanup)
	mailer, ok := rawMailer.(*testpkg.CapturingMailer)
	require.True(t, ok)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "NewUser@example.com ",
		RoleID:    2,
		CreatedBy: 42,
		FirstName: testpkg.StrPtr("Ada"),
		LastName:  testpkg.StrPtr("Lovelace"),
	}

	expectAdminTx(mock)
	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	require.Equal(t, "newuser@example.com", invitation.Email)
	require.Equal(t, int64(2), invitation.RoleID)
	require.NotNil(t, invitation.CreatedBy)
	require.Equal(t, int64(42), *invitation.CreatedBy)
	require.NotNil(t, invitation.Role)
	require.Equal(t, "user", invitation.Role.Name)

	ttl := time.Until(invitation.ExpiresAt)
	require.GreaterOrEqual(t, ttl, 47*time.Hour)
	require.LessOrEqual(t, ttl, 49*time.Hour)

	require.True(t, mailer.WaitForMessages(1, time.Second))
	require.Eventually(t, func() bool {
		updated, findErr := invitations.FindByID(context.Background(), invitation.ID)
		if findErr != nil {
			return false
		}
		return updated.EmailSentAt != nil && updated.EmailError == nil && updated.EmailRetryCount == 1 &&
			mock.ExpectationsWereMet() == nil
	}, time.Second, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	require.Equal(t, "Einladung zu moto", msg.Subject)
	require.Equal(t, "invitation.html", msg.Template)
	require.Contains(t, msg.Content.(map[string]any), "InvitationURL")

	require.Contains(t, invitations.byToken, invitation.Token)
}

// TestCreateInvitationSchoolPortalLink pins the #2207 link split: a
// school-portal role (lehrkraft) invitation links to SCHOOL_URL, every other
// role keeps linking to FRONTEND_URL.
func TestCreateInvitationSchoolPortalLink(t *testing.T) {
	t.Parallel()
	service, _, _, roles, _, _, rawMailer, mock, cleanup := newInvitationTestEnvWithMailer(t, testpkg.NewCapturingMailer())
	t.Cleanup(cleanup)
	mailer, ok := rawMailer.(*testpkg.CapturingMailer)
	require.True(t, ok)

	roles.roles[7] = &authModel.Role{Model: baseModel.Model{ID: 7}, Name: "lehrkraft", IsSystem: true}

	ctx := context.Background()

	// Sequential creates: each delivery callback persists via its own admin
	// tx, and sqlmock matches expectations strictly in order — concurrent
	// callbacks would interleave BEGIN/EXEC/COMMIT.
	expectAdminTx(mock)
	lehrkraft, err := service.CreateInvitation(ctx, InvitationRequest{
		Email:            "lehrkraft@example.com",
		RoleID:           7,
		CreatedBy:        42,
		FirstName:        testpkg.StrPtr("Karla"),
		LastName:         testpkg.StrPtr("Klassen"),
		ActorPermissions: []string{permissions.UsersManage},
	})
	require.NoError(t, err)
	require.True(t, mailer.WaitForMessages(1, time.Second))
	require.Eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
	}, time.Second, 10*time.Millisecond)

	expectAdminTx(mock)
	betreuer, err := service.CreateInvitation(ctx, InvitationRequest{
		Email:            "betreuer@example.com",
		RoleID:           2,
		CreatedBy:        42,
		FirstName:        testpkg.StrPtr("Bernd"),
		LastName:         testpkg.StrPtr("Betreuung"),
		ActorPermissions: []string{permissions.UsersManage},
	})
	require.NoError(t, err)
	require.True(t, mailer.WaitForMessages(2, time.Second))
	require.Eventually(t, func() bool {
		return mock.ExpectationsWereMet() == nil
	}, time.Second, 10*time.Millisecond)

	urlsByRecipient := map[string]string{}
	for _, msg := range mailer.Messages() {
		content, contentOK := msg.Content.(map[string]any)
		require.True(t, contentOK)
		invitationURL, urlOK := content["InvitationURL"].(string)
		require.True(t, urlOK)
		urlsByRecipient[msg.To.Address] = invitationURL
	}

	require.Equal(t,
		fmt.Sprintf("http://schule.localhost:3000/invite?token=%s", lehrkraft.Token),
		urlsByRecipient["lehrkraft@example.com"])
	require.Equal(t,
		fmt.Sprintf("http://localhost:3000/invite?token=%s", betreuer.Token),
		urlsByRecipient["betreuer@example.com"])
}

func TestInvitationEmailFailureRecordsError(t *testing.T) {
	t.Parallel()

	flaky := newFlakyMailer(3, errors.New("smtp down"))
	originalBackoff := invitationEmailBackoff
	invitationEmailBackoff = []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond}
	t.Cleanup(func() {
		invitationEmailBackoff = originalBackoff
	})
	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnvWithMailer(t, flaky)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "failure@example.com",
		RoleID:    1,
		CreatedBy: 99,
		// Role 1 is the admin role; handing it out requires users:manage.
		ActorPermissions: []string{permissions.UsersManage},
	}

	expectAdminTx(mock)
	expectAdminTx(mock)
	expectAdminTx(mock)
	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	require.Eventually(t, func() bool {
		updated, findErr := invitations.FindByID(context.Background(), invitation.ID)
		if findErr != nil {
			return false
		}
		return updated.EmailRetryCount == 3 && updated.EmailError != nil && *updated.EmailError != "" && updated.EmailSentAt == nil &&
			mock.ExpectationsWereMet() == nil
	}, time.Second, 20*time.Millisecond)

	require.Equal(t, 3, flaky.Attempts())
	require.Len(t, flaky.Messages(), 0)
}

func TestCreateInvitationRejectsLehrkraftCaregiverCombo(t *testing.T) {
	t.Parallel()

	service, _, _, roleRepo, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	// The lehrkraft system role (#1772) holds only class_day:read; the
	// caregiver upgrade would add the full user role plus a caregiver
	// profile, so the combination must be refused at creation.
	roleRepo.roles[3] = &authModel.Role{Model: baseModel.Model{ID: 3}, Name: "lehrkraft", IsSystem: true}

	_, err := service.CreateInvitation(context.Background(), InvitationRequest{
		Email:            "lehrkraft@example.com",
		RoleID:           3,
		CreatedBy:        42,
		CaregiverEnabled: true,
	})
	require.ErrorIs(t, err, ErrLehrkraftNoCaregiver)
}

func TestCreateInvitationInvalidatesExistingTokens(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	existing := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "old-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, existing))

	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    2,
		CreatedBy: 99,
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	require.NotNil(t, existing.UsedAt, "existing invitation should be invalidated")
	require.NotEqual(t, "old-token", invitation.Token)
}

func TestCreateInvitationOnlyInvalidatesExistingTokensInTargetTenant(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	otherTenantID := testpkg.UniqueTestTenantID(t)
	targetTenantID := testpkg.UniqueTestTenantID(t)
	otherTenant := &authModel.InvitationToken{
		Email:     "principal@example.com",
		Token:     "other-tenant-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	otherTenant.SetTenantID(otherTenantID)
	require.NoError(t, invitations.Create(ctx, otherTenant))

	targetTenant := &authModel.InvitationToken{
		Email:     "principal@example.com",
		Token:     "target-tenant-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	targetTenant.SetTenantID(targetTenantID)
	require.NoError(t, invitations.Create(ctx, targetTenant))

	req := InvitationRequest{
		Email:     "principal@example.com",
		RoleID:    2,
		TenantID:  targetTenantID,
		CreatedBy: 0,
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Equal(t, targetTenantID, invitation.TenantID)
	require.Nil(t, invitation.CreatedBy)

	require.Nil(t, otherTenant.UsedAt, "invite in a different tenant must remain valid")
	require.NotNil(t, targetTenant.UsedAt, "invite in the target tenant should be invalidated")
}

func TestValidateInvitationReturnsDetails(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "abc-123",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(12 * time.Hour),
		FirstName: testpkg.StrPtr("Grace"),
		LastName:  testpkg.StrPtr("Hopper"),
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	result, err := service.ValidateInvitation(ctx, "abc-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user@example.com", result.Email)
	require.Equal(t, "user", result.RoleName)
	require.Equal(t, token.FirstName, result.FirstName)
}

func TestValidateInvitationExpired(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "expired",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTxRollback(mock)
	_, err := service.ValidateInvitation(ctx, "expired")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationExpired), fmt.Sprintf("expected ErrInvitationExpired, got %v", err))
}

func TestValidateInvitationUsed(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	now := time.Now()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "used",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &now,
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTxRollback(mock)
	_, err := service.ValidateInvitation(ctx, "used")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed), "expected ErrInvitationUsed")
}

func TestAcceptInvitationCreatesAccountAndPerson(t *testing.T) {
	t.Parallel()

	service, invitations, accounts, _, accountRoles, persons, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "accept",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)

	account, err := service.AcceptInvitation(ctx, "accept", UserRegistrationData{
		FirstName:       "Katherine",
		LastName:        "Johnson",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, "user@example.com", account.Email)

	storedAccount, err := accounts.FindByEmail(ctx, "user@example.com")
	require.NoError(t, err)
	require.NotNil(t, storedAccount.PasswordHash)

	require.True(t, token.IsUsed(), "invitation should be marked used")
	require.Equal(t, 1, len(persons.people))
	require.Equal(t, 1, len(accountRoles.Assignments()))
}

func TestAcceptInvitationRollsBackOnError(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, persons, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "fail",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	persons.failCreate = true

	expectAdminTxRollback(mock)

	_, err := service.AcceptInvitation(ctx, "fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.Error(t, err)
	require.False(t, token.IsUsed(), "invitation should remain unused on failure")
	require.Equal(t, 0, len(persons.people), "person creation should not persist")

	// The account is written before the identity now (#2222): the provisioning
	// looks the account's existing person up to reuse it, which needs the
	// account id. Against a real database the surrounding transaction takes the
	// account insert back with everything else — the in-memory stubs here have
	// no transaction to roll back, so the account row they hold says nothing
	// about that. That half is covered by
	// TestAcceptInvitationRollsBackAccountMappingAndRole, which asserts against
	// a real database that the account, the school mapping and the role
	// assignment are all gone. What this test still proves is that the failure
	// aborts the acceptance: the error surfaces, the invitation stays unused,
	// and no
	// person is left behind.
}

func TestAcceptInvitationWeakPassword(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "weak",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTxRollback(mock)
	_, err := service.AcceptInvitation(ctx, "weak", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        "weak",
		ConfirmPassword: "weak",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPasswordTooWeak))
	require.False(t, token.IsUsed())
}

func TestResendInvitationSendsEmail(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, rawMailer, mock, cleanup := newInvitationTestEnvWithMailer(t, testpkg.NewCapturingMailer())
	t.Cleanup(cleanup)
	mailer, ok := rawMailer.(*testpkg.CapturingMailer)
	require.True(t, ok)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "resend",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
		Model:     baseModel.Model{UpdatedAt: time.Now().Add(-1 * time.Hour), CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	err := service.ResendInvitation(ctx, token.ID, 99)
	require.NoError(t, err)

	require.True(t, mailer.WaitForMessages(1, time.Second))
	require.Eventually(t, func() bool {
		updated, findErr := invitations.FindByID(context.Background(), token.ID)
		if findErr != nil {
			return false
		}
		// Wait for both the stub update AND the sqlmock admin tx (BEGIN → SET LOCAL ROLE → COMMIT)
		// to complete. The dispatcher callback updates the stub inside withAdminTx, so the stub
		// can reflect the new state before COMMIT is consumed by sqlmock.
		return updated.EmailSentAt != nil && updated.EmailError == nil && updated.EmailRetryCount == 1 &&
			mock.ExpectationsWereMet() == nil
	}, 2*time.Second, 10*time.Millisecond)
	// Re-read from the repo: FindByID returns a copy (like a real repo), so
	// the local token variable no longer aliases the stored row.
	refreshed, err := invitations.FindByID(ctx, token.ID)
	require.NoError(t, err)
	require.True(t, refreshed.UpdatedAt.After(time.Now().Add(-30*time.Second)), "updated_at should be refreshed")
}

func TestResendInvitationExpired(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "expired-resend",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	err := service.ResendInvitation(ctx, token.ID, 99)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationExpired))
}

func TestRevokeInvitationMarksAsUsed(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "revoke",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	err := service.RevokeInvitation(ctx, token.ID, 5)
	require.NoError(t, err)
	require.True(t, token.IsUsed(), "invitation should be marked used after revoke")
}

func TestTranslateRoleNameToGerman(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"admin", "Administrator"},
		{"Admin", "Administrator"},
		{"ADMIN", "Administrator"},
		{"user", "Betreuer"},
		{"User", "Betreuer"},
		{"guest", "Gast"},
		{"Guest", "Gast"},
		{"guardian", "Erziehungsberechtigter"},
		{"Guardian", "Erziehungsberechtigter"},
		{"lehrkraft", "Lehrkraft"},
		{"Lehrkraft", "Lehrkraft"},
		{"teacher", "teacher"},         // Not a system role, returns as-is
		{"custom_role", "custom_role"}, // Unknown role, returns as-is
		{"", ""},                       // Empty string
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := translateRoleNameToGerman(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

// Successor of TestShouldCreateTeacherForRole: the name-based helper became the
// tier-based RoleNeedsCaregiverProfile (#2222). Every assertion of the old test
// is kept; the new cases are the school's own roles, which the name check could
// not see at all.
func TestRoleNeedsCaregiverProfile(t *testing.T) {
	t.Parallel()

	system := func(name string) *authModel.Role { return &authModel.Role{Name: name, IsSystem: true} }
	custom := func(name, base string) *authModel.Role {
		return &authModel.Role{Name: name, BaseRole: &base}
	}

	require.True(t, RoleNeedsCaregiverProfile(system("teacher")))
	require.True(t, RoleNeedsCaregiverProfile(system("Teacher")))
	require.True(t, RoleNeedsCaregiverProfile(system("user")))
	require.False(t, RoleNeedsCaregiverProfile(system("admin")))
	// A Lehrkraft (#1772) gets the staff record every staff role gets, but
	// deliberately no users.teachers caregiver profile: it supervises no OGS
	// group, its scope comes from education.class_teachers.
	require.False(t, RoleNeedsCaregiverProfile(system("lehrkraft")))

	// A school's own role is decided by its tier, not by its label.
	require.True(t, RoleNeedsCaregiverProfile(custom("OGS-Kraft", authModel.BaseRoleUser)))
	require.False(t, RoleNeedsCaregiverProfile(custom("OGS-Leitung", authModel.BaseRoleAdmin)))
	// The label alone means nothing: a custom role named "teacher" with an
	// admin tier is an admin role.
	require.False(t, RoleNeedsCaregiverProfile(custom("teacher", authModel.BaseRoleAdmin)))
	require.False(t, RoleNeedsCaregiverProfile(nil))
}

// The bug of #2222: a school's own role produced a person and no staff record.
// Staff membership is decided by tier, and an unknown tier (base_role NULL on a
// role created before the column existed) counts as personnel — a staff row
// grants nothing, withholding it is what breaks the account.
func TestRoleNeedsStaffRecord(t *testing.T) {
	t.Parallel()

	custom := func(name string, base *string) *authModel.Role {
		return &authModel.Role{Name: name, BaseRole: base}
	}
	ptr := func(s string) *string { return &s }

	require.True(t, RoleNeedsStaffRecord(&authModel.Role{Name: "admin", IsSystem: true}))
	require.True(t, RoleNeedsStaffRecord(&authModel.Role{Name: "user", IsSystem: true}))
	require.True(t, RoleNeedsStaffRecord(&authModel.Role{Name: "lehrkraft", IsSystem: true}))
	require.True(t, RoleNeedsStaffRecord(custom("OGS-Leitung", ptr(authModel.BaseRoleAdmin))))
	require.True(t, RoleNeedsStaffRecord(custom("OGS-Kraft", ptr(authModel.BaseRoleUser))))
	require.True(t, RoleNeedsStaffRecord(custom("Alt-Rolle", nil)))

	require.False(t, RoleNeedsStaffRecord(&authModel.Role{Name: "guardian", IsSystem: true}))
	require.False(t, RoleNeedsStaffRecord(custom("Sorgeberechtigt", ptr(authModel.BaseRoleGuardian))))
	require.False(t, RoleNeedsStaffRecord(nil))
}

// The caregiver upgrade (#1772) is refused for the lehrkraft SYSTEM role in
// both acceptance branches; a school's custom role sharing the label is a
// different role and stays eligible.
func TestIsLehrkraftSystemRole(t *testing.T) {
	t.Parallel()

	require.True(t, IsLehrkraftSystemRole(&authModel.Role{Name: "lehrkraft", IsSystem: true}))
	require.True(t, IsLehrkraftSystemRole(&authModel.Role{Name: " Lehrkraft ", IsSystem: true}))
	require.False(t, IsLehrkraftSystemRole(&authModel.Role{Name: "lehrkraft", IsSystem: false}))
	require.False(t, IsLehrkraftSystemRole(&authModel.Role{Name: "user", IsSystem: true}))
	require.False(t, IsLehrkraftSystemRole(nil))
}

func TestAcceptInvitation_AdminCaregiverEnabledCreatesUserRoleAndTeacherProfile(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	invitations := newStubInvitationTokenRepository()
	accounts := newStubAccountRepository()
	tenantID := testpkg.UniqueTestTenantID(t)
	roles := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 21}, Name: "admin", IsSystem: true},
		&authModel.Role{Model: baseModel.Model{ID: 22}, Name: "user", IsSystem: true},
		&authModel.Role{Model: baseModel.Model{ID: 23}, TenantID: &tenantID, Name: "user", IsSystem: false},
	)
	accountRoles := newStubAccountRoleRepository()
	persons := newStubPersonRepository()
	staff, staffAll := newStubStaffRepository()
	teachers := newStubTeacherRepository()

	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitations,
		AccountRepo:       accounts,
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo:          roles,
		AccountRoleRepo:   accountRoles,
		PersonRepo:        persons,
		StaffRepo:         staff,
		TeacherRepo:       teachers,
		SchoolRepo:        newStubSchoolRepository(nil),
		FrontendURL:       "http://localhost:3000",
		DefaultFrom:       newDefaultFromEmail(),
		InvitationExpiry:  48 * time.Hour,
		DB:                bunDB,
	})

	position := "OGS-Büro"
	token := &authModel.InvitationToken{
		Email:            "admin-caregiver@example.com",
		Token:            "admin-caregiver-token",
		RoleID:           21,
		CreatedBy:        nullableCreatedBy(31),
		ExpiresAt:        time.Now().Add(10 * time.Hour),
		Position:         &position,
		CaregiverEnabled: true,
	}
	token.SetTenantID(tenantID)
	require.NoError(t, invitations.Create(context.Background(), token))

	expectAdminTx(mock)
	account, err := service.AcceptInvitation(context.Background(), token.Token, UserRegistrationData{
		FirstName:       "Ada",
		LastName:        "Lovelace",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})

	require.NoError(t, err)
	require.NotNil(t, account)

	assignments := accountRoles.Assignments()
	require.Len(t, assignments, 2)
	require.Equal(t, int64(21), assignments[0].RoleID)
	require.Equal(t, int64(22), assignments[1].RoleID)
	require.NotEqual(t, int64(23), assignments[1].RoleID, "caregiver capability must use the global system user role, not a tenant-owned role with the same name")
	require.Equal(t, tenantID, assignments[0].TenantID)
	require.Equal(t, tenantID, assignments[1].TenantID)

	createdStaff := staffAll()
	require.Len(t, createdStaff, 1)
	require.Equal(t, tenantID, createdStaff[0].TenantID)

	createdTeachers := teachers.All()
	require.Len(t, createdTeachers, 1)
	require.Equal(t, createdStaff[0].ID, createdTeachers[0].StaffID)
	require.Equal(t, tenantID, createdTeachers[0].TenantID)
	require.Equal(t, position, createdTeachers[0].Role)
}

func TestAcceptInvitationSecondAttemptFails(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "second-attempt@example.com",
		Token:     "second-attempt-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	// First acceptance
	expectAdminTx(mock)

	account, err := service.AcceptInvitation(ctx, "second-attempt-token", UserRegistrationData{
		FirstName:       "First",
		LastName:        "User",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	// Second attempt should fail
	expectAdminTxRollback(mock)
	_, err = service.AcceptInvitation(ctx, "second-attempt-token", UserRegistrationData{
		FirstName:       "Second",
		LastName:        "User",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed), "Second acceptance should fail with ErrInvitationUsed")
}

// =============================================================================
// lookupSchoolName error-path tests
// =============================================================================

func TestLookupSchoolNameZeroTenant(t *testing.T) {
	t.Parallel()

	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	svc := service.(*invitationService)
	result := svc.lookupSchoolName(context.Background(), 0)
	require.Equal(t, "", result, "tenantID=0 must return empty string")
}

func TestLookupSchoolNameNilRepo(t *testing.T) {
	t.Parallel()

	svc := &invitationService{
		logger: slog.Default(),
	}
	result := svc.lookupSchoolName(context.Background(), 42)
	require.Equal(t, "", result, "nil schoolRepo must return empty string")
}

func TestCreateInvitationAllowsExistingAccountForNewTenant(t *testing.T) {
	t.Parallel()

	service, invitations, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	accounts.storeAccount(&authModel.Account{Model: baseModel.Model{ID: 7}, Email: "principal@example.com", Active: true})

	invitation, err := service.CreateInvitation(ctx, InvitationRequest{
		Email:            "principal@example.com",
		RoleID:           1,
		TenantID:         2,
		ActorPermissions: []string{permissions.UsersManage},
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Contains(t, invitations.byToken, invitation.Token)
}

func TestAcceptInvitationDeletedSchoolRejectsAcceptance(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	schoolRepo := newStubSchoolRepository(map[int64]bool{42: true})
	staffRepo, _ := newStubStaffRepository()

	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitationRepo,
		AccountRepo:       newStubAccountRepository(),
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo: newStubRoleRepository(
			&authModel.Role{Model: baseModel.Model{ID: 2}, Name: "user", IsSystem: true},
		),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        staffRepo,
		TeacherRepo:      newStubTeacherRepository(),
		SchoolRepo:       schoolRepo,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "deleted-school@example.com",
		Token:     "deleted-school-token",
		RoleID:    2,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(42)
	require.NoError(t, invitationRepo.Create(ctx, token))

	expectAdminTxRollback(mock)
	_, err = service.AcceptInvitation(ctx, "deleted-school-token", UserRegistrationData{
		FirstName:       "Ghost",
		LastName:        "User",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationTenantDeleted),
		"expected ErrInvitationTenantDeleted, got %v", err)
	require.False(t, token.IsUsed(), "invitation must remain unused when school is deleted")
}

func TestGetTenantSubdomainForTokenUsesSubdomainNotSlug(t *testing.T) {
	t.Parallel()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	// School where slug != subdomain (issue #1977: OGS Burbach,
	// slug=ogs-burbach, subdomain=burbach). The redirect after accepting an
	// invitation must use the subdomain — tenant routing resolves by it.
	schoolRepo := newStubSchoolRepository(nil)
	schoolRepo.FindByIDFn = func(_ context.Context, id int64) (*platformModel.School, error) {
		return &platformModel.School{
			Model:     baseModel.Model{ID: id},
			Active:    true,
			Slug:      "ogs-burbach",
			Subdomain: "burbach",
		}, nil
	}

	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitationRepo,
		AccountRepo:       newStubAccountRepository(),
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo: newStubRoleRepository(
			&authModel.Role{Model: baseModel.Model{ID: 2}, Name: "user", IsSystem: true},
		),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		SchoolRepo:       schoolRepo,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "subdomain-test@example.com",
		Token:     "subdomain-test-token",
		RoleID:    2,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(42)
	require.NoError(t, invitationRepo.Create(ctx, token))

	expectAdminTx(mock)
	subdomain := service.GetTenantSubdomainForToken(ctx, "subdomain-test-token")
	require.Equal(t, "burbach", subdomain,
		"accept response must carry the subdomain, not the slug")
}

func TestAcceptInvitationReusesExistingAccountForNewTenant(t *testing.T) {
	t.Parallel()

	service, invitations, accounts, _, accountRoles, persons, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	existingHash := "old-hash"
	accounts.storeAccount(&authModel.Account{
		Model:        baseModel.Model{ID: 8},
		Email:        "principal@example.com",
		Active:       true,
		PasswordHash: &existingHash,
	})

	token := &authModel.InvitationToken{
		Email:     "principal@example.com",
		Token:     "existing-account",
		RoleID:    1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(5)
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	account, err := service.AcceptInvitation(ctx, "existing-account", UserRegistrationData{
		FirstName:       "Alex",
		LastName:        "Principal",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(8), account.ID)
	require.True(t, token.IsUsed())
	require.Equal(t, 1, len(persons.people))
	require.Equal(t, 1, len(accountRoles.Assignments()))

	updated, findErr := accounts.FindByEmail(ctx, "principal@example.com")
	require.NoError(t, findErr)
	require.NotNil(t, updated.PasswordHash)
	require.NotEqual(t, existingHash, *updated.PasswordHash)
}

// Acceptance against a previously-deactivated account must reactivate it.
func TestAcceptInvitationReactivatesInactiveAccount(t *testing.T) {
	t.Parallel()

	service, invitations, accounts, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	existingHash := "legacy-hash"
	accounts.storeAccount(&authModel.Account{
		Model:        baseModel.Model{ID: 42},
		Email:        "kontakt@example.com",
		Active:       false,
		PasswordHash: &existingHash,
	})

	token := &authModel.InvitationToken{
		Email:     "kontakt@example.com",
		Token:     "reactivate-token",
		RoleID:    1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(5)
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	account, err := service.AcceptInvitation(ctx, "reactivate-token", UserRegistrationData{
		FirstName:       "Re",
		LastName:        "Activated",
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
	require.Equal(t, int64(42), account.ID)
	require.True(t, account.Active, "account must be activated after invitation acceptance")

	stored, findErr := accounts.FindByEmail(ctx, "kontakt@example.com")
	require.NoError(t, findErr)
	require.True(t, stored.Active, "stored account must reflect activation")
	require.NotEqual(t, existingHash, *stored.PasswordHash)
}

func TestCreateInvitationRejectsInvalidRequests(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		req     InvitationRequest
		wantErr error
	}{
		{
			name:    "empty email",
			req:     InvitationRequest{Email: " ", RoleID: 1},
			wantErr: fmt.Errorf("email is required"),
		},
		{
			name:    "invalid email",
			req:     InvitationRequest{Email: "not-an-email", RoleID: 1},
			wantErr: fmt.Errorf("invalid email address"),
		},
		{
			name:    "missing role id",
			req:     InvitationRequest{Email: "person@example.com"},
			wantErr: fmt.Errorf("role id is required"),
		},
		{
			name:    "negative created by",
			req:     InvitationRequest{Email: "person@example.com", RoleID: 1, CreatedBy: -10},
			wantErr: fmt.Errorf("created_by is invalid"),
		},
		{
			name:    "negative tenant id",
			req:     InvitationRequest{Email: "person@example.com", RoleID: 1, TenantID: -10},
			wantErr: fmt.Errorf("tenant id is invalid"),
		},
		{
			name: "unknown role",
			req:  InvitationRequest{Email: "person@example.com", RoleID: 404},
			// The role-assignment policy answers "exists and may be handed out
			// here" in one step, so an unknown ID surfaces as not assignable.
			wantErr: ErrRoleNotAssignable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
			t.Cleanup(cleanup)

			_, err := service.CreateInvitation(context.Background(), tt.req)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr.Error())
		})
	}
}

func TestCreateInvitationRejectsExistingAccountWithoutTargetTenant(t *testing.T) {
	t.Parallel()

	service, _, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	accounts.storeAccount(&authModel.Account{
		Model:  baseModel.Model{ID: 101},
		Email:  "existing@example.com",
		Active: true,
	})

	_, err := service.CreateInvitation(context.Background(), InvitationRequest{
		Email:  "existing@example.com",
		RoleID: 1,
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmailAlreadyExists), "expected ErrEmailAlreadyExists, got %v", err)
}

func TestCreateInvitationRejectsExistingTenantAccess(t *testing.T) {
	t.Parallel()

	invitations := newStubInvitationTokenRepository()
	accounts := newStubAccountRepository(&authModel.Account{
		Model:  baseModel.Model{ID: 102},
		Email:  "existing@example.com",
		Active: true,
	})
	accountTenants := newStubAccountTenantRepository()
	require.NoError(t, accountTenants.Create(context.Background(), &authModel.AccountTenant{
		AccountID: 102,
		TenantID:  77,
		Status:    authModel.AccountTenantStatusActive,
	}))
	service := newTestInvitationService(t, InvitationServiceConfig{
		InvitationRepo:    invitations,
		AccountRepo:       accounts,
		AccountTenantRepo: accountTenants,
		RoleRepo:          newStubRoleRepository(&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "admin", IsSystem: true}),
		AccountRoleRepo:   newStubAccountRoleRepository(),
		PersonRepo:        newStubPersonRepository(),
		StaffRepo:         staffRepoOnly(),
		TeacherRepo:       newStubTeacherRepository(),
		SchoolRepo:        newStubSchoolRepository(nil),
		FrontendURL:       "http://localhost:3000",
		DefaultFrom:       newDefaultFromEmail(),
		InvitationExpiry:  48 * time.Hour,
	})

	_, err := service.CreateInvitation(context.Background(), InvitationRequest{
		Email:    "existing@example.com",
		RoleID:   1,
		TenantID: 77,
	})

	require.Error(t, err)
	require.True(t, errors.Is(err, ErrAccountAlreadyHasTenantAccess),
		"expected ErrAccountAlreadyHasTenantAccess, got %v", err)
}

func TestAcceptInvitationUsesInvitationNameFallback(t *testing.T) {
	t.Parallel()

	service, invitations, _, _, _, persons, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "fallback@example.com",
		Token:     "fallback-name-token",
		RoleID:    2,
		FirstName: testpkg.StrPtr("Invite"),
		LastName:  testpkg.StrPtr("Name"),
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	token.SetTenantID(5)
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	account, err := service.AcceptInvitation(ctx, token.Token, UserRegistrationData{
		Password:        testStrongCredential,
		ConfirmPassword: testStrongCredential,
	})

	require.NoError(t, err)
	require.NotNil(t, account)
	require.Len(t, persons.people, 1)
	for _, person := range persons.people {
		require.Equal(t, "Invite", person.FirstName)
		require.Equal(t, "Name", person.LastName)
	}
}

func TestAcceptInvitationRejectsPasswordMismatchAndMissingNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		data    UserRegistrationData
		wantErr error
	}{
		{
			name: "password mismatch",
			data: UserRegistrationData{
				FirstName:       "Jane",
				LastName:        "Doe",
				Password:        testStrongCredential,
				ConfirmPassword: testStrongCredential + "x",
			},
			wantErr: ErrPasswordMismatch,
		},
		{
			name: "missing names",
			data: UserRegistrationData{
				Password:        testStrongCredential,
				ConfirmPassword: testStrongCredential,
			},
			wantErr: ErrInvitationNameRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
			t.Cleanup(cleanup)

			token := &authModel.InvitationToken{
				Email:     tt.name + "@example.com",
				Token:     tt.name + "-token",
				RoleID:    2,
				ExpiresAt: time.Now().Add(10 * time.Hour),
			}
			require.NoError(t, invitations.Create(context.Background(), token))

			expectAdminTxRollback(mock)
			_, err := service.AcceptInvitation(context.Background(), token.Token, tt.data)
			require.Error(t, err)
			require.True(t, errors.Is(err, tt.wantErr), "expected %v, got %v", tt.wantErr, err)
			require.False(t, token.IsUsed())
		})
	}
}

func TestInvitationManagementErrorAndEdgePaths(t *testing.T) {
	t.Parallel()

	t.Run("list wraps repository error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		svc := service.(*invitationService)
		svc.invitationRepo = &failingInvitationTokenRepository{stubInvitationTokenRepository: invitations, listErr: errors.New("list failed")}

		_, err := svc.ListPendingInvitations(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "list failed")
	})

	t.Run("cleanup deletes expired invitations", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		require.NoError(t, invitations.Create(context.Background(), &authModel.InvitationToken{
			Email:     "expired@example.com",
			Token:     "cleanup-expired",
			RoleID:    1,
			ExpiresAt: time.Now().Add(-time.Hour),
		}))

		count, err := service.CleanupExpiredInvitations(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("cleanup wraps repository error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		svc := service.(*invitationService)
		svc.invitationRepo = &failingInvitationTokenRepository{stubInvitationTokenRepository: invitations, deleteExpiredErr: errors.New("delete failed")}

		_, err := svc.CleanupExpiredInvitations(context.Background())
		require.Error(t, err)
		require.Contains(t, err.Error(), "delete failed")
	})

	t.Run("invalidate tenant wraps repository error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		svc := service.(*invitationService)
		svc.invitationRepo = &failingInvitationTokenRepository{stubInvitationTokenRepository: invitations, invalidateTenantErr: errors.New("invalidate failed")}

		_, err := svc.InvalidatePendingInvitationsByTenantID(context.Background(), 42)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalidate failed")
	})
}

func TestResendInvitationErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		err := service.ResendInvitation(context.Background(), 404, 99)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvitationNotFound), "expected ErrInvitationNotFound, got %v", err)
	})

	t.Run("used", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		now := time.Now()
		token := &authModel.InvitationToken{
			Email:     "used-resend@example.com",
			Token:     "used-resend-token",
			RoleID:    1,
			ExpiresAt: time.Now().Add(time.Hour),
			UsedAt:    &now,
		}
		require.NoError(t, invitations.Create(context.Background(), token))

		err := service.ResendInvitation(context.Background(), token.ID, 99)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvitationUsed), "expected ErrInvitationUsed, got %v", err)
	})

	t.Run("role lookup error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		token := &authModel.InvitationToken{
			Email:     "missing-role@example.com",
			Token:     "missing-role-token",
			RoleID:    404,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, invitations.Create(context.Background(), token))

		err := service.ResendInvitation(context.Background(), token.ID, 99)
		require.Error(t, err)
		require.Contains(t, err.Error(), "role not found")
	})

	t.Run("update error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		token := &authModel.InvitationToken{
			Email:     "update-error@example.com",
			Token:     "update-error-token",
			RoleID:    1,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, invitations.Create(context.Background(), token))
		svc := service.(*invitationService)
		svc.invitationRepo = &failingInvitationTokenRepository{stubInvitationTokenRepository: invitations, updateErr: errors.New("update failed")}

		err := svc.ResendInvitation(context.Background(), token.ID, 99)
		require.Error(t, err)
		require.Contains(t, err.Error(), "update failed")
	})
}

func TestRevokeInvitationErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("not found", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		err := service.RevokeInvitation(context.Background(), 404, 99)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvitationNotFound), "expected ErrInvitationNotFound, got %v", err)
	})

	t.Run("used", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		now := time.Now()
		token := &authModel.InvitationToken{
			Email:     "used-revoke@example.com",
			Token:     "used-revoke-token",
			RoleID:    1,
			ExpiresAt: time.Now().Add(time.Hour),
			UsedAt:    &now,
		}
		require.NoError(t, invitations.Create(context.Background(), token))

		err := service.RevokeInvitation(context.Background(), token.ID, 99)
		require.Error(t, err)
		require.True(t, errors.Is(err, ErrInvitationUsed), "expected ErrInvitationUsed, got %v", err)
	})

	t.Run("mark used error", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		token := &authModel.InvitationToken{
			Email:     "mark-error@example.com",
			Token:     "mark-error-token",
			RoleID:    1,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		require.NoError(t, invitations.Create(context.Background(), token))
		svc := service.(*invitationService)
		svc.invitationRepo = &failingInvitationTokenRepository{stubInvitationTokenRepository: invitations, markErr: errors.New("mark failed")}

		err := svc.RevokeInvitation(context.Background(), token.ID, 99)
		require.Error(t, err)
		require.Contains(t, err.Error(), "mark failed")
	})
}

func TestInvitationHelpersCoverFallbacks(t *testing.T) {
	t.Parallel()

	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)
	svc := service.(*invitationService)

	require.Equal(t, "", svc.lookupSchoolName(context.Background(), 0))
	require.Equal(t, "", svc.lookupSchoolName(context.Background(), 55))
	require.Equal(t, "", sanitizeEmailError(nil))
	require.Equal(t, "smtp down", sanitizeEmailError(errors.New(" smtp down ")))

	dbErr := &baseModel.DatabaseError{Err: sql.ErrNoRows}
	require.True(t, isNotFoundError(dbErr))
	require.False(t, isNotFoundError(errors.New("other")))

	invitation := svc.buildInvitationToken("name@example.com", InvitationRequest{
		Email:     "name@example.com",
		RoleID:    1,
		FirstName: testpkg.StrPtr(" Ada "),
		LastName:  testpkg.StrPtr(" Lovelace "),
		Position:  testpkg.StrPtr(" Leitung "),
	})
	require.Equal(t, "Ada", *invitation.FirstName)
	require.Equal(t, "Lovelace", *invitation.LastName)
	require.Equal(t, "Leitung", *invitation.Position)

	tenantCtx := tenant.WithTenantID(context.Background(), 123)
	require.Same(t, tenantCtx, scopedInvitationTenantContext(tenantCtx, 123))

	svc.dispatcher = nil
	svc.sendInvitationEmail(context.Background(), &authModel.InvitationToken{
		Model: baseModel.Model{ID: 99},
		Email: "skip-email@example.com",
		Token: "skip-email-token",
	}, "admin", "", false)
}

func TestInvitationLookupErrorBranches(t *testing.T) {
	t.Parallel()

	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)
	svc := service.(*invitationService)

	_, err := svc.fetchValidInvitation(context.Background(), " ")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound), "expected ErrInvitationNotFound, got %v", err)

	_, err = svc.fetchValidInvitation(context.Background(), "missing-token")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound), "expected ErrInvitationNotFound, got %v", err)

	svc.roleRepo = &failingRoleRepository{
		stubRoleRepository: newStubRoleRepository(),
		findErr:            errors.New("role lookup failed"),
	}
	_, err = svc.lookupRoleName(context.Background(), 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "role lookup failed")

	err = svc.assignCaregiverRoleIfRequested(context.Background(), 1, &authModel.InvitationToken{
		RoleID:           1,
		CaregiverEnabled: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "role lookup failed")

	svc.roleRepo = &failingRoleRepository{
		stubRoleRepository: newStubRoleRepository(&authModel.Role{
			Model: baseModel.Model{ID: 2},
			Name:  "admin",
		}),
		listErr: errors.New("role list failed"),
	}
	err = svc.assignCaregiverRoleIfRequested(context.Background(), 1, &authModel.InvitationToken{
		RoleID:           2,
		CaregiverEnabled: true,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "role list failed")
}

func TestEnsureInvitationTargetAllowedWrapsTenantLookupError(t *testing.T) {
	t.Parallel()

	service, _, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)
	accounts.storeAccount(&authModel.Account{
		Model:  baseModel.Model{ID: 201},
		Email:  "tenant-error@example.com",
		Active: true,
	})
	svc := service.(*invitationService)
	svc.accountTenantRepo = &failingAccountTenantRepository{
		stubAccountTenantRepository: newStubAccountTenantRepository(),
		existsErr:                   errors.New("tenant lookup failed"),
	}

	err := svc.ensureInvitationTargetAllowed(context.Background(), "tenant-error@example.com", InvitationRequest{
		TenantID: 88,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "tenant lookup failed")
}

func TestCreateOrUpdateAccountErrorPaths(t *testing.T) {
	t.Parallel()

	t.Run("create account error", func(t *testing.T) {
		service, _, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		accounts.failCreate = true
		svc := service.(*invitationService)

		_, err := svc.createOrUpdateAccount(context.Background(), "create-error@example.com", "hash", nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "account create failed")
	})

	t.Run("update password error", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		svc := service.(*invitationService)

		_, err := svc.createOrUpdateAccount(context.Background(), "missing@example.com", "hash", &authModel.Account{
			Model:  baseModel.Model{ID: 202},
			Email:  "missing@example.com",
			Active: true,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "update account password")
	})

	t.Run("reactivate error", func(t *testing.T) {
		service, _, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)
		svc := service.(*invitationService)

		_, err := svc.createOrUpdateAccount(context.Background(), "inactive@example.com", "hash", &authModel.Account{
			Model:  baseModel.Model{ID: 203},
			Email:  "inactive@example.com",
			Active: false,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "update account password")

		accounts.storeAccount(&authModel.Account{
			Model:  baseModel.Model{ID: 204},
			Email:  "inactive@example.com",
			Active: false,
		})
		svc.accountRepo = &failingSetActiveAccountRepository{stubAccountRepository: accounts}
		_, err = svc.createOrUpdateAccount(context.Background(), "inactive@example.com", "hash", &authModel.Account{
			Model:  baseModel.Model{ID: 204},
			Email:  "inactive@example.com",
			Active: false,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "reactivate account on invitation")
	})
}

func TestCreateAccountWithRoleStopsOnPartialFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*invitationService, *authModel.InvitationToken)
		wantErr string
	}{
		{
			name: "account tenant mapping error",
			mutate: func(svc *invitationService, _ *authModel.InvitationToken) {
				svc.accountTenantRepo = &failingAccountTenantRepository{
					stubAccountTenantRepository: newStubAccountTenantRepository(),
					ensureActiveErr:             errors.New("mapping failed"),
				}
			},
			wantErr: "mapping failed",
		},
		{
			name: "role assignment error",
			mutate: func(svc *invitationService, _ *authModel.InvitationToken) {
				svc.accountRoleRepo = failingAccountRoleRepository{}
			},
			wantErr: "role assignment failed",
		},
		{
			name: "caregiver role error",
			mutate: func(svc *invitationService, invitation *authModel.InvitationToken) {
				invitation.RoleID = 1
				invitation.CaregiverEnabled = true
				// Caregiver enablement resolves the system "user" role; drop it
				// so the lookup fails, which is what this case asserts.
				svc.roleRepo = newStubRoleRepository(
					&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "admin", IsSystem: true},
				)
			},
			wantErr: "user role not found",
		},
		{
			name: "staff creation error",
			mutate: func(svc *invitationService, invitation *authModel.InvitationToken) {
				invitation.RoleID = 10
				svc.roleRepo = newStubRoleRepository(&authModel.Role{
					Model:    baseModel.Model{ID: 10},
					Name:     "admin",
					IsSystem: true,
				})
				svc.staffRepo = failingStaffRepository{StaffRepoMock: staffRepoOnly()}
			},
			wantErr: "staff failed",
		},
		{
			name: "teacher creation error",
			mutate: func(svc *invitationService, invitation *authModel.InvitationToken) {
				invitation.RoleID = 11
				svc.roleRepo = newStubRoleRepository(&authModel.Role{
					Model:    baseModel.Model{ID: 11},
					Name:     "user",
					IsSystem: true,
				})
				svc.teacherRepo = failingTeacherRepository{stubTeacherRepository: newStubTeacherRepository()}
			},
			wantErr: "teacher failed",
		},
		{
			name: "mark invitation used error",
			mutate: func(svc *invitationService, _ *authModel.InvitationToken) {
				svc.invitationRepo = &failingInvitationTokenRepository{
					stubInvitationTokenRepository: newStubInvitationTokenRepository(),
					markErr:                       errors.New("mark failed"),
				}
			},
			wantErr: "mark failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
			t.Cleanup(cleanup)
			svc := service.(*invitationService)
			invitation := &authModel.InvitationToken{
				Model:  baseModel.Model{ID: 301},
				Email:  tt.name + "@example.com",
				Token:  tt.name + "-token",
				RoleID: 1,
			}
			invitation.SetTenantID(55)
			tt.mutate(svc, invitation)

			_, err := svc.createAccountWithRole(context.Background(), invitation, "hash", "First", "Last", nil)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

type failingInvitationTokenRepository struct {
	*stubInvitationTokenRepository

	listErr             error
	deleteExpiredErr    error
	invalidateTenantErr error
	updateErr           error
	markErr             error
}

func (r *failingInvitationTokenRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authModel.InvitationToken, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.stubInvitationTokenRepository.List(ctx, filters)
}

func (r *failingInvitationTokenRepository) DeleteExpired(ctx context.Context, now time.Time) (int, error) {
	if r.deleteExpiredErr != nil {
		return 0, r.deleteExpiredErr
	}
	return r.stubInvitationTokenRepository.DeleteExpired(ctx, now)
}

func (r *failingInvitationTokenRepository) InvalidateByTenantID(ctx context.Context, tenantID int64) (int, error) {
	if r.invalidateTenantErr != nil {
		return 0, r.invalidateTenantErr
	}
	return r.stubInvitationTokenRepository.InvalidateByTenantID(ctx, tenantID)
}

func (r *failingInvitationTokenRepository) Update(ctx context.Context, token *authModel.InvitationToken) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	return r.stubInvitationTokenRepository.Update(ctx, token)
}

func (r *failingInvitationTokenRepository) MarkAsUsed(ctx context.Context, id int64) error {
	if r.markErr != nil {
		return r.markErr
	}
	return r.stubInvitationTokenRepository.MarkAsUsed(ctx, id)
}

type failingAccountTenantRepository struct {
	*stubAccountTenantRepository

	ensureActiveErr error
	existsErr       error
}

func (r *failingAccountTenantRepository) EnsureActive(ctx context.Context, accountTenant *authModel.AccountTenant) error {
	if r.ensureActiveErr != nil {
		return r.ensureActiveErr
	}
	return r.stubAccountTenantRepository.EnsureActive(ctx, accountTenant)
}

func (r *failingAccountTenantRepository) ExistsByAccountAndTenant(ctx context.Context, accountID, tenantID int64) (bool, error) {
	if r.existsErr != nil {
		return false, r.existsErr
	}
	return r.stubAccountTenantRepository.ExistsByAccountAndTenant(ctx, accountID, tenantID)
}

type failingSetActiveAccountRepository struct {
	*stubAccountRepository
}

func (r *failingSetActiveAccountRepository) SetActive(context.Context, int64, bool) error {
	return errors.New("set active failed")
}

type failingAccountRoleRepository struct {
	noopAccountRoleRepository
}

func (failingAccountRoleRepository) Create(context.Context, *authModel.AccountRole) error {
	return errors.New("role assignment failed")
}

type failingStaffRepository struct {
	*testpkg.StaffRepoMock
}

func (failingStaffRepository) Create(context.Context, *userModel.Staff) error {
	return errors.New("staff failed")
}

type failingTeacherRepository struct {
	*stubTeacherRepository
}

func (failingTeacherRepository) Create(context.Context, *userModel.Teacher) error {
	return errors.New("teacher failed")
}

type failingRoleRepository struct {
	*stubRoleRepository

	findErr error
	listErr error
}

func (r *failingRoleRepository) FindByID(ctx context.Context, id interface{}) (*authModel.Role, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	return r.stubRoleRepository.FindByID(ctx, id)
}

func (r *failingRoleRepository) List(ctx context.Context, filters map[string]interface{}) ([]*authModel.Role, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	return r.stubRoleRepository.List(ctx, filters)
}

// TestCreateInvitationRejectsRoleEscalation covers the escalation path that made
// the invitation endpoint dangerous: the "user" (Betreuer) role carries
// users:create globally, so without a role-grant check an ordinary staff account
// could invite a fresh address into the admin role and log in as that account.
func TestCreateInvitationRejectsRoleEscalation(t *testing.T) {
	t.Parallel()

	// Permissions of a Betreuer: enough to create person records, not to hand
	// out roles.
	betreuer := []string{permissions.UsersCreate, permissions.UsersUpdate}

	t.Run("betreuer may not invite into the admin role", func(t *testing.T) {
		service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		_, err := service.CreateInvitation(context.Background(), InvitationRequest{
			Email:            "attacker@example.com",
			RoleID:           1, // admin
			CreatedBy:        42,
			ActorPermissions: betreuer,
		})

		require.Error(t, err)
		require.True(t, errors.Is(err, ErrRoleGrantNotPermitted),
			"expected ErrRoleGrantNotPermitted, got %v", err)
		require.Empty(t, invitations.byToken, "no invitation may be persisted")
	})

	t.Run("an account with no permissions may not either", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		_, err := service.CreateInvitation(context.Background(), InvitationRequest{
			Email:     "attacker@example.com",
			RoleID:    1,
			CreatedBy: 42,
		})

		require.Error(t, err)
		require.True(t, errors.Is(err, ErrRoleGrantNotPermitted),
			"expected ErrRoleGrantNotPermitted, got %v", err)
	})

	t.Run("betreuer may still invite into the user role", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		invitation, err := service.CreateInvitation(context.Background(), InvitationRequest{
			Email:            "colleague@example.com",
			RoleID:           2, // user
			CreatedBy:        42,
			ActorPermissions: betreuer,
		})

		require.NoError(t, err)
		require.NotNil(t, invitation)
		require.Equal(t, int64(2), invitation.RoleID)
	})

	t.Run("an admin may invite into the admin role", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		invitation, err := service.CreateInvitation(context.Background(), InvitationRequest{
			Email:            "leitung@example.com",
			RoleID:           1,
			CreatedBy:        42,
			ActorPermissions: []string{permissions.UsersManage},
		})

		require.NoError(t, err)
		require.NotNil(t, invitation)
		require.Equal(t, int64(1), invitation.RoleID)
	})

	t.Run("the operator flow is not subject to the tenant check", func(t *testing.T) {
		service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
		t.Cleanup(cleanup)

		invitation, err := service.CreateInvitation(context.Background(), InvitationRequest{
			Email:         "schulleitung@example.com",
			RoleID:        1,
			CreatedBy:     42,
			OperatorGrant: true,
		})

		require.NoError(t, err)
		require.NotNil(t, invitation)
	})
}
