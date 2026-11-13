package email

import (
	"bufio"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mailcontract "github.com/moto-ogs/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

// smtpAnswers is what the fake server replies; an empty answer accepts.
type smtpAnswers struct {
	mailFrom, rcptTo, dataClose string
	// plainServer does not advertise enhanced status codes.
	plainServer bool
}

// fakeSMTP runs one scripted exchange per connection and keeps the DATA text.
func fakeSMTP(t *testing.T, answers smtpAnswers) (*gomail.Client, func() string) {
	t.Helper()
	var mu sync.Mutex
	var data strings.Builder
	reply := func(conn net.Conn, scripted, accepted string) {
		if scripted == "" {
			scripted = accepted
		}
		_, _ = conn.Write([]byte(scripted + "\r\n"))
	}
	serve := func(conn net.Conn) {
		defer func() { _ = conn.Close() }()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 smtp.test ESMTP\r\n"))
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO") && !answers.plainServer:
				_, _ = conn.Write([]byte("250-smtp.test\r\n250 ENHANCEDSTATUSCODES\r\n"))
			case strings.HasPrefix(line, "MAIL FROM"):
				reply(conn, answers.mailFrom, "250 ok")
			case strings.HasPrefix(line, "RCPT TO"):
				reply(conn, answers.rcptTo, "250 ok")
			case strings.HasPrefix(line, "DATA"):
				_, _ = conn.Write([]byte("354 go\r\n"))
				for {
					text, err := reader.ReadString('\n')
					if err != nil || text == ".\r\n" {
						break
					}
					mu.Lock()
					data.WriteString(text)
					mu.Unlock()
				}
				reply(conn, answers.dataClose, "250 queued")
			case strings.HasPrefix(line, "QUIT"):
				_, _ = conn.Write([]byte("221 bye\r\n"))
				return
			default:
				_, _ = conn.Write([]byte("250 ok\r\n"))
			}
		}
	}
	client, err := gomail.NewClient("smtp.test", gomail.WithPort(25), gomail.WithTLSPolicy(gomail.NoTLS), gomail.WithoutNoop(),
		gomail.WithDialContextFunc(func(context.Context, string, string) (net.Conn, error) {
			clientConn, serverConn := net.Pipe()
			go serve(serverConn)
			return clientConn, nil
		}))
	require.NoError(t, err)
	return client, func() string {
		mu.Lock()
		defer mu.Unlock()
		return data.String()
	}
}

func identityMailer(t *testing.T, client *gomail.Client) *SMTPMailer {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.html"), []byte(`<html><body>{{.Content}}</body></html>`), 0o600))
	templates, err := parseTemplates(dir)
	require.NoError(t, err)
	return &SMTPMailer{client: client, templates: templates, logger: slog.New(slog.DiscardHandler), defaultFrom: mailcontract.NewEmail("moto", "noreply@mail.example")}
}

func identityMessage() mailcontract.Message {
	return mailcontract.Message{
		To: mailcontract.NewEmail("", "guardian@example.com"), Subject: "Test", Template: "test.html",
		Content: map[string]any{}, DeliveryToken: "token-1",
	}
}

func TestSMTPMailerSendsTheDeliveryIdentity(t *testing.T) {
	t.Parallel()
	client, sent := fakeSMTP(t, smtpAnswers{})

	receipt, err := identityMailer(t, client).SendWithReceipt(context.Background(), identityMessage())

	require.NoError(t, err)
	assert.Equal(t, "token-1@mail.example", receipt.MessageID)
	assert.Contains(t, sent(), "Message-ID: <token-1@mail.example>")
	assert.Contains(t, sent(), "X-Moto-Outbox-Id: token-1")
}

func TestSMTPMailerGeneratesAMessageIDWithoutAToken(t *testing.T) {
	t.Parallel()
	client, sent := fakeSMTP(t, smtpAnswers{})
	message := identityMessage()
	message.DeliveryToken = ""

	receipt, err := identityMailer(t, client).SendWithReceipt(context.Background(), message)

	require.NoError(t, err)
	assert.NotEmpty(t, receipt.MessageID)
	assert.NotContains(t, sent(), "X-Moto-Outbox-Id")
}

