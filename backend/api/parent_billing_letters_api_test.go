// Router-level tests for the confirmation letters of a released billing run
// (#4669, ADR 0121): the letter of every issued invoice in the layout of the
// Träger, rendered once and never changed, the preview before, the
// download after, the right "Abrechnung", the Träger boundary and the trail.
package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type billingLettersBody struct {
	PaymentDueOn      string `json:"payment_due_on"`
	Notes             string `json:"notes"`
	EmergencyContacts []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"emergency_contacts"`
	CreatedByAccountID *int64 `json:"created_by_account_id"`
}

type billingRunLettersBody struct {
	ID        int64  `json:"id"`
	Status    string `json:"status"`
	Documents []struct {
		ID     int64  `json:"id"`
		Number string `json:"number"`
	} `json:"documents"`
	Letters *billingLettersBody `json:"letters"`
}

func lettersBody(due string) map[string]any {
	return map[string]any{
		"payment_due_on": due,
		"notes":          "Bitte denken Sie an: Frühstück, Wechselkleidung, Hausschuhe.\nKinder mit ansteckenden Krankheiten bleiben zu Hause.",
		"emergency_contacts": []map[string]any{
			{"name": "OGS Am Lindenhof", "phone": "0170 0000001"},
			{"name": "OGS Schule am Hang", "phone": "0170 0000002"},
		},
	}
}

func letterPath(runID, documentID int64) string {
	return billingRunPath(runID, fmt.Sprintf("/documents/%d/letter", documentID))
}

// releasedBillingRun confirms both families, sets the number range and
// releases a run of the period.
func releasedBillingRun(t *testing.T, f reviewFixture, billing string) billingRunLettersBody {
	t.Helper()
	for _, id := range []int64{f.siblings, f.again} {
		require.Equal(t, http.StatusOK, f.act(t, id, "confirm").Code)
	}
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[billingRunLettersBody](t, rec.Body.Bytes())
	released.ID = run.ID
	return released
}

// brandTraeger gives the Träger of the office its letterhead: logo, colour,
// sender, footer, contact person and a default bank account.
func brandTraeger(t *testing.T, f reviewFixture) {
	t.Helper()
	testpkg.OwnTraegerMasterData(t, f.db, f.office.organization)
	token := f.office.token
	require.Equal(t, http.StatusOK, uploadLogo(t, f.traeger, token, "logo", "logo.png", pngLogo(t, 120, 40)).Code)
	rec := traegerJSON(t, f.traeger, token, http.MethodPut, "/traeger/master-data/contact",
		masterDataContact{Name: "Paula Kern", Phone: "05402 1234-0", Email: "ferien@ganztag.example"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, token, http.MethodPost, "/traeger/master-data/bank-accounts", masterDataBankAccount{
		Label: "Elternbeiträge", AccountHolder: "Ganztag im Quartier e.V.", IBAN: traegerTestIBAN, BankName: "Beispielbank",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	for key, value := range map[string]string{
		configModels.KeyBrandingPrimaryColor:   "#217A78",
		configModels.KeyBrandingLetterheadText: "Ganztag im Quartier e.V.\nAm Markt 4\n49143 Bissendorf",
		configModels.KeyBrandingFooterText:     "Vorstand: Anna Brandt",
	} {
		rec := traegerJSON(t, f.traeger, token, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", key, rec.Body.String())
	}
}

func TestBillingLettersConfirmEveryFamilyOnceInTheLayoutOfTheTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Schreiben Ferien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	// A run in preview has no invoice to confirm yet.
	preview := startBillingRun(t, f, billing, f.period.ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(preview.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_not_released", code)
	_, err := f.db.ExecContext(context.Background(), `DELETE FROM parent_billing.runs WHERE id = ?`, preview.ID)
	require.NoError(t, err)

	run := releasedBillingRun(t, f, billing)
	require.Len(t, run.Documents, 2)

	// A letter asks for payment: without a bank account it is refused.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.bank_account_missing", code)
	brandTraeger(t, f)

	for body, field := range map[string]map[string]any{
		"payment_due_on":      {"notes": "x"},
		"payment in the past": {"payment_due_on": "2020-01-01"},
		"emergency_contacts":  {"payment_due_on": "2099-01-15", "emergency_contacts": []map[string]any{{"name": "OGS"}}},
	} {
		rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), field)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", body, rec.Body.String())
		code, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, "parent_billing.billing_letters_invalid", code, body)
	}
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), map[string]any{"payment_due_on": "15.01.2099"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_letters_invalid", code, "a date not in YYYY-MM-DD")

	// The preview shows the letter of the first family and stores nothing.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters/preview"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, `inline; filename="bestaetigung-fe-0001.pdf"`, rec.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, run.Documents[0].ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "the preview stored no letter: %s", rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_letter_not_found", code)

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	letters := decodeSchoolPortalData[billingLettersBody](t, rec.Body.Bytes())
	assert.Equal(t, "2099-01-15", letters.PaymentDueOn)
	assert.Contains(t, letters.Notes, "Hausschuhe")
	require.Len(t, letters.EmergencyContacts, 2)
	assert.Equal(t, "OGS Am Lindenhof", letters.EmergencyContacts[0].Name)
	assert.NotNil(t, letters.CreatedByAccountID)

	// A run has its letters once; they never change.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-02-01"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_letters_exist", code)

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[billingRunLettersBody](t, rec.Body.Bytes())
	require.NotNil(t, detail.Letters, "the released run shows its letters")
	assert.Equal(t, "2099-01-15", detail.Letters.PaymentDueOn)

	for _, document := range detail.Documents {
		rec := traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, document.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
		assert.Equal(t, fmt.Sprintf(`attachment; filename="bestaetigung-%s.pdf"`, map[string]string{"FE-0001": "fe-0001", "FE-0002": "fe-0002"}[document.Number]),
			rec.Header().Get("Content-Disposition"))
		assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
		var stored []byte
		var checksumMatches bool
		require.NoError(t, f.db.NewRaw(`SELECT pdf, sha256 = encode(sha256(pdf), 'hex') FROM parent_billing.document_letters WHERE document_id = ?`, document.ID).
			Scan(context.Background(), &stored, &checksumMatches))
		assert.Equal(t, stored, rec.Body.Bytes(), "the download is the stored letter")
		assert.True(t, checksumMatches, "the letter keeps its SHA-256")
	}

	// Every read of a letter and the creation stand in the trail; the read
	// of the contact person and the bank account in the master data log.
	actions := runActions(t, f, run.ID)
	assert.Contains(t, actions, "letters")
	var letterReads []string
	require.NoError(t, f.db.NewRaw(`SELECT details->>'letter' FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'read' AND details ? 'letter' ORDER BY id`, run.ID).Scan(context.Background(), &letterReads))
	assert.Equal(t, []string{"preview", "download", "download"}, letterReads)
	assert.Contains(t, masterDataLog(t, f.db, f.office.organization), "master_data:read")
}

func TestBillingLettersNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Schreiben Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Schreiben Fremd", identityaccess.TraegerRoleBilling)
	run := releasedBillingRun(t, f, billing)
	brandTraeger(t, f)
	document := run.Documents[0].ID

	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, billingRunPath(run.ID, "/letters/preview"), lettersBody("2099-01-15")},
		{http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15")},
		{http.MethodGet, letterPath(run.ID, document), nil},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, f.office.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right Abrechnung: %s", request.method, request.path, rec.Body.String())
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	var stored, events int
	require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT count(*) FROM parent_billing.run_letters WHERE run_id = ?) + (SELECT count(*) FROM parent_billing.document_letters WHERE run_id = ?),
			(SELECT count(*) FROM parent_billing.run_events WHERE run_id = ? AND (action = 'letters' OR details ? 'letter'))`, run.ID, run.ID, run.ID).
		Scan(context.Background(), &stored, &events))
	assert.Zero(t, stored, "refused requests stored no letter")
	assert.Zero(t, events, "refused requests read no letter")

	// The letter of an invoice of another run of the Träger is not found
	// under this run.
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID+1000, document), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, letterPath(run.ID, document), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger reads no stored letter: %s", rec.Body.String())
}
