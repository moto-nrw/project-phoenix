package auth_test

import (
	"bytes"
	"context"
	"html/template"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/email"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// An address that already owns an account gets the invitation mail variant
// without registration: the button leads to the parents portal login and
// the text asks for the existing credentials (#3320).
func TestEnqueueExistingAccountEmail_RendersPortalLoginHint(t *testing.T) {
	t.Parallel()
	outbox := &stubOutboxEnqueuer{}
	mailer := services.NewGuardianInvitationMailer(services.GuardianInvitationMailerConfig{
		Outbox:      outbox,
		FrontendURL: "https://eltern.example.test/",
	})

	mailer.EnqueueExistingAccount(context.Background(), services.GuardianMailRecipient{
		FirstName: " Olga ", LastName: "Muster", Email: " admin@example.test ",
	}, "OGS Musterschule")

	require.Len(t, outbox.requests, 1)
	req := outbox.requests[0]
	assert.Equal(t, platformModels.EmailKindGuardianInvitation, req.Kind)
	assert.Equal(t, "admin@example.test", req.Payload["recipient_email"])
	assert.Equal(t, "https://eltern.example.test/login", req.Payload["invitation_url"])
	assert.Equal(t, true, req.Payload["existing_account"])

	render := services.NewGuardianInvitationRenderer(services.GuardianInvitationRendererConfig{})
	msg, err := render(context.Background(), req.Payload)
	require.NoError(t, err)
	assert.Equal(t, "Ihr Zugang zum Eltern-Portal – OGS Musterschule", msg.Subject)
	assert.Equal(t, "guardian-invitation.html", msg.Template)

	body := renderGuardianTemplate(t, msg)
	assert.Contains(t, body, "Ihr Zugang zum Eltern-Portal")
	assert.Contains(t, body, "Olga Muster")
	assert.Contains(t, body, "bisherigen Zugangsdaten")
	assert.Contains(t, body, `href="https://eltern.example.test/login"`)
	assert.NotContains(t, body, "Einladung annehmen", "no registration for an existing account")
	assert.NotContains(t, body, "läuft in", "the login link does not expire")
}

func TestEnqueueExistingAccountEmail_SkipsWithoutAddress(t *testing.T) {
	t.Parallel()
	outbox := &stubOutboxEnqueuer{}
	mailer := services.NewGuardianInvitationMailer(services.GuardianInvitationMailerConfig{Outbox: outbox})

	mailer.EnqueueExistingAccount(context.Background(), services.GuardianMailRecipient{Email: "  "}, "")
	assert.Empty(t, outbox.requests)
}

// The token invitation keeps its registration wording.
func TestGuardianInvitationRenderer_TokenInvitationUnchanged(t *testing.T) {
	t.Parallel()
	render := services.NewGuardianInvitationRenderer(services.GuardianInvitationRendererConfig{})
	msg, err := render(context.Background(), map[string]any{
		"recipient_email": "new@example.test",
		"invitation_url":  "https://eltern.example.test/accept-guardian-invite/abc",
		"expiry_hours":    float64(48),
	})
	require.NoError(t, err)
	assert.Equal(t, "Einladung zum Eltern-Portal", msg.Subject)

	body := renderGuardianTemplate(t, msg)
	assert.Contains(t, body, "Einladung annehmen")
	assert.Contains(t, body, "48 Stunden")
	assert.NotContains(t, body, "bisherigen Zugangsdaten")
}

func renderGuardianTemplate(t *testing.T, msg *email.Message) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "..", "templates", "email"))
	require.NoError(t, err)
	tpl, err := template.ParseGlob(filepath.Join(dir, "*.html"))
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, tpl.ExecuteTemplate(&buf, msg.Template, msg.Content))
	return buf.String()
}
