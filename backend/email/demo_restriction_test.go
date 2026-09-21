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

var blockedInTheDemo = []string{"invitation.html", "password-reset.html", "guardian-invitation.html", "mfa-email-code.html", ""}

// The mail lock of the demo environment (#3465): a visitor with administrator
// rights must not reach arbitrary addresses from our sender.
func TestDemoEnvironmentDeliversOnlyTheDemoMails(t *testing.T) {
	t.Parallel()
	transport := &recordingMailer{}
	mailer := RestrictToDemoMails(transport, " Demo ", nil)

	for _, template := range blockedInTheDemo {
		require.NoError(t, mailer.Send(Message{Template: template}), "a dropped mail is no failure: %q", template)
	}
	assert.Empty(t, transport.templates, "invitation, password reset and guardian invitation stay on the server")

	require.NoError(t, mailer.Send(Message{Template: TemplateDemoAccess}))
	contextMailer, ok := mailer.(ContextMailer)
	require.True(t, ok, "synchronous delivery needs the context transport")
	require.NoError(t, contextMailer.SendContext(context.Background(), Message{Template: TemplateDemoLead}))
	assert.Equal(t, []string{TemplateDemoAccess, TemplateDemoLead}, transport.templates)
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
