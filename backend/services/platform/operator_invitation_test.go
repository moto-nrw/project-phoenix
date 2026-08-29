package platform_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// --- Mock for OperatorInvitationTokenRepository ---

type mockInvitationTokenRepo struct {
	createFn               func(ctx context.Context, token *platform.OperatorInvitationToken) error
	findByIDFn             func(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error)
	findValidByTokenFn     func(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	consumeByTokenFn       func(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error)
	markAsUsedFn           func(ctx context.Context, id int64) (bool, error)
	listPendingFn          func(ctx context.Context) ([]*platform.OperatorInvitationToken, error)
	invalidateByEmailFn    func(ctx context.Context, email string) (int, error)
	extendExpiryFn         func(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error)
	updateDeliveryResultFn func(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	deleteExpiredFn        func(ctx context.Context) (int, error)
	countRecentFn          func(ctx context.Context, createdByID int64, since time.Time) (int, error)
}

func (m *mockInvitationTokenRepo) Create(ctx context.Context, token *platform.OperatorInvitationToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	return nil
}

func (m *mockInvitationTokenRepo) FindByID(ctx context.Context, id int64) (*platform.OperatorInvitationToken, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) FindValidByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if m.findValidByTokenFn != nil {
		return m.findValidByTokenFn(ctx, tokenStr)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
	if m.consumeByTokenFn != nil {
		return m.consumeByTokenFn(ctx, tokenStr)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) MarkAsUsed(ctx context.Context, id int64) (bool, error) {
	if m.markAsUsedFn != nil {
		return m.markAsUsedFn(ctx, id)
	}
	return true, nil
}

func (m *mockInvitationTokenRepo) ListPending(ctx context.Context) ([]*platform.OperatorInvitationToken, error) {
	if m.listPendingFn != nil {
		return m.listPendingFn(ctx)
	}
	return nil, nil
}

func (m *mockInvitationTokenRepo) InvalidateByEmail(ctx context.Context, email string) (int, error) {
	if m.invalidateByEmailFn != nil {
		return m.invalidateByEmailFn(ctx, email)
	}
	return 0, nil
}

func (m *mockInvitationTokenRepo) ExtendExpiry(ctx context.Context, id int64, newExpiresAt time.Time) (bool, error) {
	if m.extendExpiryFn != nil {
		return m.extendExpiryFn(ctx, id, newExpiresAt)
	}
	return true, nil
}

func (m *mockInvitationTokenRepo) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	if m.updateDeliveryResultFn != nil {
		return m.updateDeliveryResultFn(ctx, tokenID, sentAt, emailError, retryCount)
	}
	return nil
}

func (m *mockInvitationTokenRepo) DeleteExpired(ctx context.Context) (int, error) {
	if m.deleteExpiredFn != nil {
		return m.deleteExpiredFn(ctx)
	}
	return 0, nil
}

func (m *mockInvitationTokenRepo) CountRecentByCreatedBy(ctx context.Context, createdByID int64, since time.Time) (int, error) {
	if m.countRecentFn != nil {
		return m.countRecentFn(ctx, createdByID, since)
	}
	return 0, nil
}

// --- Mock for OperatorRepository with Create support ---

type mockOperatorRepoWithCreate struct {
	mockOperatorRepo
	createFn func(ctx context.Context, operator *platform.Operator) error
}

func (m *mockOperatorRepoWithCreate) Create(ctx context.Context, operator *platform.Operator) error {
	if m.createFn != nil {
		return m.createFn(ctx, operator)
	}
	return nil
}

// --- Helper to build service with sqlmock for tx-based tests ---

func newInvitationTestService(
	t *testing.T,
	operatorRepo platform.OperatorRepository,
	auditLogRepo platform.OperatorAuditLogRepository,
	invitationTokenRepo platform.OperatorInvitationTokenRepository,
	bunDB *bun.DB,
) platformSvc.OperatorAuthAndInvitationService {
	t.Helper()
	// Dispatcher is required by InviteOperator and ResendOperatorInvitation guards.
	// Tests that exercise dispatch behavior use newTestServiceWithDispatcher instead.
	dispatcher := email.NewDispatcher(email.NewMockMailer(), slog.Default())
	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:        operatorRepo,
		AuditLogRepo:        auditLogRepo,
		InvitationTokenRepo: invitationTokenRepo,
		DB:                  bunDB,
		Logger:              slog.Default(),
		Dispatcher:          dispatcher,
		DefaultFrom:         email.NewEmail("moto", "no-reply@test.local"),
		FrontendURL:         "https://example.com",
		OperatorFrontendURL: "https://operator.example.com",
		InvitationExpiry:    48 * time.Hour,
	})
	require.NoError(t, err)
	return service
}

