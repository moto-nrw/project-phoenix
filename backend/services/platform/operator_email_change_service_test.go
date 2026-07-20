package platform

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/email"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: nil, Logger: slog.Default()}}

	token := &platform.OperatorEmailChangeToken{OperatorID: 42, Token: "test-token"}
	// Should not panic
	svc.dispatchVerificationEmail(context.Background(), token, "new@example.com")
}

func TestDispatchNotificationEmail_NilDispatcher(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: nil, Logger: slog.Default()}}

	op := &platform.Operator{Email: "old@example.com", DisplayName: "Test"}
	// Should not panic
	svc.dispatchNotificationEmail(context.Background(), op, "n***@example.com")
}

func TestDispatchChangeConfirmedEmail_NilDispatcher(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: nil, Logger: slog.Default()}}

	// Should not panic
	svc.dispatchChangeConfirmedEmail(context.Background(), "old@example.com", "Test")
}

// =============================================================================
// CleanupExpiredEmailChangeTokens tests
// =============================================================================

func TestCleanupExpiredEmailChangeTokens_NilRepo(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: nil, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

	count, err := svc.CleanupExpiredEmailChangeTokens(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// =============================================================================
// InitiateEmailChange precondition tests
// =============================================================================

func TestInitiateEmailChange_MissingFrontendURL(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{FrontendURL: "", EmailChangeTokenRepo: &mockEmailChangeTokenRepo{}, Dispatcher: &email.Dispatcher{}, Logger: slog.Default()}}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "frontend_url")
}

func TestInitiateEmailChange_MissingTokenRepo(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{FrontendURL: "https://example.com", EmailChangeTokenRepo: nil, Dispatcher: &email.Dispatcher{}, Logger: slog.Default()}}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email_change_token_repo")
}

func TestInitiateEmailChange_MissingDispatcher(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{FrontendURL: "https://example.com", EmailChangeTokenRepo: &mockEmailChangeTokenRepo{}, Dispatcher: nil, Logger: slog.Default()}}

	err := svc.InitiateEmailChange(context.Background(), 1, "new@example.com", "pass", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email dispatcher")
}

func TestConfirmEmailChange_MissingTokenRepo(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: nil, Logger: slog.Default()}}

	_, err := svc.ConfirmEmailChange(context.Background(), "some-token", net.IPv4(127, 0, 0, 1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email_change_token_repo")
}

// =============================================================================
// isUniqueViolation tests
// =============================================================================

func TestIsUniqueViolation_NilError(t *testing.T) {
	assert.False(t, modelBase.IsUniqueViolation(nil))
}

func TestIsUniqueViolation_NonPgError(t *testing.T) {
	assert.False(t, modelBase.IsUniqueViolation(errors.New("some generic error")))
}

// =============================================================================
// getLogger nil-safety test
// =============================================================================

func TestGetLogger_NilLogger(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Logger: nil}}
	logger := svc.getLogger()
	assert.NotNil(t, logger, "should return slog.Default() when logger is nil")
}

func TestGetLogger_WithLogger(t *testing.T) {
	customLogger := slog.Default().With("test", true)
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Logger: customLogger}}
	assert.Equal(t, customLogger, svc.getLogger())
}

func TestLogOperatorRefreshDecision_NilLoggerDoesNotPanic(t *testing.T) {
	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Logger: nil}}
	assert.NotPanics(t, func() {
		svc.logOperatorRefreshDecision("expired", 42, 3)
	})
}

// =============================================================================
// Dispatch function tests with real Dispatcher + mock mailer
// =============================================================================

// waitForDispatch blocks until the async dispatcher goroutine signals ch,
// failing the test after a generous timeout. Replaces fixed sleeps, which
// both wasted wall time and read shared state without a happens-before
// edge.
func waitForDispatch(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for %s", what)
	}
}

// trySend signals ch without blocking; signals beyond the buffer are
// dropped, which is fine for a "dispatched at least once" marker.
func trySend(ch chan<- struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}

