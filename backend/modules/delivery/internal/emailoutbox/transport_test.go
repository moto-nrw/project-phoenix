package emailoutbox

import (
	"context"
	"log/slog"
	"testing"

	email "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
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
	return email.Receipt{ProviderMessageID: "provider-1"}, nil
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
