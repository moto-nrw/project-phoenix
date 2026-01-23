package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/email"
	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
)

// testStrongPassword is a valid password for unit tests that meets strength requirements.
// This is NOT a real secret - it's only used with mocked services in tests.
const testStrongPassword = "Str0ngP@ssword!" //nolint:gosec // Test-only constant, not a real credential

func newInvitationTestEnv(t *testing.T) (InvitationService, *stubInvitationTokenRepository, *stubAccountRepository, *stubRoleRepository, *stubAccountRoleRepository, *stubPersonRepository, *capturingMailer, sqlmock.Sqlmock, func()) {
	service, invitations, accounts, roles, accountRoles, persons, mailer, mock, cleanup := newInvitationTestEnvWithMailer(t, newCapturingMailer())
	capturing, _ := mailer.(*capturingMailer)
	return service, invitations, accounts, roles, accountRoles, persons, capturing, mock, cleanup
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

	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        staffRepo,
		TeacherRepo:      teacherRepo,
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
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

func TestCreateInvitationSuccess(t *testing.T) {
	service, invitations, _, _, _, _, mailer, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "NewUser@example.com ",
		RoleID:    2,
		CreatedBy: 42,
		FirstName: strPtr("Ada"),
		LastName:  strPtr("Lovelace"),
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	require.Equal(t, "newuser@example.com", invitation.Email)
	require.Equal(t, int64(2), invitation.RoleID)
	require.Equal(t, int64(42), invitation.CreatedBy)
	require.NotNil(t, invitation.Role)
	require.Equal(t, "Teacher", invitation.Role.Name)

	ttl := time.Until(invitation.ExpiresAt)
	require.GreaterOrEqual(t, ttl, 47*time.Hour)
	require.LessOrEqual(t, ttl, 49*time.Hour)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

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
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnvWithMailer(t, flaky)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "failure@example.com",
		RoleID:    1,
		CreatedBy: 99,
	}

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
		CreatedBy: 1,
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

func TestValidateInvitationReturnsDetails(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "abc-123",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(12 * time.Hour),
		FirstName: strPtr("Grace"),
		LastName:  strPtr("Hopper"),
	}
	require.NoError(t, invitations.Create(ctx, token))

	result, err := service.ValidateInvitation(ctx, "abc-123")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "user@example.com", result.Email)
	require.Equal(t, "Teacher", result.RoleName)
	require.Equal(t, token.FirstName, result.FirstName)
}

func TestValidateInvitationExpired(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "expired",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	_, err := service.ValidateInvitation(ctx, "expired")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationExpired), fmt.Sprintf("expected ErrInvitationExpired, got %v", err))
}

func TestValidateInvitationUsed(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	now := time.Now()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "used",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &now,
	}
	require.NoError(t, invitations.Create(ctx, token))

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
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectCommit()

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
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	persons.failCreate = true

	mock.ExpectBegin()
	mock.ExpectRollback()

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
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "weak",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

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
	service, invitations, _, _, _, _, mailer, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "resend",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		Model:     baseModel.Model{UpdatedAt: time.Now().Add(-1 * time.Hour), CreatedAt: time.Now().Add(-2 * time.Hour)},
	}
	require.NoError(t, invitations.Create(ctx, token))

	err := service.ResendInvitation(ctx, token.ID, 99)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

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
		CreatedBy: 1,
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
		CreatedBy: 1,
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
		{"user", "Nutzer"},
		{"User", "Nutzer"},
		{"guest", "Gast"},
		{"Guest", "Gast"},
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

// =============================================================================
// Concurrent Invitation Acceptance Tests
// =============================================================================

