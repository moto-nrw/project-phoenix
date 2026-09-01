package email

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gomail "github.com/wneessen/go-mail"
)

func TestSMTPMailerSendContextReturnsWhileExchangeIsInFlight(t *testing.T) {
	t.Parallel()

	clientConn, serverConn := net.Pipe()
	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	dataStarted := make(chan struct{})
	releaseServer := make(chan struct{})
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		reader := bufio.NewReader(serverConn)
		_, _ = serverConn.Write([]byte("220 smtp.test ESMTP\r\n"))
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				_, _ = serverConn.Write([]byte("250 smtp.test\r\n"))
			case strings.HasPrefix(line, "MAIL FROM"):
				_, _ = serverConn.Write([]byte("250 sender accepted\r\n"))
			case strings.HasPrefix(line, "RCPT TO"):
				_, _ = serverConn.Write([]byte("250 recipient accepted\r\n"))
			case strings.HasPrefix(line, "DATA"):
				close(dataStarted)
				_, _ = serverConn.Write([]byte("354 send message\r\n"))
				// Stop reading: the client must remain blocked in DATA until
				// cancellation closes the connection.
				<-releaseServer
				return
			default:
				_, _ = serverConn.Write([]byte("250 ok\r\n"))
			}
		}
	}()

	client, err := gomail.NewClient(
		"smtp.test",
		gomail.WithPort(25),
		gomail.WithTLSPolicy(gomail.NoTLS),
		gomail.WithoutNoop(),
		gomail.WithDialContextFunc(func(context.Context, string, string) (net.Conn, error) {
			return clientConn, nil
		}),
	)
	require.NoError(t, err)

	mailer := &SMTPMailer{client: client}
	message := gomail.NewMsg()
	require.NoError(t, message.From("sender@example.invalid"))
	require.NoError(t, message.To("recipient@example.invalid"))
	message.Subject("Cancellation test")
	message.SetBodyString(gomail.TypeTextPlain, "body")
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		result <- mailer.sendMessageContext(ctx, message)
	}()

	select {
	case <-dataStarted:
	case <-time.After(time.Second):
		t.Fatal("SMTP client did not reach the DATA exchange")
	}
	cancel()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("SendContext did not report cancellation for the in-flight SMTP exchange")
	}
	close(releaseServer)
	<-serverDone
}
