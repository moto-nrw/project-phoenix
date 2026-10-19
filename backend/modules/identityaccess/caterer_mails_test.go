package identityaccess_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// ADR 0074: the caterer's invitation links to its school on the caterer
// portal and says how long it is valid.
func TestCatererInvitationMailNamesTheSchoolInItsLink(t *testing.T) {
	t.Parallel()
	queued := identityaccess.CatererInvitationMailOf(" kueche@example.com ", "Küche am Markt", "OGS Beispiel",
		"https://caterer.example/", 42, "abc123", time.Now().Add(14*24*time.Hour+time.Hour))
	assert.Equal(t, "https://caterer.example/einladung/42/abc123", queued.InvitationURL)
	assert.Equal(t, "kueche@example.com", queued.Recipient)
	assert.Equal(t, 14, queued.ExpiryDays)

	mail, err := identityaccess.CatererInvitationMail(queued)
	require.NoError(t, err)
	assert.Equal(t, "Einladung zum Mittagessen bei moto – OGS Beispiel", mail.Subject)
	assert.Equal(t, "caterer-invitation.html", mail.Template)
	assert.Equal(t, queued.InvitationURL, mail.Content.(map[string]any)["InvitationURL"])

	_, err = identityaccess.CatererInvitationMail(identityaccess.CatererMail{InvitationURL: queued.InvitationURL})
	require.Error(t, err, "a mail without recipient is not rendered")
	_, err = identityaccess.CatererInvitationMail(identityaccess.CatererMail{Recipient: queued.Recipient})
	require.Error(t, err, "a mail without link is not rendered")
}
