// Router-level tests for the debtor number of a holiday registration
// (#5621): the Träger switch asks for it at the confirmation, the
// Träger-Büro changes it until an issued invoice copies it, every
// confirmation and change names it in the registration's history, the list,
// the detail and the export show it, the released invoice, the letter and
// the overview for the Kämmerei keep their copy, and no other Träger reads
// or changes it.
package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func holidayDebtorNumberPath(periodID, registrationID int64) string {
	return holidayContributionPath(periodID, registrationID) + "/debtor-number"
}

const holidayConfirmationSettingsPath = "/traeger/holiday-contributions/confirmation-settings"

// askForDebtorNumber sets the Träger switch through the Träger settings.
func askForDebtorNumber(t *testing.T, f reviewFixture, on bool) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut,
		"/traeger/settings/values/"+configModels.KeyParentBillingDebtorNumberOnConfirm, map[string]any{"value": on})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func (f reviewFixture) confirmWith(t *testing.T, id int64, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, id), body)
}

func (f reviewFixture) changeDebtorNumber(t *testing.T, token string, id int64, number string) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodPut, holidayDebtorNumberPath(f.period.ID, id), map[string]any{"debtor_number": number})
}

type debtorNumberDetail struct {
	ID           int64  `json:"id"`
	Status       string `json:"status"`
	DebtorNumber string `json:"debtor_number"`
	History      []struct {
		Action  string         `json:"action"`
		Details map[string]any `json:"details"`
	} `json:"history"`
}

func (f reviewFixture) debtorNumberDetail(t *testing.T, id int64) debtorNumberDetail {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[debtorNumberDetail](t, rec.Body.Bytes())
}