func TestDispatchVerificationEmail_MessageContent(t *testing.T) {
	var captured email.Message
	sent := make(chan struct{}, 1)
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			captured = m
			trySend(sent)
			return nil
		},
	}

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	frontendURL := "https://app.example.com"
	defaultFrom := email.NewEmail("moto", "noreply@example.com")
	expiry := 30 * time.Minute

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: dispatcher, FrontendURL: frontendURL, DefaultFrom: defaultFrom, EmailChangeExpiry: expiry, EmailChangeTokenRepo: &mockEmailChangeTokenRepo{}, Logger: slog.Default()}}

	token := &platform.OperatorEmailChangeToken{
		OperatorID: 42,
		Token:      "test-verify-token",
	}
	newEmail := "new@example.com"

	svc.dispatchVerificationEmail(context.Background(), token, newEmail)

	waitForDispatch(t, sent, "verification email dispatch")

	assert.True(t, mailer.SendInvoked.Load(), "mailer should have been invoked")
	assert.Equal(t, newEmail, captured.To.Address)
	assert.Equal(t, "", captured.To.Name)
	assert.Equal(t, "E-Mail-Adresse bestätigen", captured.Subject)
	assert.Equal(t, "operator-email-change-verify.html", captured.Template)

	content, ok := captured.Content.(map[string]any)
	require.True(t, ok, "content should be map[string]any")

	expectedVerifyURL := frontendURL + "/operator/email-confirm?token=" + token.Token
	assert.Equal(t, expectedVerifyURL, content["VerifyURL"])
	assert.Equal(t, int(expiry.Minutes()), content["ExpiryMinutes"])

	expectedLogoURL := frontendURL + "/images/moto-logo-mit-schriftzug.png"
	assert.Equal(t, expectedLogoURL, content["LogoURL"])
}

func TestDispatchNotificationEmail_MessageContent(t *testing.T) {
	var captured email.Message
	sent := make(chan struct{}, 1)
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			captured = m
			trySend(sent)
			return nil
		},
	}

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	frontendURL := "https://app.example.com"
	defaultFrom := email.NewEmail("moto", "noreply@example.com")

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: dispatcher, FrontendURL: frontendURL, DefaultFrom: defaultFrom, Logger: slog.Default()}}

	operator := &platform.Operator{
		Email:       "operator@example.com",
		DisplayName: "Test Admin",
	}
	maskedNewEmail := "n***@example.com"

	svc.dispatchNotificationEmail(context.Background(), operator, maskedNewEmail)

	waitForDispatch(t, sent, "notification email dispatch")

	assert.True(t, mailer.SendInvoked.Load(), "mailer should have been invoked")
	assert.Equal(t, operator.Email, captured.To.Address)
	assert.Equal(t, operator.DisplayName, captured.To.Name)
	assert.Equal(t, "E-Mail-Änderung angefordert", captured.Subject)
	assert.Equal(t, "operator-email-change-notify.html", captured.Template)

	content, ok := captured.Content.(map[string]any)
	require.True(t, ok, "content should be map[string]any")

	assert.Equal(t, maskedNewEmail, content["MaskedNewEmail"])
	assert.Equal(t, operator.DisplayName, content["DisplayName"])

	expectedLogoURL := frontendURL + "/images/moto-logo-mit-schriftzug.png"
	assert.Equal(t, expectedLogoURL, content["LogoURL"])
}

func TestDispatchChangeConfirmedEmail_MessageContent(t *testing.T) {
	var captured email.Message
	sent := make(chan struct{}, 1)
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			captured = m
			trySend(sent)
			return nil
		},
	}

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	frontendURL := "https://app.example.com"
	defaultFrom := email.NewEmail("moto", "noreply@example.com")

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: dispatcher, FrontendURL: frontendURL, DefaultFrom: defaultFrom, Logger: slog.Default()}}

	oldEmail := "old@example.com"
	displayName := "Test Admin"

	svc.dispatchChangeConfirmedEmail(context.Background(), oldEmail, displayName)

	waitForDispatch(t, sent, "change-confirmed email dispatch")

	assert.True(t, mailer.SendInvoked.Load(), "mailer should have been invoked")
	assert.Equal(t, oldEmail, captured.To.Address)
	assert.Equal(t, displayName, captured.To.Name)
	assert.Equal(t, "E-Mail-Adresse wurde geändert", captured.Subject)
	assert.Equal(t, "operator-email-change-confirmed.html", captured.Template)

	content, ok := captured.Content.(map[string]any)
	require.True(t, ok, "content should be map[string]any")

	expectedLogoURL := frontendURL + "/images/moto-logo-mit-schriftzug.png"
	assert.Equal(t, expectedLogoURL, content["LogoURL"])
	assert.Equal(t, displayName, content["DisplayName"])
}