func setupSqlMock(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	bunDB := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, bunDB.Close())
		require.NoError(t, sqlDB.Close())
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return bunDB, mock
}

func expectAdminTx(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
}

// =====================================================================
// InviteOperator Tests
// =====================================================================

func TestInviteOperator_Success(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	var createdToken *platform.OperatorInvitationToken
	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			createdToken = token
			token.ID = 100
			return nil
		},
	}

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, email string) (*platform.Operator, error) {
			return nil, nil // No existing operator
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "new@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	require.NotNil(t, createdToken)
	assert.Equal(t, "new@example.com", createdToken.Email)
	assert.Equal(t, int64(42), createdToken.CreatedBy)
	assert.NotEmpty(t, createdToken.Token)
	assert.True(t, createdToken.ExpiresAt.After(time.Now()))
}

func TestInviteOperator_NormalizesEmail(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	var capturedEmail string
	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			capturedEmail = token.Email
			token.ID = 100
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "  NEW@Example.COM  ", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", capturedEmail)
}

func TestInviteOperator_InvalidEmailFormat(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	err := service.InviteOperator(context.Background(), "not-an-email", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var invalidData *platformSvc.InvalidDataError
	assert.True(t, assert.ErrorAs(t, err, &invalidData))
}

func TestInviteOperator_EmailTooLong(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	longEmail := ""
	for i := 0; i < 256; i++ {
		longEmail += "a"
	}
	longEmail += "@example.com"

	err := service.InviteOperator(context.Background(), longEmail, nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var invalidData *platformSvc.InvalidDataError
	assert.True(t, assert.ErrorAs(t, err, &invalidData))
}

func TestInviteOperator_EmailAlreadyOperator(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			op := &platform.Operator{Email: "existing@example.com"}
			op.ID = 99
			return op, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	err := service.InviteOperator(context.Background(), "existing@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var emailExists *platformSvc.OperatorInvitationEmailExistsError
	assert.True(t, assert.ErrorAs(t, err, &emailExists))
}

func TestInviteOperator_InvalidatesPreviousInvitations(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	var invalidatedEmail string
	invitationRepo := &mockInvitationTokenRepo{
		invalidateByEmailFn: func(_ context.Context, email string) (int, error) {
			invalidatedEmail = email
			return 1, nil // One previous invitation invalidated
		},
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			token.ID = 100
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "re-invite@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	assert.Equal(t, "re-invite@example.com", invalidatedEmail)
}

func TestInviteOperator_NilInvitationTokenRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	// Service with nil invitation token repo
	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
		FrontendURL:  "https://example.com",
	})
	require.NoError(t, err)

	err = service.InviteOperator(context.Background(), "test@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invitation_token_repo to be configured")
}

func TestInviteOperator_MissingFrontendURL(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:        &mockOperatorRepo{},
		AuditLogRepo:        &mockAuditLogRepoShared{},
		InvitationTokenRepo: &mockInvitationTokenRepo{},
		DB:                  bunDB,
		Logger:              slog.Default(),
		// FrontendURL intentionally missing
	})
	require.NoError(t, err)

	err = service.InviteOperator(context.Background(), "test@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontend_url to be configured")
}

