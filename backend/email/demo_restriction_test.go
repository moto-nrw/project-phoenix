package email

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingMailer struct{ templates []string }

func (m *recordingMailer) Send(message Message) error {
	m.templates = append(m.templates, message.Template)
	return nil
}

const mfaCodeTemplate = "mfa-email-code.html"

var blockedInTheDemo = []string{"invitation.html", "password-reset.html", "guardian-invitation.html", mfaCodeTemplate, ""}

// The mail lock of the demo environment (#3465): a visitor with administrator
// rights must not reach arbitrary addresses from our sender.
func TestDemoEnvironmentDeliversOnlyTheDemoMails(t *testing.T) {
	t.Parallel()
	transport := &recordingMailer{}
	mailer := RestrictToDemoMails(transport, " Demo ", nil)

	for _, template := range blockedInTheDemo {
		err := mailer.Send(Message{Template: template})
		if template == mfaCodeTemplate {
			// A sign-in waits for this code, so it must hear that none comes.
			require.ErrorIs(t, err, ErrNotDeliveredInDemo)
			continue
		}
		require.NoError(t, err, "a dropped mail is no failure: %q", template)
	}
	assert.Empty(t, transport.templates, "invitation, password reset, guardian invitation and sign-in code stay on the server")

	require.NoError(t, mailer.Send(Message{Template: TemplateDemoAccess}))
	contextMailer, ok := mailer.(ContextMailer)
	require.True(t, ok, "synchronous delivery needs the context transport")
	require.NoError(t, contextMailer.SendContext(context.Background(), Message{Template: TemplateDemoLead}))
	assert.Equal(t, []string{TemplateDemoAccess, TemplateDemoLead}, transport.templates)
}

// The dispatcher checks a cancelled context only for transports without
// SendContext; the lock always has one, so it checks for the transport.
func TestMailLockSendsNothingAfterCancellation(t *testing.T) {
	t.Parallel()
	transport := &recordingMailer{}
	mailer, ok := RestrictToDemoMails(transport, "demo", nil).(ContextMailer)
	require.True(t, ok)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	require.ErrorIs(t, mailer.SendContext(ctx, Message{Template: TemplateDemoAccess}), context.Canceled)
	assert.Empty(t, transport.templates)
}

func TestIsDemoEnvironment(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"demo", " Demo ", "DEMO"} {
		assert.True(t, IsDemoEnvironment(appEnv), "%q", appEnv)
	}
	for _, appEnv := range []string{"", "development", "test", "staging", "production", "demo2"} {
		assert.False(t, IsDemoEnvironment(appEnv), "%q", appEnv)
	}
}

func TestMailLockHasNoEffectOutsideTheDemoEnvironment(t *testing.T) {
	t.Parallel()
	for _, appEnv := range []string{"", "development", "test", "staging", "production"} {
		transport := &recordingMailer{}
		mailer := RestrictToDemoMails(transport, appEnv, nil)
		assert.Same(t, transport, mailer, "APP_ENV=%q keeps the transport itself", appEnv)
		for _, template := range blockedInTheDemo {
			require.NoError(t, mailer.Send(Message{Template: template}))
		}
		assert.Equal(t, blockedInTheDemo, transport.templates, "APP_ENV=%q", appEnv)
	}
}

// The lock sits on the one transport every sender shares, not on a caller.
func TestNewMailerLocksTheDemoTransport(t *testing.T) {
	t.Parallel()
	config := MailerConfig{Host: "localhost", Port: 1025, TemplateDir: "../templates", AppEnv: "demo"}
	mailer, err := NewMailer(config)
	require.NoError(t, err)
	assert.IsType(t, demoRestrictedMailer{}, mailer)
	// No SMTP server listens here: a blocked mail succeeds only because it
	// never reaches the transport.
	assert.NoError(t, mailer.Send(Message{Template: "invitation.html", To: NewEmail("", "fremd@example.org")}))

	config.AppEnv = "production"
	mailer, err = NewMailer(config)
	require.NoError(t, err)
	assert.IsType(t, &SMTPMailer{}, mailer)
}