func TestDispatchVerificationEmail_CallbackWiring(t *testing.T) {
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			return nil
		},
	}

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	var updateCalled bool
	var capturedTokenID int64
	persisted := make(chan struct{}, 1)
	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, tokenID int64, _ *time.Time, _ *string, _ int) error {
			updateCalled = true
			capturedTokenID = tokenID
			trySend(persisted)
			return nil
		},
	}

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: dispatcher, FrontendURL: "https://app.example.com", DefaultFrom: email.NewEmail("moto", "noreply@example.com"), EmailChangeExpiry: 30 * time.Minute, EmailChangeTokenRepo: repo, Logger: slog.Default()}}

	token := &platform.OperatorEmailChangeToken{
		OperatorID: 42,
		Token:      "callback-test-token",
	}
	token.ID = 99

	svc.dispatchVerificationEmail(context.Background(), token, "new@example.com")

	waitForDispatch(t, persisted, "delivery-result persistence callback")

	assert.True(t, updateCalled, "persistEmailChangeDelivery should have been called via callback")
	assert.Equal(t, int64(99), capturedTokenID)
}

func TestDispatchNotificationEmail_NoCallback(t *testing.T) {
	// The notification dispatch uses nil Callback — verify that no delivery
	// persistence is attempted (fire-and-forget).
	var updateCalled bool
	repo := &mockEmailChangeTokenRepo{
		updateDeliveryResultFn: func(_ context.Context, _ int64, _ *time.Time, _ *string, _ int) error {
			updateCalled = true
			return nil
		},
	}

	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			return nil
		},
	}

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{Dispatcher: dispatcher, FrontendURL: "https://app.example.com", DefaultFrom: email.NewEmail("moto", "noreply@example.com"), EmailChangeTokenRepo: repo, Logger: slog.Default()}}

	operator := &platform.Operator{
		Email:       "operator@example.com",
		DisplayName: "Test Admin",
	}

	svc.dispatchNotificationEmail(context.Background(), operator, "n***@example.com")

	time.Sleep(200 * time.Millisecond)

	assert.False(t, updateCalled, "notification email should not trigger delivery persistence (nil callback)")
}

// =============================================================================
// persistEmailChangeDelivery — final failure logging path
// =============================================================================

func TestPersistEmailChangeDelivery_FinalFailure(t *testing.T) {
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

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{EmailChangeTokenRepo: repo, Logger: slog.Default()}}

	result := email.DeliveryResult{
		Status:  email.DeliveryStatusFailed,
		Err:     fmt.Errorf("permanent SMTP failure: mailbox not found"),
		Attempt: 3,
		Final:   true,
	}
	meta := email.DeliveryMetadata{ReferenceID: 77, Recipient: "bad@example.com"}

	// Should not panic — the final failure branch logs at Error level
	svc.persistEmailChangeDelivery(context.Background(), meta, 0, result)

	assert.Nil(t, capturedSentAt, "sentAt should be nil for failed delivery")
	require.NotNil(t, capturedErr, "error text should be captured")
	assert.Equal(t, "permanent SMTP failure: mailbox not found", *capturedErr)
	assert.Equal(t, 3, capturedRetry) // baseRetry(0) + result.Attempt(3)
}

// =============================================================================
// Internal mock types for InitiateEmailChange / ConfirmEmailChange tests
// =============================================================================

