package email

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const acceptedWithID = "250 2.0.0 Ok: queued id=0194a7b2-3c4d-7e8f-9a0b-1c2d3e4f5a6b"

var lettermintTestSending = mailcontract.LettermintSending{SecurityFrom: "noreply@mail.example", NoticeFrom: "nachrichten@mail.example"}

func TestLettermintSendingChoosesRouteAndSenderByMailKind(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		security bool
		route    string
		from     string
	}{
		"security mail": {true, "X-Lettermint-Route: sicherheit", "noreply@mail.example"},
		"other mail":    {false, "X-Lettermint-Route: mitteilungen", "nachrichten@mail.example"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, sent := fakeSMTP(t, smtpAnswers{dataClose: acceptedWithID})
			mailer := identityMailer(t, client)
			mailer.lettermint = lettermintTestSending
			message := identityMessage()
			message.Security = test.security
			message.From = mailcontract.NewEmail("OGS Musterschule", "old@strato.example")
			message.ReplyTo = mailcontract.NewEmail("OGS Musterschule", "ogs@schule.example")

			receipt, err := mailer.SendWithReceipt(context.Background(), message)

			require.NoError(t, err)
			assert.Equal(t, "0194a7b2-3c4d-7e8f-9a0b-1c2d3e4f5a6b", receipt.ProviderMessageID)
			assert.Equal(t, "token-1@mail.example", receipt.MessageID)
			assert.Contains(t, sent(), test.route)
			assert.Contains(t, sent(), "X-LM-Preserve-Message-ID: true")
			assert.Contains(t, sent(), `"OGS Musterschule" <`+test.from+`>`)
			assert.Contains(t, sent(), "ogs@schule.example", "replies keep going to the school")
			assert.NotContains(t, sent(), "old@strato.example")
			assert.NotContains(t, sent(), "List-Unsubscribe")
		})
	}
}

func TestPlainSMTPKnowsNothingOfLettermint(t *testing.T) {
	t.Parallel()
	client, sent := fakeSMTP(t, smtpAnswers{dataClose: acceptedWithID})
	message := identityMessage()
	message.Security = true
	message.From = mailcontract.NewEmail("moto", "kontakt@strato.example")

	receipt, err := identityMailer(t, client).SendWithReceipt(context.Background(), message)

	require.NoError(t, err)
	assert.Empty(t, receipt.ProviderMessageID, "another server's answer is no provider ID")
	assert.NotContains(t, sent(), "X-Lettermint-Route")
	assert.NotContains(t, sent(), "X-LM-Preserve-Message-ID")
	assert.Contains(t, sent(), "kontakt@strato.example")
}

func TestAcceptedMailWithoutProviderIDStaysSent(t *testing.T) {
	t.Parallel()
	client, _ := fakeSMTP(t, smtpAnswers{dataClose: "250 queued"})
	mailer := identityMailer(t, client)
	mailer.lettermint = lettermintTestSending

	receipt, err := mailer.SendWithReceipt(context.Background(), identityMessage())

	require.NoError(t, err)
	assert.Empty(t, receipt.ProviderMessageID)
}

// fallbackPair is a Lettermint transport that answers as scripted and a
// fallback transport that accepts.
func fallbackPair(t *testing.T, primary smtpAnswers) (*fallbackMailer, func() string, func() string) {
	t.Helper()
	primaryClient, primarySent := fakeSMTP(t, primary)
	primaryMailer := identityMailer(t, primaryClient)
	primaryMailer.lettermint = lettermintTestSending
	fallbackClient, fallbackSent := fakeSMTP(t, smtpAnswers{dataClose: acceptedWithID})
	fallbackMailerSMTP := identityMailer(t, fallbackClient)
	fallbackMailerSMTP.fixedFrom = "app-sicherheit@example.org"
	return newFallbackMailer(primaryMailer, fallbackMailerSMTP, slog.New(slog.DiscardHandler)), primarySent, fallbackSent
}

func securityTestMessage() mailcontract.Message {
	message := identityMessage()
	message.Security = true
	message.From = mailcontract.NewEmail("moto", "noreply@mail.example")
	return message
}

func TestSecurityMailLeavesOverTheFallbackWhenTheProviderFails(t *testing.T) {
	t.Parallel()
	for name, answers := range map[string]smtpAnswers{
		"temporary": {mailFrom: "451 try later"},
		"rejected":  {rcptTo: "550 suppressed"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mailer, _, fallbackSent := fallbackPair(t, answers)

			receipt, err := mailer.SendWithReceipt(context.Background(), securityTestMessage())

			require.NoError(t, err)
			assert.Empty(t, receipt.ProviderMessageID, "the mailbox gives no ID events could refer to")
			assert.Equal(t, "token-1@example.org", receipt.MessageID)
			assert.Contains(t, fallbackSent(), "app-sicherheit@example.org")
			assert.Contains(t, fallbackSent(), "X-Moto-Outbox-Id: token-1")
			assert.NotContains(t, fallbackSent(), "X-Lettermint-Route")
		})
	}
}

