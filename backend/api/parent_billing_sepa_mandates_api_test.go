// Router-level tests of the SEPA direct debit (#4695, ADR 0171): the
// creditor of the Träger, the mandate of a family with its sealed IBAN, its
// trail, the pre-notification an invoice copies, the fallback to a
// transfer or the refused release without a mandate, the right
// "Abrechnung" and the Träger boundary.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const (
	sepaCreditorPath = "/traeger/sepa-creditor"
	sepaMandatesPath = "/traeger/sepa-mandates"
	sepaIBAN         = "DE89370400440532013000"
	sepaCreditorID   = "DE98ZZZ09999999999"
)

type sepaMandateBody struct {
	ID              int64   `json:"id"`
	ContractID      int64   `json:"contract_id"`
	FamilyEmail     string  `json:"family_email"`
	PartyLastName   string  `json:"party_last_name"`
	Reference       string  `json:"reference"`
	SignedOn        string  `json:"signed_on"`
	Kind            string  `json:"kind"`
	Sequence        string  `json:"sequence"`
	AccountHolder   string  `json:"account_holder"`
	CreditorID      string  `json:"creditor_id"`
	CreditorCurrent bool    `json:"creditor_current"`
	IBANMasked      string  `json:"iban_masked"`
	Source          string  `json:"source"`
	RevokedOn       *string `json:"revoked_on"`
	RevokedAt       *string `json:"revoked_at"`
	CreatedAtValid  string  `json:"created_at"`
}

type sepaPaymentBody struct {
	Method   string `json:"method"`
	DueOn    string `json:"due_on"`
	DebitGap string `json:"debit_gap"`
	Mandate  *struct {
		MandateID    int64  `json:"mandate_id"`
		Reference    string `json:"reference"`
		CreditorID   string `json:"creditor_id"`
		IBANMasked   string `json:"iban_masked"`
		SequenceType string `json:"sequence_type"`
		NoticeShort  bool   `json:"notice_short"`
	} `json:"mandate"`
}

type sepaRunBody struct {
	ID       int64 `json:"id"`
	Invoices []struct {
		ID         int64           `json:"id"`
		ContractID int64           `json:"contract_id"`
		Payment    sepaPaymentBody `json:"payment"`
	} `json:"invoices"`
	PreviewCount int    `json:"preview_count"`
	PreviewCents int64  `json:"preview_cents"`
	Fingerprint  string `json:"fingerprint"`
}

func sepaRun(t *testing.T, f careBillingFixture, id int64) sepaRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[sepaRunBody](t, rec.Body.Bytes())
}

func saveCreditor(t *testing.T, f careBillingFixture, body map[string]any) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, sepaCreditorPath, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func createMandate(t *testing.T, f careBillingFixture, body map[string]any) sepaMandateBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), sepaIBAN, "no answer carries the IBAN")
	return decodeSchoolPortalData[sepaMandateBody](t, rec.Body.Bytes())
}

func mandateBody(contract int64, reference string) map[string]any {
	return map[string]any{"contract_id": contract, "reference": reference, "signed_on": "2026-09-01", "kind": "recurring",
		"sequence": "recurring", "account_holder": "Eva Beispiel", "iban": "de89 3704 0044 0532 0130 00"}
}

