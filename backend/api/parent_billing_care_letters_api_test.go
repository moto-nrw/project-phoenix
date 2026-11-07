// Router-level tests for the letters of a released monthly run (#4684,
// ADR 0165): the letters once, every family once as a series, one letter
// first or again, the state of the last mail, the print, the Storno before
// and after the send, the new invoice of a cancelled one, the recipient check
// at the send, the right "Abrechnung" and the Träger boundary.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type careLetterDocumentBody struct {
	InvoiceID          int64  `json:"invoice_id"`
	Kind               string `json:"kind"`
	Number             string `json:"number"`
	InvoiceNumber      string `json:"invoice_number"`
	RecipientEmail     string `json:"recipient_email"`
	TotalCents         int64  `json:"total_cents"`
	Cancelled          bool   `json:"cancelled"`
	CancellationNumber string `json:"cancellation_number"`
	HasLetter          bool   `json:"has_letter"`
	Status             string `json:"status"`
	Sends              int    `json:"sends"`
}

type careLettersBody struct {
	Letters *struct {
		PaymentDueOn string `json:"payment_due_on"`
	} `json:"letters"`
	Documents []careLetterDocumentBody `json:"documents"`
	Summary   struct {
		Letters      int `json:"letters"`
		Sent         int `json:"sent"`
		Queued       int `json:"queued"`
		Failed       int `json:"failed"`
		NotSent      int `json:"not_sent"`
		WithoutEmail int `json:"without_email"`
	} `json:"summary"`
}

type careSeriesBody struct {
	Queued           int `json:"queued"`
	Already          int `json:"already"`
	WithoutEmail     int `json:"without_email"`
	RecipientChanged int `json:"recipient_changed"`
}

type careLetterMail struct {
	ID      int64           `bun:"id"`
	Payload json.RawMessage `bun:"payload"`
}

// releasedCareRun releases a run of the SJR month with one invoice per
// child; Noah's family has no address.
func releasedCareRun(t *testing.T, f careBillingFixture, children ...string) (careRunBody, map[string]int64) {
	t.Helper()
	school := f.office.schools[0]
	contracts := map[string]int64{}
	for _, child := range children {
		contracts[child] = careContract(t, f.db, f.office.organization, school, child, careMonth, sjrServices()...)
	}
	if noah, ok := contracts["Noah"]; ok {
		_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = '' WHERE contract_id = ?`, noah)
		require.NoError(t, err)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	preview := f.run(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()), contracts
}

func careLetters(t *testing.T, f careBillingFixture, runID int64) careLettersBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(runID, "/letters"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careLettersBody](t, rec.Body.Bytes())
}

func careLetterOf(body careLettersBody, contract careInvoiceBody, kind string) careLetterDocumentBody {
	for _, document := range body.Documents {
		if document.InvoiceID == contract.ID && document.Kind == kind {
			return document
		}
	}
	return careLetterDocumentBody{}
}

func invoiceOfContract(t *testing.T, run careRunBody, contractID int64) careInvoiceBody {
	t.Helper()
	for _, invoice := range run.Invoices {
		if invoice.ContractID == contractID {
			return invoice
		}
	}
	require.FailNow(t, "no invoice for the contract")
	return careInvoiceBody{}
}

func careLetterMails(t *testing.T, f careBillingFixture, runID int64) []careLetterMail {
	t.Helper()
	var mails []careLetterMail
	require.NoError(t, f.db.NewRaw(`SELECT id, payload FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'parent_billing_letter' AND related_entity_type = 'parent_billing_care_run'
			AND related_entity_id = ? ORDER BY id`, f.office.organization, runID).Scan(context.Background(), &mails))
	return mails
}

func careInboxPushes(t *testing.T, f careBillingFixture) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM platform.push_outbox
		WHERE organization_id = ? AND related_entity_type = 'parent_billing_document'`, f.office.organization).
		Scan(context.Background(), &count))
	return count
}

// brandCareTraeger gives the Träger its letterhead and bank account through a
// colleague with the right "Verwaltung"; the office itself only bills.
func brandCareTraeger(t *testing.T, f careBillingFixture) {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	administration := f.office
	administration.token = answer.AccessToken
	brandTraegerOffice(t, f.db, f.traeger, administration)
}

func careLettersInput() map[string]any {
	return map[string]any{"payment_due_on": "2099-01-15", "notes": "Am 2. Januar ist die Betreuung geschlossen."}
}

