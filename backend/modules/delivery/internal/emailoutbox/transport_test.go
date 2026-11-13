package emailoutbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	deliveryContract "github.com/moto-ogs/project-phoenix/modules/delivery/contract"
	email "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/require"
)

type contextCheckingMailTransactions struct{}

func (contextCheckingMailTransactions) WithinTenant(ctx context.Context, _ int64, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(ctx)
}

type cancelledMailTransport struct{ cancel context.CancelFunc }

func (cancelledMailTransport) Send(email.Message) error { panic("ContextMailer must be used") }
func (m cancelledMailTransport) SendContext(context.Context, email.Message) error {
	m.cancel()
	return context.Canceled
}

type recordingMailRenderer struct {
	recorded bool
	result   error
}

func (*recordingMailRenderer) Render(context.Context, *Intent) (*email.Message, error) {
	return &email.Message{Subject: "Mail result"}, nil
}
func (r *recordingMailRenderer) RecordDelivery(ctx context.Context, _ *Intent, result error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.recorded, r.result = true, result
	return nil
}

type receiptMailTransport struct{ sent *email.Message }

func (receiptMailTransport) Send(email.Message) error { panic("ReceiptMailer must be used") }
func (m receiptMailTransport) SendWithReceipt(_ context.Context, message email.Message) (email.Receipt, error) {
	*m.sent = message
	return email.Receipt{ProviderMessageID: "provider-1", UsedFallback: message.UseFallback}, nil
}

func TestTransportHandsTheMailKindToTheMailer(t *testing.T) {
	t.Parallel()
	sent := &email.Message{}
	transport := Transport{Mailer: receiptMailTransport{sent: sent}, Tenants: contextCheckingMailTransactions{}, Logger: slog.Default()}

	receipt, err := transport.Send(context.Background(), &Intent{TenantID: 42, DeliveryToken: "token-1", Security: true, UseFallback: true},
		&recordingMailRenderer{}, email.NewEmail("", "guardian@example.com"))

	require.NoError(t, err)
	require.Equal(t, "provider-1", receipt.ProviderMessageID)
	require.True(t, sent.Security)
	require.True(t, sent.UseFallback)
	require.Equal(t, "token-1", sent.DeliveryToken)
}

func TestTransportRecordsAttemptAfterSendingContextEnds(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	renderer := &recordingMailRenderer{}
	transport := Transport{Mailer: cancelledMailTransport{cancel: cancel}, Tenants: contextCheckingMailTransactions{}, Logger: slog.Default()}
	_, err := transport.Send(ctx, &Intent{TenantID: 42}, renderer, email.NewEmail("", "recipient@example.test"))
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, ctx.Err(), context.Canceled)
	require.True(t, renderer.recorded, "the caller must still see the failed transport attempt in its delivery record")
	require.ErrorIs(t, renderer.result, context.Canceled)
}

func (contextCheckingMailTransactions) WithinSystem(ctx context.Context, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn(ctx)
}

func TestProviderResultPersistsFallbackReceipt(t *testing.T) {
	t.Parallel()
	sent := &email.Message{}
	provider := MailPushProvider{
		Registry: NewTemplateRegistry(map[string]Renderer{"security": &recordingMailRenderer{}}),
		Mailer:   receiptMailTransport{sent: sent}, Tenants: contextCheckingMailTransactions{}, Logger: slog.Default(),
	}
	result, err := provider.SendEmail(context.Background(), deliveryContract.ClaimedIntent{
		TenantID: 42, Template: "security", Security: true, UseFallback: true, EmailPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	require.JSONEq(t, `{"provider_message_id":"provider-1","used_fallback":true,"status_code":202}`, string(encoded))
}

// A request the fallback can no longer answer ends on this try: a retry over the same
// configuration cannot change that.
func TestProviderEndsAFallbackRequestWithoutAFallback(t *testing.T) {
	t.Parallel()
	provider := MailPushProvider{
		Registry: NewTemplateRegistry(map[string]Renderer{"security": &recordingMailRenderer{}}),
		Mailer:   removedFallbackTransport{}, Tenants: contextCheckingMailTransactions{}, Logger: slog.Default(),
	}
	_, err := provider.SendEmail(context.Background(), deliveryContract.ClaimedIntent{
		TenantID: 42, Template: "security", Security: true, UseFallback: true, EmailPayload: json.RawMessage(`{}`),
	})
	require.ErrorIs(t, err, deliveryContract.ErrPermanent)
	require.NotErrorIs(t, err, deliveryContract.ErrRecipientRejected, "the address is not at fault")
}

type removedFallbackTransport struct{}

func (removedFallbackTransport) Send(email.Message) error { panic("ReceiptMailer must be used") }
func (removedFallbackTransport) SendWithReceipt(context.Context, email.Message) (email.Receipt, error) {
	return email.Receipt{}, email.ErrFallbackRemoved
}

type throttlingMailTransport struct{}

func (throttlingMailTransport) Send(email.Message) error { panic("ReceiptMailer must be used") }
func (throttlingMailTransport) SendWithReceipt(context.Context, email.Message) (email.Receipt, error) {
	return email.Receipt{}, fmt.Errorf("%w: 421 4.7.0 rate limit exceeded", email.ErrThrottled)
}

// #6737: a throttled send postpones the mail without an attempt while the mail
// is younger than ThrottleWindow. After that each send counts again, so the mail
// still ends after its attempts.
func TestProviderPostponesAThrottledMailWithinTheWindow(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		age      time.Duration
		deferred bool
	}{
		"new mail":           {age: time.Minute, deferred: true},
		"just inside":        {age: ThrottleWindow - time.Minute, deferred: true},
		"beyond the window":  {age: ThrottleWindow + time.Minute},
		"unknown queue time": {age: -1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			renderer := &recordingMailRenderer{}
			provider := MailPushProvider{
				Registry: NewTemplateRegistry(map[string]Renderer{"notice": renderer}),
				Mailer:   throttlingMailTransport{}, Tenants: contextCheckingMailTransactions{}, Logger: slog.Default(),
			}
			intent := deliveryContract.ClaimedIntent{TenantID: 42, Template: "notice", EmailPayload: json.RawMessage(`{}`)}
			if tc.age >= 0 {
				intent.CreatedAt = time.Now().Add(-tc.age)
			}

			_, err := provider.SendEmail(context.Background(), intent)

			require.Error(t, err)
			require.Equal(t, tc.deferred, errors.Is(err, deliveryContract.ErrDeferred), "deferred")
			require.Equal(t, tc.deferred, errors.Is(err, deliveryContract.ErrThrottled), "throttled")
			require.NotErrorIs(t, err, deliveryContract.ErrPermanent)
			require.True(t, renderer.recorded, "the delivery record still sees the send")
			require.Equal(t, tc.deferred, errors.Is(renderer.result, ErrSendThrottled), "the record learns whether the send counts")
		})
	}
}
