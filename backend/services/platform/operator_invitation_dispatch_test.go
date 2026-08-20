package platform_test

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/email"
	"github.com/moto-nrw/project-phoenix/models/platform"
	platformSvc "github.com/moto-nrw/project-phoenix/services/platform"
)

// capturingMailer records all sent messages for assertions.
type capturingMailer struct {
	mu       sync.Mutex
	messages []email.Message
	sendFn   func(m email.Message) error
}

func (c *capturingMailer) Send(m email.Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, m)
	if c.sendFn != nil {
		return c.sendFn(m)
	}
	return nil
}

func (c *capturingMailer) getMessages() []email.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	copied := make([]email.Message, len(c.messages))
	copy(copied, c.messages)
	return copied
}

// waitForMessages polls until n messages are captured or timeout.
func (c *capturingMailer) waitForMessages(n int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		count := len(c.messages)
		c.mu.Unlock()
		if count >= n {
			return true
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false
}

// newTestServiceWithDispatcher creates a service wired with a real Dispatcher for email testing.
func newTestServiceWithDispatcher(
	t *testing.T,
	operatorRepo platform.OperatorRepository,
	auditLogRepo platform.OperatorAuditLogRepository,
	invitationTokenRepo platform.OperatorInvitationTokenRepository,
	bunDB *bun.DB,
	mailer email.Mailer,
) platformSvc.OperatorAuthAndInvitationService {
	t.Helper()

	dispatcher := email.NewDispatcher(mailer, slog.Default())
	dispatcher.SetDefaults(1, []time.Duration{10 * time.Millisecond})

	service, err := platformSvc.NewOperatorAuthService(platformSvc.OperatorAuthServiceConfig{
		OperatorRepo:        operatorRepo,
		AuditLogRepo:        auditLogRepo,
		InvitationTokenRepo: invitationTokenRepo,
		DB:                  bunDB,
		Logger:              slog.Default(),
		Dispatcher:          dispatcher,
		DefaultFrom:         email.NewEmail("moto", "no-reply@example.com"),
		FrontendURL:         "https://app.example.com",
		OperatorFrontendURL: "https://operator.example.com",
		InvitationExpiry:    48 * time.Hour,
	})
	require.NoError(t, err)
	return service
}

// =====================================================================
// dispatchOperatorInvitationEmail Tests
// =====================================================================

func TestInviteOperator_DispatchesEmail(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	mailer := &capturingMailer{}

	var createdTokenStr string
	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			token.ID = 100
			createdTokenStr = token.Token
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Creator Name"}
			op.ID = id
			return op, nil
		},
	}

	service := newTestServiceWithDispatcher(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB, mailer)

	err := service.InviteOperator(context.Background(), "invitee@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	// Wait for async email dispatch
	require.True(t, mailer.waitForMessages(1, 2*time.Second), "expected at least 1 email to be sent")

	msgs := mailer.getMessages()
	require.Len(t, msgs, 1)

	msg := msgs[0]
	assert.Equal(t, "invitee@example.com", msg.To.Address)
	assert.Equal(t, "Einladung als Operator zu moto", msg.Subject)
	assert.Equal(t, "operator-invitation.html", msg.Template)
	assert.Equal(t, "no-reply@example.com", msg.From.Address)

	// Verify template content bindings
	content, ok := msg.Content.(map[string]any)
	require.True(t, ok, "expected map content")
	assert.Contains(t, content["InvitationURL"], "operator.example.com/invite?token="+createdTokenStr)
	assert.Equal(t, 48, content["ExpiryHours"])
	assert.Equal(t, "Creator Name", content["InviterName"])
	assert.Contains(t, content["LogoURL"], "app.example.com/images/moto-logo-mit-schriftzug.png")
}

func TestInviteOperator_DispatchesEmail_InviterLookupFails_FallbackName(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	mailer := &capturingMailer{}

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
		// The pre-tx rate-limit lock must succeed so the invitation reaches
		// email dispatch; this test targets failure of the post-tx cosmetic
		// name lookup inside dispatchOperatorInvitationEmail, not the lock.
		findByIDForUpdateFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Locker"}
			op.ID = id
			return op, nil
		},
		findByIDFn: func(_ context.Context, _ int64) (*platform.Operator, error) {
			return nil, fmt.Errorf("db error") // Inviter lookup fails
		},
	}

	service := newTestServiceWithDispatcher(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB, mailer)

	err := service.InviteOperator(context.Background(), "invitee2@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	require.True(t, mailer.waitForMessages(1, 2*time.Second))
	msgs := mailer.getMessages()
	require.Len(t, msgs, 1)

	content := msgs[0].Content.(map[string]any)
	assert.Equal(t, "Ein Operator", content["InviterName"], "should fallback to generic name")
}