func TestAcceptInvitationConcurrent(t *testing.T) {
	// Skip: This mock-based test has a race condition because the stub returns
	// shared mutable state. In production, PostgreSQL transactions provide proper
	// isolation. Use integration tests with real DB for true concurrency testing.
	t.Skip("Skipped: Mock-based concurrent test has inherent race condition in stub infrastructure")

	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "concurrent@example.com",
		Token:     "concurrent-token",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	// Set up expectations for one successful transaction
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Launch multiple concurrent acceptance attempts
	const numAttempts = 5
	results := make(chan error, numAttempts)
	var wg sync.WaitGroup

	for i := range numAttempts {
		wg.Add(1)
		go func(attemptNum int) {
			defer wg.Done()
			_, err := service.AcceptInvitation(ctx, "concurrent-token", UserRegistrationData{
				FirstName:       fmt.Sprintf("User%d", attemptNum),
				LastName:        "Concurrent",
				Password:        testStrongPassword,
				ConfirmPassword: testStrongPassword,
			})
			results <- err
		}(i)
	}

	wg.Wait()
	close(results)

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for err := range results {
		if err == nil {
			successCount++
		} else {
			failureCount++
		}
	}

	// At least one should succeed (the first one)
	// The stub doesn't enforce true concurrency, so all might succeed
	// In a real scenario with database locks, only one would succeed
	require.GreaterOrEqual(t, successCount, 1, "At least one acceptance should succeed")

	// Token should be marked as used
	require.True(t, token.IsUsed(), "Token should be marked as used after acceptance")
}

func TestAcceptInvitationSecondAttemptFails(t *testing.T) {
	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "second-attempt@example.com",
		Token:     "second-attempt-token",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	// First acceptance
	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.AcceptInvitation(ctx, "second-attempt-token", UserRegistrationData{
		FirstName:       "First",
		LastName:        "User",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	// Second attempt should fail
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
// CreateInvitation Error Path Tests
// =============================================================================

func TestCreateInvitationEmptyEmail(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "   ",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err := service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "email is required")
}

func TestCreateInvitationInvalidEmailFormat(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "not-an-email",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err := service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid email address")
}

func TestCreateInvitationEmailAlreadyRegistered(t *testing.T) {
	service, _, accounts, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	existingAccount := &authModel.Account{
		Email:  "existing@example.com",
		Active: true,
	}
	require.NoError(t, accounts.Create(ctx, existingAccount))

	req := InvitationRequest{
		Email:     "existing@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err := service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrEmailAlreadyExists))
}

func TestCreateInvitationInvalidRoleID(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	tests := []struct {
		name   string
		roleID int64
	}{
		{"zero role", 0},
		{"negative role", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := InvitationRequest{
				Email:     "user@example.com",
				RoleID:    tt.roleID,
				CreatedBy: 42,
			}
			_, err := service.CreateInvitation(ctx, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "role id is required")
		})
	}
}

func TestCreateInvitationInvalidCreatedBy(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	tests := []struct {
		name      string
		createdBy int64
	}{
		{"zero created_by", 0},
		{"negative created_by", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := InvitationRequest{
				Email:     "user@example.com",
				RoleID:    1,
				CreatedBy: tt.createdBy,
			}
			_, err := service.CreateInvitation(ctx, req)
			require.Error(t, err)
			require.Contains(t, err.Error(), "created_by is required")
		})
	}
}

func TestCreateInvitationRoleNotFound(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    999, // Non-existent role
		CreatedBy: 42,
	}

	_, err := service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "role not found")
}

func TestCreateInvitationWithPositionField(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	position := "  Head Teacher  "
	req := InvitationRequest{
		Email:     "teacher@example.com",
		RoleID:    2,
		CreatedBy: 42,
		FirstName: strPtr("  John  "),
		LastName:  strPtr("  Doe  "),
		Position:  &position,
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)
	require.Equal(t, "Head Teacher", *invitation.Position)
	require.Equal(t, "John", *invitation.FirstName)
	require.Equal(t, "Doe", *invitation.LastName)
	require.Contains(t, invitations.byToken, invitation.Token)
}

// =============================================================================
// ValidateInvitation Error Path Tests
// =============================================================================

func TestValidateInvitationEmptyToken(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	_, err := service.ValidateInvitation(ctx, "   ")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound))
}

func TestValidateInvitationTokenNotFound(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	_, err := service.ValidateInvitation(ctx, "nonexistent-token")
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound))
}

// =============================================================================
// AcceptInvitation Error Path Tests
// =============================================================================

func TestAcceptInvitationPasswordMismatch(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "mismatch",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	_, err := service.AcceptInvitation(ctx, "mismatch", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: "DifferentP@ssword1",
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrPasswordMismatch))
	require.False(t, token.IsUsed())
}

