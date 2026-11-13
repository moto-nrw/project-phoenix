// Router-level tests of the parent account at the public holiday
// registration (ADR 0216 points 2 and 4): a sealed proof of the prefill
// links the registration to the account, a parent without one may ask for
// an account and gets the offer in the registration's Träger transaction,
// and the answer is the same in every case.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// holidayTestSetupLinks signs a recognizable setup token; Identity & Access
// tests the real signature.
type holidayTestSetupLinks struct{}

func (holidayTestSetupLinks) IssueHolidayParentSetupToken(email string, organizationID int64) (string, error) {
	return "setup-" + email, nil
}

func (holidayTestSetupLinks) PreviewHolidayParentSetup(context.Context, string) (identityaccess.HolidayParentSetupPreview, error) {
	return identityaccess.HolidayParentSetupPreview{}, errors.New("not served in this test")
}

func (holidayTestSetupLinks) CompleteHolidayParentSetup(context.Context, string, identityaccess.GuardianRegistration) (bool, error) {
	return false, errors.New("not served in this test")
}

func (holidayTestSetupLinks) HasTraegerParentAccess(context.Context, int64) (bool, error) {
	return false, errors.New("not served in this test")
}

type holidayAccountRow struct {
	ID        int64  `bun:"id"`
	AccountID *int64 `bun:"parent_account_id"`
	Requested bool   `bun:"parent_account_requested"`
}

type holidayOfferMailRow struct {
	RelatedID      int64           `bun:"related_entity_id"`
	IdempotencyKey string          `bun:"idempotency_key"`
	Recipient      json.RawMessage `bun:"recipient,type:jsonb"`
	Payload        json.RawMessage `bun:"payload,type:jsonb"`
}

func holidayRegistrationAccounts(t *testing.T, f holidayRegistrationFixture, organization int64) []holidayAccountRow {
	t.Helper()
	var rows []holidayAccountRow
	require.NoError(t, f.db.NewRaw(`SELECT id, parent_account_id, parent_account_requested FROM enrollment.holiday_registrations
		WHERE organization_id = ? ORDER BY id`, organization).Scan(context.Background(), &rows))
	return rows
}

func holidayOfferMails(t *testing.T, f holidayRegistrationFixture, organization int64) []holidayOfferMailRow {
	t.Helper()
	var rows []holidayOfferMailRow
	require.NoError(t, f.db.NewRaw(`SELECT related_entity_id, idempotency_key, recipient, payload FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'holiday_parent_account_offer' ORDER BY id`, organization).Scan(context.Background(), &rows))
	return rows
}

// submitWithAccount posts the siblings' registration with the account
// fields and returns the answer's keys.
func submitWithAccount(t *testing.T, f holidayRegistrationFixture, token string, offer holidayOfferBody, email string, fields map[string]any) []string {
	t.Helper()
	body := siblingsRegistration(offer, email)
	for key, value := range fields {
		body[key] = value
	}
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(token), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var receipt struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	keys := keysOf(receipt.Data)
	slices.Sort(keys)
	return keys
}

func TestHolidayRegistrationAskingForAnAccountMailsTheOfferWithTheRegistration(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Angebot", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)

	plain := submitWithAccount(t, f, period.PublicToken, offer, "eva.ohne@example.org", nil)
	asked := submitWithAccount(t, f, period.PublicToken, offer, "Eva.Konto@example.org", map[string]any{"parent_account_requested": true})
	assert.Equal(t, plain, asked, "the answer does not tell whether an account was asked for")

	rows := holidayRegistrationAccounts(t, f, office.organization)
	require.Len(t, rows, 2)
	assert.False(t, rows[0].Requested)
	assert.True(t, rows[1].Requested)
	assert.Nil(t, rows[0].AccountID)
	assert.Nil(t, rows[1].AccountID, "asking links no account")

	mails := holidayOfferMails(t, f, office.organization)
	require.Len(t, mails, 1, "exactly one offer, for the registration that asked")
	assert.Equal(t, rows[1].ID, mails[0].RelatedID)
	assert.Equal(t, "holiday-parent-account-offer:"+jsonInt(rows[1].ID), mails[0].IdempotencyKey, "one offer per registration")
	assert.Contains(t, string(mails[0].Recipient), "Eva.Konto@example.org")
	var payload map[string]any
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, jsonInt(office.organization), jsonInt(int64(payload["organization_id"].(float64))), "the offer names its Träger")
	assert.NotContains(t, payload, "setup_url", "the outbox holds no setup link: the mail issues it when it is sent")
	assert.NotContains(t, payload, "login_url")
	assert.NotContains(t, string(mails[0].Payload), "ferienkonto", "no credential in the outbox")
	assert.Equal(t, "Träger Konto Angebot", payload["traeger_name"])
	assert.Equal(t, "Eva Muster", payload["guardian_name"])
	assert.NotContains(t, string(mails[0].Payload), "Lia", "the offer names no child")
}

