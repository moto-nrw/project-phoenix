package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// The stored originals of the Jahresbescheide, their mail and the inbox
// push (#6347, decision A) through the routes.

type careNoticeLetterBody struct {
	NoticeID       int64  `json:"notice_id"`
	ContractID     int64  `json:"contract_id"`
	Kind           string `json:"kind"`
	Number         string `json:"number"`
	AmendsNumber   string `json:"amends_number"`
	AmendedBy      string `json:"amended_by_number"`
	RecipientEmail string `json:"recipient_email"`
	HasLetter      bool   `json:"has_letter"`
	SHA256         string `json:"sha256"`
	InInbox        bool   `json:"in_inbox"`
	Status         string `json:"status"`
	Sends          int    `json:"sends"`
}

type careNoticeLettersBody struct {
	Notices []careNoticeLetterBody `json:"notices"`
	Summary struct {
		Notices      int `json:"notices"`
		Letters      int `json:"letters"`
		Sent         int `json:"sent"`
		Queued       int `json:"queued"`
		Failed       int `json:"failed"`
		NotSent      int `json:"not_sent"`
		WithoutEmail int `json:"without_email"`
	} `json:"summary"`
}

// releasedNoticeRun is a branded Träger with Mia's issued Jahresbescheid
// from September to December 2030.
func releasedNoticeRun(t *testing.T, name string) (careBillingFixture, int64, careNoticeRunBody, traegerOffice) {
	t.Helper()
	f, contract := newNoticeFixture(t, name)
	writeLegalRemedy(t, f.db, f.office.organization)
	administration := brandedAdministration(t, f)
	run := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID))
	require.Len(t, run.Notices, 1)
	return f, contract, run, administration
}

// brandedAdministration brands the Träger as brandCareTraeger does and
// returns the colleague with the right "Verwaltung", who may change the
// letterhead later.
func brandedAdministration(t *testing.T, f careBillingFixture) traegerOffice {
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
	return administration
}

func noticeLetters(t *testing.T, f careBillingFixture, runID int64) careNoticeLettersBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careNoticeRunPath(runID, "/letters"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careNoticeLettersBody](t, rec.Body.Bytes())
}

func noticePDF(t *testing.T, f careBillingFixture, runID, contractID int64) []byte {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		careNoticeRunPath(runID, "/notices/"+strconv.FormatInt(contractID, 10)+"/pdf"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	return rec.Body.Bytes()
}

func noticeMails(t *testing.T, f careBillingFixture, runID int64) []careLetterMail {
	t.Helper()
	var mails []careLetterMail
	require.NoError(t, f.db.NewRaw(`SELECT id, payload FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'parent_billing_letter' AND related_entity_type = 'parent_billing_care_notice_run'
			AND related_entity_id = ? ORDER BY id`, f.office.organization, runID).Scan(context.Background(), &mails))
	return mails
}

// storedSHA256 is the SHA-256 the database computes of the stored PDF.
func storedSHA256(t *testing.T, f careBillingFixture, runID int64) string {
	t.Helper()
	var sum string
	require.NoError(t, f.db.NewRaw(`SELECT encode(sha256(pdf), 'hex') FROM parent_billing.care_notice_letters WHERE run_id = ?`, runID).
		Scan(context.Background(), &sum))
	return sum
}

// The original of a Jahresbescheid is stored once and stays byte for byte
// what it was, after the letterhead, the template of the fee notice, the
// address of the family, the contract and the bank account changed. Without the stored PDF a
// download would render the letterhead of today.
func TestCareNoticeOriginalStaysByteIdenticalAfterEveryLaterChange(t *testing.T) {
	t.Parallel()
	f, contract, run, administration := releasedNoticeRun(t, "Träger Bescheid Original")
	path := careNoticeRunPath(run.ID, "/notices/"+strconv.FormatInt(contract, 10)+"/pdf")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "no original before it is stored: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	created := decodeSchoolPortalData[careNoticeLettersBody](t, rec.Body.Bytes())
	require.Len(t, created.Notices, 1)
	assert.True(t, created.Notices[0].HasLetter)
	original := noticePDF(t, f, run.ID, contract)
	assert.Equal(t, storedSHA256(t, f, run.ID), created.Notices[0].SHA256, "the overview names the stored bytes")
	var stored []byte
	require.NoError(t, f.db.NewRaw(`SELECT pdf FROM parent_billing.care_notice_letters WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &stored))
	assert.Equal(t, stored, original, "the download is the stored PDF")

	// Letterhead, template, address, contract and bank account change afterwards.
	rec = traegerJSON(t, f.traeger, administration.token, http.MethodPut, "/traeger/settings/values/"+configModels.KeyBrandingLetterheadText,
		map[string]any{"value": "Neuer Träger e.V.\nNeue Straße 1\n75175 Pforzheim"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `INSERT INTO platform.traeger_document_templates (organization_id, kind, version, subject, body, legal_remedy)
		VALUES (?, 'bescheid', 2, 'Gebührenbescheid', 'Neu', 'Neue Belehrung: Klage beim Verwaltungsgericht.')`, f.office.organization)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE care_contract.contract_parties SET first_name = 'Erika', email = 'erika.neu@example.org'
		WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `INSERT INTO care_contract.contract_version_services (organization_id, contract_version_id,
			weekday, care_offering_id, module_name, ends_at)
		SELECT organization_id, id, 'wed', ?, 'Betreuung bis 16:30 Uhr', '16:30'::time FROM care_contract.contract_versions
		WHERE contract_id = ?`, careModuleUntil1630, contract)
	require.NoError(t, err)

	var accountID int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM platform.traeger_bank_accounts WHERE organization_id = ? ORDER BY id LIMIT 1`,
		f.office.organization).Scan(ctx, &accountID))
	rec = traegerJSON(t, f.traeger, administration.token, http.MethodPut,
		"/traeger/master-data/bank-accounts/"+strconv.FormatInt(accountID, 10), masterDataBankAccount{
			Label: "Elternbeiträge", AccountHolder: "Neuer Träger e.V.", IBAN: traegerTestIBAN, BankName: "Andere Bank", IsDefault: true,
		})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "the original is stored once: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_letters_exist")
	assert.Equal(t, original, noticePDF(t, f, run.ID, contract), "the original never changes")
	assert.Equal(t, created.Notices[0].SHA256, noticeLetters(t, f, run.ID).Notices[0].SHA256)

	// No role changes it in the database either.
	_, err = f.db.ExecContext(ctx, `UPDATE parent_billing.care_notice_letters SET pdf = '\x255044'::bytea WHERE run_id = ?`, run.ID)
	assert.ErrorContains(t, err, "never changes")
}