// mockOperatorRepoInternal implements platform.OperatorRepository for internal
// tests that need sqlmock-backed transactions.
type mockOperatorRepoInternal struct {
	findByIDFn          func(ctx context.Context, id int64) (*platform.Operator, error)
	findByIDForUpdateFn func(ctx context.Context, id int64) (*platform.Operator, error)
	findByEmailFn       func(ctx context.Context, email string) (*platform.Operator, error)
	updateFn            func(ctx context.Context, operator *platform.Operator) error
	createFn            func(ctx context.Context, operator *platform.Operator) error
	deleteFn            func(ctx context.Context, id int64) error
	listFn              func(ctx context.Context) ([]*platform.Operator, error)
	updateLastLoginFn   func(ctx context.Context, id int64) error
}

func (m *mockOperatorRepoInternal) FindByID(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOperatorRepoInternal) FindByIDForUpdate(ctx context.Context, id int64) (*platform.Operator, error) {
	if m.findByIDForUpdateFn != nil {
		return m.findByIDForUpdateFn(ctx, id)
	}
	return nil, nil
}

func (m *mockOperatorRepoInternal) FindByEmail(ctx context.Context, emailAddr string) (*platform.Operator, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, emailAddr)
	}
	return nil, nil
}

func (m *mockOperatorRepoInternal) Update(ctx context.Context, operator *platform.Operator) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, operator)
	}
	return nil
}

func (m *mockOperatorRepoInternal) Create(ctx context.Context, operator *platform.Operator) error {
	if m.createFn != nil {
		return m.createFn(ctx, operator)
	}
	return nil
}

func (m *mockOperatorRepoInternal) Delete(ctx context.Context, id int64) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockOperatorRepoInternal) List(ctx context.Context) ([]*platform.Operator, error) {
	if m.listFn != nil {
		return m.listFn(ctx)
	}
	return nil, nil
}

func (m *mockOperatorRepoInternal) UpdateLastLogin(ctx context.Context, id int64) error {
	if m.updateLastLoginFn != nil {
		return m.updateLastLoginFn(ctx, id)
	}
	return nil
}

// IncrementMFAAttempts / ResetMFAAttempts are part of OperatorRepository as
// of #1430 review item #6 (atomic MFA lockout counter). Email-change tests
// don't exercise the MFA flow, so the mock panics if any test reaches
// them — that's a wiring red flag, not a no-op.
func (m *mockOperatorRepoInternal) IncrementMFAAttempts(_ context.Context, _ int64, _ int, _ time.Duration) (platform.OperatorMFAAttemptResult, error) {
	panic("IncrementMFAAttempts not implemented in mockOperatorRepoInternal")
}

func (m *mockOperatorRepoInternal) ResetMFAAttempts(_ context.Context, _ int64) error {
	panic("ResetMFAAttempts not implemented in mockOperatorRepoInternal")
}

// mockAuditLogRepoInternal implements platform.OperatorAuditLogRepository for
// internal tests.
type mockAuditLogRepoInternal struct {
	createFn             func(ctx context.Context, entry *platform.OperatorAuditLog) error
	findByOperatorIDFn   func(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error)
	findByResourceTypeFn func(ctx context.Context, resourceType string, limit int) ([]*platform.OperatorAuditLog, error)
	findByDateRangeFn    func(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error)
}

func (m *mockAuditLogRepoInternal) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	if m.createFn != nil {
		return m.createFn(ctx, entry)
	}
	return nil
}

func (m *mockAuditLogRepoInternal) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*platform.OperatorAuditLog, error) {
	if m.findByOperatorIDFn != nil {
		return m.findByOperatorIDFn(ctx, operatorID, limit)
	}
	return nil, nil
}

func (m *mockAuditLogRepoInternal) FindByResourceType(ctx context.Context, resourceType string, limit int) ([]*platform.OperatorAuditLog, error) {
	if m.findByResourceTypeFn != nil {
		return m.findByResourceTypeFn(ctx, resourceType, limit)
	}
	return nil, nil
}

func (m *mockAuditLogRepoInternal) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	if m.findByDateRangeFn != nil {
		return m.findByDateRangeFn(ctx, start, end, limit)
	}
	return nil, nil
}

// =============================================================================
// emailChangeTestSetup — helper struct and factory for sqlmock-backed tests
// =============================================================================