func TestInviteOperator_InvalidateError(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	invitationRepo := &mockInvitationTokenRepo{
		invalidateByEmailFn: func(_ context.Context, _ string) (int, error) {
			return 0, fmt.Errorf("db error")
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "test@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidate previous invitations")
}

func TestInviteOperator_RateLimit_BlocksAtLimit(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	createCalled := false
	invalidateCalled := false
	invitationRepo := &mockInvitationTokenRepo{
		countRecentFn: func(_ context.Context, createdByID int64, since time.Time) (int, error) {
			assert.Equal(t, int64(42), createdByID)
			// Window should be ~1 hour ago; allow some jitter.
			assert.WithinDuration(t, time.Now().Add(-1*time.Hour), since, 5*time.Second)
			return 5, nil
		},
		invalidateByEmailFn: func(_ context.Context, _ string) (int, error) {
			invalidateCalled = true
			return 0, nil
		},
		createFn: func(_ context.Context, _ *platform.OperatorInvitationToken) error {
			createCalled = true
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "new@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var rateLimit *platformSvc.OperatorInvitationRateLimitError
	assert.True(t, assert.ErrorAs(t, err, &rateLimit))
	assert.False(t, invalidateCalled, "previous invitations must NOT be invalidated when rate limited")
	assert.False(t, createCalled, "token must NOT be created when rate limited")
}

func TestInviteOperator_RateLimit_AllowsJustBelowLimit(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	var createdToken *platform.OperatorInvitationToken
	invitationRepo := &mockInvitationTokenRepo{
		countRecentFn: func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 4, nil // one below the limit
		},
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			createdToken = token
			token.ID = 101
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "new@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotNil(t, createdToken)
	assert.Equal(t, "new@example.com", createdToken.Email)
}

func TestInviteOperator_RateLimit_CountError(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	invitationRepo := &mockInvitationTokenRepo{
		countRecentFn: func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, fmt.Errorf("db error during count")
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "new@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "check invitation rate limit")
	// Must NOT be returned as a user-facing rate limit error.
	var rateLimit *platformSvc.OperatorInvitationRateLimitError
	assert.False(t, errors.As(err, &rateLimit))
}

func TestInviteOperator_RateLimit_LockFailure(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	countCalled := false
	invitationRepo := &mockInvitationTokenRepo{
		countRecentFn: func(_ context.Context, _ int64, _ time.Time) (int, error) {
			countCalled = true
			return 0, nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByIDForUpdateFn: func(_ context.Context, _ int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("lock timeout")
		},
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.InviteOperator(context.Background(), "new@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lock inviter row")
	assert.False(t, countCalled, "rate limit count must not run after lock failure")
}

func TestInviteOperator_AuditLogErrorDoesNotBlock(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			token.ID = 100
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}
	auditLogRepo := &mockAuditLogRepoShared{
		createFn: func(_ context.Context, _ *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit log db error")
		},
	}

	service := newInvitationTestService(t, operatorRepo, auditLogRepo, invitationRepo, bunDB)

	// Should succeed despite audit log failure (fire-and-forget)
	err := service.InviteOperator(context.Background(), "test@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestInviteOperator_WithDisplayName(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	var capturedDisplayName *string
	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			capturedDisplayName = token.DisplayName
			token.ID = 100
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	name := "New Operator"
	err := service.InviteOperator(context.Background(), "test@example.com", &name, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	require.NotNil(t, capturedDisplayName)
	assert.Equal(t, "New Operator", *capturedDisplayName)
}

// =====================================================================
// ValidateOperatorInvitation Tests
// =====================================================================

func TestValidateOperatorInvitation_Success(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	expectedToken := &platform.OperatorInvitationToken{
		Email:     "test@example.com",
		Token:     "abc-123",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	expectedToken.ID = 100

	invitationRepo := &mockInvitationTokenRepo{
		findValidByTokenFn: func(_ context.Context, tokenStr string) (*platform.OperatorInvitationToken, error) {
			assert.Equal(t, "abc-123", tokenStr)
			return expectedToken, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	token, err := service.ValidateOperatorInvitation(context.Background(), "abc-123")
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "test@example.com", token.Email)
}

func TestValidateOperatorInvitation_TokenNotFound(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findValidByTokenFn: func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
			return nil, nil // Not found
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	token, err := service.ValidateOperatorInvitation(context.Background(), "nonexistent")
	require.Error(t, err)
	assert.Nil(t, token)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestValidateOperatorInvitation_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	token, err := service.ValidateOperatorInvitation(context.Background(), "abc-123")
	require.Error(t, err)
	assert.Nil(t, token)
}

func TestValidateOperatorInvitation_RepoError(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findValidByTokenFn: func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
			return nil, fmt.Errorf("db error")
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	token, err := service.ValidateOperatorInvitation(context.Background(), "abc-123")
	require.Error(t, err)
	assert.Nil(t, token)
}

// =====================================================================
// AcceptOperatorInvitation Tests
// =====================================================================

func TestAcceptOperatorInvitation_Success(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	invitationRepo := &mockInvitationTokenRepo{
		consumeByTokenFn: func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "invited@example.com",
				Token:     "abc-123",
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			token.ID = 100
			return token, nil
		},
	}

	var createdOperator *platform.Operator
	operatorRepo := &mockOperatorRepoWithCreate{
		mockOperatorRepo: mockOperatorRepo{
			findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
				return nil, nil // No existing operator
			},
		},
		createFn: func(_ context.Context, op *platform.Operator) error {
			createdOperator = op
			op.ID = 200
			return nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", "New Operator", "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
	require.NotNil(t, operator)

	assert.Equal(t, int64(200), operator.ID)
	assert.Equal(t, "invited@example.com", operator.Email)
	assert.Equal(t, "New Operator", operator.DisplayName)
	assert.True(t, operator.Active)
	require.NotNil(t, createdOperator)
	assert.NotEmpty(t, createdOperator.PasswordHash)
}

func TestAcceptOperatorInvitation_WeakPassword(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", "Name", "weak", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)

	var invalidData *platformSvc.InvalidDataError
	assert.True(t, assert.ErrorAs(t, err, &invalidData))
}

func TestAcceptOperatorInvitation_EmptyDisplayName(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", "", "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)

	var invalidData *platformSvc.InvalidDataError
	assert.True(t, assert.ErrorAs(t, err, &invalidData))
}

func TestAcceptOperatorInvitation_DisplayNameTooLong(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, &mockInvitationTokenRepo{}, bunDB)

	longName := ""
	for i := 0; i < 101; i++ {
		longName += "a"
	}

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", longName, "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)

	var invalidData *platformSvc.InvalidDataError
	assert.True(t, assert.ErrorAs(t, err, &invalidData))
}

func TestAcceptOperatorInvitation_TokenNotFound(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	invitationRepo := &mockInvitationTokenRepo{
		consumeByTokenFn: func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
			return nil, nil // Token not found
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "expired-token", "Name", "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestAcceptOperatorInvitation_EmailAlreadyExists(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectRollback()

	invitationRepo := &mockInvitationTokenRepo{
		consumeByTokenFn: func(_ context.Context, _ string) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{Email: "existing@example.com"}
			token.ID = 100
			return token, nil
		},
	}

	operatorRepo := &mockOperatorRepoWithCreate{
		mockOperatorRepo: mockOperatorRepo{
			findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
				op := &platform.Operator{Email: "existing@example.com"}
				op.ID = 99
				return op, nil // Already exists
			},
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", "Name", "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)

	var emailExists *platformSvc.OperatorInvitationEmailExistsError
	assert.True(t, assert.ErrorAs(t, err, &emailExists))
}

func TestAcceptOperatorInvitation_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	operator, err := service.AcceptOperatorInvitation(context.Background(), "abc-123", "Name", "SecureP@ss1", net.ParseIP("127.0.0.1"))
	require.Error(t, err)
	assert.Nil(t, operator)
}

// =====================================================================
// RevokeOperatorInvitation Tests
// =====================================================================

func TestRevokeOperatorInvitation_Success(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "pending@example.com",
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			token.ID = id
			return token, nil
		},
		markAsUsedFn: func(_ context.Context, id int64) (bool, error) {
			assert.Equal(t, int64(100), id)
			return true, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.RevokeOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestRevokeOperatorInvitation_NotFound(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			return nil, nil // Not found
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.RevokeOperatorInvitation(context.Background(), 999, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestRevokeOperatorInvitation_AlreadyUsed(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	usedAt := time.Now()
	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "used@example.com",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				UsedAt:    &usedAt,
			}
			token.ID = 100
			return token, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.RevokeOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestRevokeOperatorInvitation_Expired(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "expired@example.com",
				ExpiresAt: time.Now().Add(-1 * time.Hour),
			}
			token.ID = 100
			return token, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.RevokeOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestRevokeOperatorInvitation_MarkAsUsedFails(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "pending@example.com",
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			token.ID = 100
			return token, nil
		},
		markAsUsedFn: func(_ context.Context, _ int64) (bool, error) {
			return false, nil // Concurrent consumption
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.RevokeOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestRevokeOperatorInvitation_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.RevokeOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
}

// =====================================================================
// ResendOperatorInvitation Tests
// =====================================================================

func TestResendOperatorInvitation_Success(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "pending@example.com",
				Token:     "abc-123",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				CreatedBy: 42,
			}
			token.ID = id
			return token, nil
		},
		extendExpiryFn: func(_ context.Context, _ int64, _ time.Time) (bool, error) {
			return true, nil
		},
	}

	operatorRepo := &mockOperatorRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Admin"}
			op.ID = 42
			return op, nil
		},
	}

	service := newInvitationTestService(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.ResendOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)
}

func TestResendOperatorInvitation_NotFound(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			return nil, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.ResendOperatorInvitation(context.Background(), 999, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestResendOperatorInvitation_AlreadyUsed(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	usedAt := time.Now()
	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "used@example.com",
				ExpiresAt: time.Now().Add(24 * time.Hour),
				UsedAt:    &usedAt,
			}
			token.ID = 100
			return token, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.ResendOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestResendOperatorInvitation_ExtendExpiryFails(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, _ int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "pending@example.com",
				ExpiresAt: time.Now().Add(24 * time.Hour),
			}
			token.ID = 100
			return token, nil
		},
		extendExpiryFn: func(_ context.Context, _ int64, _ time.Time) (bool, error) {
			return false, nil // Concurrent consumption
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	err := service.ResendOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)

	var notFound *platformSvc.OperatorInvitationNotFoundError
	assert.True(t, assert.ErrorAs(t, err, &notFound))
}

func TestResendOperatorInvitation_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	err = service.ResendOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.Error(t, err)
}

// =====================================================================
// ListPendingOperatorInvitations Tests
// =====================================================================

func TestListPendingOperatorInvitations_Success(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	expected := []*platform.OperatorInvitationToken{
		{Email: "a@example.com"},
		{Email: "b@example.com"},
	}

	invitationRepo := &mockInvitationTokenRepo{
		listPendingFn: func(_ context.Context) ([]*platform.OperatorInvitationToken, error) {
			return expected, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	result, err := service.ListPendingOperatorInvitations(context.Background())
	require.NoError(t, err)
	assert.Len(t, result, 2)
}

func TestListPendingOperatorInvitations_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	result, err := service.ListPendingOperatorInvitations(context.Background())
	require.NoError(t, err) // Should return nil, nil - not error
	assert.Nil(t, result)
}

// =====================================================================
// CleanupExpiredOperatorInvitations Tests
// =====================================================================

func TestCleanupExpiredOperatorInvitations_Success(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		deleteExpiredFn: func(_ context.Context) (int, error) {
			return 5, nil
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	count, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestCleanupExpiredOperatorInvitations_NilRepo(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	service, err := newTestOperatorAuthService(t, platformSvc.OperatorAuthServiceConfig{
		OperatorRepo: &mockOperatorRepo{},
		AuditLogRepo: &mockAuditLogRepoShared{},
		DB:           bunDB,
		Logger:       slog.Default(),
	})
	require.NoError(t, err)

	count, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.NoError(t, err) // Should return 0, nil
	assert.Equal(t, 0, count)
}

func TestCleanupExpiredOperatorInvitations_RepoError(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	invitationRepo := &mockInvitationTokenRepo{
		deleteExpiredFn: func(_ context.Context) (int, error) {
			return 0, fmt.Errorf("db error")
		},
	}

	service := newInvitationTestService(t, &mockOperatorRepo{}, &mockAuditLogRepoShared{}, invitationRepo, bunDB)

	count, err := service.CleanupExpiredOperatorInvitations(context.Background())
	require.Error(t, err)
	assert.Equal(t, 0, count)
}

// =====================================================================
// Error Type Tests
// =====================================================================

func TestOperatorInvitationNotFoundError_Message(t *testing.T) {
	t.Parallel()

	err := &platformSvc.OperatorInvitationNotFoundError{}
	assert.Contains(t, err.Error(), "not found")
}

func TestOperatorInvitationEmailExistsError_Message(t *testing.T) {
	t.Parallel()

	err := &platformSvc.OperatorInvitationEmailExistsError{}
	assert.Contains(t, err.Error(), "already exists")
}
