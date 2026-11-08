// Router-level tests of who gets an invoice, how it is paid and by when
// (#4689, ADR 0167): the terms and bundling per service, the deviation of a
// family, the due dates of a run, the copy an issued invoice keeps, the
// right "Abrechnung", the Träger boundary and the trail.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const (
	billingTermsPath  = "/traeger/billing-terms"
	familyBillingPath = "/traeger/family-billing"
	runDueDatesPath   = "/traeger/billing-run-due-dates"
)

type paymentBody struct {
	Method    string `json:"method"`
	DueOn     string `json:"due_on"`
	DueSource string `json:"due_source"`
}

type termsInvoiceBody struct {
	FamilyEmail string `json:"family_email"`
	ID          int64  `json:"id"`
	Number      string `json:"number"`
	ContractID  int64  `json:"contract_id"`
	Recipient   struct {
		FirstName  string `json:"first_name"`
		LastName   string `json:"last_name"`
		Email      string `json:"email"`
		Kind       string `json:"kind"`
		SecondName string `json:"second_name"`
		Address    *struct {
			Street string `json:"street"`
			City   string `json:"city"`
		} `json:"address"`
	} `json:"recipient"`
	Payment    *paymentBody `json:"payment"`
	TotalCents int64        `json:"total_cents"`
}

type termsRunBody struct {
	ID       int64              `json:"id"`
	Status   string             `json:"status"`
	Invoices []termsInvoiceBody `json:"invoices"`
	DueDates []struct {
		PaymentMethod string `json:"payment_method"`
		DueOn         string `json:"due_on"`
	} `json:"due_dates"`
	PreviewCount int   `json:"preview_count"`
	PreviewCents int64 `json:"preview_cents"`
}

type termsDocumentBody struct {
	Number         string       `json:"number"`
	RecipientEmail string       `json:"recipient_email"`
	RecipientKind  string       `json:"recipient_kind"`
	Payment        *paymentBody `json:"payment"`
	TotalCents     int64        `json:"total_cents"`
}

type familyBillingBody struct {
	ID            int64  `json:"id"`
	FamilyEmail   string `json:"family_email"`
	ContractID    int64  `json:"contract_id"`
	RecipientKind string `json:"recipient_kind"`
	PayerEmail    string `json:"payer_email"`
	PaymentMethod string `json:"payment_method"`
	DueDay        *int   `json:"due_day"`
}

func termsRun(t *testing.T, f careBillingFixture, token string, id int64) termsRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[termsRunBody](t, rec.Body.Bytes())
}