func TestOtherMailNeverLeavesOverTheFallback(t *testing.T) {
	t.Parallel()
	mailer, _, fallbackSent := fallbackPair(t, smtpAnswers{mailFrom: "451 try later"})

	_, err := mailer.SendWithReceipt(context.Background(), identityMessage())

	require.Error(t, err)
	assert.Empty(t, fallbackSent())
}

func TestFailedFallbackReportsBothTransports(t *testing.T) {
	t.Parallel()
	primaryClient, _ := fakeSMTP(t, smtpAnswers{mailFrom: "451 provider down"})
	fallbackClient, _ := fakeSMTP(t, smtpAnswers{rcptTo: "550 5.1.1 no such mailbox"})
	mailer := newFallbackMailer(identityMailer(t, primaryClient), identityMailer(t, fallbackClient), slog.New(slog.DiscardHandler))

	_, err := mailer.SendWithReceipt(context.Background(), securityTestMessage())

	require.Error(t, err)
	assert.ErrorIs(t, err, mailcontract.ErrRecipientRejected, "the fallback's answer decides whether a retry helps")
	assert.Contains(t, err.Error(), "provider")
}

func TestProviderGaveUpSendsOverTheFallbackOnly(t *testing.T) {
	t.Parallel()
	mailer, primarySent, fallbackSent := fallbackPair(t, smtpAnswers{})
	message := securityTestMessage()
	message.UseFallback = true

	_, err := mailer.SendWithReceipt(context.Background(), message)

	require.NoError(t, err)
	assert.Empty(t, primarySent())
	assert.NotEmpty(t, fallbackSent())
}

type stubReceiptMailer struct {
	send func(context.Context) (mailcontract.Receipt, error)
}

func (m stubReceiptMailer) SendWithReceipt(ctx context.Context, _ mailcontract.Message) (mailcontract.Receipt, error) {
	return m.send(ctx)
}

func TestHangingProviderLeavesTheFallbackItsTime(t *testing.T) {
	t.Parallel()
	mailer := newFallbackMailer(
		stubReceiptMailer{send: func(ctx context.Context) (mailcontract.Receipt, error) {
			<-ctx.Done()
			return mailcontract.Receipt{}, ctx.Err()
		}},
		stubReceiptMailer{send: func(ctx context.Context) (mailcontract.Receipt, error) {
			return mailcontract.Receipt{MessageID: "fallback"}, ctx.Err()
		}},
		slog.New(slog.DiscardHandler))
	mailer.primaryTimeout = 10 * time.Millisecond

	receipt, err := mailer.SendWithReceipt(context.Background(), securityTestMessage())

	require.NoError(t, err)
	assert.Equal(t, "fallback", receipt.MessageID)
}

func TestCancelledCallerGetsNoFallback(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	sentFallback := false
	mailer := newFallbackMailer(
		stubReceiptMailer{send: func(context.Context) (mailcontract.Receipt, error) {
			cancel()
			return mailcontract.Receipt{}, errors.New("connection closed")
		}},
		stubReceiptMailer{send: func(context.Context) (mailcontract.Receipt, error) {
			sentFallback = true
			return mailcontract.Receipt{}, nil
		}},
		slog.New(slog.DiscardHandler))

	_, err := mailer.SendWithReceipt(ctx, securityTestMessage())

	require.Error(t, err)
	assert.False(t, sentFallback)
}

func TestDemoKeepsTheReceiptOfADemoMail(t *testing.T) {
	t.Parallel()
	client, _ := fakeSMTP(t, smtpAnswers{dataClose: acceptedWithID})
	inner := identityMailer(t, client)
	inner.lettermint = lettermintTestSending
	restricted, ok := RestrictToDemoMails(inner, "demo", slog.New(slog.DiscardHandler)).(mailcontract.ReceiptMailer)
	require.True(t, ok)

	message := identityMessage()
	message.Template = mailcontract.TemplateDemoAccess
	_, err := restricted.SendWithReceipt(context.Background(), message)
	assert.Error(t, err, "the template is not in this test's directory, so the send reached the transport")

	kept := identityMessage()
	receipt, err := restricted.SendWithReceipt(context.Background(), kept)
	require.NoError(t, err)
	assert.Empty(t, receipt.MessageID, "a mail the demo keeps has no identity")
}
