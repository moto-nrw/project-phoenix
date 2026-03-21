package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
)

// testStrongPassword is a valid password for unit tests that meets strength requirements.
// This is NOT a real secret - it's only used with mocked services in tests.
const testStrongPassword = "Str0ngP@ssword!" //nolint:gosec // Test-only constant, not a real credential

func newInvitationTestEnv(t *testing.T) (InvitationService, *stubInvitationTokenRepository, *stubAccountRepository, *stubRoleRepository, *stubAccountRoleRepository, *stubPersonRepository, *capturingMailer, sqlmock.Sqlmock, func()) {
	service, invitations, accounts, roles, accountRoles, persons, _, mock, cleanup := newInvitationTestEnvWithMailer(t, nil)
	return service, invitations, accounts, roles, accountRoles, persons, nil, mock, cleanup
}

func newInvitationTestEnvWithMailer(t *testing.T, mailer email.Mailer) (InvitationService, *stubInvitationTokenRepository, *stubAccountRepository, *stubRoleRepository, *stubAccountRoleRepository, *stubPersonRepository, email.Mailer, sqlmock.Sqlmock, func()) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
		&authModel.Role{Model: baseModel.Model{ID: 2}, Name: "Teacher"},
	)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo := newStubStaffRepository()
	teacherRepo := newStubTeacherRepository()

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:    invitationRepo,
		AccountRepo:       accountRepo,
		AccountTenantRepo: newStubAccountTenantRepository(),
		RoleRepo:          roleRepo,
		AccountRoleRepo:   accountRoleRepo,
		PersonRepo:        personRepo,
		StaffRepo:         staffRepo,
		TeacherRepo:       teacherRepo,
		SchoolRepo:        &stubSchoolRepository{},
		Mailer:            mailer,
		Dispatcher:        dispatcher,
		FrontendURL:       "http://localhost:3000",
		DefaultFrom:       newDefaultFromEmail(),
		InvitationExpiry:  48 * time.Hour,
		DB:                bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}

	return service, invitationRepo, accountRepo, roleRepo, accountRoleRepo, personRepo, mailer, mock, cleanup
}

func strPtr(s string) *string {
	return &s
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
	service, invitations, _, _, _, _, rawMailer, mock, cleanup := newInvitationTestEnvWithMailer(t, newCapturingMailer())
	t.Cleanup(cleanup)
	mailer, ok := rawMailer.(*capturingMailer)
	require.True(t, ok)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "NewUser@example.com ",
		RoleID:    2,
		CreatedBy: 42,
		FirstName: strPtr("Ada"),
		LastName:  strPtr("Lovelace"),
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
	require.Equal(t, "Teacher", invitation.Role.Name)

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

func TestInvitationEmailFailureRecordsError(t *testing.T) {
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
		return updated.EmailRetryCount == 3 && updated.EmailError != nil && *updated.EmailError != "" && updated.EmailSentAt == nil
	}, time.Second, 20*time.Millisecond)

	require.Equal(t, 3, flaky.Attempts())
	require.Len(t, flaky.Messages(), 0)
}

func TestCreateInvitationInvalidatesExistingTokens(t *testing.T) {
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
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	otherTenant := &authModel.InvitationToken{
		Email:     "principal@example.com",
		Token:     "other-tenant-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	otherTenant.SetTenantID(1)
	require.NoError(t, invitations.Create(ctx, otherTenant))

	targetTenant := &authModel.InvitationToken{
		Email:     "principal@example.com",
		Token:     "target-tenant-token",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	targetTenant.SetTenantID(2)
	require.NoError(t, invitations.Create(ctx, targetTenant))

	req := InvitationRequest{
		Email:     "principal@example.com",
		RoleID:    2,
		TenantID:  2,
		CreatedBy: 0,
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Equal(t, int64(2), invitation.TenantID)
	require.Nil(t, invitation.CreatedBy)

	require.Nil(t, otherTenant.UsedAt, "invite in a different tenant must remain valid")
	require.NotNil(t, targetTenant.UsedAt, "invite in the target tenant should be invalidated")
}

func TestValidateInvitationReturnsDetails(t *testing.T) {
	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "abc-123",
		RoleID:    2,
		CreatedBy: nullableCreatedBy(1),
		ExpiresAt: time.Now().Add(12 * time.Hour),
		FirstName: strPtr("Grace"),
		LastName:  strPtr("Hopper"),
	}
	require.NoError(t, invitations.Create(ctx, token))

	expectAdminTx(mock)
	result, err := service.ValidateInvitation(ctx, "abc-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user@example.com", result.Email)
	require.Equal(t, "Teacher", result.RoleName)
	require.Equal(t, token.FirstName, result.FirstName)
}

func TestValidateInvitationExpired(t *testing.T) {
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
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
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
	service, invitations, accounts, _, _, persons, _, mock, cleanup := newInvitationTestEnv(t)
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
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.False(t, token.IsUsed(), "invitation should remain unused on failure")

	_, err = accounts.FindByEmail(ctx, "user@example.com")
	require.ErrorIs(t, err, sql.ErrNoRows)
	require.Equal(t, 0, len(persons.people), "person creation should not persist")
}

func TestAcceptInvitationWeakPassword(t *testing.T) {
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
	service, invitations, _, _, _, _, rawMailer, mock, cleanup := newInvitationTestEnvWithMailer(t, newCapturingMailer())
	t.Cleanup(cleanup)
	mailer, ok := rawMailer.(*capturingMailer)
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
		return updated.EmailSentAt != nil && updated.EmailError == nil && updated.EmailRetryCount == 1
	}, time.Second, 10*time.Millisecond)

	require.True(t, token.UpdatedAt.After(time.Now().Add(-30*time.Second)), "updated_at should be refreshed")
}

func TestResendInvitationExpired(t *testing.T) {
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

func TestShouldCreateTeacherForRole(t *testing.T) {
	require.True(t, shouldCreateTeacherForRole("teacher"))
	require.True(t, shouldCreateTeacherForRole("Teacher"))
	require.True(t, shouldCreateTeacherForRole("user"))
	require.False(t, shouldCreateTeacherForRole("admin"))
}

func TestAcceptInvitationSecondAttemptFails(t *testing.T) {
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
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	// Second attempt should fail
	expectAdminTxRollback(mock)
	_, err = service.AcceptInvitation(ctx, "second-attempt-token", UserRegistrationData{
		FirstName:       "Second",
		LastName:        "User",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed), "Second acceptance should fail with ErrInvitationUsed")
}

// =============================================================================
// lookupSchoolName error-path tests
// =============================================================================

func TestLookupSchoolNameZeroTenant(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	svc := service.(*invitationService)
	result := svc.lookupSchoolName(context.Background(), 0)
	require.Equal(t, "", result, "tenantID=0 must return empty string")
}

func TestLookupSchoolNameNilRepo(t *testing.T) {
	svc := &invitationService{
		logger: slog.Default(),
	}
	result := svc.lookupSchoolName(context.Background(), 42)
	require.Equal(t, "", result, "nil schoolRepo must return empty string")
}

func TestCreateInvitationAllowsExistingAccountForNewTenant(t *testing.T) {
	service, invitations, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	accounts.storeAccount(&authModel.Account{Model: baseModel.Model{ID: 7}, Email: "principal@example.com", Active: true})

	invitation, err := service.CreateInvitation(ctx, InvitationRequest{
		Email:    "principal@example.com",
		RoleID:   1,
		TenantID: 2,
	})
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Contains(t, invitations.byToken, invitation.Token)
}

func TestAcceptInvitationReusesExistingAccountForNewTenant(t *testing.T) {
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
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
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
