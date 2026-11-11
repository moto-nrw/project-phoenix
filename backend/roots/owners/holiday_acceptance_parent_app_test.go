package owners

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// acceptanceAccounts knows the accounts in known, by address, and the
// parent accounts among them in guardians; failure fails every lookup.
type acceptanceAccounts struct {
	known     map[string]int64
	guardians map[int64]bool
	failure   error
}

func (a *acceptanceAccounts) FindAccountByEmail(_ context.Context, email string) (identityaccess.Account, error) {
	if a.failure != nil {
		return identityaccess.Account{}, a.failure
	}
	if id, ok := a.known[email]; ok {
		return identityaccess.Account{ID: id, Email: email}, nil
	}
	return identityaccess.Account{}, identityaccess.ErrAccountNotFound
}

func (a *acceptanceAccounts) FindGuardianTenant(_ context.Context, accountID int64) (bool, int64, error) {
	return a.guardians[accountID], 0, nil
}

func renderAcceptance(t *testing.T, accounts HolidayAcceptanceAccounts, parentsURL string, mail enrollmentOwner.HolidayRegistrationAcceptedMail) (*enrollmentOwner.RenderedMail, error) {
	t.Helper()
	raw, err := json.Marshal(mail)
	require.NoError(t, err)
	renderer := holidayAcceptanceParentApp(enrollmentCompose.NewMailRenderers().HolidayRegistrationAccepted, accounts, parentsURL)
	return renderer(context.Background(), enrollmentOwner.MailKindHolidayRegistrationAccepted, raw)
}

// TestHolidayAcceptanceLinksTheParentsPortalOnlyForAParentAccount pins the button
// of the acceptance mail (#6672): a parent with a parent account gets the link to
// the parents portal when the mail is sent, everyone else none, and a link
// in the outbox payload is never trusted.
func TestHolidayAcceptanceLinksTheParentsPortalOnlyForAParentAccount(t *testing.T) {
	t.Parallel()
	// IDs of fakes, no database rows.
	parentID, staffID := int64(41), int64(42)
	accounts := &acceptanceAccounts{
		known:     map[string]int64{"eva@example.org": parentID, "erzieherin@example.org": staffID},
		guardians: map[int64]bool{parentID: true},
	}
	mail := enrollmentOwner.HolidayRegistrationAcceptedMail{
		Recipient: "eva@example.org", GuardianName: "Eva Muster", TraegerName: "Gemeinde Bissendorf", PeriodName: "Weihnachtsferien",
		TraegerText: "Bitte Wechselkleidung mitgeben.",
	}

	rendered, err := renderAcceptance(t, accounts, "https://eltern.example.org/", mail)
	require.NoError(t, err)
	content := rendered.Content.(map[string]any)
	assert.Equal(t, "https://eltern.example.org/", content["ParentAppURL"], "a parent with an account gets the parents portal")
	assert.Equal(t, "Bitte Wechselkleidung mitgeben.", content["TraegerText"])
	assert.Equal(t, "Ferienanmeldung bestätigt: Weihnachtsferien", rendered.Subject)
	assert.Equal(t, "holiday-registration-accepted.html", rendered.Template)

	mail.Recipient, mail.ParentAppURL = "ohne@example.org", "https://evil.example.org/"
	rendered, err = renderAcceptance(t, accounts, "https://eltern.example.org", mail)
	require.NoError(t, err)
	assert.Empty(t, rendered.Content.(map[string]any)["ParentAppURL"], "a parent without an account gets no link, whatever the payload says")

	mail.Recipient = "erzieherin@example.org"
	rendered, err = renderAcceptance(t, accounts, "https://eltern.example.org", mail)
	require.NoError(t, err)
	assert.Empty(t, rendered.Content.(map[string]any)["ParentAppURL"], "an account without the guardian role is no parent account")

	mail.Recipient = "eva@example.org"
	rendered, err = renderAcceptance(t, nil, "https://eltern.example.org", mail)
	require.NoError(t, err)
	assert.Empty(t, rendered.Content.(map[string]any)["ParentAppURL"], "without the account lookup the mail goes out without the link")

	rendered, err = renderAcceptance(t, accounts, "", mail)
	require.NoError(t, err)
	assert.Empty(t, rendered.Content.(map[string]any)["ParentAppURL"], "without a parents portal the mail goes out without the link")

	failing := &acceptanceAccounts{failure: errors.New("database unavailable")}
	_, err = renderAcceptance(t, failing, "https://eltern.example.org", mail)
	require.Error(t, err, "a failed lookup fails the attempt, so the worker tries again")
}