func TestAcceptInvitationMissingNamesNoFallback(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "no-names",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		// No FirstName or LastName on invitation
	}
	require.NoError(t, invitations.Create(ctx, token))

	_, err := service.AcceptInvitation(ctx, "no-names", UserRegistrationData{
		FirstName:       "",
		LastName:        "",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNameRequired))
}

func TestAcceptInvitationUsesInvitationNameFallback(t *testing.T) {
	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "with-names",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		FirstName: strPtr("Invitation"),
		LastName:  strPtr("Name"),
	}
	require.NoError(t, invitations.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.AcceptInvitation(ctx, "with-names", UserRegistrationData{
		FirstName:       "", // Empty - should fall back to invitation
		LastName:        "", // Empty - should fall back to invitation
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
}

func TestAcceptInvitationPartialNameFallback(t *testing.T) {
	service, invitations, _, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "partial-names",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		FirstName: strPtr("InvitationFirst"),
		// No LastName on invitation
	}
	require.NoError(t, invitations.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.AcceptInvitation(ctx, "partial-names", UserRegistrationData{
		FirstName:       "",         // Empty - falls back to invitation
		LastName:        "UserLast", // Provided by user
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)
}

func TestAcceptInvitationExpiredToken(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "expired-accept",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	_, err := service.AcceptInvitation(ctx, "expired-accept", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationExpired))
}

func TestAcceptInvitationAlreadyUsedToken(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	usedAt := time.Now()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "used-accept",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &usedAt,
	}
	require.NoError(t, invitations.Create(ctx, token))

	_, err := service.AcceptInvitation(ctx, "used-accept", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed))
}

func TestAcceptInvitationAccountCreationFails(t *testing.T) {
	service, invitations, accounts, _, _, _, _, mock, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "account-fail",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	accounts.failCreate = true

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err := service.AcceptInvitation(ctx, "account-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.False(t, token.IsUsed())
}

// =============================================================================
// ResendInvitation Error Path Tests
// =============================================================================

func TestResendInvitationNotFound(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	err := service.ResendInvitation(ctx, 99999, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound))
}

func TestResendInvitationAlreadyUsed(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	usedAt := time.Now()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "used-resend",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &usedAt,
	}
	require.NoError(t, invitations.Create(ctx, token))

	err := service.ResendInvitation(ctx, token.ID, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed))
}

// =============================================================================
// RevokeInvitation Error Path Tests
// =============================================================================

func TestRevokeInvitationNotFound(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	err := service.RevokeInvitation(ctx, 99999, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationNotFound))
}

func TestRevokeInvitationAlreadyUsed(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	usedAt := time.Now()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "used-revoke",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &usedAt,
	}
	require.NoError(t, invitations.Create(ctx, token))

	err := service.RevokeInvitation(ctx, token.ID, 1)
	require.Error(t, err)
	require.True(t, errors.Is(err, ErrInvitationUsed))
}

// =============================================================================
// ListPendingInvitations Tests
// =============================================================================

func TestListPendingInvitationsSuccess(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Add a pending invitation
	token1 := &authModel.InvitationToken{
		Email:     "pending1@example.com",
		Token:     "pending-1",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token1))

	// Add a used invitation (should not appear)
	usedAt := time.Now()
	token2 := &authModel.InvitationToken{
		Email:     "used@example.com",
		Token:     "used-list",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &usedAt,
	}
	require.NoError(t, invitations.Create(ctx, token2))

	// Add an expired invitation (should not appear)
	token3 := &authModel.InvitationToken{
		Email:     "expired@example.com",
		Token:     "expired-list",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token3))

	list, err := service.ListPendingInvitations(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	require.Equal(t, "pending1@example.com", list[0].Email)
}

func TestListPendingInvitationsEmpty(t *testing.T) {
	service, _, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	list, err := service.ListPendingInvitations(ctx)
	require.NoError(t, err)
	require.Empty(t, list)
}

// =============================================================================
// CleanupExpiredInvitations Tests
// =============================================================================

