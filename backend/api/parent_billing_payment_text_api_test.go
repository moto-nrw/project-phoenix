// Router-level tests for the payment text of a Träger (#5622, ADR 0121
// addendum): the Träger-Büro writes its own payment sentence with fixed
// placeholders, saving refuses any other text in curly braces and more than
// 600 characters, every change stands in the Träger's change log, no Träger
// reads or writes the text of another, letters refuse a text that names the
// debtor number while an invoice has none, and finished letters never
// change.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// bissendorfPaymentText is the payment text of the Bissendorf letter: the
// transfer with the debtor number and the sentence for families with a
// direct debit mandate.
const bissendorfPaymentText = "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist} auf das Konto unten. " +
	"Geben Sie als Verwendungszweck Ihre Debitorennummer {Debitorennummer} an. " +
	"Haben Sie uns bereits ein SEPA-Lastschriftmandat erteilt, buchen wir den Betrag ab."

const paymentTextPath = "/traeger/settings/values/" + configModels.KeyParentBillingPaymentText

func setPaymentText(t *testing.T, f reviewFixture, text string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, paymentTextPath, map[string]any{"value": text})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestPaymentTextAdmitsOnlyItsPlaceholdersAndStaysWithItsTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Zahlungstext Träger")
	foreign := newTraegerOffice(t, f.db, f.traeger, "Zahlungstext Nachbar", identityaccess.TraegerRoleAdministration)

	for name, text := range map[string]string{
		"an unknown placeholder":       "Bitte überweisen Sie {Betrag} für {Kind}.",
		"a placeholder in lower case":  "Bitte überweisen Sie {betrag}.",
		"an open brace":                "Bitte überweisen Sie {Betrag bis zum {Zahlungsfrist}.",
		"a closing brace alone":        "Bitte überweisen Sie Betrag} bis zum {Zahlungsfrist}.",
		"601 characters":               strings.Repeat("ä", 601),
		"601 characters with brackets": "{Betrag}" + strings.Repeat("a", 593),
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, paymentTextPath, map[string]any{"value": text})
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", name, rec.Body.String())
		code, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, common.CodeGeneralInput, code, name)
	}
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT count(*) FROM config.traeger_setting_values WHERE setting_key = ? AND organization_id = ?)
			+ (SELECT count(*) FROM config.traeger_setting_audit WHERE setting_key = ? AND organization_id = ?)`,
		configModels.KeyParentBillingPaymentText, f.office.organization, configModels.KeyParentBillingPaymentText, f.office.organization).
		Scan(context.Background(), &stored))
	assert.Zero(t, stored, "a refused text is neither stored nor logged")

	setPaymentText(t, f, "{Rechnungsnummer}"+strings.Repeat("ö", 600-len([]rune("{Rechnungsnummer}"))))
	setPaymentText(t, f, bissendorfPaymentText)
	_, items := traegerSchemaItems(t, f.traeger, f.office.token)
	require.Contains(t, items, configModels.KeyParentBillingPaymentText)
	assert.Equal(t, bissendorfPaymentText, items[configModels.KeyParentBillingPaymentText].Value)
	assert.Equal(t, "traeger", items[configModels.KeyParentBillingPaymentText].Source)

	_, foreignItems := traegerSchemaItems(t, f.traeger, foreign.token)
	require.Contains(t, foreignItems, configModels.KeyParentBillingPaymentText)
	assert.Equal(t, "", foreignItems[configModels.KeyParentBillingPaymentText].Value, "another Träger does not read the text")
	assert.Equal(t, "default", foreignItems[configModels.KeyParentBillingPaymentText].Source)
	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodPut, paymentTextPath, map[string]any{"value": "Bitte überweisen Sie {Betrag}."})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, items = traegerSchemaItems(t, f.traeger, f.office.token)
	assert.Equal(t, bissendorfPaymentText, items[configModels.KeyParentBillingPaymentText].Value, "another Träger writes only its own text")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, paymentTextPath, nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM config.traeger_setting_audit
		WHERE organization_id = ? AND setting_key = ? ORDER BY id`, f.office.organization, configModels.KeyParentBillingPaymentText).
		Scan(context.Background(), &actions))
	assert.Equal(t, []string{"set", "set", "reset"}, actions, "every change stands in the change log of the Träger")

	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, paymentTextPath, map[string]any{"value": "Bitte überweisen Sie {Betrag}."})
	assert.Equal(t, http.StatusForbidden, rec.Code, "the billing right does not write the text: %s", rec.Body.String())
}

type paymentTextRefusal struct {
	Code    string `json:"code"`
	Details struct {
		Count          int      `json:"count"`
		InvoiceNumbers []string `json:"invoice_numbers"`
	} `json:"details"`
}

func TestBillingLettersWithPaymentTextNeedTheDebtorNumberItNames(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Zahlungstext Schreiben")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	askForDebtorNumber(t, f, true)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	askForDebtorNumber(t, f, false)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	documents := decodeSchoolPortalData[billingRunLettersBody](t, rec.Body.Bytes()).Documents
	brandTraeger(t, f)
	setPaymentText(t, f, bissendorfPaymentText)

	for _, suffix := range []string{"/letters/preview", "/letters"} {
		rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, suffix), lettersBody("2099-01-15"))
		require.Equal(t, http.StatusConflict, rec.Code, "%s: %s", suffix, rec.Body.String())
		var refusal paymentTextRefusal
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &refusal))
		assert.Equal(t, common.CodeParentBillingPaymentTextDebtorNumberMissing, refusal.Code, suffix)
		assert.Equal(t, 1, refusal.Details.Count, suffix)
		assert.Equal(t, []string{"FE-0002"}, refusal.Details.InvoiceNumbers, "%s names the invoice without a number", suffix)
	}
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT count(*) FROM parent_billing.run_letters WHERE run_id = ?)
			+ (SELECT count(*) FROM parent_billing.document_letters WHERE run_id = ?)
			+ (SELECT count(*) FROM parent_billing.run_events WHERE run_id = ? AND (action = 'letters' OR details ? 'letter'))`,
		run.ID, run.ID, run.ID).Scan(context.Background(), &stored))
	assert.Zero(t, stored, "a refused creation or preview stores and reads nothing")

	// Without the placeholder the letters go out; a text written later
	// changes no finished letter.
	setPaymentText(t, f, "Bitte überweisen Sie {Betrag} bis zum {Zahlungsfrist}. Verwendungszweck: {Rechnungsnummer}.")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters/preview"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	before := map[int64][]byte{}
	for _, document := range documents {
		rec := traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, document.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		before[document.ID] = rec.Body.Bytes()
	}
	require.Len(t, before, 2)
	setPaymentText(t, f, bissendorfPaymentText+" Ihre Rechnung: {Rechnungsnummer}.")
	for _, document := range documents {
		rec := traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, document.ID), nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, before[document.ID], rec.Body.Bytes(), "a finished letter keeps its payment text")
	}
}