func setTerms(t *testing.T, router http.Handler, token, service string, body map[string]any) {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodPut, billingTermsPath+"/"+service, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func dueDates(transfer, debit string) map[string]any {
	return map[string]any{"due_dates": []map[string]any{
		{"payment_method": "transfer", "due_on": transfer}, {"payment_method": "direct_debit", "due_on": debit},
	}}
}

func runDueDatesPathOf(kind string, id int64) string {
	return runDueDatesPath + "/" + kind + "/" + strconv.FormatInt(id, 10)
}

func partyEmailOf(t *testing.T, f careBillingFixture, contract int64) string {
	t.Helper()
	var email string
	require.NoError(t, f.db.NewRaw(`SELECT email FROM care_contract.contract_parties WHERE contract_id = ?`, contract).
		Scan(context.Background(), &email))
	return email
}

// The SJR case of #4689: one Träger bills the monthly care by direct debit
// and the holiday care by transfer. Each run's due date per payment method
// reaches every invoice of that method; the holiday invoice stays an invoice
// of its own with its own number range, and both keep what the release
// copied.
func TestCareByDirectDebitAndHolidayByTransferStaySeparateInvoices(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Zahlungsweg SJR")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	testpkg.OwnTraegerCareContracts(t, f.db, f.office.organization)
	care := careBillingFixture{db: f.db, traeger: f.traeger, office: traegerOffice{organization: f.office.organization,
		schools: f.office.schools, token: billing}}
	setNumberRange(t, f, billing, "FE-{NNNN}")
	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, numberRangesPath+"/care", map[string]any{"pattern": "MB-{JJJJ}-{NNNN}"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The defaults before the Träger chooses: transfer within 14 days, the
	// lunch on the invoice of the care, the holiday care on its own.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingTermsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	terms := decodeSchoolPortalData[struct {
		Terms []struct {
			Service       string `json:"service"`
			PaymentMethod string `json:"payment_method"`
			DueDays       int    `json:"due_days"`
			BundleWith    string `json:"bundle_with"`
			Custom        bool   `json:"custom"`
		} `json:"terms"`
	}](t, rec.Body.Bytes()).Terms
	require.Len(t, terms, 3)
	assert.Equal(t, "care", terms[0].Service)
	assert.Equal(t, "transfer", terms[0].PaymentMethod)
	assert.Equal(t, 14, terms[0].DueDays)
	assert.Equal(t, "care", terms[1].BundleWith, "the lunch travels on the care invoice")
	assert.Equal(t, "", terms[2].BundleWith, "the holiday care is billed on its own")
	assert.False(t, terms[0].Custom)

	setTerms(t, f.traeger, billing, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})
	setTerms(t, f.traeger, billing, "holiday", map[string]any{"payment_method": "transfer", "due_days": 14})
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, billingTermsPath+"/holiday",
		map[string]any{"payment_method": "transfer", "due_days": 14, "bundle_with": "care"})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the holiday care never joins the care invoice: %s", rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_terms_invalid", code)

	// The family registered the holidays with the address of its contract.
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'EVA.Muster@example.org'
		WHERE contract_id = ?`, contract)
	require.NoError(t, err)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	monthly := care.startRun(t, careMonth)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, runDueDatesPathOf("care", monthly.ID), dueDates("2030-01-02", "2030-01-04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := termsRun(t, care, billing, monthly.ID)
	require.Len(t, preview.DueDates, 2)
	require.Len(t, preview.Invoices, 1)
	assert.Equal(t, &paymentBody{Method: "direct_debit", DueOn: "2030-01-04", DueSource: "run"}, preview.Invoices[0].Payment,
		"the care is collected on the run's direct debit date")
	assert.Equal(t, "applicant", preview.Invoices[0].Recipient.Kind)

	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	holiday := startBillingRun(t, f, billing, f.period.ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, runDueDatesPathOf("holiday", holiday.ID), dueDates("2026-12-20", "2026-12-22"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	holidayPreview := billingRun(t, f, billing, holiday.ID)

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, careRunPath(monthly.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[termsRunBody](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(holiday.ID, "/release"), releaseBody(holidayPreview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	documents := decodeSchoolPortalData[struct {
		Documents []termsDocumentBody `json:"documents"`
	}](t, rec.Body.Bytes()).Documents

	require.Len(t, released.Invoices, 1)
	require.Len(t, documents, 1)
	monthlyInvoice, holidayInvoice := released.Invoices[0], documents[0]
	assert.Regexp(t, `^MB-`, monthlyInvoice.Number)
	assert.Regexp(t, `^FE-`, holidayInvoice.Number, "the holiday invoice is its own invoice with its own number")
	assert.Equal(t, &paymentBody{Method: "direct_debit", DueOn: "2030-01-04", DueSource: "run"}, monthlyInvoice.Payment)
	assert.Equal(t, &paymentBody{Method: "transfer", DueOn: "2026-12-20", DueSource: "run"}, holidayInvoice.Payment)

	// The release copied the payment: the run's dates and the terms change
	// nothing on the issued invoices.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, runDueDatesPathOf("care", monthly.ID), dueDates("2030-02-02", "2030-02-04"))
	assert.Equal(t, http.StatusConflict, rec.Code, "a released run keeps its dates: %s", rec.Body.String())
	setTerms(t, f.traeger, billing, "care", map[string]any{"payment_method": "transfer", "due_days": 30})
	again := termsRun(t, care, billing, monthly.ID)
	assert.Equal(t, monthlyInvoice.Payment, again.Invoices[0].Payment)
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.care_invoices SET due_on = DATE '2030-03-01' WHERE id = ?`,
		monthlyInvoice.ID)
	require.Error(t, err, "an issued invoice never changes its due date")
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.run_due_dates SET due_on = DATE '2030-03-01'
		WHERE care_run_id = ?`, monthly.ID)
	require.ErrorContains(t, err, "never change", "the database keeps the dates of a released run")
	_, err = f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.run_due_dates (organization_id, care_run_id,
		payment_method, due_on) SELECT organization_id, id, 'transfer', DATE '2030-03-01' FROM parent_billing.care_runs WHERE id = ?`, monthly.ID)
	require.ErrorContains(t, err, "never change", "no date joins a released run")

	// The families view counts the holiday and the monthly invoice of the
	// one family, with both payment methods, and opens both runs.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, "/traeger/billing-families", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	type familyBody struct {
		ID             int64    `json:"id"`
		InvoiceCount   int      `json:"invoice_count"`
		IssuedCents    int64    `json:"issued_cents"`
		PaymentMethods []string `json:"payment_methods"`
		Documents      []struct {
			Service       string `json:"service"`
			RunID         int64  `json:"run_id"`
			Number        string `json:"number"`
			PaymentMethod string `json:"payment_method"`
			PeriodName    string `json:"period_name"`
		} `json:"documents"`
	}
	families := decodeSchoolPortalData[[]familyBody](t, rec.Body.Bytes())
	var both *familyBody
	for i := range families {
		if families[i].InvoiceCount == 2 {
			both = &families[i]
		}
	}
	require.NotNil(t, both, "one family with the holiday and the monthly invoice: %s", rec.Body.String())
	assert.Equal(t, holidayInvoice.TotalCents+monthlyInvoice.TotalCents, both.IssuedCents)
	assert.Equal(t, []string{"direct_debit", "transfer"}, both.PaymentMethods)
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, fmt.Sprintf("/traeger/billing-families/%d", both.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[familyBody](t, rec.Body.Bytes())
	require.Len(t, detail.Documents, 2)
	services := map[string]int64{}
	for _, document := range detail.Documents {
		services[document.Service] = document.RunID
	}
	assert.Equal(t, map[string]int64{"care": monthly.ID, "holiday": holiday.ID}, services, "each document names the run of its service")
	var careReads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events WHERE run_id = ? AND action = 'read'
		AND details->>'view' IN ('families', 'family')`, monthly.ID).Scan(context.Background(), &careReads))
	assert.Equal(t, 2, careReads, "the list and the family page record their read of the monthly invoice")

	// The overview for the Kämmerei names payment method and due date.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(holiday.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Zahlungsweg")
	assert.Contains(t, rec.Body.String(), "Überweisung")
	assert.Contains(t, rec.Body.String(), "20.12.2026")
}

// A family deviation sends the invoice to another payer and lets the family
// pay on its own day of the month; the family stays the address of the
// registration, and an issued invoice keeps the recipient it had.
func TestFamilyDeviationReachesTheNextReleaseOnly(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Zahler")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	family := partyEmailOf(t, f, contract)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	first := f.startRun(t, careMonth)
	firstPreview := termsRun(t, f, f.office.token, first.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(first.ID, "/release"),
		map[string]any{"document_count": firstPreview.PreviewCount, "total_cents": firstPreview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := decodeSchoolPortalData[termsRunBody](t, rec.Body.Bytes()).Invoices[0]
	assert.Equal(t, "Eva", issued.Recipient.FirstName)
	require.NotNil(t, issued.Payment)
	assert.Equal(t, "transfer", issued.Payment.Method, "without a choice: transfer")
	assert.Equal(t, "term", issued.Payment.DueSource, "due after the payment term")

	invalid := []map[string]any{
		{"contract_id": contract, "recipient_kind": "payer", "payer_first_name": "Oma", "payer_last_name": "Muster",
			"payer_email": "kein-mail", "payer_address": map[string]any{"street": "Amtsweg 2", "postal_code": "75175", "city": "Pforzheim"}},
		{"contract_id": contract, "recipient_kind": "payer", "payer_first_name": "Oma", "payer_last_name": "Muster",
			"payer_email": "oma@example.org", "payer_address": map[string]any{"street": "Amtsweg 2", "postal_code": "751", "city": "Pforzheim"}},
		{"contract_id": contract, "recipient_kind": "both"},
		{"contract_id": contract, "recipient_kind": "applicant", "due_day": 31},
		{"contract_id": contract, "recipient_kind": "everyone"},
	}
	for _, body := range invalid {
		rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		code, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, "parent_billing.family_billing_invalid", code)
	}
	day := 15
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, map[string]any{
		"contract_id": contract, "recipient_kind": "payer", "payer_first_name": "Oma", "payer_last_name": "Muster",
		"payer_email": "oma@example.org", "payer_address": map[string]any{"street": "Amtsweg 2", "postal_code": "75175", "city": "Pforzheim"},
		"payment_method": "direct_debit", "due_day": day,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	saved := decodeSchoolPortalData[struct {
		Family *familyBillingBody `json:"family"`
	}](t, rec.Body.Bytes()).Family
	require.NotNil(t, saved)
	assert.Equal(t, family, saved.FamilyEmail, "the family is the address of the registration, not the payer's")

	// The issued invoice keeps its recipient; the next month goes to the payer.
	released := termsRun(t, f, f.office.token, first.ID)
	assert.Equal(t, "Eva", released.Invoices[0].Recipient.FirstName, "issued invoices never change")
	next := f.startRun(t, "2030-02-01")
	preview := termsRun(t, f, f.office.token, next.ID)
	var shown int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing_events WHERE family_billing_id = ? AND action = 'read'`,
		saved.ID).Scan(context.Background(), &shown))
	assert.Positive(t, shown, "a run that shows the payer records the read of the deviation")
	require.Len(t, preview.Invoices, 1)
	invoice := preview.Invoices[0]
	assert.Equal(t, family, invoice.FamilyEmail)
	assert.Equal(t, "payer", invoice.Recipient.Kind)
	assert.Equal(t, "oma@example.org", invoice.Recipient.Email)
	require.NotNil(t, invoice.Recipient.Address)
	assert.Equal(t, "Pforzheim", invoice.Recipient.Address.City)
	require.NotNil(t, invoice.Payment)
	assert.Equal(t, "direct_debit", invoice.Payment.Method)
	assert.Equal(t, "family", invoice.Payment.DueSource)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var familyEmail, kind, street string
	require.NoError(t, f.db.NewRaw(`SELECT family_email, recipient_kind, recipient_address_street FROM parent_billing.care_invoices
		WHERE run_id = ?`, next.ID).Scan(context.Background(), &familyEmail, &kind, &street))
	assert.Equal(t, family, familyEmail, "the release copies the family")
	assert.Equal(t, "payer", kind)
	assert.Equal(t, "Amtsweg 2", street)

	// A family with monthly invoices only opens by the negated ID of its
	// first invoice (#4689): both months, the payer's invoice included.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/billing-families", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	listed := decodeSchoolPortalData[[]struct {
		ID           int64 `json:"id"`
		InvoiceCount int   `json:"invoice_count"`
	}](t, rec.Body.Bytes())
	require.Len(t, listed, 1, "the payer does not split the family")
	assert.Negative(t, listed[0].ID)
	assert.Equal(t, 2, listed[0].InvoiceCount)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("/traeger/billing-families/%d", listed[0].ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"recipient_kind":"payer"`)

	// Back to the registration: the deviation goes away.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, map[string]any{"contract_id": contract, "recipient_kind": "applicant"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"family":null`)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, familyBillingPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]familyBillingBody](t, rec.Body.Bytes()))
	var actions []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM parent_billing.family_billing_events WHERE family_billing_id = ? ORDER BY id`,
		saved.ID).Scan(context.Background(), &actions))
	assert.Equal(t, "save", actions[0], "every change of the deviation is in its trail")
	assert.Equal(t, "delete", actions[len(actions)-1])
	assert.Contains(t, actions, "read")
}

// Every route needs the right "Abrechnung"; another Träger neither reads nor
// changes the terms, deviations or due dates of this one.
func TestBillingTermsNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Zahlung Grenze")
	school := f.office.schools[0]
	contract := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	run := f.startRun(t, careMonth)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, map[string]any{
		"contract_id": contract, "recipient_kind": "both", "second_first_name": "Jan", "second_last_name": "Beispiel"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	deviation := decodeSchoolPortalData[struct {
		Family *familyBillingBody `json:"family"`
	}](t, rec.Body.Bytes()).Family
	require.NotNil(t, deviation)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, familyBillingPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing_events WHERE family_billing_id = ? AND action = 'read'`,
		deviation.ID).Scan(context.Background(), &reads))
	assert.Equal(t, 1, reads, "every read of a deviation is in its trail")

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, billingTermsPath, nil},
		{http.MethodPut, billingTermsPath + "/care", map[string]any{"payment_method": "transfer", "due_days": 1}},
		{http.MethodGet, familyBillingPath, nil},
		{http.MethodPut, familyBillingPath, map[string]any{"contract_id": contract, "recipient_kind": "applicant"}},
		{http.MethodDelete, familyBillingPath + fmt.Sprintf("/%d", deviation.ID), nil},
		{http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-01-02", "2030-01-04")},
		{http.MethodPut, runDueDatesPathOf("holiday", run.ID), dueDates("2030-01-02", "2030-01-04")},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
	}

	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Zahlung Fremd", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, familyBillingPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]familyBillingBody](t, rec.Body.Bytes()), "no deviation of another Träger")
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPut, familyBillingPath, map[string]any{
		"contract_id": contract, "recipient_kind": "payer", "payer_first_name": "Fremd", "payer_last_name": "Zahler",
		"payer_email": "fremd@example.org", "payer_address": map[string]any{"street": "Weg 1", "postal_code": "12345", "city": "Ort"}})
	assert.Equal(t, http.StatusConflict, rec.Code, "a contract of another Träger names no family here: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodDelete, familyBillingPath+fmt.Sprintf("/%d", deviation.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPut, runDueDatesPathOf("care", run.ID), dueDates("2030-01-02", "2030-01-04"))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	setTerms(t, f.traeger, foreign.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 3})

	var kind string
	require.NoError(t, f.db.NewRaw(`SELECT recipient_kind FROM parent_billing.family_billing WHERE id = ?`, deviation.ID).
		Scan(context.Background(), &kind))
	assert.Equal(t, "both", kind, "the deviation stays as its Träger set it")
	preview := termsRun(t, f, f.office.token, run.ID)
	assert.Empty(t, preview.DueDates, "no due date of another Träger")
	assert.Empty(t, preview.Invoices, "no price yet")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, billingTermsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), `"due_days":3`, "the other Träger's terms stay there")
}

// The terms and the deviations read the same statements for any number of
// deviations.
func TestBillingTermsQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Zahlung Budget")
	measure := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	deviate := func(child string) {
		contract := careContract(t, f.db, f.office.organization, f.office.schools[0], child, careMonth, sjrServices()...)
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath, map[string]any{
			"contract_id": contract, "recipient_kind": "applicant", "payment_method": "direct_debit"})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	deviate("Mia")
	setTerms(t, f.traeger, f.office.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})
	families, terms := measure(familyBillingPath), measure(billingTermsPath)
	deviate("Noah")
	deviate("Lia")
	setTerms(t, f.traeger, f.office.token, "holiday", map[string]any{"payment_method": "transfer", "due_days": 10})
	assert.Len(t, measure(familyBillingPath).Statements, len(families.Statements), "the deviations read the same statements for any number")
	assert.Len(t, measure(billingTermsPath).Statements, len(terms.Statements))
	querybudgets.AssertQueryBudget(t, "api.traeger.family_billing", families)
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_terms", terms)
}