func TestCleanupExpiredInvitationsSuccess(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Add an expired invitation
	token1 := &authModel.InvitationToken{
		Email:     "expired1@example.com",
		Token:     "cleanup-expired",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token1))

	// Add a used invitation
	usedAt := time.Now()
	token2 := &authModel.InvitationToken{
		Email:     "used@example.com",
		Token:     "cleanup-used",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		UsedAt:    &usedAt,
	}
	require.NoError(t, invitations.Create(ctx, token2))

	// Add a valid invitation (should not be cleaned up)
	token3 := &authModel.InvitationToken{
		Email:     "valid@example.com",
		Token:     "cleanup-valid",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token3))

	count, err := service.CleanupExpiredInvitations(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, count) // Expired and used should be cleaned

	// Valid token should still exist
	remaining, err := invitations.FindByToken(ctx, "cleanup-valid")
	require.NoError(t, err)
	require.NotNil(t, remaining)
}

func TestCleanupExpiredInvitationsNoneToClean(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()

	// Add only valid invitations
	token := &authModel.InvitationToken{
		Email:     "valid@example.com",
		Token:     "no-cleanup",
		RoleID:    2,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	count, err := service.CleanupExpiredInvitations(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestSanitizeEmailError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"nil error", nil, ""},
		{"simple error", errors.New("smtp failed"), "smtp failed"},
		{"whitespace error", errors.New("  error with spaces  "), "error with spaces"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeEmailError(tt.err)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNotFoundErrorWithDatabaseError(t *testing.T) {
	// Test with nil
	require.False(t, isNotFoundError(nil))

	// Test with sql.ErrNoRows directly
	require.True(t, isNotFoundError(sql.ErrNoRows))

	// Test with wrapped sql.ErrNoRows
	wrapped := fmt.Errorf("wrapped: %w", sql.ErrNoRows)
	require.True(t, isNotFoundError(wrapped))

	// Test with DatabaseError wrapping sql.ErrNoRows
	dbErr := &baseModel.DatabaseError{
		Op:  "find",
		Err: sql.ErrNoRows,
	}
	require.True(t, isNotFoundError(dbErr))

	// Test with other errors
	require.False(t, isNotFoundError(errors.New("some other error")))

	// Test with DatabaseError containing non-NotFound error
	dbErrOther := &baseModel.DatabaseError{
		Op:  "find",
		Err: errors.New("connection failed"),
	}
	require.False(t, isNotFoundError(dbErrOther))
}

// =============================================================================
// NewInvitationService Configuration Tests
// =============================================================================

func TestNewInvitationServiceTrimsFrontendURL(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	defer func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}()

	mailer := newCapturingMailer()
	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   newStubInvitationTokenRepository(),
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		FrontendURL:      "  http://localhost:3000/  ",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	svc := service.(*invitationService)
	require.Equal(t, "http://localhost:3000", svc.frontendURL)
}

func TestNewInvitationServiceCreatesDispatcherFromMailer(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	defer func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}()

	mailer := newCapturingMailer()
	// Pass mailer but no dispatcher
	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   newStubInvitationTokenRepository(),
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       nil, // Will be created from mailer
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	svc := service.(*invitationService)
	require.NotNil(t, svc.dispatcher)
}

func TestNewInvitationServiceNilDispatcherAndMailer(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	defer func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}()

	// Pass neither mailer nor dispatcher
	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   newStubInvitationTokenRepository(),
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           nil,
		Dispatcher:       nil,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	svc := service.(*invitationService)
	require.Nil(t, svc.dispatcher)
}

// =============================================================================
// Email Sending Edge Cases
// =============================================================================

func TestCreateInvitationWithNilDispatcher(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	defer func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}()

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)

	// Create service without dispatcher
	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           nil,
		Dispatcher:       nil,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	// Should succeed even without dispatcher (email sending is skipped)
	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)
}

func TestCreateInvitationWithEmptyFrontendURL(t *testing.T) {
	service, invitations, _, _, _, _, mailer, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	// Access internal service to test default URL fallback
	svc := service.(*invitationService)
	originalURL := svc.frontendURL
	svc.frontendURL = ""
	defer func() { svc.frontendURL = originalURL }()

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	invitation, err := service.CreateInvitation(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, invitation)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	content := msg.Content.(map[string]any)
	invitationURL := content["InvitationURL"].(string)
	require.Contains(t, invitationURL, "http://localhost:3000")
	require.Contains(t, invitations.byToken, invitation.Token)
}

// =============================================================================
// Email Delivery Callback Tests
// =============================================================================