func TestHolidayRegistrationWithTheAccountProofLinksTheAccount(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Nachweis", identityaccess.TraegerRoleAdministration)
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Fremd", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	otherPeriod := openHolidays(t, f, other)
	offer := holidayOffer(t, f, period.PublicToken)
	account := testpkg.CreateTestAccount(t, f.db, "holiday-proof-account")
	proofs := enrollmentCompose.NewHolidayParentAccountProofs(testHealthCipher())
	proof, err := proofs.HolidayParentAccountProof(office.organization, period.ID, account.ID)
	require.NoError(t, err)
	foreignTraeger, err := proofs.HolidayParentAccountProof(other.organization, otherPeriod.ID, account.ID)
	require.NoError(t, err)
	otherPeriodProof, err := proofs.HolidayParentAccountProof(office.organization, period.ID+1000000, account.ID)
	require.NoError(t, err)
	tampered := []byte(proof)
	tampered[len(tampered)/2] ^= 0x01

	keys := submitWithAccount(t, f, period.PublicToken, offer, "eva.proof@example.org", map[string]any{"parent_account_proof": proof, "parent_account_requested": true})
	for _, invalid := range []string{foreignTraeger, otherPeriodProof, string(tampered), "not-a-proof"} {
		assert.Equal(t, keys, submitWithAccount(t, f, period.PublicToken, offer, "eva.proof@example.org", map[string]any{"parent_account_proof": invalid}),
			"an invalid proof is no error and changes the answer in nothing")
	}

	rows := holidayRegistrationAccounts(t, f, office.organization)
	require.Len(t, rows, 5)
	require.NotNil(t, rows[0].AccountID)
	assert.Equal(t, account.ID, *rows[0].AccountID, "the proof links the registration to the account")
	for _, row := range rows[1:] {
		assert.Nil(t, row.AccountID, "a proof of another Träger, another period, a tampered or a garbage proof links nothing")
	}
	assert.Empty(t, holidayOfferMails(t, f, office.organization), "a linked registration gets no offer")
}

