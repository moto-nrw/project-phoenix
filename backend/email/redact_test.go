package email

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

// A send error keeps its diagnosis (status, server reply) but loses every
// address, and still unwraps to the original for errors.Is.
func TestRedactAddresses(t *testing.T) {
	t.Parallel()

	assert.NoError(t, redactAddresses(nil))

	plain := errors.New("dial tcp 127.0.0.1:1: connect: connection refused")
	assert.Same(t, plain, redactAddresses(plain), "an error without an address is returned unchanged")

	original := errors.New("failed to send mail: 550 5.1.1 <jane.doe+ogs@example.invalid>: Recipient address rejected, affected recipient(s): jane.doe+ogs@example.invalid")
	redacted := redactAddresses(original)
	assert.Equal(t, "failed to send mail: 550 5.1.1 <[address]>: Recipient address rejected, affected recipient(s): [address]", redacted.Error())
	assert.ErrorIs(t, redacted, original)
}

// The Message-ID go-mail reports has the form local@host too; it is the
// handle for a failed send in the provider's logs and must survive.
func TestRedactAddresses_KeepsSendErrorMessageID(t *testing.T) {
	t.Parallel()

	err := redactAddresses(messageIDError{text: "rejected, affected recipient(s): jane@example.invalid, affected message ID: <Ab.c_1@mail.host>", id: "<Ab.c_1@mail.host>"})
	assert.Equal(t, "rejected, affected recipient(s): [address], affected message ID: <Ab.c_1@mail.host>", err.Error())
}

type messageIDError struct{ text, id string }

func (e messageIDError) Error() string     { return e.text }
func (e messageIDError) MessageID() string { return e.id }

// #2108: a rejected recipient must not reach the log or the returned error in
// clear text. The fake server echoes the mailbox in its 550 reply the way real
// SMTP servers do, and go-mail appends it again as "affected recipient(s)".
func TestSMTPMailer_RejectedRecipientStaysOutOfLogsAndError(t *testing.T) {
	t.Parallel()

	const recipient = "erika.muster@example.invalid"
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "regression.html"),
		[]byte(`<!DOCTYPE html><html><body><p>{{.Message}}</p></body></html>`), 0o644))
	templates, err := parseTemplates(tempDir)
	require.NoError(t, err)

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})
	go func() {
		reader := bufio.NewReader(serverConn)
		_, _ = serverConn.Write([]byte("220 smtp.test ESMTP\r\n"))
		for {
			line, readErr := reader.ReadString('\n')
			if readErr != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = serverConn.Write([]byte("250 smtp.test\r\n"))
			case strings.HasPrefix(line, "RCPT TO"):
				_, _ = serverConn.Write([]byte("550 5.1.1 <" + recipient + ">: Recipient address rejected\r\n"))
			case strings.HasPrefix(line, "QUIT"):
				_, _ = serverConn.Write([]byte("221 bye\r\n"))
				return
			default:
				_, _ = serverConn.Write([]byte("250 ok\r\n"))
			}
		}
	}()

	client, err := gomail.NewClient("smtp.test",
		gomail.WithPort(25),
		gomail.WithTLSPolicy(gomail.NoTLS),
		gomail.WithoutNoop(),
		gomail.WithDialContextFunc(func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		}),
	)
	require.NoError(t, err)

	var logs bytes.Buffer
	mailer := &SMTPMailer{
		client:      client,
		templates:   templates,
		logger:      slog.New(slog.NewTextHandler(&logs, nil)),
		defaultFrom: NewEmail("moto", "system@example.invalid"),
	}

	sendErr := mailer.SendContext(context.Background(), Message{
		To:       NewEmail("Erika Muster", recipient),
		Subject:  "Einladung zum Eltern-Portal",
		Template: "regression.html",
		Content:  map[string]string{"Message": "Hallo"},
	})

	require.Error(t, sendErr)
	assert.NotContains(t, sendErr.Error(), recipient)
	assert.Contains(t, sendErr.Error(), "550", "the server reply stays diagnosable")
	var sendError *gomail.SendError
	require.ErrorAs(t, sendErr, &sendError, "callers can still inspect the go-mail error")
	assert.Contains(t, sendErr.Error(), sendError.MessageID(), "the Message-ID survives the redaction")
	assert.NotContains(t, logs.String(), recipient)
	assert.Contains(t, logs.String(), "message_id=", "the Message-ID identifies the mail instead")
}