func TestPersistInvitationDeliverySuccess(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "persist-success",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	svc := service.(*invitationService)
	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: token.ID,
		Token:       token.Token,
		Recipient:   token.Email,
	}

	sentTime := time.Now()
	result := email.DeliveryResult{
		Status:  email.DeliveryStatusSent,
		Attempt: 1,
		SentAt:  sentTime,
		Final:   true,
	}

	svc.persistInvitationDelivery(ctx, meta, 0, result)

	updated, err := invitations.FindByID(ctx, token.ID)
	require.NoError(t, err)
	require.NotNil(t, updated.EmailSentAt)
	require.Nil(t, updated.EmailError)
	require.Equal(t, 1, updated.EmailRetryCount)
}

func TestPersistInvitationDeliveryFailure(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "persist-failure",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitations.Create(ctx, token))

	svc := service.(*invitationService)
	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: token.ID,
		Token:       token.Token,
		Recipient:   token.Email,
	}

	result := email.DeliveryResult{
		Status:  email.DeliveryStatusFailed,
		Attempt: 3,
		Err:     errors.New("smtp connection refused"),
		Final:   true,
	}

	svc.persistInvitationDelivery(ctx, meta, 0, result)

	updated, err := invitations.FindByID(ctx, token.ID)
	require.NoError(t, err)
	require.Nil(t, updated.EmailSentAt)
	require.NotNil(t, updated.EmailError)
	require.Contains(t, *updated.EmailError, "smtp connection refused")
	require.Equal(t, 3, updated.EmailRetryCount)
}

func TestPersistInvitationDeliveryBaseRetryCount(t *testing.T) {
	service, invitations, _, _, _, _, _, _, cleanup := newInvitationTestEnv(t)
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:           "user@example.com",
		Token:           "persist-retry",
		RoleID:          1,
		CreatedBy:       1,
		ExpiresAt:       time.Now().Add(10 * time.Hour),
		EmailRetryCount: 2, // Already had 2 retries
	}
	require.NoError(t, invitations.Create(ctx, token))

	svc := service.(*invitationService)
	meta := email.DeliveryMetadata{
		Type:        "invitation",
		ReferenceID: token.ID,
		Token:       token.Token,
		Recipient:   token.Email,
	}

	sentTime := time.Now()
	result := email.DeliveryResult{
		Status:  email.DeliveryStatusSent,
		Attempt: 1, // First attempt in this dispatch
		SentAt:  sentTime,
		Final:   true,
	}

	svc.persistInvitationDelivery(ctx, meta, 2, result) // baseRetry=2

	updated, err := invitations.FindByID(ctx, token.ID)
	require.NoError(t, err)
	require.Equal(t, 3, updated.EmailRetryCount) // 2 + 1 = 3
}

// =============================================================================
// WithTx Tests
// =============================================================================

func TestWithTxClonesService(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	defer func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}()

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   newStubInvitationTokenRepository(),
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	mock.ExpectBegin()
	tx, err := bunDB.Begin()
	require.NoError(t, err)

	cloned := service.(*invitationService).WithTx(tx)
	require.NotNil(t, cloned)

	clonedSvc := cloned.(*invitationService)
	require.Equal(t, "http://localhost:3000", clonedSvc.frontendURL)
	require.Equal(t, 48*time.Hour, clonedSvc.invitationExpiry)
}

// =============================================================================
// System Role Staff/Teacher Creation Tests
// =============================================================================