// emailChangeTestSetup holds all mocks and references for an email change test.
type emailChangeTestSetup struct {
	OperatorRepo *mockOperatorRepoInternal
	AuditLogRepo *mockAuditLogRepoInternal
	TokenRepo    *mockEmailChangeTokenRepo
	Mailer       *email.MockMailer
	Dispatcher   *email.Dispatcher
}

// newEmailChangeTestService creates a fully wired operatorAuthService backed by
// sqlmock. Callers can customise the setup via option functions applied before
// the service is constructed.
func newEmailChangeTestService(t *testing.T, opts ...func(*emailChangeTestSetup)) (*operatorAuthService, sqlmock.Sqlmock, *emailChangeTestSetup) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	bunDB := bun.NewDB(sqlDB, pgdialect.New())

	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error { return nil },
	}
	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{0})

	setup := &emailChangeTestSetup{
		OperatorRepo: &mockOperatorRepoInternal{},
		AuditLogRepo: &mockAuditLogRepoInternal{},
		TokenRepo:    &mockEmailChangeTokenRepo{},
		Mailer:       mailer,
		Dispatcher:   dispatcher,
	}

	for _, opt := range opts {
		opt(setup)
	}

	svc := &operatorAuthService{OperatorAuthServiceConfig: OperatorAuthServiceConfig{OperatorRepo: setup.OperatorRepo, AuditLogRepo: setup.AuditLogRepo, EmailChangeTokenRepo: setup.TokenRepo, Dispatcher: setup.Dispatcher, DB: bunDB, FrontendURL: "https://app.example.com", DefaultFrom: email.NewEmail("moto", "noreply@example.com"), EmailChangeExpiry: 30 * time.Minute, Logger: slog.Default()}}

	return svc, mock, setup
}

// fastHashPassword creates an Argon2id hash with minimal cost parameters
// suitable for unit tests.
func fastHashPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := userpass.HashPassword(password, &userpass.PasswordParams{
		Memory:      1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  16,
		KeyLength:   32,
	})
	require.NoError(t, err)
	return hash
}

// testOperator returns a minimal active Operator for testing.
func testOperator(id int64, emailAddr, passwordHash string) *platform.Operator {
	op := &platform.Operator{
		Email:        emailAddr,
		DisplayName:  "Test Operator",
		PasswordHash: passwordHash,
		Active:       true,
	}
	op.ID = id
	return op
}

// =============================================================================
// InitiateEmailChange — error path tests (sqlmock-backed)
// =============================================================================