func mandateEvents(t *testing.T, f careBillingFixture, mandate int64) []string {
	t.Helper()
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action || ':' || source FROM parent_billing.sepa_mandate_events
		WHERE mandate_id = ? ORDER BY id`, mandate).Scan(context.Background(), &actions))
	return actions
}

// The SJR case: the care is collected by direct debit. The office keeps the
// creditor and the mandate of the family; the IBAN is sealed at rest and
// only its mask leaves. The run shows the mandate on the invoice, the
// release copies it, and every read and change is in the mandate's trail.
func TestSEPAMandateReachesTheInvoiceAsPrenotification(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Lastschrift SJR")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setTerms(t, f.traeger, f.office.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})

	// The defaults, a creditor identifier with wrong check digits, then the
	// identifier of the Träger.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, sepaCreditorPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"prenotification_days":14`)
	assert.Contains(t, rec.Body.String(), `"missing_mandate":"transfer"`)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, sepaCreditorPath,
		map[string]any{"creditor_id": "DE97ZZZ09999999999", "prenotification_days": 3, "missing_mandate": "transfer"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_creditor_invalid", code)
	assert.Equal(t, []string{"creditor_id"}, fields)
	saveCreditor(t, f, map[string]any{"creditor_id": "de98 zzz0 9999 9999 99", "prenotification_days": 3, "missing_mandate": "transfer"})

	// A wrong IBAN names the field, never the value.
	wrong := mandateBody(contract, "SJR-2026-0001")
	wrong["iban"] = "DE89370400440532013001"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, wrong)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_mandate_invalid", code)
	assert.Equal(t, []string{"iban"}, fields)
	assert.NotContains(t, rec.Body.String(), "DE89370400440532013001")

	mandate := createMandate(t, f, mandateBody(contract, "SJR-2026-0001"))
	assert.Equal(t, "SJR-2026-0001", mandate.Reference)
	assert.Equal(t, "DE•• •••• •••• •••• ••30 00", mandate.IBANMasked)
	assert.Equal(t, "Beispiel", mandate.PartyLastName)
	assert.Equal(t, "office", mandate.Source)
	assert.Nil(t, mandate.RevokedOn)

	// Sealed at rest: no column holds the IBAN, its digits or the account
	// holder; the row as text does not either.
	var sealedHoldsPlain bool
	require.NoError(t, f.db.NewRaw(`SELECT position(convert_to(?, 'UTF8') IN iban_sealed) > 0
			OR position(convert_to(?, 'UTF8') IN iban_sealed) > 0
			OR position(convert_to(?, 'UTF8') IN account_holder_sealed) > 0
			OR position(? IN mandate::text) > 0 OR position(? IN mandate::text) > 0
		FROM parent_billing.sepa_mandates AS mandate WHERE id = ?`, sepaIBAN, "0532013000", "Eva Beispiel", "Eva Beispiel", sepaIBAN,
		mandate.ID).Scan(context.Background(), &sealedHoldsPlain))
	assert.False(t, sealedHoldsPlain, "IBAN and account holder are sealed at rest")
	assert.Equal(t, "Eva Beispiel", mandate.AccountHolder, "the owner opens the account holder for the office")

	// One mandate in force per family; a reference once per Träger.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, mandateBody(contract, "SJR-2026-0002"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_mandate_in_force")
	other := careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, mandateBody(other, "SJR-2026-0001"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_mandate_reference_taken")

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-01-02", "2030-01-04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := sepaRun(t, f, run.ID)
	var lia *sepaPaymentBody
	for i := range preview.Invoices {
		if preview.Invoices[i].ContractID == contract {
			lia = &preview.Invoices[i].Payment
		}
	}
	require.NotNil(t, lia, "the run bills the contract")
	assert.Equal(t, "direct_debit", lia.Method)
	require.NotNil(t, lia.Mandate, "the invoice carries the mandate in force")
	assert.Equal(t, "SJR-2026-0001", lia.Mandate.Reference)
	assert.Equal(t, sepaCreditorID, lia.Mandate.CreditorID)
	assert.Equal(t, "DE•• •••• •••• •••• ••30 00", lia.Mandate.IBANMasked)
	assert.Equal(t, "RCUR", lia.Mandate.SequenceType)
	assert.Contains(t, mandateEvents(t, f, mandate.ID), "read:run", "the run's read of the mask is in the trail")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var copied string
	require.NoError(t, f.db.NewRaw(`SELECT concat_ws('|', mandate_reference, mandate_signed_on, creditor_id, debtor_iban_masked, sequence_type)
		FROM parent_billing.care_invoices WHERE run_id = ? AND care_contract_id = ?`, run.ID, contract).Scan(context.Background(), &copied))
	assert.Equal(t, "SJR-2026-0001|2026-09-01|DE98ZZZ09999999999|DE•• •••• •••• •••• ••30 00|RCUR", copied,
		"the release copies the mandate onto the invoice")

	// A revocation ends the mandate; the revoked one stays as history, and
	// the issued invoice keeps its copy.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, fmt.Sprintf("%s/%d/revoke", sepaMandatesPath, mandate.ID),
		map[string]any{"revoked_on": "2026-10-15"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"revoked_on":"2026-10-15"`)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, fmt.Sprintf("%s/%d", sepaMandatesPath, mandate.ID),
		map[string]any{"account_holder": "Jan Beispiel", "sequence": "recurring"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_mandate_revoked")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s?contract_id=%d", sepaMandatesPath, contract), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	listed := decodeSchoolPortalData[[]sepaMandateBody](t, rec.Body.Bytes())
	require.Len(t, listed, 1)
	assert.NotNil(t, listed[0].RevokedOn)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/%d/history", sepaMandatesPath, mandate.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	// The list after the revocation and the trail itself are reads of the
	// bank data, each with its own entry.
	assert.Equal(t, []string{"create:office", "read:run", "revoke:office", "read:office", "read:office"}, mandateEvents(t, f, mandate.ID))
	require.NoError(t, f.db.NewRaw(`SELECT mandate_reference FROM parent_billing.care_invoices WHERE run_id = ? AND care_contract_id = ?`,
		run.ID, contract).Scan(context.Background(), &copied))
	assert.Equal(t, "SJR-2026-0001", copied)

	// A new mandate after the revocation; a change of its account is
	// recorded with the fields, never the values.
	next := createMandate(t, f, mandateBody(contract, "SJR-2026-0003"))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, fmt.Sprintf("%s/%d", sepaMandatesPath, next.ID),
		map[string]any{"account_holder": "Jan Beispiel", "iban": "AT611904300234573201", "sequence": "recurring"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"iban_masked":"AT•• •••• •••• •••• 3201"`)
	var fieldsChanged string
	require.NoError(t, f.db.NewRaw(`SELECT array_to_string(fields, ',') FROM parent_billing.sepa_mandate_events
		WHERE mandate_id = ? AND action = 'update'`, next.ID).Scan(context.Background(), &fieldsChanged))
	assert.Equal(t, "account_holder,iban", fieldsChanged)
}

