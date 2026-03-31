package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock implementations for internal tests ---

type mockEmailChangeTokenRepo struct {
	updateDeliveryResultFn    func(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error
	invalidateExpiredTokensFn func(ctx context.Context) (int, error)
	deleteStaleTokensFn       func(ctx context.Context) (int, error)
	countRecentByOperatorIDFn func(ctx context.Context, operatorID int64, since time.Time) (int, error)
	invalidateByOperatorIDFn  func(ctx context.Context, operatorID int64) error
	createFn                  func(ctx context.Context, token *platform.OperatorEmailChangeToken) error
	consumeByTokenFn          func(ctx context.Context, tokenStr string) (*platform.OperatorEmailChangeToken, error)
}

func (m *mockEmailChangeTokenRepo) Create(ctx context.Context, token *platform.OperatorEmailChangeToken) error {
	if m.createFn != nil {
		return m.createFn(ctx, token)
	}
	return nil
}

func (m *mockEmailChangeTokenRepo) ConsumeByToken(ctx context.Context, tokenStr string) (*platform.OperatorEmailChangeToken, error) {
	if m.consumeByTokenFn != nil {
		return m.consumeByTokenFn(ctx, tokenStr)
	}
	return nil, nil
}

func (m *mockEmailChangeTokenRepo) InvalidateByOperatorID(ctx context.Context, operatorID int64) error {
	if m.invalidateByOperatorIDFn != nil {
		return m.invalidateByOperatorIDFn(ctx, operatorID)
	}
	return nil
}

func (m *mockEmailChangeTokenRepo) UpdateDeliveryResult(ctx context.Context, tokenID int64, sentAt *time.Time, emailError *string, retryCount int) error {
	if m.updateDeliveryResultFn != nil {
		return m.updateDeliveryResultFn(ctx, tokenID, sentAt, emailError, retryCount)
	}
	return nil
}

func (m *mockEmailChangeTokenRepo) CountRecentByOperatorID(ctx context.Context, operatorID int64, since time.Time) (int, error) {
	if m.countRecentByOperatorIDFn != nil {
		return m.countRecentByOperatorIDFn(ctx, operatorID, since)
	}
	return 0, nil
}

func (m *mockEmailChangeTokenRepo) InvalidateExpiredTokens(ctx context.Context) (int, error) {
	if m.invalidateExpiredTokensFn != nil {
		return m.invalidateExpiredTokensFn(ctx)
	}
	return 0, nil
}

func (m *mockEmailChangeTokenRepo) DeleteStaleTokens(ctx context.Context) (int, error) {
	if m.deleteStaleTokensFn != nil {
		return m.deleteStaleTokensFn(ctx)
	}
	return 0, nil
}

// =============================================================================
// persistEmailChangeDelivery tests
// =============================================================================

func TestPersistEmailChangeDelivery_Success(t *testing.T) {
	var capturedSentAt *time.Time
	var capturedErr *string
	var capturedRetry int

	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, _ int64, sentAt *time.Time, emailError *string, retryCount int) error {
			capturedSentAt = sentAt
			capturedErr = emailError
			capturedRetry = retryCount
			return nil
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	sentTime := time.Now().Truncate(time.Second)
	result := email.DeliveryResult{
		Status: email.DeliveryStatusSent,
		SentAt: sentTime,
		Final:  true,
	}
	meta := email.DeliveryMetadata{ReferenceID: 42, Recipient: "test@example.com"}

	svc.persistEmailChangeDelivery(context.Background(), meta, 0, result)

	require.NotNil(t, capturedSentAt)
	assert.WithinDuration(t, sentTime, *capturedSentAt, time.Second)
	assert.Nil(t, capturedErr)
	assert.Equal(t, 0, capturedRetry) // baseRetry(0) + result.Attempt(0)
}

func TestPersistEmailChangeDelivery_Failure(t *testing.T) {
	var capturedSentAt *time.Time
	var capturedErr *string
	var capturedRetry int

	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, _ int64, sentAt *time.Time, emailError *string, retryCount int) error {
			capturedSentAt = sentAt
			capturedErr = emailError
			capturedRetry = retryCount
			return nil
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	result := email.DeliveryResult{
		Status:  email.DeliveryStatusFailed,
		Err:     fmt.Errorf("SMTP timeout"),
		Attempt: 2,
		Final:   true,
	}
	meta := email.DeliveryMetadata{ReferenceID: 42, Recipient: "test@example.com"}

	svc.persistEmailChangeDelivery(context.Background(), meta, 1, result)

	assert.Nil(t, capturedSentAt)
	require.NotNil(t, capturedErr)
	assert.Equal(t, "SMTP timeout", *capturedErr)
	assert.Equal(t, 3, capturedRetry) // baseRetry(1) + result.Attempt(2)
}

func TestPersistEmailChangeDelivery_UpdateError(t *testing.T) {
	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, _ int64, _ *time.Time, _ *string, _ int) error {
			return fmt.Errorf("database unavailable")
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	// Should not panic — just logs the error
	result := email.DeliveryResult{Status: email.DeliveryStatusSent, SentAt: time.Now()}
	meta := email.DeliveryMetadata{ReferenceID: 42}
	svc.persistEmailChangeDelivery(context.Background(), meta, 0, result)
}

func TestPersistEmailChangeDelivery_PendingStatus(t *testing.T) {
	var capturedSentAt *time.Time
	var capturedErr *string

	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, _ int64, sentAt *time.Time, emailError *string, _ int) error {
			capturedSentAt = sentAt
			capturedErr = emailError
			return nil
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	// Neither sent nor error — pending/retrying status with nil error
	result := email.DeliveryResult{
		Status:  email.DeliveryStatusPending,
		Attempt: 1,
		Final:   false,
	}
	meta := email.DeliveryMetadata{ReferenceID: 42}
	svc.persistEmailChangeDelivery(context.Background(), meta, 0, result)

	assert.Nil(t, capturedSentAt, "sentAt should be nil for non-sent status")
	assert.Nil(t, capturedErr, "error should be nil when result.Err is nil")
}

// =============================================================================
// Dispatch nil-dispatcher guard tests
// =============================================================================

func TestDispatchVerificationEmail_NilDispatcher(t *testing.T) {
	svc := &operatorAuthService{
		dispatcher: nil,
		logger:     slog.Default(),
	}

	token := &platform.OperatorEmailChangeToken{OperatorID: 42, Token: "test-token"}
	// Should not panic
	svc.dispatchVerificationEmail(context.Background(), token, "new@example.com")
}

func TestDispatchNotificationEmail_NilDispatcher(t *testing.T) {
	svc := &operatorAuthService{
		dispatcher: nil,
		logger:     slog.Default(),
	}

	op := &platform.Operator{Email: "old@example.com", DisplayName: "Test"}
	// Should not panic
	svc.dispatchNotificationEmail(context.Background(), op, "n***@example.com")
}

func TestDispatchChangeConfirmedEmail_NilDispatcher(t *testing.T) {
	svc := &operatorAuthService{
		dispatcher: nil,
		logger:     slog.Default(),
	}

	// Should not panic
	svc.dispatchChangeConfirmedEmail(context.Background(), "old@example.com", "Test")
}

// =============================================================================
// CleanupExpiredEmailChangeTokens tests
// =============================================================================

func TestCleanupExpiredEmailChangeTokens_NilRepo(t *testing.T) {
	svc := &operatorAuthService{
		emailChangeTokenRepo: nil,
		logger:               slog.Default(),
	}

	count, err := svc.CleanupExpiredEmailChangeTokens(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCleanupExpiredEmailChangeTokens_InvalidateError(t *testing.T) {
	repo := &mockEmailChangeTokenRepo{
		invalidateExpiredTokensFn: func(_ context.Context) (int, error) {
			return 0, fmt.Errorf("database error")
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	_, err := svc.CleanupExpiredEmailChangeTokens(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidate expired tokens")
}

func TestCleanupExpiredEmailChangeTokens_Success(t *testing.T) {
	repo := &mockEmailChangeTokenRepo{
		invalidateExpiredTokensFn: func(_ context.Context) (int, error) {
			return 3, nil
		},
		deleteStaleTokensFn: func(_ context.Context) (int, error) {
			return 5, nil
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	count, err := svc.CleanupExpiredEmailChangeTokens(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 5, count)
}

func TestCleanupExpiredEmailChangeTokens_NoInvalidated(t *testing.T) {
	repo := &mockEmailChangeTokenRepo{
		invalidateExpiredTokensFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
		deleteStaleTokensFn: func(_ context.Context) (int, error) {
			return 0, nil
		},
	}

	svc := &operatorAuthService{
		emailChangeTokenRepo: repo,
		logger:               slog.Default(),
	}

	count, err := svc.CleanupExpiredEmailChangeTokens(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// InitiateEmailChange precondition tests
// =============================================================================

func TestInitiateEmailChange_MissingFrontendURL(t *testing.T) {
	svc := &operatorAuthService{
		frontendURL:          "",
		emailChangeTokenRepo: &mockEmailChangeTokenRepo{},
		dispatcher:           &email.Dispatcher{},
		logger:               slog.Default(),
	}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontend_url")
}

func TestInitiateEmailChange_MissingTokenRepo(t *testing.T) {
	svc := &operatorAuthService{
		frontendURL:          "https://example.com",
		emailChangeTokenRepo: nil,
		dispatcher:           &email.Dispatcher{},
		logger:               slog.Default(),
	}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email_change_token_repo")
}

func TestInitiateEmailChange_MissingDispatcher(t *testing.T) {
	svc := &operatorAuthService{
		frontendURL:          "https://example.com",
		emailChangeTokenRepo: &mockEmailChangeTokenRepo{},
		dispatcher:           nil,
		logger:               slog.Default(),
	}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email dispatcher")
}

func TestConfirmEmailChange_MissingTokenRepo(t *testing.T) {
	svc := &operatorAuthService{
		emailChangeTokenRepo: nil,
		logger:               slog.Default(),
	}

	_, err := svc.ConfirmEmailChange(context.Background(), "some-token", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email_change_token_repo")
}

// =============================================================================
// isUniqueViolation tests
// =============================================================================

func TestIsUniqueViolation_NilError(t *testing.T) {
	assert.False(t, isUniqueViolation(nil))
}

func TestIsUniqueViolation_NonPgError(t *testing.T) {
	assert.False(t, isUniqueViolation(errors.New("some generic error")))
}

// =============================================================================
// getLogger nil-safety test
// =============================================================================

func TestGetLogger_NilLogger(t *testing.T) {
	svc := &operatorAuthService{logger: nil}
	logger := svc.getLogger()
	assert.NotNil(t, logger, "should return slog.Default() when logger is nil")
}

func TestGetLogger_WithLogger(t *testing.T) {
	customLogger := slog.Default().With("test", true)
	svc := &operatorAuthService{logger: customLogger}
	assert.Equal(t, customLogger, svc.getLogger())
}
