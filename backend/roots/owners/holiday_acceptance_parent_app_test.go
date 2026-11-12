package owners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	known           map[string]int64
	guardians       map[int64]bool
	failure         error
	guardianFailure error
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
	return a.guardians[accountID], 0, a.guardianFailure
}

func renderAcceptance(t *testing.T, accounts HolidayAcceptanceAccounts, parentsURL string, mail enrollmentOwner.HolidayRegistrationAcceptedMail) (*enrollmentOwner.RenderedMail, error) {
	t.Helper()
	raw, err := json.Marshal(mail)
	require.NoError(t, err)
	renderer := enrollmentCompose.NewMailRenderers(newHolidayAcceptanceAccounts(accounts, parentsURL), parentsURL).HolidayRegistrationAccepted
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

	failing = &acceptanceAccounts{known: accounts.known, guardianFailure: errors.New("guardian lookup unavailable")}
	_, err = renderAcceptance(t, failing, "https://eltern.example.org", mail)
	require.Error(t, err, "a failed guardian lookup also fails the attempt")
}

// setupAccounts are acceptanceAccounts that also serve the setup link of a
// parent account at a Träger and know the accounts with access at one.
type setupAccounts struct {
	*acceptanceAccounts
	traegerParents map[int64]bool
}

func (a setupAccounts) IssueHolidayParentSetupToken(email string, organizationID int64) (string, error) {
	return fmt.Sprintf("setup-%d-%s", organizationID, email), nil
}

func (a setupAccounts) PreviewHolidayParentSetup(context.Context, string) (identityaccess.HolidayParentSetupPreview, error) {
	return identityaccess.HolidayParentSetupPreview{}, errors.New("not served in this test")
}

func (a setupAccounts) CompleteHolidayParentSetup(context.Context, string, identityaccess.GuardianRegistration) (bool, error) {
	return false, errors.New("not served in this test")
}

func (a setupAccounts) HasTraegerParentAccess(_ context.Context, accountID int64) (bool, error) {
	return a.traegerParents[accountID], nil
}

// TestHolidayAcceptanceOffersAParentAccountToEveryoneWithoutOne pins ADR 0216
// point 11: the acceptance mail links the parents portal for an account the
// portal signs in, also one with access at a Träger only, and offers every
// other address the setup of a parent account at the mail's Träger, also for
// a registration from before. A link in the outbox is never trusted, and a
// mail enqueued without its Träger offers no setup.
func TestHolidayAcceptanceOffersAParentAccountToEveryoneWithoutOne(t *testing.T) {
	t.Parallel()
	// IDs of fakes, no database rows.
	traegerParentID, staffID, organizationID := int64(51), int64(52), int64(71)
	accounts := setupAccounts{
		acceptanceAccounts: &acceptanceAccounts{known: map[string]int64{"traeger@example.org": traegerParentID, "erzieherin@example.org": staffID}},
		traegerParents:     map[int64]bool{traegerParentID: true},
	}
	mail := enrollmentOwner.HolidayRegistrationAcceptedMail{
		Recipient: "traeger@example.org", GuardianName: "Jana Gastfamilie", TraegerName: "Gemeinde Bissendorf",
		PeriodName: "Herbstferien", OrganizationID: organizationID, SetupURL: "https://evil.example.org/",
	}

	rendered, err := renderAcceptance(t, accounts, "https://eltern.example.org", mail)
	require.NoError(t, err)
	content := rendered.Content.(map[string]any)
	assert.Equal(t, "https://eltern.example.org/", content["ParentAppURL"], "an account with access at a Träger is a parent account")
	assert.Empty(t, content["SetupURL"])

	for _, recipient := range []string{"ohne@example.org", "erzieherin@example.org"} {
		mail.Recipient = recipient
		rendered, err = renderAcceptance(t, accounts, "https://eltern.example.org/", mail)
		require.NoError(t, err)
		content = rendered.Content.(map[string]any)
		assert.Empty(t, content["ParentAppURL"], recipient)
		assert.Equal(t, "https://eltern.example.org/ferienkonto/setup-71-"+recipient, content["SetupURL"],
			"%s gets the setup of a parent account at the mail's Träger, never the link of the outbox", recipient)
	}

	mail.OrganizationID = 0
	rendered, err = renderAcceptance(t, accounts, "https://eltern.example.org", mail)
	require.NoError(t, err)
	assert.Empty(t, rendered.Content.(map[string]any)["SetupURL"], "a mail enqueued without its Träger offers no setup")
}

// TestHolidayParentAccountOfferIsDecidedAtSending pins ADR 0216 point 4: the
// offer of a parent account is decided when Delivery sends it, by the rule of
// the acceptance mail. An address with a parent account gets the sign-in
// only; every other address the setup link issued at sending, never one from
// the outbox, with the sign-in as a second way.
func TestHolidayParentAccountOfferIsDecidedAtSending(t *testing.T) {
	t.Parallel()
	// IDs of fakes, no database rows.
	guardianID, traegerParentID, organizationID := int64(61), int64(62), int64(73)
	accounts := setupAccounts{
		acceptanceAccounts: &acceptanceAccounts{
			known:     map[string]int64{"eltern@example.org": guardianID, "traeger@example.org": traegerParentID},
			guardians: map[int64]bool{guardianID: true},
		},
		traegerParents: map[int64]bool{traegerParentID: true},
	}
	render := func(recipient string) map[string]any {
		t.Helper()
		raw, err := json.Marshal(map[string]any{
			"recipient_email": recipient, "guardian_name": "Jana Gastfamilie", "traeger_name": "Gemeinde Bissendorf",
			"period_name": "Herbstferien", "organization_id": organizationID, "setup_url": "https://evil.example.org/ferienkonto/alt",
		})
		require.NoError(t, err)
		renderer := enrollmentCompose.NewMailRenderers(newHolidayAcceptanceAccounts(accounts, "https://eltern.example.org/"), "https://eltern.example.org/").HolidayParentAccountOffer
		rendered, err := renderer(context.Background(), enrollmentOwner.MailKindHolidayParentAccountOffer, raw)
		require.NoError(t, err)
		assert.Equal(t, "holiday-parent-account-offer.html", rendered.Template)
		return rendered.Content.(map[string]any)
	}

	for _, recipient := range []string{"eltern@example.org", "traeger@example.org"} {
		content := render(recipient)
		assert.Empty(t, content["SetupURL"], "%s has a parent account: the sign-in only", recipient)
		assert.Equal(t, "https://eltern.example.org/login", content["LoginURL"])
	}
	content := render("neu@example.org")
	assert.Equal(t, "https://eltern.example.org/ferienkonto/setup-73-neu@example.org", content["SetupURL"],
		"an address without an account gets the link issued at sending, never the one of the outbox")
	assert.Equal(t, "https://eltern.example.org/login", content["LoginURL"])

	raw, err := json.Marshal(map[string]any{"recipient_email": "neu@example.org", "organization_id": organizationID})
	require.NoError(t, err)
	_, err = enrollmentCompose.NewMailRenderers(nil, "").HolidayParentAccountOffer(context.Background(), enrollmentOwner.MailKindHolidayParentAccountOffer, raw)
	require.Error(t, err, "without the account lookup and the parents portal the offer is not sent")
}
