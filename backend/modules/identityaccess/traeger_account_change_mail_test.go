package identityaccess_test

import (
	"bytes"
	"html/template"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// The notice of a Träger role change or removal (#5474) names who acted and
// when, in Berlin time, the roles before and after and whom to ask. A change
// made with the person's own account says so instead of naming the address.
func TestTraegerAccountChangeMailRendersBothChanges(t *testing.T) {
	t.Parallel()
	// The SMTP mailer parses every shipped template without functions.
	templates, err := template.ParseGlob(filepath.Join("..", "..", "templates", "email", "*.html"))
	require.NoError(t, err)
	render := func(mail identityaccess.Mail) string {
		t.Helper()
		var out bytes.Buffer
		require.NoError(t, templates.ExecuteTemplate(&out, mail.Template, mail.Content))
		return out.String()
	}
	notice := identityaccess.TraegerAccountChangeNotice{
		TraegerName: "Träger Nord", Action: identityaccess.TraegerAccountRolesChanged,
		TargetAccountID: 21, TargetEmail: "kollegin@example.test", ActorAccountID: 11, ActorEmail: "leitung@example.test",
		RolesBefore: []string{identityaccess.TraegerRoleReadOnly},
		RolesAfter:  []string{identityaccess.TraegerRoleBilling, identityaccess.TraegerRoleReadOnly},
		OccurredAt:  time.Date(2026, time.November, 2, 22, 30, 0, 0, time.UTC),
	}

	changed := identityaccess.TraegerAccountChangeMail(notice, "https://moto.example")
	assert.Equal(t, "Ihre Rollen im Träger-Büro haben sich geändert – Träger Nord", changed.Subject)
	body := render(changed)
	assert.Contains(t, body, "<strong>leitung@example.test</strong> hat am 02.11.2026 um 23:30 Uhr Ihre Rollen")
	assert.Contains(t, body, "Vorher: <strong>Nur lesen</strong>")
	assert.Contains(t, body, "Jetzt: <strong>Abrechnung und Nur lesen</strong>")
	assert.Contains(t, body, "wenden Sie sich an leitung@example.test")
	assert.Contains(t, body, "mailto:kontakt@moto-ogs.de")

	notice.Action, notice.RolesAfter = identityaccess.TraegerAccountMemberRemoved, nil
	notice.ActorAccountID, notice.ActorEmail = notice.TargetAccountID, notice.TargetEmail
	removed := identityaccess.TraegerAccountChangeMail(notice, "https://moto.example")
	assert.Equal(t, "Sie sind nicht mehr im Träger-Büro – Träger Nord", removed.Subject)
	body = render(removed)
	assert.Contains(t, body, "Das geschah am 02.11.2026 um 23:30 Uhr mit Ihrem eigenen Konto.")
	assert.Contains(t, body, "Bisher: <strong>Nur lesen</strong>")
	assert.Contains(t, body, "Sie können sich im Träger-Portal nicht mehr anmelden.")
	assert.NotContains(t, body, "wenden Sie sich an kollegin@example.test", "a change of one's own names nobody to ask")
}
