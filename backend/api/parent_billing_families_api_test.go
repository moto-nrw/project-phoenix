// Router-level tests for the invoices and amounts per family (#4691): every
// document of a family across runs, who pays each invoice, the sums
// issued, cancelled, credited and open, the right "Abrechnung", the Träger
// boundary and the trail of every run the view reads.
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

const billingFamiliesPath = "/traeger/billing-families"

type billingFamilyBody struct {
	ID                 int64    `json:"id"`
	RecipientFirstName string   `json:"recipient_first_name"`
	RecipientLastName  string   `json:"recipient_last_name"`
	RecipientEmail     string   `json:"recipient_email"`
	DebtorNumbers      []string `json:"debtor_numbers"`
	Payments           []struct {
		Kind           string `json:"kind"`
		CostBearerName string `json:"cost_bearer_name"`
	} `json:"payments"`
	InvoiceCount     int   `json:"invoice_count"`
	OpenInvoiceCount int   `json:"open_invoice_count"`
	DocumentCount    int   `json:"document_count"`
	IssuedCents      int64 `json:"issued_cents"`
	CancelledCents   int64 `json:"cancelled_cents"`
	CreditedCents    int64 `json:"credited_cents"`
	OpenCents        int64 `json:"open_cents"`
	Documents        []struct {
		ID              int64    `json:"id"`
		RunID           int64    `json:"run_id"`
		Kind            string   `json:"kind"`
		Number          string   `json:"number"`
		PeriodName      string   `json:"period_name"`
		ReferenceNumber string   `json:"reference_number"`
		Reason          string   `json:"reason"`
		ChildNames      []string `json:"child_names"`
		TotalCents      int64    `json:"total_cents"`
		Status          string   `json:"status"`
		OpenCents       int64    `json:"open_cents"`
		Payment         *struct {
			Kind string `json:"kind"`
		} `json:"payment"`
		RecipientEmail *string `json:"recipient_email"`
	} `json:"documents"`
}

func billingFamilies(t *testing.T, f reviewFixture, token string) map[string]billingFamilyBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingFamiliesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	byEmail := map[string]billingFamilyBody{}
	for _, family := range decodeSchoolPortalData[[]billingFamilyBody](t, rec.Body.Bytes()) {
		byEmail[family.RecipientEmail] = family
	}
	return byEmail
}

func billingFamily(t *testing.T, f reviewFixture, token string, id int64) billingFamilyBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, fmt.Sprintf("%s/%d", billingFamiliesPath, id), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[billingFamilyBody](t, rec.Body.Bytes())
}

// releaseSecondPeriodFor registers one more family of another period with
// the address, confirms and bills it, and returns the released run.
func releaseSecondPeriodFor(t *testing.T, f reviewFixture, billing, email string) int64 {
	t.Helper()
	second := openHolidays(t, f.holidayRegistrationFixture, f.office)
	offer := holidayOffer(t, f.holidayRegistrationFixture, second.PublicToken)
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(second.PublicToken), siblingsRegistration(offer, email))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	registration := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, holidayConfirmPath(second.ID, registration.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, second.ID).ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return run.ID
}