// The letters go out once per family, show their state, print together and
// a failed mail is sent again; the family without an address prints.
func TestCareLettersSendEveryFamilyOnceAndShowTheirState(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsschreiben")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia", "Noah")
	mia, noah := invoiceOfContract(t, run, contracts["Mia"]), invoiceOfContract(t, run, contracts["Noah"])

	before := careLetters(t, f, run.ID)
	assert.Nil(t, before.Letters, "no letters before they are created")
	assert.False(t, careLetterOf(before, mia, "invoice").HasLetter)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "no mail before the letters: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/preview"), careLettersInput())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	assert.Equal(t, http.StatusConflict, rec.Code, "a run has its letters once: %s", rec.Body.String())
	assert.Zero(t, careInboxPushes(t, f), "a created letter is not in the inbox yet")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/letter", mia.ID)), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{Queued: 1, WithoutEmail: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
	mails := careLetterMails(t, f, run.ID)
	require.Len(t, mails, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, "care_invoice", payload["document_kind"])
	assert.Equal(t, "Monatsbeitrag Januar 2030", payload["period_name"])
	assert.Equal(t, mia.Number, payload["number"])
	assert.Equal(t, 1, careInboxPushes(t, f), "the first send announces the letter in the inbox")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{Already: 1, WithoutEmail: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()),
		"a second series sends nothing twice")
	oneMail := careRunPath(run.ID, fmt.Sprintf("/invoices/%d/mail", mia.ID))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, oneMail, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a waiting mail is not sent again: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/mail", noah.ID)), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a family without an address prints: %s", rec.Body.String())

	state := careLetters(t, f, run.ID)
	assert.Equal(t, "queued", careLetterOf(state, mia, "invoice").Status)
	assert.Equal(t, "not_sent", careLetterOf(state, noah, "invoice").Status)
	assert.Equal(t, 2, state.Summary.Letters)
	assert.Equal(t, 1, state.Summary.Queued)
	assert.Equal(t, 1, state.Summary.WithoutEmail)

	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter' WHERE id = ?`, mails[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", careLetterOf(careLetters(t, f, run.ID), mia, "invoice").Status)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, oneMail, nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "a failed mail is sent again: %s", rec.Body.String())
	assert.Len(t, careLetterMails(t, f, run.ID), 2)
	assert.Equal(t, 1, careInboxPushes(t, f), "only the first send pushes")
	assert.Equal(t, 2, careLetterOf(careLetters(t, f, run.ID), mia, "invoice").Sends)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/letters/print"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/letters/print?only=without_email"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT action FROM parent_billing.care_run_events WHERE run_id = ? ORDER BY action`, run.ID).
		Scan(context.Background(), &actions))
	assert.Equal(t, []string{"create", "letters", "mail", "read", "release"}, actions)
}

// A Storno of an invoice never sent writes nothing to the family; the next
// run of the month issues the invoice anew and names the cancelled one.
func TestCareInvoiceCancelledBeforeTheSendReachesNoFamilyAndIsIssuedAgain(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsstorno")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia", "Lea")
	mia, lea := invoiceOfContract(t, run, contracts["Mia"]), invoiceOfContract(t, run, contracts["Lea"])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	cancel := careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", mia.ID))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, cancel, map[string]any{"reason": " "})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a Storno needs a reason: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, cancel, map[string]any{"reason": "Falscher Vertrag"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	letters := decodeSchoolPortalData[careLettersBody](t, rec.Body.Bytes())
	assert.True(t, careLetterOf(letters, mia, "invoice").Cancelled)
	storno := careLetterOf(letters, mia, "cancellation")
	assert.Regexp(t, `^MB-\d{4}-0003$`, storno.Number, "the Storno takes the next number of the invoices")
	assert.Equal(t, -mia.TotalCents, storno.TotalCents)
	assert.False(t, storno.HasLetter, "a Storno of an invoice never sent has no letter")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, cancel, map[string]any{"reason": "Noch einmal"})
	assert.Equal(t, http.StatusConflict, rec.Code, "an invoice is cancelled once: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, 1, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()).Queued, "only the invoice still valid goes out")
	mails := careLetterMails(t, f, run.ID)
	require.Len(t, mails, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, lea.Number, payload["number"], "the cancelled invoice and its Storno never reach the family")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/mail", mia.ID)), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a cancelled invoice is not sent: %s", rec.Body.String())
	assert.Equal(t, 1, careInboxPushes(t, f))

	// The next run of the month bills Mia again and names the cancelled invoice.
	next := f.startRun(t, careMonth)
	preview := f.run(t, next.ID)
	require.Len(t, preview.Invoices, 1)
	assert.Equal(t, contracts["Mia"], preview.Invoices[0].ContractID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissued := decodeSchoolPortalData[careRunBody](t, rec.Body.Bytes()).Invoices[0]
	var replaces int64
	require.NoError(t, f.db.NewRaw(`SELECT replaces_invoice_id FROM parent_billing.care_invoices WHERE id = ?`, reissued.ID).
		Scan(context.Background(), &replaces))
	assert.Equal(t, mia.ID, replaces)
	assert.Empty(t, f.run(t, f.startRun(t, careMonth).ID).Invoices, "the new invoice bills the contract again, once")
}

// A Storno of a sent invoice gets its letter at once, which the next series
// sends; the cancelled invoice is not sent again.
func TestCareInvoiceCancelledAfterTheSendGetsItsLetter(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsstorno gesendet")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia")
	mia := invoiceOfContract(t, run, contracts["Mia"])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", mia.ID)),
		map[string]any{"reason": "Vertrag endete im Dezember"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	storno := careLetterOf(decodeSchoolPortalData[careLettersBody](t, rec.Body.Bytes()), mia, "cancellation")
	assert.True(t, storno.HasLetter, "the family saw the invoice, so it gets the Storno")
	assert.Equal(t, "not_sent", storno.Status)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, fmt.Sprintf("/invoices/%d/letter?kind=cancellation", mia.ID)), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, 1, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()).Queued)
	mails := careLetterMails(t, f, run.ID)
	require.Len(t, mails, 2)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(mails[1].Payload, &payload))
	assert.Equal(t, "care_cancellation", payload["document_kind"])
	assert.Equal(t, storno.Number, payload["number"])
	assert.Equal(t, 2, careInboxPushes(t, f), "the Storno is a letter of its own in the inbox")
}

// A send goes only to the address that is still the party the contract
// bills; a changed party waits for a Storno.
func TestCareLetterGoesOnlyToThePartyTheContractStillBills(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsempfänger")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia")
	mia := invoiceOfContract(t, run, contracts["Mia"])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'neu@example.org' WHERE contract_id = ?`,
		contracts["Mia"])
	require.NoError(t, err)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/mail", mia.ID)), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_recipient_changed")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{RecipientChanged: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
	assert.Empty(t, careLetterMails(t, f, run.ID))
	assert.Zero(t, careInboxPushes(t, f))
}