func TestAcceptInvitationCreatesStaffAndTeacherForSystemRole(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	systemRole := &authModel.Role{
		Model:    baseModel.Model{ID: 1},
		Name:     "Admin",
		IsSystem: true, // This is a system role
	}
	roleRepo := newStubRoleRepository(systemRole)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo := newStubStaffRepository()
	teacherRepo := newStubTeacherRepository()

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        staffRepo,
		TeacherRepo:      teacherRepo,
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	position := "Principal"
	token := &authModel.InvitationToken{
		Email:     "admin@example.com",
		Token:     "system-role-token",
		RoleID:    1, // System role
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
		Position:  &position,
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.AcceptInvitation(ctx, "system-role-token", UserRegistrationData{
		FirstName:       "Admin",
		LastName:        "User",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	// Verify staff was created
	require.Len(t, staffRepo.staff, 1)

	// Verify teacher was created with position
	require.Len(t, teacherRepo.teachers, 1)
	for _, teacher := range teacherRepo.teachers {
		require.Equal(t, "Principal", teacher.Role)
	}
}

func TestAcceptInvitationSkipsStaffTeacherForNonSystemRole(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	nonSystemRole := &authModel.Role{
		Model:    baseModel.Model{ID: 2},
		Name:     "Custom",
		IsSystem: false, // Not a system role
	}
	roleRepo := newStubRoleRepository(nonSystemRole)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo := newStubStaffRepository()
	teacherRepo := newStubTeacherRepository()

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        staffRepo,
		TeacherRepo:      teacherRepo,
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "non-system-role-token",
		RoleID:    2, // Non-system role
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectCommit()

	account, err := service.AcceptInvitation(ctx, "non-system-role-token", UserRegistrationData{
		FirstName:       "Regular",
		LastName:        "User",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.NoError(t, err)
	require.NotNil(t, account)

	// Staff and teacher should NOT be created for non-system role
	require.Empty(t, staffRepo.staff)
	require.Empty(t, teacherRepo.teachers)
}

// =============================================================================
// Role Lookup Error Handling Tests
// =============================================================================

func TestValidateInvitationRoleLookupNonFatalError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	failingRoleRepo := &failingStubRoleRepository{
		err: errors.New("database connection lost"),
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         failingRoleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "role-error",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	_, err = service.ValidateInvitation(ctx, "role-error")
	require.Error(t, err)
	require.Contains(t, err.Error(), "database connection lost")
}

func TestResendInvitationRoleLookupError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	failingRoleRepo := &failingStubRoleRepository{
		err: errors.New("role lookup failed"),
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         failingRoleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "resend-role-error",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	err = service.ResendInvitation(ctx, token.ID, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "role lookup failed")
}

// =============================================================================
// Additional Coverage Tests for Lower-Coverage Functions
// =============================================================================

func TestCreateInvitationWithCreatorLookupError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	// Use a failing account repository
	accountRepo := &failingStubAccountRepository{
		stubAccountRepository: *newStubAccountRepository(),
		failFindByID:          true,
		findByIDError:         errors.New("database error"),
	}
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err = service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "lookup creator")
}

func TestInvalidatePreviousInvitationsError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	// Create a failing invitation repo
	invitationRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: newStubInvitationTokenRepository(),
		failInvalidateByEmail:         true,
	}
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err = service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalidate invitations")
}

func TestListPendingInvitationsError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: newStubInvitationTokenRepository(),
		failList:                      true,
	}

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	_, err = service.ListPendingInvitations(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "list invitations")
}

func TestCleanupExpiredInvitationsError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: newStubInvitationTokenRepository(),
		failDeleteExpired:             true,
	}

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	_, err = service.CleanupExpiredInvitations(context.Background())
	require.Error(t, err)
	require.Contains(t, err.Error(), "cleanup invitations")
}

func TestRevokeInvitationMarkAsUsedError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	// We'll test the MarkAsUsed error path via a custom failing repo
	failingRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: invitationRepo,
		failMarkAsUsed:                true,
	}

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   failingRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "revoke-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, failingRepo.Create(ctx, token))

	err = service.RevokeInvitation(ctx, token.ID, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "revoke invitation")
}

func TestResendInvitationUpdateError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	failingRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: invitationRepo,
		failUpdate:                    true,
	}
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   failingRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "resend-update-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, failingRepo.Create(ctx, token))

	err = service.ResendInvitation(ctx, token.ID, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "resend invitation")
}

func TestAcceptInvitationLinkPersonError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := &failingStubPersonRepository{
		stubPersonRepository: *newStubPersonRepository(),
		failLinkToAccount:    true,
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "link-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.AcceptInvitation(ctx, "link-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "link person to account")
}

func TestAcceptInvitationAssignRoleError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)
	accountRoleRepo := &failingStubAccountRoleRepository{
		stubAccountRoleRepository: *newStubAccountRoleRepository(),
		failCreate:                true,
	}
	personRepo := newStubPersonRepository()

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "role-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.AcceptInvitation(ctx, "role-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "assign role")
}