// Without a mandate the Träger's choice decides: by default the invoice
// goes as a transfer with the hint in the review; with "block" the release
// refuses the run.
func TestDirectDebitWithoutMandateFallsBackToTransferOrBlocksTheRelease(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Lastschrift ohne Mandat")
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setTerms(t, f.traeger, f.office.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-01-02", "2030-01-04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	preview := sepaRun(t, f, run.ID)
	require.Len(t, preview.Invoices, 1)
	assert.Equal(t, sepaPaymentBody{Method: "transfer", DueOn: "2030-01-02", DebitGap: "mandate_missing"}, preview.Invoices[0].Payment,
		"a transfer on the run's transfer date, with the hint")

	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "block"})
	blocked := sepaRun(t, f, run.ID)
	assert.Equal(t, sepaPaymentBody{Method: "direct_debit", DueOn: "2030-01-04", DebitGap: "mandate_missing"}, blocked.Invoices[0].Payment)
	assert.NotEqual(t, preview.Fingerprint, blocked.Fingerprint, "the choice is part of the reviewed state")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"),
		map[string]any{"document_count": blocked.PreviewCount, "total_cents": blocked.PreviewCents, "fingerprint": blocked.Fingerprint})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.direct_debit_without_mandate")
	var issued int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE run_id = ?`, run.ID).Scan(context.Background(), &issued))
	assert.Zero(t, issued, "the refused release issues nothing")
}

func TestSEPAMandatesNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Lastschrift Grenze")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	mandate := createMandate(t, f, mandateBody(contract, "SJR-G-1"))

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	item := fmt.Sprintf("%s/%d", sepaMandatesPath, mandate.ID)
	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, sepaCreditorPath, nil},
		{http.MethodPut, sepaCreditorPath, map[string]any{"creditor_id": "", "prenotification_days": 14, "missing_mandate": "transfer"}},
		{http.MethodGet, sepaMandatesPath, nil},
		{http.MethodPost, sepaMandatesPath, mandateBody(contract, "SJR-G-2")},
		{http.MethodPut, item, map[string]any{"account_holder": "X", "sequence": "first"}},
		{http.MethodPost, item + "/revoke", map[string]any{"revoked_on": "2026-10-01"}},
		{http.MethodGet, item + "/history", nil},
	} {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	const mandateState = `SELECT encode(account_holder_sealed, 'hex') || '|' || (revoked_on IS NULL)::text
		FROM parent_billing.sepa_mandates WHERE id = ?`
	var before string
	require.NoError(t, f.db.NewRaw(mandateState, mandate.ID).Scan(context.Background(), &before))
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Lastschrift Fremd", identityaccess.TraegerRoleBilling)
	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, sepaMandatesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]sepaMandateBody](t, rec.Body.Bytes()), "no mandate of another Träger")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, sepaCreditorPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), sepaCreditorID, "no creditor of another Träger")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPost, sepaMandatesPath, mandateBody(contract, "SJR-F-1"))
	assert.Equal(t, http.StatusConflict, rec.Code, "a contract of another Träger names no family here: %s", rec.Body.String())
	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPut, item, map[string]any{"account_holder": "Fremd", "sequence": "first"}},
		{http.MethodPost, item + "/revoke", map[string]any{"revoked_on": "2026-10-01"}},
		{http.MethodGet, item + "/history", nil},
	} {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	var after string
	require.NoError(t, f.db.NewRaw(mandateState, mandate.ID).Scan(context.Background(), &after))
	assert.Equal(t, before, after, "the mandate stays as its Träger set it")
	assert.Contains(t, after, "|true", "and in force")
}

// The list reads the same statements for any number of mandates.
func TestSEPAMandatesQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Lastschrift Budget")
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	measure := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, sepaMandatesPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	mandateFor := func(child string) {
		contract := careContract(t, f.db, f.office.organization, f.office.schools[0], child, careMonth, sjrServices()...)
		createMandate(t, f, mandateBody(contract, "SJR-Q-"+child))
	}
	mandateFor("Mia")
	first := measure()
	mandateFor("Noah")
	mandateFor("Lia")
	assert.Len(t, measure().Statements, len(first.Statements), "the mandates read the same statements for any number")
	querybudgets.AssertQueryBudget(t, "api.traeger.sepa_mandates", first)
}

func TestSEPAMandateReplacementGetsANewAutomaticReference(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Ersatzmandat")
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	body := mandateBody(contract, "")
	first := createMandate(t, f, body)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, fmt.Sprintf("%s/%d/revoke", sepaMandatesPath, first.ID),
		map[string]any{"revoked_on": "2026-10-15"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	next := createMandate(t, f, body)
	assert.NotEqual(t, first.Reference, next.Reference)
	assert.NotEmpty(t, next.Reference)
	assert.LessOrEqual(t, len(next.Reference), 35)
	// The unique constraint still reserves the revoked reference.
	other := careContract(t, f.db, f.office.organization, f.office.schools[0], "Noah", careMonth, sjrServices()...)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, mandateBody(other, first.Reference))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_mandate_reference_taken")
}

func TestSEPAMandateChangeKeepsTheBICRequiredByTheStoredIBAN(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger BIC")
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	body := mandateBody(contract, "CH-1")
	body["iban"], body["bic"] = "CH9300762011623852957", "POFICHBEXXX"
	mandate := createMandate(t, f, body)
	path := fmt.Sprintf("%s/%d", sepaMandatesPath, mandate.ID)
	change := map[string]any{"account_holder": "Eva Beispiel", "iban": "", "bic": "", "sequence": "recurring"}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, change)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_mandate_invalid", code)
	assert.Equal(t, []string{"bic"}, fields)
	change["bic"] = "POFICHBEXXX"
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, change)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, mandate.IBANMasked, decodeSchoolPortalData[sepaMandateBody](t, rec.Body.Bytes()).IBANMasked)
	// Switching to an EEA account permits clearing the old BIC.
	change["iban"], change["bic"] = sepaIBAN, ""
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, path, change)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// A mandate is bound to the creditor identifier it was given to (ADR 0171).
// After the Träger changes its identifier, the mandate stays as it was, is no
// proof for the new creditor and the invoice goes as a transfer with the hint;
// a new mandate is collected again. A mandate needs a creditor identifier.
func TestSEPAMandateStaysBoundToItsCreditor(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Gläubigerwechsel")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaMandatesPath, mandateBody(contract, "SJR-W-0"))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_creditor_missing", "no mandate without a creditor identifier")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setTerms(t, f.traeger, f.office.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 1, "missing_mandate": "transfer"})
	mandate := createMandate(t, f, mandateBody(contract, "SJR-W-1"))
	assert.Equal(t, sepaCreditorID, mandate.CreditorID)
	assert.True(t, mandate.CreditorCurrent)

	saveCreditor(t, f, map[string]any{"creditor_id": "DE98ABC09999999999", "prenotification_days": 1, "missing_mandate": "transfer"})
	var bound string
	require.NoError(t, f.db.NewRaw(`SELECT creditor_id FROM parent_billing.sepa_mandates WHERE id = ?`, mandate.ID).
		Scan(context.Background(), &bound))
	assert.Equal(t, sepaCreditorID, bound, "a change of the creditor identifier leaves the mandate as it was given")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s?contract_id=%d", sepaMandatesPath, contract), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	listed := decodeSchoolPortalData[[]sepaMandateBody](t, rec.Body.Bytes())
	require.Len(t, listed, 1)
	assert.False(t, listed[0].CreditorCurrent, "the list shows that the mandate needs renewing")

	run := f.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-01-02", "2030-01-04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := sepaRun(t, f, run.ID)
	require.Len(t, preview.Invoices, 1)
	assert.Equal(t, sepaPaymentBody{Method: "transfer", DueOn: "2030-01-02", DebitGap: "creditor_changed"}, preview.Invoices[0].Payment,
		"no direct debit with a mandate of the earlier creditor")
}

// Every read of a mandate's bank data is in its trail: a change that changes
// nothing still shows the account, and so does the trail itself.
func TestSEPAMandateReadsWithoutChangeAreAudited(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandat lesen")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	mandate := createMandate(t, f, mandateBody(contract, "SJR-R-1"))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, fmt.Sprintf("%s/%d", sepaMandatesPath, mandate.ID),
		map[string]any{"account_holder": "Eva Beispiel", "iban": "", "bic": "", "sequence": "recurring"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"create:office", "read:office"}, mandateEvents(t, f, mandate.ID), "an unchanged account is a read")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/%d/history", sepaMandatesPath, mandate.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"create:office", "read:office", "read:office"}, mandateEvents(t, f, mandate.ID), "the trail is a read too")
	var actors int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandate_events WHERE mandate_id = ? AND actor_account_id IS NOT NULL`,
		mandate.ID).Scan(context.Background(), &actors))
	assert.Equal(t, 3, actors, "every entry names the account")
}
