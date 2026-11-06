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

	mailcontract "github.com/moto-nrw/project-phoenix/modules/delivery/mail"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

// smtpAnswers is what the fake server replies; an empty answer accepts.
type smtpAnswers struct {
	mailFrom, rcptTo, dataClose string
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
		answers             smtpAnswers
		permanent, rejected bool
	}{
		"unknown mailbox":  {answers: smtpAnswers{rcptTo: "550 5.1.1 <guardian@example.com> unknown"}, permanent: true, rejected: true},
		"full mailbox":     {answers: smtpAnswers{rcptTo: "452 4.2.2 mailbox full"}},
		"content refused":  {answers: smtpAnswers{dataClose: "554 5.7.1 rejected"}, permanent: true},
		"busy after data":  {answers: smtpAnswers{dataClose: "451 4.3.0 try later"}},
		"sender misconfig": {answers: smtpAnswers{mailFrom: "550 5.7.1 sender not allowed"}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			client, _ := fakeSMTP(t, tc.answers)

			err := identityMailer(t, client).SendContext(context.Background(), identityMessage())

			require.Error(t, err)
			assert.Equal(t, tc.permanent, errors.Is(err, mailcontract.ErrPermanentRejection), "permanent")
			assert.Equal(t, tc.rejected, errors.Is(err, mailcontract.ErrRecipientRejected), "recipient")
			assert.NotContains(t, err.Error(), "guardian@example.com")
		})
	}
}