// Every letter route needs the right "Abrechnung" and stays with the Träger.
func TestCareLettersNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsschreiben Recht")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia")
	mia := invoiceOfContract(t, run, contracts["Mia"])
	routes := []struct{ method, path string }{
		{http.MethodGet, "/letters"}, {http.MethodPost, "/letters"}, {http.MethodPost, "/letters/preview"},
		{http.MethodGet, "/letters/print"}, {http.MethodPost, "/letters/mail"},
		{http.MethodGet, fmt.Sprintf("/invoices/%d/letter", mia.ID)}, {http.MethodPost, fmt.Sprintf("/invoices/%d/mail", mia.ID)},
		{http.MethodPost, fmt.Sprintf("/invoices/%d/cancellation", mia.ID)},
	}
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	stranger := newTraegerOffice(t, f.db, f.traeger, "Träger Monatsschreiben Fremd", identityaccess.TraegerRoleBilling)
	body := map[string]any{"payment_due_on": "2099-01-15", "reason": "Falscher Vertrag"}
	for _, route := range routes {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, route.method, careRunPath(run.ID, route.path), body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", route.method, route.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, stranger.token, route.method, careRunPath(run.ID, route.path), body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", route.method, route.path, rec.Body.String())
	}
	assert.Equal(t, 1, len(careLetters(t, f, run.ID).Documents), "nothing changed through the refused routes")
}

// The letters of a run read with a flat number of statements.
func TestCareLettersQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsschreiben Budget")
	brandCareTraeger(t, f)
	run, _ := releasedCareRun(t, f, "Mia", "Lea", "Ben")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	counter := testpkg.CaptureQueries(t, f.db)
	careLetters(t, f, run.ID)
	querybudgets.AssertQueryBudget(t, "api.traeger.care_billing_run_letters", counter.Queries())
}