// The link to the parent account lives in the registration's row and goes
// with it when the retention deletes the ended period's registrations (ADR
// 0216 point 10, ADR 0118); the account stays.
func TestHolidayRegistrationRetentionTakesTheAccountLinkAlong(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Löschfrist", identityaccess.TraegerRoleAdministration)
	body := januaryHolidays(office)
	body.RegistrationOpensOn, body.RegistrationClosesOn = "2020-01-01", "2099-12-31"
	body.Weeks[0].StartsOn, body.Weeks[0].EndsOn = "2020-07-06", "2020-07-10"
	body.Weeks[1].StartsOn, body.Weeks[1].EndsOn = "2020-07-13", "2020-07-17"
	created := createHolidayPeriod(t, f.traeger, office, body)
	rec := traegerJSON(t, f.traeger, office.token, http.MethodPost, holidayPeriodPath(created.ID)+"/publish", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	period := decodeSchoolPortalData[traegerPeriodLink](t, rec.Body.Bytes())
	account := testpkg.CreateTestAccount(t, f.db, "holiday-retention-account")
	proof, err := enrollmentCompose.NewHolidayParentAccountProofs(testHealthCipher()).HolidayParentAccountProof(office.organization, period.ID, account.ID)
	require.NoError(t, err)
	submitWithAccount(t, f, period.PublicToken, holidayOffer(t, f, period.PublicToken), "frist.konto@example.org", map[string]any{"parent_account_proof": proof})
	require.Len(t, holidayRegistrationAccounts(t, f, office.organization), 1)

	ctx := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinAdmin(ctx, func(adminCtx context.Context) error {
		_, err := owners.NewHolidayRegistrationRetention(nil).DeleteExpiredHolidayRegistrations(adminCtx)
		return err
	}))

	assert.Empty(t, holidayRegistrationAccounts(t, f, office.organization), "the registration goes with its link")
	var accounts int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM auth.accounts WHERE id = ?`, account.ID).Scan(context.Background(), &accounts))
	assert.Equal(t, 1, accounts, "the account follows the rules of parent accounts, not the registration")
}

// submitAsSeeder posts the registration with the seed header to host and
// returns the answer's data.
func submitAsSeeder(t *testing.T, router http.Handler, token, host string, body map[string]any) map[string]any {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, registrationPath(token), bytes.NewReader(payload))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Phoenix-Seed-Token", "true")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var receipt struct {
		Data map[string]any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &receipt))
	return receipt.Data
}

// Only the local seeder receives the setup link of the offer in the answer,
// like the invitation tokens it redeems: the seed header on a local host in a
// development, test or demo environment, and only when an offer went out.
// The header on another host, or a registration without an offer, answers
// as every other caller.
func TestHolidayRegistrationHandsTheSetupLinkOnlyToTheLocalSeeder(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Seeder", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	offer := holidayOffer(t, f, period.PublicToken)
	asking := func(email string) map[string]any {
		body := siblingsRegistration(offer, email)
		body["parent_account_requested"] = true
		return body
	}

	local := submitAsSeeder(t, f.public, period.PublicToken, "localhost:8080", asking("seed.gast@example.org"))
	assert.Equal(t, "https://eltern.example.org/ferienkonto/setup-seed.gast@example.org", local["parent_account_setup_url"])

	remote := submitAsSeeder(t, f.public, period.PublicToken, "eltern.example.org", asking("fremd.gast@example.org"))
	assert.NotContains(t, remote, "parent_account_setup_url", "the seed header on another host exposes nothing")
	withoutOffer := submitAsSeeder(t, f.public, period.PublicToken, "localhost:8080", siblingsRegistration(offer, "ohne.angebot@example.org"))
	assert.NotContains(t, withoutOffer, "parent_account_setup_url", "no offer, no link")
	assert.ElementsMatch(t, []string{"id", "submitted_at", "placements", "status_token"}, slices.Collect(maps.Keys(remote)))
	assert.ElementsMatch(t, []string{"id", "submitted_at", "placements", "status_token"}, slices.Collect(maps.Keys(withoutOffer)))
	offers := holidayOfferMails(t, f, office.organization)
	assert.Len(t, offers, 2, "both registrations that asked got their offer")
	for _, offerMail := range offers {
		assert.NotContains(t, string(offerMail.Payload), "ferienkonto", "the link the seeder receives is stored nowhere")
	}
}

// In production the seed header exposes nothing, even on a local host.
func TestHolidayRegistrationHandsNoSetupLinkInProduction(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixtureIn(t, owners.HolidayTestSchoolCalendar{}, "production")
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Konto Produktion", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	body := siblingsRegistration(holidayOffer(t, f, period.PublicToken), "prod.gast@example.org")
	body["parent_account_requested"] = true

	data := submitAsSeeder(t, f.public, period.PublicToken, "localhost:8080", body)

	assert.ElementsMatch(t, []string{"id", "submitted_at", "placements", "status_token"}, slices.Collect(maps.Keys(data)))
	assert.Len(t, holidayOfferMails(t, f, office.organization), 1, "the offer still goes out by mail")
}