// One click of the office sends the stored PDF by mail and puts it in the
// inbox: the attachment is the stored original, a series sends nothing twice,
// a waiting mail is not sent again, a failed one is, and only the first send
// pushes. Every step stands in the trail.
func TestCareNoticeMailCarriesTheStoredOriginalOnceAndAgainOnRequest(t *testing.T) {
	t.Parallel()
	f, contract, run, _ := releasedNoticeRun(t, "Träger Bescheid Versand")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "no mail before the original: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Zero(t, careInboxPushes(t, f), "a stored original is not in the inbox yet")
	original := noticePDF(t, f, run.ID, contract)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{Queued: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
	mails := noticeMails(t, f, run.ID)
	require.Len(t, mails, 1)
	var payload struct {
		DocumentKind string `json:"document_kind"`
		LetterKind   string `json:"letter_kind"`
		Number       string `json:"number"`
		PDF          []byte `json:"pdf"`
	}
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, "care_fee_notice", payload.DocumentKind)
	assert.Equal(t, run.Notices[0].Number, payload.Number)
	assert.Equal(t, original, payload.PDF, "the attachment is the stored original")
	assert.Equal(t, 1, careInboxPushes(t, f), "the first send announces the notice in the inbox")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{Already: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()), "a second series sends nothing twice")
	one := careNoticeRunPath(run.ID, "/notices/"+strconv.FormatInt(contract, 10)+"/mail")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, one, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "a waiting mail is not sent again: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.letter_mail_pending")
	assert.Equal(t, "queued", noticeLetters(t, f, run.ID).Notices[0].Status)

	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'dead_letter' WHERE id = ?`, mails[0].ID)
	require.NoError(t, err)
	assert.Equal(t, "failed", noticeLetters(t, f, run.ID).Notices[0].Status)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, one, nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "a failed mail is sent again: %s", rec.Body.String())
	mails = noticeMails(t, f, run.ID)
	require.Len(t, mails, 2)
	require.NoError(t, json.Unmarshal(mails[1].Payload, &payload))
	assert.Equal(t, original, payload.PDF, "the repeat carries the same original")
	assert.Equal(t, 1, careInboxPushes(t, f), "only the first send pushes")
	assert.Equal(t, 2, noticeLetters(t, f, run.ID).Notices[0].Sends)
	var keys []string
	require.NoError(t, f.db.NewRaw(`SELECT idempotency_key FROM platform.email_outbox WHERE id IN (?, ?) ORDER BY id`, mails[0].ID, mails[1].ID).
		Scan(context.Background(), &keys))
	require.Len(t, keys, 2)
	assert.NotEqual(t, keys[0], keys[1], "each send is one outbox entry with its own key")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careNoticeRunPath(run.ID, "/letters/print"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT DISTINCT action FROM parent_billing.care_notice_run_events WHERE run_id = ? ORDER BY action`, run.ID).
		Scan(context.Background(), &actions))
	assert.Equal(t, []string{"create", "letters", "mail", "read", "release"}, actions)
}