func TestAcceptInvitationCreateStaffError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	systemRole := &authModel.Role{
		Model:    baseModel.Model{ID: 1},
		Name:     "Admin",
		IsSystem: true,
	}
	roleRepo := newStubRoleRepository(systemRole)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo := &failingStubStaffRepository{
		stubStaffRepository: *newStubStaffRepository(),
		failCreate:          true,
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        staffRepo,
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "staff-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.AcceptInvitation(ctx, "staff-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create staff")
}

func TestAcceptInvitationCreateTeacherError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	accountRepo := newStubAccountRepository()
	systemRole := &authModel.Role{
		Model:    baseModel.Model{ID: 1},
		Name:     "Admin",
		IsSystem: true,
	}
	roleRepo := newStubRoleRepository(systemRole)
	accountRoleRepo := newStubAccountRoleRepository()
	personRepo := newStubPersonRepository()
	staffRepo := newStubStaffRepository()
	teacherRepo := &failingStubTeacherRepository{
		stubTeacherRepository: *newStubTeacherRepository(),
		failCreate:            true,
	}

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   invitationRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  accountRoleRepo,
		PersonRepo:       personRepo,
		StaffRepo:        staffRepo,
		TeacherRepo:      teacherRepo,
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "teacher-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, invitationRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.AcceptInvitation(ctx, "teacher-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "create teacher")
}

func TestAcceptInvitationMarkAsUsedError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	invitationRepo := newStubInvitationTokenRepository()
	failingRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: invitationRepo,
		failMarkAsUsed:                true,
	}
	accountRepo := newStubAccountRepository()
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin", IsSystem: false},
	)

	mailer := newCapturingMailer()
	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond})

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   failingRepo,
		AccountRepo:      accountRepo,
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           mailer,
		Dispatcher:       dispatcher,
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	token := &authModel.InvitationToken{
		Email:     "user@example.com",
		Token:     "mark-used-fail",
		RoleID:    1,
		CreatedBy: 1,
		ExpiresAt: time.Now().Add(10 * time.Hour),
	}
	require.NoError(t, failingRepo.Create(ctx, token))

	mock.ExpectBegin()
	mock.ExpectRollback()

	_, err = service.AcceptInvitation(ctx, "mark-used-fail", UserRegistrationData{
		FirstName:       "Jane",
		LastName:        "Doe",
		Password:        testStrongPassword,
		ConfirmPassword: testStrongPassword,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "mark invitation used")
}

func TestFetchValidInvitationRepositoryError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	failingRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: newStubInvitationTokenRepository(),
		failFindByToken:               true,
		findByTokenError:              errors.New("database unavailable"),
	}

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   failingRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         newStubRoleRepository(),
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	_, err = service.ValidateInvitation(context.Background(), "some-token")
	require.Error(t, err)
	require.Contains(t, err.Error(), "database unavailable")
}

func TestCreateInvitationRepositoryCreateError(t *testing.T) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	failingRepo := &failingStubInvitationRepository{
		stubInvitationTokenRepository: newStubInvitationTokenRepository(),
		failCreate:                    true,
	}
	roleRepo := newStubRoleRepository(
		&authModel.Role{Model: baseModel.Model{ID: 1}, Name: "Admin"},
	)

	service := NewInvitationService(InvitationServiceConfig{
		InvitationRepo:   failingRepo,
		AccountRepo:      newStubAccountRepository(),
		RoleRepo:         roleRepo,
		AccountRoleRepo:  newStubAccountRoleRepository(),
		PersonRepo:       newStubPersonRepository(),
		StaffRepo:        newStubStaffRepository(),
		TeacherRepo:      newStubTeacherRepository(),
		Mailer:           newCapturingMailer(),
		FrontendURL:      "http://localhost:3000",
		DefaultFrom:      newDefaultFromEmail(),
		InvitationExpiry: 48 * time.Hour,
		DB:               bunDB,
	})

	cleanup := func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
	}
	t.Cleanup(cleanup)

	ctx := context.Background()
	req := InvitationRequest{
		Email:     "user@example.com",
		RoleID:    1,
		CreatedBy: 42,
	}

	_, err = service.CreateInvitation(ctx, req)
	require.Error(t, err)
	require.Contains(t, err.Error(), "create invitation")
}