// TestInitiateEmailChange_LockFailure verifies that when FindByIDForUpdate
// returns an error inside the transaction, the service propagates
// "failed to lock operator row" and the transaction rolls back.
func TestInitiateEmailChange_LockFailure(t *testing.T) {
	hash := fastHashPassword(t, "test-password")
	op := testOperator(42, "current@example.com", hash)

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("database lock timeout")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to lock operator row")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateEmailChange_RateLimitRepoError verifies that a repository error
// from CountRecentByOperatorID propagates as "failed to check rate limit".
func TestInitiateEmailChange_RateLimitRepoError(t *testing.T) {
	hash := fastHashPassword(t, "test-password")
	op := testOperator(42, "current@example.com", hash)

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.TokenRepo.countRecentByOperatorIDFn = func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, fmt.Errorf("count query failed")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check rate limit")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateEmailChange_TOCTOURace verifies that a concurrent password change
// detected inside the transaction (TOCTOU guard) returns PasswordMismatchError.
// FindByID is called twice: once outside the tx (returns hash-A), once inside
// the tx (returns hash-B, simulating a concurrent ChangePassword commit).
func TestInitiateEmailChange_TOCTOURace(t *testing.T) {
	hashA := fastHashPassword(t, "test-password")
	hashB := fastHashPassword(t, "other-password")
	opA := testOperator(42, "current@example.com", hashA)
	opB := testOperator(42, "current@example.com", hashB)

	var mu sync.Mutex
	findByIDCallCount := 0

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			mu.Lock()
			findByIDCallCount++
			call := findByIDCallCount
			mu.Unlock()
			if call == 1 {
				return opA, nil // outside tx: original hash
			}
			return opB, nil // inside tx: different hash (concurrent password change)
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return opA, nil // lock acquired
		}
		s.TokenRepo.countRecentByOperatorIDFn = func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, nil // within rate limit
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	var mismatch *PasswordMismatchError
	assert.ErrorAs(t, err, &mismatch, "expected PasswordMismatchError for TOCTOU race")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateEmailChange_EmailTaken_SilentSuccess verifies the anti-enumeration
// behaviour: when the new email is already in use, the service returns nil
// without creating a token or dispatching any email.
func TestInitiateEmailChange_EmailTaken_SilentSuccess(t *testing.T) {
	hash := fastHashPassword(t, "test-password")
	op := testOperator(42, "current@example.com", hash)
	existingOp := testOperator(99, "new@example.com", "irrelevant-hash")

	tokenCreateCalled := false
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error { return nil },
	}

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.Mailer = mailer
		s.Dispatcher = email.NewDispatcher(mailer, slog.Default())
		s.Dispatcher.SetDefaults(1, []time.Duration{0})
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.TokenRepo.countRecentByOperatorIDFn = func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return existingOp, nil // email already taken
		}
		s.TokenRepo.createFn = func(_ context.Context, _ *platform.OperatorEmailChangeToken) error {
			tokenCreateCalled = true
			return nil
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.NoError(t, err, "should return nil for anti-enumeration")
	assert.False(t, tokenCreateCalled, "token should NOT be created when email is taken")
	// Wait briefly for any async goroutine that should NOT fire
	time.Sleep(100 * time.Millisecond)
	assert.False(t, mailer.SendInvoked.Load(), "mailer should NOT be invoked when email is taken")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateEmailChange_InvalidateError verifies that an error from
// InvalidateByOperatorID is propagated.
func TestInitiateEmailChange_InvalidateError(t *testing.T) {
	hash := fastHashPassword(t, "test-password")
	op := testOperator(42, "current@example.com", hash)

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.TokenRepo.countRecentByOperatorIDFn = func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return nil, nil // email not taken
		}
		s.TokenRepo.invalidateByOperatorIDFn = func(_ context.Context, _ int64) error {
			return fmt.Errorf("invalidation failed")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalidation failed")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestInitiateEmailChange_AuditLogFailure_DoesNotBlockEmails verifies that an
// audit log write failure does not prevent the email from being dispatched.
// The audit log is fire-and-forget by design.
func TestInitiateEmailChange_AuditLogFailure_DoesNotBlockEmails(t *testing.T) {
	hash := fastHashPassword(t, "test-password")
	op := testOperator(42, "current@example.com", hash)

	sent := make(chan struct{}, 1)
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			trySend(sent)
			return nil
		},
	}

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.Mailer = mailer
		s.Dispatcher = email.NewDispatcher(mailer, slog.Default())
		s.Dispatcher.SetDefaults(1, []time.Duration{0})
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.findByIDForUpdateFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.TokenRepo.countRecentByOperatorIDFn = func(_ context.Context, _ int64, _ time.Time) (int, error) {
			return 0, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return nil, nil
		}
		s.TokenRepo.invalidateByOperatorIDFn = func(_ context.Context, _ int64) error {
			return nil
		}
		s.TokenRepo.createFn = func(_ context.Context, token *platform.OperatorEmailChangeToken) error {
			token.ID = 100 // simulate DB-assigned ID
			return nil
		}
		s.AuditLogRepo.createFn = func(_ context.Context, _ *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit log database unavailable")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := svc.InitiateEmailChange(context.Background(), 42, "new@example.com", "test-password", net.IPv4(127, 0, 0, 1))

	require.NoError(t, err, "audit log failure must not block the email change flow")

	waitForDispatch(t, sent, "email dispatch after audit failure")
	assert.True(t, mailer.SendInvoked.Load(), "dispatcher should still send emails despite audit failure")
	require.NoError(t, mock.ExpectationsWereMet())
}

// =============================================================================
// ConfirmEmailChange — error path tests (sqlmock-backed)
// =============================================================================

// TestConfirmEmailChange_ConsumeTokenError verifies that an error from
// ConsumeByToken propagates as "failed to consume".
func TestConfirmEmailChange_ConsumeTokenError(t *testing.T) {
	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.TokenRepo.consumeByTokenFn = func(_ context.Context, _ string) (*platform.OperatorEmailChangeToken, error) {
			return nil, fmt.Errorf("consume query failed")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := svc.ConfirmEmailChange(context.Background(), "some-token", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to consume")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestConfirmEmailChange_EmailUniquenessRecheckError verifies that an error
// from FindByEmail during the uniqueness recheck propagates correctly.
func TestConfirmEmailChange_EmailUniquenessRecheckError(t *testing.T) {
	token := &platform.OperatorEmailChangeToken{
		OperatorID: 42,
		NewEmail:   "new@example.com",
		Token:      "valid-token",
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.TokenRepo.consumeByTokenFn = func(_ context.Context, _ string) (*platform.OperatorEmailChangeToken, error) {
			return token, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return nil, fmt.Errorf("email lookup failed")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := svc.ConfirmEmailChange(context.Background(), "valid-token", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to check email uniqueness")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestConfirmEmailChange_OperatorUpdateError verifies that a non-unique-violation
// error from operatorRepo.Update propagates as "failed to update operator email".
func TestConfirmEmailChange_OperatorUpdateError(t *testing.T) {
	token := &platform.OperatorEmailChangeToken{
		OperatorID: 42,
		NewEmail:   "new@example.com",
		Token:      "valid-token",
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	op := testOperator(42, "old@example.com", "some-hash")

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.TokenRepo.consumeByTokenFn = func(_ context.Context, _ string) (*platform.OperatorEmailChangeToken, error) {
			return token, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return nil, nil // email not taken
		}
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.updateFn = func(_ context.Context, _ *platform.Operator) error {
			return fmt.Errorf("disk full")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	_, err := svc.ConfirmEmailChange(context.Background(), "valid-token", net.IPv4(127, 0, 0, 1))

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to update operator email")
	require.NoError(t, mock.ExpectationsWereMet())
}

// TestConfirmEmailChange_AuditLogFailure_DoesNotBlock verifies that audit log
// failures after a successful email change do not prevent the new email from
// being returned. The dispatcher should still fire.
func TestConfirmEmailChange_AuditLogFailure_DoesNotBlock(t *testing.T) {
	token := &platform.OperatorEmailChangeToken{
		OperatorID: 42,
		NewEmail:   "new@example.com",
		Token:      "valid-token",
		Expiry:     time.Now().Add(30 * time.Minute),
		Used:       false,
	}
	op := testOperator(42, "old@example.com", "some-hash")

	sent := make(chan struct{}, 1)
	mailer := &email.MockMailer{
		SendFn: func(m email.Message) error {
			trySend(sent)
			return nil
		},
	}

	svc, mock, _ := newEmailChangeTestService(t, func(s *emailChangeTestSetup) {
		s.Mailer = mailer
		s.Dispatcher = email.NewDispatcher(mailer, slog.Default())
		s.Dispatcher.SetDefaults(1, []time.Duration{0})
		s.TokenRepo.consumeByTokenFn = func(_ context.Context, _ string) (*platform.OperatorEmailChangeToken, error) {
			return token, nil
		}
		s.OperatorRepo.findByEmailFn = func(_ context.Context, addr string) (*platform.Operator, error) {
			return nil, nil
		}
		s.OperatorRepo.findByIDFn = func(_ context.Context, id int64) (*platform.Operator, error) {
			return op, nil
		}
		s.OperatorRepo.updateFn = func(_ context.Context, _ *platform.Operator) error {
			return nil
		}
		s.AuditLogRepo.createFn = func(_ context.Context, _ *platform.OperatorAuditLog) error {
			return fmt.Errorf("audit log database unavailable")
		}
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	newEmail, err := svc.ConfirmEmailChange(context.Background(), "valid-token", net.IPv4(127, 0, 0, 1))

	require.NoError(t, err, "audit log failure must not block confirmation")
	assert.Equal(t, "new@example.com", newEmail)

	waitForDispatch(t, sent, "confirmation email dispatch")
	assert.True(t, mailer.SendInvoked.Load(), "confirmation email should be dispatched despite audit failure")
	require.NoError(t, mock.ExpectationsWereMet())
}
