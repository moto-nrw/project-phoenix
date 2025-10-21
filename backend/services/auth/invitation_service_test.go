package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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

	dispatcher := email.NewDispatcher(mailer)
	dispatcher.SetDefaults(3, []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 40 * time.Millisecond})

	service := NewInvitationService(
		invitationRepo,
		accountRepo,
		roleRepo,
		accountRoleRepo,
		personRepo,
		mailer,
		dispatcher,
		"http://localhost:3000",
		newDefaultFromEmail(),
		48*time.Hour,
		bunDB,
	)

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

	ttl := time.Until(invitation.ExpiresAt)
	require.GreaterOrEqual(t, ttl, 47*time.Hour)
	require.LessOrEqual(t, ttl, 49*time.Hour)

	require.Eventually(t, func() bool {
		return len(mailer.Messages()) == 1
	}, 200*time.Millisecond, 10*time.Millisecond)

	msg := mailer.Messages()[0]
	require.Equal(t, "You're Invited to Project Phoenix", msg.Subject)
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
		Password:        "Str0ngP@ssword!",
		ConfirmPassword: "Str0ngP@ssword!",
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
		Password:        "Str0ngP@ssword!",
		ConfirmPassword: "Str0ngP@ssword!",
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