// A notice goes only to the party the contract still bills: a missing
// address prints, a changed party or a payer who no longer is the payer is
// refused, and nothing reaches the outbox or the inbox.
func TestCareNoticeMailRefusesMissingAddressAndChangedPayer(t *testing.T) {
	t.Parallel()
	for name, change := range map[string]string{
		"missing address": `UPDATE care_contract.contract_parties SET email = '' WHERE contract_id = ?`,
		"changed party":   `UPDATE care_contract.contract_parties SET email = 'neu@example.org' WHERE contract_id = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, contract := newNoticeFixture(t, "Träger Bescheid Empfänger")
			writeLegalRemedy(t, f.db, f.office.organization)
			brandCareTraeger(t, f)
			if name == "missing address" {
				_, err := f.db.ExecContext(context.Background(), change, contract)
				require.NoError(t, err)
			}
			run := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID))
			rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
			require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
			if name == "changed party" {
				_, err := f.db.ExecContext(context.Background(), change, contract)
				require.NoError(t, err)
			}
			rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost,
				careNoticeRunPath(run.ID, "/notices/"+strconv.FormatInt(contract, 10)+"/mail"), nil)
			assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
			want := careSeriesBody{RecipientChanged: 1}
			if name == "missing address" {
				assert.Contains(t, rec.Body.String(), "parent_billing.recipient_email_missing")
				want = careSeriesBody{WithoutEmail: 1}
			} else {
				assert.Contains(t, rec.Body.String(), "parent_billing.care_recipient_changed")
			}
			rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
			require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
			assert.Equal(t, want, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
			assert.Empty(t, noticeMails(t, f, run.ID))
			assert.Zero(t, careInboxPushes(t, f))
			if name == "missing address" {
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careNoticeRunPath(run.ID, "/letters/print?only=without_email"), nil)
				require.Equal(t, http.StatusOK, rec.Code, "a notice without an address prints: %s", rec.Body.String())
			}
		})
	}
}

// A payer who deviates from the contract party gets the notice: the release
// copies the payer, and the send checks the payer of today.
func TestCareNoticeMailGoesToTheDeviatingPayer(t *testing.T) {
	t.Parallel()
	f, contract := newNoticeFixture(t, "Träger Bescheid Zahler")
	writeLegalRemedy(t, f.db, f.office.organization)
	brandCareTraeger(t, f)
	input := map[string]any{
		"contract_id": contract, "recipient_kind": "payer",
		"payer_first_name": "Oma", "payer_last_name": "Muster", "payer_email": "oma@example.org",
		"payer_address": map[string]any{"street": "Amtsweg 2", "postal_code": "75175", "city": "Pforzheim"},
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, input)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID))
	assert.Equal(t, "oma@example.org", run.Notices[0].Recipient.Email)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	input["payer_email"] = "opa@example.org"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, input)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{RecipientChanged: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()),
		"a payer who no longer pays gets nothing")

	input["payer_email"] = "oma@example.org"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, input)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, careSeriesBody{Queued: 1}, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
	mails := noticeMails(t, f, run.ID)
	require.Len(t, mails, 1)
	var email string
	require.NoError(t, f.db.NewRaw(`SELECT recipient->>'address' FROM platform.email_outbox WHERE id = ?`, mails[0].ID).
		Scan(context.Background(), &email))
	assert.Equal(t, "oma@example.org", email)
}

// A change notice is a document of its own: its own original, its own mail
// kind, and the overview names the notice it amends; the first notice keeps
// its original.
func TestCareChangeNoticeHasItsOwnOriginalThatNamesItsPredecessor(t *testing.T) {
	t.Parallel()
	f, contract, first, _ := releasedNoticeRun(t, "Träger Änderungsbescheid Original")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(first.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	firstOriginal := noticePDF(t, f, first.ID, contract)
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `DELETE FROM care_contract.contract_version_services
		WHERE weekday = 'tue' AND contract_version_id IN (SELECT id FROM care_contract.contract_versions WHERE contract_id = ?)`, contract)
	require.NoError(t, err)

	change := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-11-01", "2030-10-20").ID))
	require.Len(t, change.Notices, 1)
	require.Equal(t, "change", change.Notices[0].Kind)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(change.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	letters := decodeSchoolPortalData[careNoticeLettersBody](t, rec.Body.Bytes())
	require.Len(t, letters.Notices, 1)
	assert.Equal(t, "change", letters.Notices[0].Kind)
	assert.Equal(t, first.Notices[0].Number, letters.Notices[0].AmendsNumber, "the change notice names its predecessor")
	changeOriginal := noticePDF(t, f, change.ID, contract)
	assert.NotEqual(t, firstOriginal, changeOriginal)
	assert.Equal(t, firstOriginal, noticePDF(t, f, first.ID, contract), "the first notice keeps its original")
	assert.Equal(t, change.Notices[0].Number, noticeLetters(t, f, first.ID).Notices[0].AmendedBy)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(change.ID, "/letters/mail"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	mails := noticeMails(t, f, change.ID)
	require.Len(t, mails, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(mails[0].Payload, &payload))
	assert.Equal(t, "care_change_notice", payload["document_kind"])
}

// Every route of the originals needs the right "Abrechnung" and stays with
// its Träger; a run that issued nothing yet has no original.
func TestCareNoticeLettersNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f, contract, run, administration := releasedNoticeRun(t, "Träger Bescheid Recht")
	notice := "/notices/" + strconv.FormatInt(contract, 10)
	routes := []struct{ method, path string }{
		{http.MethodGet, "/letters"}, {http.MethodPost, "/letters"}, {http.MethodGet, "/letters/print"},
		{http.MethodPost, "/letters/mail"}, {http.MethodGet, notice + "/pdf"}, {http.MethodPost, notice + "/mail"},
	}
	stranger := newTraegerOffice(t, f.db, f.traeger, "Träger Bescheid Fremd", identityaccess.TraegerRoleBilling)
	for _, route := range routes {
		rec := traegerJSON(t, f.traeger, administration.token, route.method, careNoticeRunPath(run.ID, route.path), nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", route.method, route.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, stranger.token, route.method, careNoticeRunPath(run.ID, route.path), nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", route.method, route.path, rec.Body.String())
	}
	assert.False(t, noticeLetters(t, f, run.ID).Notices[0].HasLetter, "nothing changed through the refused routes")

	open := f.startNoticeRun(t, "2030-10-01", "")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(open.ID, "/letters"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_not_released")
}

// The originals of a run read with a flat number of statements: a run of one
// notice and a run of three read the same.
func TestCareNoticeLettersQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f, _, run, _ := releasedNoticeRun(t, "Träger Bescheid Budget")
	school := f.office.schools[0]
	for _, child := range []string{"Lea", "Ben"} {
		careContract(t, f.db, f.office.organization, school, child, careMonth, sjrServices()...)
	}
	second := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID))
	require.Len(t, second.Notices, 2)
	read := func(runID int64) testpkg.QueryMeasurement {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(runID, "/letters"), nil)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(runID, "/letters/mail"), nil)
		require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, careNoticeRunPath(runID, "/letters"), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		recorder := httptest.NewRecorder()
		f.traeger.ServeHTTP(recorder, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := read(run.ID)
	assert.Len(t, read(second.ID).Statements, len(one.Statements), fmt.Sprintf("flat for any number of notices (%d)", len(one.Statements)))
	querybudgets.AssertQueryBudget(t, "api.traeger.care_notice_run_letters", one)
}

// A notice paid by transfer names the account to pay to: without a bank
// account of the Träger no original is stored, so no family gets a notice
// that asks for a transfer without saying where to. Once the account exists,
// the original is stored with it.
func TestCareNoticeOriginalByTransferNeedsTheBankAccount(t *testing.T) {
	t.Parallel()
	f, contract := newNoticeFixture(t, "Träger Bescheid ohne Konto")
	writeLegalRemedy(t, f.db, f.office.organization)
	run := f.releaseNoticeRun(t, f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "2030-09-24").ID))
	require.Len(t, run.Notices, 1)
	var method string
	require.NoError(t, f.db.NewRaw(`SELECT payment_method FROM parent_billing.care_notices WHERE id = ?`, run.Notices[0].ID).
		Scan(context.Background(), &method))
	require.NotEqual(t, "direct_debit", method, "the fixture pays by transfer")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.bank_account_missing")
	assert.False(t, noticeLetters(t, f, run.ID).Notices[0].HasLetter, "no original without the account")

	brandedAdministration(t, f)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(run.ID, "/letters"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	noticePDF(t, f, run.ID, contract)
}