func TestResendOperator_DispatchesEmail(t *testing.T) {
	t.Parallel()

	bunDB, _ := setupSqlMock(t)

	mailer := &capturingMailer{}

	invitationRepo := &mockInvitationTokenRepo{
		findByIDFn: func(_ context.Context, id int64) (*platform.OperatorInvitationToken, error) {
			token := &platform.OperatorInvitationToken{
				Email:     "resend@example.com",
				Token:     "resend-token-uuid",
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
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Resender"}
			op.ID = id
			return op, nil
		},
	}

	service := newTestServiceWithDispatcher(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB, mailer)

	err := service.ResendOperatorInvitation(context.Background(), 100, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	require.True(t, mailer.waitForMessages(1, 2*time.Second))
	msgs := mailer.getMessages()
	require.Len(t, msgs, 1)

	assert.Equal(t, "resend@example.com", msgs[0].To.Address)
	assert.Equal(t, "operator-invitation.html", msgs[0].Template)
}

// =====================================================================
// persistInvitationDelivery Tests (via callback)
// =====================================================================

func TestInviteOperator_PersistsDeliveryResult_OnSuccess(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	mailer := &capturingMailer{}

	var deliverySentAt *time.Time
	var deliveryError *string
	var deliveryRetryCount int
	var deliveryCallbackDone = make(chan struct{})

	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			token.ID = 200
			return nil
		},
		updateDeliveryResultFn: func(_ context.Context, _ int64, sentAt *time.Time, emailError *string, retryCount int) error {
			deliverySentAt = sentAt
			deliveryError = emailError
			deliveryRetryCount = retryCount
			close(deliveryCallbackDone)
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Creator"}
			op.ID = id
			return op, nil
		},
	}

	service := newTestServiceWithDispatcher(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB, mailer)

	err := service.InviteOperator(context.Background(), "persist-ok@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	// Wait for the callback to fire
	select {
	case <-deliveryCallbackDone:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("delivery callback never fired")
	}

	assert.NotNil(t, deliverySentAt, "sentAt should be set on success")
	assert.Nil(t, deliveryError, "no error on success")
	assert.Equal(t, 1, deliveryRetryCount, "attempt count starts at 1")
}

func TestInviteOperator_PersistsDeliveryResult_OnFailure(t *testing.T) {
	t.Parallel()

	bunDB, mock := setupSqlMock(t)
	expectAdminTx(mock)
	mock.ExpectCommit()

	mailer := &capturingMailer{
		sendFn: func(_ email.Message) error {
			return fmt.Errorf("SMTP connection refused")
		},
	}

	var deliveryError *string
	var deliveryCallbackDone = make(chan struct{}, 1)

	invitationRepo := &mockInvitationTokenRepo{
		createFn: func(_ context.Context, token *platform.OperatorInvitationToken) error {
			token.ID = 300
			return nil
		},
		updateDeliveryResultFn: func(_ context.Context, _ int64, _ *time.Time, emailError *string, _ int) error {
			deliveryError = emailError
			select {
			case deliveryCallbackDone <- struct{}{}:
			default:
			}
			return nil
		},
	}
	operatorRepo := &mockOperatorRepo{
		findByEmailFn: func(_ context.Context, _ string) (*platform.Operator, error) {
			return nil, nil
		},
		findByIDFn: func(_ context.Context, id int64) (*platform.Operator, error) {
			op := &platform.Operator{DisplayName: "Creator"}
			op.ID = id
			return op, nil
		},
	}

	service := newTestServiceWithDispatcher(t, operatorRepo, &mockAuditLogRepoShared{}, invitationRepo, bunDB, mailer)

	err := service.InviteOperator(context.Background(), "persist-fail@example.com", nil, 42, net.ParseIP("127.0.0.1"))
	require.NoError(t, err)

	select {
	case <-deliveryCallbackDone:
		// ok
	case <-time.After(3 * time.Second):
		t.Fatal("delivery callback never fired")
	}

	require.NotNil(t, deliveryError, "error should be persisted on failure")
	assert.Contains(t, *deliveryError, "SMTP connection refused")
}
