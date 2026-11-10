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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

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
	InInbox            bool   `json:"in_inbox"`
	Printed            bool   `json:"printed"`
	Replaced           bool   `json:"replaced"`
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
		WithoutInbox int `json:"without_inbox"`
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
	testpkg.OwnTraegerMasterData(t, f.db, f.office.organization)
	brandTraegerOffice(t, f.traeger, administration)
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
	assertInvoiceMailHistory(t, f.traeger, f.office.token, "care-invoices", mia.ID, 1, 0)
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
	// The fixture's contracts name no child record: the office learns that
	// both letters reach the family by mail or on paper only.
	assert.False(t, careLetterOf(state, mia, "invoice").InInbox)
	assert.Equal(t, 2, state.Summary.WithoutInbox)

	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter' WHERE id = ?`, mails[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", careLetterOf(careLetters(t, f, run.ID), mia, "invoice").Status)
	assertInvoiceMailHistory(t, f.traeger, f.office.token, "care-invoices", mia.ID, 1, 0)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, oneMail, nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "a failed mail is sent again: %s", rec.Body.String())
	assert.Len(t, careLetterMails(t, f, run.ID), 2)
	assert.Equal(t, 1, careInboxPushes(t, f), "only the first send pushes")
	assert.Equal(t, 2, careLetterOf(careLetters(t, f, run.ID), mia, "invoice").Sends)
	_, err = f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW() WHERE id = ?`, careLetterMails(t, f, run.ID)[1].ID)
	require.NoError(t, err)
	assertInvoiceMailHistory(t, f.traeger, f.office.token, "care-invoices", mia.ID, 2, 1)

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
	assert.False(t, storno.Replaced, "a cancelled invoice still needs a replacement before release")
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
	assert.True(t, careLetterOf(careLetters(t, f, run.ID), mia, "cancellation").Replaced, "the old run must stop offering reissue after release")
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

// Both send paths use the effective recipient and still refuse changed payers.
func TestCareLettersResolveThePayerAndRefuseAChangedPayer(t *testing.T) {
	t.Parallel()
	for _, bulk := range []bool{false, true} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("bulk=%t/changed=%t", bulk, changed), func(t *testing.T) {
				f := newCareBillingFixture(t, "Träger Zahlerpost")
				brandCareTraeger(t, f)
				_, contracts := releasedCareRun(t, f, "Mia")
				input := map[string]any{
					"contract_id": contracts["Mia"], "recipient_kind": "payer",
					"payer_first_name": "Oma", "payer_last_name": "Muster", "payer_email": "oma@example.org",
					"payer_address": map[string]any{"street": "Amtsweg 2", "postal_code": "75175", "city": "Pforzheim"},
				}
				rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, input)
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				run := f.startRun(t, "2030-02-01")
				preview := f.run(t, run.ID)
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				invoice := invoiceOfContract(t, f.run(t, run.ID), contracts["Mia"])
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
				require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
				if changed {
					input["payer_email"] = "neu@example.org"
					rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, input)
					require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				}
				readsOf := func() int {
					var reads int
					require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing_events
						WHERE care_contract_id = ? AND action = 'read'`, contracts["Mia"]).Scan(context.Background(), &reads))
					return reads
				}
				before := readsOf()
				path := fmt.Sprintf("/invoices/%d/mail", invoice.ID)
				if bulk {
					path = "/letters/mail"
				}
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, path), nil)
				if changed && !bulk {
					assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
					assert.Contains(t, rec.Body.String(), "parent_billing.care_recipient_changed")
				} else {
					require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
				}
				if bulk {
					want := careSeriesBody{Queued: 1}
					if changed {
						want = careSeriesBody{RecipientChanged: 1}
					}
					assert.Equal(t, want, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
				}
				if !changed || bulk {
					assert.Equal(t, before+1, readsOf(), "the send audits its read of the family deviation")
				}
				mails := careLetterMails(t, f, run.ID)
				if changed {
					assert.Empty(t, mails)
				} else {
					require.Len(t, mails, 1)
					var email string
					require.NoError(t, f.db.NewRaw(`SELECT recipient->>'address' FROM platform.email_outbox WHERE id = ?`, mails[0].ID).
						Scan(context.Background(), &email))
					assert.Equal(t, "oma@example.org", email)
				}
			})
		}
	}
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