// =============================================================================
// Failing Stub Repositories
// =============================================================================

// failingStubRoleRepository always returns an error on FindByID
type failingStubRoleRepository struct {
	noopRoleRepository
	err error
}

func (r *failingStubRoleRepository) FindByID(_ context.Context, _ any) (*authModel.Role, error) {
	return nil, r.err
}

// failingStubInvitationRepository wraps stubInvitationTokenRepository with configurable failures.
type failingStubInvitationRepository struct {
	*stubInvitationTokenRepository
	failCreate            bool
	failInvalidateByEmail bool
	failList              bool
	failDeleteExpired     bool
	failMarkAsUsed        bool
	failUpdate            bool
	failFindByToken       bool
	findByTokenError      error
}

func (r *failingStubInvitationRepository) Create(ctx context.Context, token *authModel.InvitationToken) error {
	if r.failCreate {
		return errors.New("create failed")
	}
	return r.stubInvitationTokenRepository.Create(ctx, token)
}

func (r *failingStubInvitationRepository) InvalidateByEmail(_ context.Context, _ string) (int, error) {
	if r.failInvalidateByEmail {
		return 0, errors.New("invalidate by email failed")
	}
	return 0, nil
}

func (r *failingStubInvitationRepository) List(_ context.Context, _ map[string]any) ([]*authModel.InvitationToken, error) {
	if r.failList {
		return nil, errors.New("list failed")
	}
	return nil, nil
}

func (r *failingStubInvitationRepository) DeleteExpired(_ context.Context, _ time.Time) (int, error) {
	if r.failDeleteExpired {
		return 0, errors.New("delete expired failed")
	}
	return 0, nil
}

func (r *failingStubInvitationRepository) MarkAsUsed(_ context.Context, _ int64) error {
	if r.failMarkAsUsed {
		return errors.New("mark as used failed")
	}
	return nil
}

func (r *failingStubInvitationRepository) Update(_ context.Context, _ *authModel.InvitationToken) error {
	if r.failUpdate {
		return errors.New("update failed")
	}
	return nil
}

func (r *failingStubInvitationRepository) FindByToken(ctx context.Context, token string) (*authModel.InvitationToken, error) {
	if r.failFindByToken {
		return nil, r.findByTokenError
	}
	return r.stubInvitationTokenRepository.FindByToken(ctx, token)
}

// failingStubAccountRepository wraps stubAccountRepository with configurable failures.
type failingStubAccountRepository struct {
	stubAccountRepository
	failFindByID   bool
	findByIDError  error
	failFindByEmail bool
}

func (r *failingStubAccountRepository) FindByID(_ context.Context, _ any) (*authModel.Account, error) {
	if r.failFindByID {
		return nil, r.findByIDError
	}
	return nil, sql.ErrNoRows
}

// failingStubPersonRepository wraps stubPersonRepository with configurable failures.
type failingStubPersonRepository struct {
	stubPersonRepository
	failLinkToAccount bool
}

func (r *failingStubPersonRepository) LinkToAccount(_ context.Context, _, _ int64) error {
	if r.failLinkToAccount {
		return errors.New("link to account failed")
	}
	return nil
}

// failingStubAccountRoleRepository wraps stubAccountRoleRepository with configurable failures.
type failingStubAccountRoleRepository struct {
	stubAccountRoleRepository
	failCreate bool
}

func (r *failingStubAccountRoleRepository) Create(_ context.Context, _ *authModel.AccountRole) error {
	if r.failCreate {
		return errors.New("create account role failed")
	}
	return nil
}

// failingStubStaffRepository wraps stubStaffRepository with configurable failures.
type failingStubStaffRepository struct {
	stubStaffRepository
	failCreate bool
}

func (r *failingStubStaffRepository) Create(_ context.Context, _ *userModel.Staff) error {
	if r.failCreate {
		return errors.New("create staff failed")
	}
	return nil
}

// failingStubTeacherRepository wraps stubTeacherRepository with configurable failures.
type failingStubTeacherRepository struct {
	stubTeacherRepository
	failCreate bool
}

func (r *failingStubTeacherRepository) Create(_ context.Context, _ *userModel.Teacher) error {
	if r.failCreate {
		return errors.New("create teacher failed")
	}
	return nil
}