func (f reviewFixture) frozenContributions(t *testing.T, id int64) int {
	t.Helper()
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.holiday_contributions WHERE holiday_registration_id = ?`, id).
		Scan(context.Background(), &count))
	return count
}

func assertDebtorNumberRefused(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, common.CodeParentBillingDebtorNumberInvalid, code)
	assert.Equal(t, []string{"debtor_number"}, fields)
}

func confirmationSettings(t *testing.T, f reviewFixture, token string) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, token, http.MethodGet, holidayConfirmationSettingsPath, nil)
}

func TestConfirmationWithoutTheSwitchTakesTheDebtorNumberAsItComes(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Schalter aus")

	rec := confirmationSettings(t, f, f.office.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"debtor_number_required":false,"debtor_number_format":{"digits_only":false,"min_digits":0,"max_digits":0},"cost_bearers":[]}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())), "the switch is off by default")

	rec = f.confirmWith(t, f.siblings, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.debtorNumberDetail(t, f.siblings).DebtorNumber, "without the switch a confirmation needs no number")

	rec = f.confirmWith(t, f.again, map[string]any{"debtor_number": "  10 4711  "})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := f.debtorNumberDetail(t, f.again)
	assert.Equal(t, "10 4711", detail.DebtorNumber, "a number given anyway is kept, trimmed")
	require.NotEmpty(t, detail.History)
	confirm := detail.History[len(detail.History)-1]
	assert.Equal(t, "confirm", confirm.Action)
	assert.Equal(t, map[string]any{"before": "", "after": "10 4711"}, confirm.Details["debtor_number"],
		"the confirmation names the number in the history")
}

func TestConfirmationWithTheSwitchRequiresTheDebtorNumber(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Schalter an")
	askForDebtorNumber(t, f, true)
	rec := confirmationSettings(t, f, f.office.token)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"debtor_number_required":true,"debtor_number_format":{"digits_only":false,"min_digits":0,"max_digits":0},"cost_bearers":[]}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())))

	for name, body := range map[string]map[string]any{
		"without":     nil,
		"empty":       {"debtor_number": ""},
		"blank":       {"debtor_number": "   "},
		"too long":    {"debtor_number": strings.Repeat("9", 36)},
		"line break":  {"debtor_number": "10\n4711"},
		"return":      {"debtor_number": "10\r4711"},
		"umlaut long": {"debtor_number": strings.Repeat("ä", 36)},
	} {
		assertDebtorNumberRefused(t, f.confirmWith(t, f.siblings, body))
		detail := f.debtorNumberDetail(t, f.siblings)
		assert.Equal(t, "received", detail.Status, "%s: the registration stays received", name)
		assert.Empty(t, detail.DebtorNumber, name)
		assert.Zero(t, f.frozenContributions(t, f.siblings), "%s: no contribution is frozen", name)
	}

	rec = f.confirmWith(t, f.siblings, map[string]any{"debtor_number": " " + strings.Repeat("ä", 35) + " "})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, strings.Repeat("ä", 35), f.debtorNumberDetail(t, f.siblings).DebtorNumber,
		"35 characters fit; spaces before and after go")
	assert.NotZero(t, f.frozenContributions(t, f.siblings))
}

func TestDebtorNumberChangesUntilAnInvoiceIsIssued(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Ändern")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	assert.Equal(t, http.StatusConflict, f.changeDebtorNumber(t, f.office.token, f.siblings, "1").Code,
		"a received registration gets its number at the confirmation")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, nil).Code, "confirmed before the Träger switched it on")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	askForDebtorNumber(t, f, true)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	assert.Equal(t, 2, run.MissingDebtorNumbers, "the run counts the invoices without a number while the switch is on")

	rec := f.changeDebtorNumber(t, f.office.token, f.siblings, " 10 4711 ")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `{"id":`+jsonInt(f.siblings)+`,"debtor_number":"10 4711"}`, string(decodeSchoolPortalRaw(t, rec.Body.Bytes())))
	assertDebtorNumberRefused(t, f.changeDebtorNumber(t, f.office.token, f.siblings, " "))
	assertDebtorNumberRefused(t, f.changeDebtorNumber(t, f.office.token, f.siblings, strings.Repeat("1", 36)))
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "freeze").Code)
	rec = f.changeDebtorNumber(t, f.office.token, f.siblings, "10 4712")
	require.Equal(t, http.StatusOK, rec.Code, "freezing locks cancellations and rebookings, not the number; the family took 10 4711 "+
		"from this registration, so the change corrects both without a choice (#6692): %s", rec.Body.String())
	assert.Equal(t, "10 4712", f.familyNumber(t, "eva.muster@example.org"))
	rec = f.changeDebtorNumber(t, f.office.token, f.siblings, "10 4712")
	require.Equal(t, http.StatusOK, rec.Code, "the same number again changes nothing: %s", rec.Body.String())

	history := f.debtorNumberDetail(t, f.siblings).History
	var changes []any
	for _, event := range history {
		if event.Action == "debtor_number" {
			changes = append(changes, event.Details["debtor_number"])
		}
	}
	assert.Equal(t, []any{
		map[string]any{"before": "", "after": "10 4711"},
		map[string]any{"before": "10 4711", "after": "10 4712"},
	}, changes, "every change names the number before and after")

	run = billingRun(t, f, billing, run.ID)
	assert.Equal(t, 1, run.MissingDebtorNumbers)
	askForDebtorNumber(t, f, false)
	assert.Zero(t, billingRun(t, f, billing, run.ID).MissingDebtorNumbers, "without the switch the run counts nothing")
	run = billingRun(t, f, billing, run.ID)
	assert.False(t, f.contribution(t, f.siblings).Invoiced, "no invoice before the release")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, f.contribution(t, f.siblings).Invoiced, "the contribution names the issued invoice, so the page offers no change")

	rec = f.changeDebtorNumber(t, f.office.token, f.siblings, "99")
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, common.CodeParentBillingDebtorNumberLocked, code)
	assert.Equal(t, http.StatusConflict, f.changeDebtorNumber(t, f.office.token, f.again, "98").Code,
		"an invoice without a number locks it too")
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "reopen").Code)
	assert.Equal(t, http.StatusConflict, f.changeDebtorNumber(t, f.office.token, f.siblings, "99").Code, "reopening changes nothing")
	assert.Equal(t, "10 4712", f.debtorNumberDetail(t, f.siblings).DebtorNumber)
}

type billingDebtorBody struct {
	Documents []struct {
		RegistrationID int64  `json:"registration_id"`
		DebtorNumber   string `json:"debtor_number"`
	} `json:"documents"`
}

func TestReleasedInvoiceKeepsTheDebtorNumberForLetterAndOverview(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Rechnung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	askForDebtorNumber(t, f, true)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)
	askForDebtorNumber(t, f, false)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	numbers := func(body []byte) map[int64]string {
		out := map[int64]string{}
		for _, document := range decodeSchoolPortalData[billingDebtorBody](t, body).Documents {
			out[document.RegistrationID] = document.DebtorNumber
		}
		return out
	}
	want := map[int64]string{f.siblings: "10 4711", f.again: ""}
	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, numbers(rec.Body.Bytes()), "the preview names the number of the registration")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, numbers(rec.Body.Bytes()), "the release copies the number into the invoice")

	_, err := f.db.ExecContext(t.Context(), `UPDATE enrollment.holiday_registrations SET debtor_number = '55' WHERE id = ?`, f.siblings)
	require.NoError(t, err)
	_, err = f.db.ExecContext(t.Context(), `UPDATE parent_billing.documents SET debtor_number = '55' WHERE holiday_registration_id = ?`, f.siblings)
	assert.ErrorContains(t, err, "never changes", "an issued invoice refuses a change of its number")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, want, numbers(rec.Body.Bytes()), "the invoice keeps its own copy")

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")), "\n")
	assert.Equal(t, "Rechnungsnummer;Empfänger;Debitorennummer;Kind;Position;Betrag in Euro;Zahlungsweg;Fällig am", strings.TrimSpace(lines[0]))
	assert.Contains(t, rec.Body.String(), ";Muster, Eva;10 4711;Lia Muster;")
	assert.Contains(t, rec.Body.String(), ";Muster, Eva;10 4711;;Summe der Rechnung;", "the sum of the invoice names the number")
	assert.Contains(t, rec.Body.String(), "FE-0002;Muster, Eva;;LIA muster;", "an invoice without a number keeps the cell empty")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=xlsx"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, text := range []string{"Debitorennummer", "10 4711"} {
		assert.Contains(t, xlsxText(t, rec.Body.Bytes()), text)
	}
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=pdf"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, strings.HasPrefix(rec.Body.String(), "%PDF"), "the PDF renders the same table")
}

func TestDebtorNumberShowsInListDetailAndExportOfTheTraegerOnly(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Anzeige")
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, map[string]any{"debtor_number": "10 4711"}).Code)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := decodeSchoolPortalData[struct {
		Registrations []struct {
			ID           int64  `json:"id"`
			DebtorNumber string `json:"debtor_number"`
		} `json:"registrations"`
	}](t, rec.Body.Bytes())
	got := map[int64]string{}
	for _, registration := range list.Registrations {
		got[registration.ID] = registration.DebtorNumber
	}
	assert.Equal(t, map[int64]string{f.siblings: "10 4711", f.again: ""}, got, "the list names the number")
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, f.siblings).DebtorNumber, "the detail names the number")

	for _, format := range []string{"csv", "xlsx"} {
		req := httptest.NewRequest(http.MethodGet, f.path("/export?format="+format), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		if format == "csv" {
			assert.Contains(t, rec.Body.String(), ";Telefon zweites Elternteil;Debitorennummer;")
			assert.Contains(t, rec.Body.String(), ";10 4711;")
			continue
		}
		assert.Contains(t, xlsxText(t, rec.Body.Bytes()), "Debitorennummer")
	}

	readOnly := traegerColleague(t, f, identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Debitor Anzeige Fremd", identityaccess.TraegerRoleAdministration)
	assert.Equal(t, http.StatusForbidden, f.changeDebtorNumber(t, readOnly, f.siblings, "1").Code, "a role without the right gets 403")
	assert.Equal(t, http.StatusForbidden, confirmationSettings(t, f, readOnly).Code)
	assert.Equal(t, http.StatusNotFound, f.changeDebtorNumber(t, foreign.token, f.siblings, "1").Code, "another Träger does not change it")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger does not read it")
	assert.NotContains(t, rec.Body.String(), "4711")
	assert.Equal(t, "10 4711", f.debtorNumberDetail(t, f.siblings).DebtorNumber)
}

func decodeSchoolPortalRaw(t *testing.T, body []byte) json.RawMessage {
	t.Helper()
	return decodeSchoolPortalData[json.RawMessage](t, body)
}

func jsonInt(value int64) string {
	raw, _ := json.Marshal(value)
	return string(raw)
}

// A holiday invoice without a number of its registration takes the debtor
// number of the family, as the monthly invoice does (#6677): the warning
// before the release counts it, and the release copies it into the invoice
// and so into the overview for the Kämmerei.
func TestHolidayInvoiceTakesTheDebtorNumberOfTheFamily(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Debitor Familie")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	_, err := f.db.ExecContext(t.Context(), `UPDATE care_contract.contract_parties SET email = 'EVA.Muster@example.org'
		WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, familyBillingPath,
		map[string]any{"contract_id": contract, "recipient_kind": "applicant", "debtor_number": "20 815"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	require.Equal(t, http.StatusOK, f.confirmWith(t, f.siblings, nil).Code)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	askForDebtorNumber(t, f, true)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	assert.Equal(t, 1, run.MissingDebtorNumbers, "the number of the family counts; only the family without one is missing")

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	numbers := map[int64]string{}
	for _, document := range decodeSchoolPortalData[billingDebtorBody](t, rec.Body.Bytes()).Documents {
		numbers[document.RegistrationID] = document.DebtorNumber
	}
	assert.Equal(t, map[int64]string{f.siblings: "20 815", f.again: ""}, numbers, "the release copies the number of the family")
	assert.Empty(t, f.debtorNumberDetail(t, f.siblings).DebtorNumber, "the registration keeps its own, empty number")

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), ";Muster, Eva;20 815;", "the overview for the Kämmerei names the number of the family")
}