// The letters of a run read with a flat number of statements: a run of one
// invoice and a later run of the month with two read the same.
func TestCareLettersQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsschreiben Budget")
	brandCareTraeger(t, f)
	first, _ := releasedCareRun(t, f, "Mia")
	school := f.office.schools[0]
	for _, child := range []string{"Lea", "Ben"} {
		careContract(t, f.db, f.office.organization, school, child, careMonth, sjrServices()...)
	}
	second := f.startRun(t, careMonth)
	preview := f.run(t, second.ID)
	require.Len(t, preview.Invoices, 2)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(second.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	read := func(runID int64) testpkg.QueryMeasurement {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/letters"), careLettersInput())
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/letters/mail"), nil)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, careRunPath(runID, "/letters"), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		recorder := httptest.NewRecorder()
		f.traeger.ServeHTTP(recorder, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := read(first.ID)
	assert.Len(t, read(second.ID).Statements, len(one.Statements), "the letters read the same statements for any number of invoices")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_billing_run_letters", one)
}

// Printing may already have put the invoice in the family's hands. Its Storno
// must therefore be printable, without publishing either letter to the inbox.
func TestCareInvoiceCancelledAfterPrintingGetsAPaperNotice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Papierstorno")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Noah", "Mia")
	noah, mia := invoiceOfContract(t, run, contracts["Noah"]), invoiceOfContract(t, run, contracts["Mia"])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/letters/print?only=without_email"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	printed := careLetters(t, f, run.ID)
	assert.True(t, careLetterOf(printed, noah, "invoice").Printed)
	assert.False(t, careLetterOf(printed, mia, "invoice").Printed)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", noah.ID)),
		map[string]any{"reason": "Vertrag endete im Dezember"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	letters := decodeSchoolPortalData[careLettersBody](t, rec.Body.Bytes())
	assert.True(t, careLetterOf(letters, noah, "cancellation").HasLetter, "the printed invoice needs a printable Storno")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careRunPath(run.ID, fmt.Sprintf("/invoices/%d/letter?kind=cancellation", noah.ID)), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, "/letters/print?only=without_email"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, careLetterMails(t, f, run.ID))
	assert.Zero(t, careInboxPushes(t, f))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", mia.ID)),
		map[string]any{"reason": "Falscher Vertrag"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.False(t, careLetterOf(decodeSchoolPortalData[careLettersBody](t, rec.Body.Bytes()), mia, "cancellation").HasLetter,
		"the print filter must not mark the other invoice as printed")
}

// A letter opened alone may have been printed and handed over, like one of
// the print of the run: its Storno gets a letter even though no mail went out.
func TestCareInvoiceOpenedAloneGetsTheLetterOfItsStorno(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsstorno geöffnet")
	brandCareTraeger(t, f)
	run, contracts := releasedCareRun(t, f, "Mia", "Lea")
	mia, lea := invoiceOfContract(t, run, contracts["Mia"]), invoiceOfContract(t, run, contracts["Lea"])
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/letter", mia.ID)), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	for _, invoice := range []careInvoiceBody{mia, lea} {
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", invoice.ID)),
			map[string]any{"reason": "Falscher Vertrag"})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	}
	letters := careLetters(t, f, run.ID)
	assert.True(t, careLetterOf(letters, mia, "cancellation").HasLetter, "the opened letter may be on paper at the family")
	assert.False(t, careLetterOf(letters, lea, "cancellation").HasLetter, "a letter never opened, printed or sent stayed in the office")
}

// stornoUnderWay stands for a Storno whose transaction holds the run and has
// written the Storno, but not committed yet.
func stornoUnderWay(t *testing.T, f careBillingFixture, runID, invoiceID int64) bun.Tx {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), `SELECT 1 FROM parent_billing.care_runs WHERE id = ? FOR UPDATE`, runID)
	require.NoError(t, err)
	_, err = tx.ExecContext(context.Background(), `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id, invoice_id,
		number, number_year, number_sequence, issued_on, reason, total_cents)
		SELECT organization_id, run_id, id, number || '-S', number_year, number_sequence + 1000, issued_on, 'Test', -total_cents
		FROM parent_billing.care_invoices WHERE id = ?`, invoiceID)
	require.NoError(t, err)
	return tx
}

// waitForCareRun waits until a request waits for the row lock of a run.
func waitForCareRun(t *testing.T, f careBillingFixture, done <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		require.NoError(t, f.db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query ILIKE '%care_run_row%FOR UPDATE%')`).Scan(context.Background(), &waiting))
		if waiting {
			return
		}
		select {
		case <-done:
			t.Fatal("the request finished without waiting for the Storno")
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("the request never waited for the Storno")
}

// A Storno under way and a read or send of the same invoice run one after
// the other: the read and the send wait for the run, then find the invoice
// cancelled and hand out nothing, because it never reached the family and
// has no Storno letter.
func TestCareInvoiceIsNotHandedOutWhileItsStornoIsUnderWay(t *testing.T) {
	t.Parallel()
	for _, route := range []struct{ method, suffix string }{
		{http.MethodGet, "/letter"}, {http.MethodPost, "/mail"},
	} {
		t.Run(strings.TrimPrefix(route.suffix, "/"), func(t *testing.T) {
			f := newCareBillingFixture(t, "Träger Monatsstorno gleichzeitig "+route.suffix)
			brandCareTraeger(t, f)
			run, contracts := releasedCareRun(t, f, "Mia")
			mia := invoiceOfContract(t, run, contracts["Mia"])
			rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

			storno := stornoUnderWay(t, f, run.ID, mia.ID)
			done := make(chan struct{})
			var answer *httptest.ResponseRecorder
			go func() {
				defer close(done)
				answer = traegerJSON(t, f.traeger, f.office.token, route.method, careRunPath(run.ID, fmt.Sprintf("/invoices/%d%s", mia.ID, route.suffix)), nil)
			}()
			waitForCareRun(t, f, done)
			require.NoError(t, storno.Commit())
			<-done
			assert.Equal(t, http.StatusConflict, answer.Code, answer.Body.String())
			assert.Contains(t, answer.Body.String(), "parent_billing.care_invoice_cancelled")
			assert.Empty(t, careLetterMails(t, f, run.ID), "no mail of the cancelled invoice")
		})
	}
}