func TestBillingFamiliesShowEveryDocumentWhoPaysAndWhatIsOpen(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Familien Beträge")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	released := correctedRun(t, f, billing, run.ID)
	first, second := documentByNumber(t, released, "FE-0001"), documentByNumber(t, released, "FE-0002")
	require.NotEqual(t, first.RegistrationID, second.RegistrationID)
	child := second.Children[0]
	require.Greater(t, child.TotalCents, int64(500))

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, first.ID), map[string]any{"reason": "Anschrift falsch"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, creditPath(run.ID, second.ID),
		map[string]any{"child_id": child.ChildID, "amount_cents": 500, "reason": "Erster Tag krank"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// The family of the first invoice registers again for another period,
	// with its address in other letters: it stays one family.
	var firstEmail string
	require.NoError(t, f.db.NewRaw(`SELECT recipient_email FROM parent_billing.documents WHERE id = ?`, first.ID).
		Scan(context.Background(), &firstEmail))
	secondRun := releaseSecondPeriodFor(t, f, billing, "EVA.Muster@Example.org")
	require.Equal(t, "eva.muster@example.org", firstEmail)

	families := billingFamilies(t, f, billing)
	require.Len(t, families, 2, "two addresses, two families")
	eva, papa := families["EVA.Muster@Example.org"], families["papa.muster@example.org"]
	require.NotZero(t, eva.ID, "the family shows the address of its latest document")
	assert.Equal(t, first.ID, eva.ID, "a family is named by its first document")
	assert.Equal(t, 2, eva.InvoiceCount, "the invoice of each period")
	assert.Equal(t, 3, eva.DocumentCount, "two invoices and the cancellation")
	assert.Equal(t, first.TotalCents, eva.CancelledCents)
	assert.Zero(t, eva.CreditedCents)
	assert.Equal(t, eva.IssuedCents-eva.CancelledCents, eva.OpenCents, "the cancelled invoice asks for nothing, the new one for all")
	assert.Equal(t, 1, eva.OpenInvoiceCount)

	assert.Equal(t, second.TotalCents, papa.IssuedCents)
	assert.EqualValues(t, 500, papa.CreditedCents)
	assert.Zero(t, papa.CancelledCents)
	assert.Equal(t, second.TotalCents-500, papa.OpenCents, "open is what the credit leaves")
	require.Len(t, papa.Payments, 1)
	assert.Equal(t, "family", papa.Payments[0].Kind)
	assert.Empty(t, papa.Documents, "the list carries no documents")

	detail := billingFamily(t, f, billing, papa.ID)
	assert.Equal(t, papa.OpenCents, detail.OpenCents)
	require.Len(t, detail.Documents, 2)
	credit, invoice := detail.Documents[0], detail.Documents[1]
	assert.Equal(t, "credit", credit.Kind, "the newest document first")
	assert.Equal(t, second.Number, credit.ReferenceNumber)
	assert.Equal(t, "Erster Tag krank", credit.Reason)
	assert.Nil(t, credit.Payment, "a credit has no payer")
	assert.Equal(t, "invoice", invoice.Kind)
	assert.Equal(t, "partly_credited", invoice.Status)
	assert.Equal(t, second.TotalCents-500, invoice.OpenCents)
	require.NotNil(t, invoice.Payment)
	assert.Equal(t, "family", invoice.Payment.Kind)
	assert.NotEmpty(t, invoice.ChildNames)
	assert.NotEmpty(t, invoice.PeriodName)
	assert.Nil(t, invoice.RecipientEmail, "a document does not repeat the recipient")

	evaDetail := billingFamily(t, f, billing, eva.ID)
	require.Len(t, evaDetail.Documents, 3)
	statuses := map[string]string{}
	runs := map[int64]bool{}
	for _, document := range evaDetail.Documents {
		statuses[document.Number] = document.Status
		runs[document.RunID] = true
	}
	assert.Equal(t, "cancelled", statuses[first.Number])
	assert.True(t, runs[secondRun], "the family reaches across runs")
	// Any document of the family opens it.
	assert.Equal(t, eva.ID, billingFamily(t, f, billing, evaDetail.Documents[0].ID).ID)

	// Every read stands in the trail of every run it showed.
	var views []string
	require.NoError(t, f.db.NewRaw(`SELECT details->>'view' FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'read' AND details->>'view' IS NOT NULL ORDER BY id`, secondRun).Scan(context.Background(), &views))
	assert.Equal(t, []string{"families", "family", "family"}, views)
	var documentIDs []int64
	require.NoError(t, f.db.NewRaw(`SELECT jsonb_array_elements_text(details->'document_ids')::bigint FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'read' AND details->>'view' = 'family' ORDER BY id DESC LIMIT 2`, run.ID).
		Scan(context.Background(), &documentIDs))
	assert.NotEmpty(t, documentIDs, "the trail names the documents the reader saw")
}

func TestBillingFamiliesNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Familien Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Familien Fremd", identityaccess.TraegerRoleBilling)
	run := releasedBillingRun(t, f, billing)
	familyPath := fmt.Sprintf("%s/%d", billingFamiliesPath, run.Documents[0].ID)

	for _, path := range []string{billingFamiliesPath, familyPath} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "GET %s without the right Abrechnung: %s", path, rec.Body.String())
	}

	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, billingFamiliesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]billingFamilyBody](t, rec.Body.Bytes()), "another Träger lists none of these families")
	for _, path := range []string{familyPath, billingFamiliesPath + "/999999999"} {
		rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, path, nil)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		code, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, "parent_billing.family_not_found", code)
	}
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.run_events WHERE run_id = ? AND details->>'view' IS NOT NULL`, run.ID).
		Scan(context.Background(), &reads))
	assert.Zero(t, reads, "refused reads leave no trail")
}

func TestBillingFamilyListQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Familien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, billingFamiliesPath, nil)
		req.Header.Set("Authorization", "Bearer "+billing)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Queries())
	}
	releasedBillingRun(t, f, billing)
	families := list()
	releaseSecondPeriodFor(t, f, billing, "neu.familie@example.org")
	assert.Len(t, list().Statements, len(families.Statements), "the list reads and records the same statements for any number of families and runs")
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_families", families)
}