func TestSMTPMailerClassifiesRefusals(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		answers                        smtpAnswers
		permanent, rejected, throttled bool
	}{
		"unknown mailbox":    {answers: smtpAnswers{rcptTo: "550 5.1.1 <guardian@example.com> unknown"}, permanent: true, rejected: true},
		"unknown at data":    {answers: smtpAnswers{dataClose: "550 5.1.1 <guardian@example.com> unknown"}, permanent: true, rejected: true},
		"undeliverable mail": {answers: smtpAnswers{dataClose: "554 5.6.0 cannot convert"}, permanent: true},
		"full mailbox":       {answers: smtpAnswers{rcptTo: "452 4.2.2 mailbox full"}, throttled: true},
		"busy after data":    {answers: smtpAnswers{dataClose: "451 4.3.0 try later"}, throttled: true},
		"rate limited":       {answers: smtpAnswers{mailFrom: "421 4.7.0 rate limit exceeded"}, throttled: true},
		"quota at rcpt":      {answers: smtpAnswers{rcptTo: "450 4.7.1 sending quota reached"}, throttled: true},
		"sender misconfig":   {answers: smtpAnswers{mailFrom: "550 5.7.1 sender not allowed"}},
		"sender at rcpt":     {answers: smtpAnswers{rcptTo: "554 5.7.1 sender address rejected"}},
		"relay denied":       {answers: smtpAnswers{rcptTo: "550 5.7.1 relaying denied"}},
		"auth required":      {answers: smtpAnswers{rcptTo: "530 5.7.0 authentication required"}},
		"quota exhausted":    {answers: smtpAnswers{rcptTo: "550 5.4.5 daily quota exceeded"}},
		"blocked at data":    {answers: smtpAnswers{dataClose: "554 5.7.1 domain not verified"}},
		"no enhanced status": {answers: smtpAnswers{rcptTo: "550 unknown", plainServer: true}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, _ := fakeSMTP(t, tc.answers)

			err := identityMailer(t, client).SendContext(context.Background(), identityMessage())

			require.Error(t, err)
			assert.Equal(t, tc.permanent, errors.Is(err, mailcontract.ErrPermanentRejection), "permanent")
			assert.Equal(t, tc.rejected, errors.Is(err, mailcontract.ErrRecipientRejected), "recipient")
			assert.Equal(t, tc.throttled, errors.Is(err, mailcontract.ErrThrottled), "throttled")
			assert.NotContains(t, err.Error(), "guardian@example.com")
		})
	}
}

func TestSMTPMailerClassifiesAddressStatuses(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		status   string
		rejected bool
	}{
		{status: "5.1.0"},                  // Unspecified address error.
		{status: "5.1.1", rejected: true},  // Bad destination mailbox.
		{status: "5.1.2", rejected: true},  // Bad destination system.
		{status: "5.1.3", rejected: true},  // Bad destination syntax.
		{status: "5.1.4", rejected: true},  // Ambiguous destination mailbox.
		{status: "5.1.5"},                  // Valid destination address.
		{status: "5.1.6", rejected: true},  // Moved without forwarding.
		{status: "5.1.7"},                  // Bad sender syntax.
		{status: "5.1.8"},                  // Bad sender system.
		{status: "5.1.9"},                  // Relayed to a non-compliant mailer.
		{status: "5.1.10", rejected: true}, // Recipient domain has null MX.
		{status: "5.1.99"},                 // Unknown status must stay retryable.
	} {
		for _, stage := range []string{"rcpt", "data", "sender"} {
			t.Run(tc.status+"/"+stage, func(t *testing.T) {
				t.Parallel()
				answer := "550 " + tc.status + " address rejected"
				answers := map[string]smtpAnswers{
					"rcpt": {rcptTo: answer}, "data": {dataClose: answer}, "sender": {mailFrom: answer},
				}
				client, _ := fakeSMTP(t, answers[stage])

				err := identityMailer(t, client).SendContext(context.Background(), identityMessage())

				require.Error(t, err)
				rejected := tc.rejected && stage != "sender"
				assert.Equal(t, rejected, errors.Is(err, mailcontract.ErrPermanentRejection), "permanent")
				assert.Equal(t, rejected, errors.Is(err, mailcontract.ErrRecipientRejected), "recipient")
			})
		}
	}
}

// #6737: throttling is expected under load and has its own alarm, so it is no
// ERROR line; other failures stay one.
func TestSMTPMailerLogsThrottlingAsWarning(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		answers smtpAnswers
		level   string
	}{
		"throttled": {answers: smtpAnswers{mailFrom: "421 4.7.0 rate limit exceeded"}, level: "level=WARN"},
		"refused":   {answers: smtpAnswers{mailFrom: "550 5.7.1 sender not allowed"}, level: "level=ERROR"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, _ := fakeSMTP(t, tc.answers)
			mailer := identityMailer(t, client)
			var logs strings.Builder
			var mu sync.Mutex
			mailer.logger = slog.New(slog.NewTextHandler(lockedWriter{w: &logs, mu: &mu}, nil))

			require.Error(t, mailer.SendContext(context.Background(), identityMessage()))

			mu.Lock()
			defer mu.Unlock()
			assert.Contains(t, logs.String(), tc.level+` msg="email send failed"`)
			assert.NotContains(t, logs.String(), "guardian@example.com")
		})
	}
}

type lockedWriter struct {
	w  *strings.Builder
	mu *sync.Mutex
}

func (l lockedWriter) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.w.Write(p)
}
