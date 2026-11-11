// Router-level tests of the release contract of a holiday run (#6506,
// ADR 0204): the release names the fingerprint of the preview the
// Abrechnung checked and refuses other content with the same count and sum,
// before any number is reserved or invoice written. Every request runs in a
// transaction of its own: the review, each correction and the release.
package api

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// assertNothingIssued checks that a refused release left neither an invoice
// nor a used number behind.
func assertNothingIssued(t *testing.T, f reviewFixture, runID int64) {
	t.Helper()
	var documents, counters int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.documents WHERE run_id = ?`, runID).Scan(context.Background(), &documents))
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.number_range_counters WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &counters))
	assert.Zero(t, documents, "a refused release writes no invoice")
	assert.Zero(t, counters, "a refused release reserves no number")
}

func correctChild(t *testing.T, f reviewFixture, token string, runID, registrationID, childID, totalCents int64, reason string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunPath(runID, "/corrections"),
		map[string]any{"registration_id": registrationID, "child_id": childID, "total_cents": totalCents, "reason": reason})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func refusedAsChanged(t *testing.T, f reviewFixture, token string, runID int64, checked billingRunBody) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunPath(runID, "/release"), releaseBody(checked))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_changed", code)
}

// TestHolidayReleaseRefusesAmountsMovedBetweenFamilies is the case of the
// issue: A checks family X with 10 € and family Y with 20 €, B corrects them
// to 20 € and 10 €, and A releases with the preview A checked. Count and sum
// are the same; the content is not. The release refuses and issues nothing;
// the preview read again is releasable.
func TestHolidayReleaseRefusesAmountsMovedBetweenFamilies(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Fingerabdruck")
	checker := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	corrector := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	require.Equal(t, http.StatusOK, f.act(t, f.again, "confirm").Code)
	setNumberRange(t, f, checker, "FE-{NNNN}")
	run := startBillingRun(t, f, checker, f.period.ID)
	preview := billingRun(t, f, checker, run.ID)
	require.Len(t, preview.Documents, 2)
	x, y := preview.Documents[0], preview.Documents[1]
	require.Equal(t, f.siblings, x.RegistrationID)
	xChild, yChild := x.Children[0].ChildID, y.Children[0].ChildID
	correctChild(t, f, corrector, run.ID, x.RegistrationID, xChild, 1000, "Geschwister zahlen 10 Euro")
	correctChild(t, f, corrector, run.ID, y.RegistrationID, yChild, 2000, "Zwei Tage gebucht")

	checked := billingRun(t, f, checker, run.ID)
	assert.NotEmpty(t, checked.Fingerprint)
	assert.Equal(t, checked.Fingerprint, billingRun(t, f, checker, run.ID).Fingerprint, "reading again changes no fingerprint")
	correctChild(t, f, corrector, run.ID, x.RegistrationID, xChild, 2000, "Doch zwei Tage")
	correctChild(t, f, corrector, run.ID, y.RegistrationID, yChild, 1000, "Doch nur ein Tag")
	current := billingRun(t, f, checker, run.ID)
	require.Equal(t, checked.PreviewCount, current.PreviewCount, "the count stays")
	require.Equal(t, checked.PreviewCents, current.PreviewCents, "the sum stays")
	assert.NotEqual(t, checked.Fingerprint, current.Fingerprint, "the content does not")

	refusedAsChanged(t, f, checker, run.ID, checked)
	assertNothingIssued(t, f, run.ID)
	stale := releaseBody(current)
	stale["fingerprint"] = ""
	rec := traegerJSON(t, f.traeger, checker, http.MethodPost, billingRunPath(run.ID, "/release"), stale)
	require.Equal(t, http.StatusConflict, rec.Code, "a release without a fingerprint checked nothing: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, checker, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(current))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Documents, 2)
	assert.Equal(t, int64(2000), released.Documents[0].Children[0].TotalCents, "the release issues what was checked last")
	assert.Equal(t, int64(1000), released.Documents[1].Children[0].TotalCents)
}

// TestHolidayReleaseRefusesAChangedRecipientOrPaymentOfTheSameSum: a debtor
// number entered, a payment term, a payment method and a recipient changed
// after the review keep every amount, and still make the release refuse.
func TestHolidayReleaseRefusesAChangedRecipientOrPaymentOfTheSameSum(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Zahlung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := startBillingRun(t, f, billing, f.period.ID)
	checked := billingRun(t, f, billing, run.ID)

	rec := f.changeDebtorNumber(t, f.office.token, f.siblings, "10 4711")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	current := billingRun(t, f, billing, run.ID)
	require.Equal(t, checked.PreviewCents, current.PreviewCents)
	refusedAsChanged(t, f, billing, run.ID, checked)

	setTerms(t, f.traeger, billing, "holiday", map[string]any{"payment_method": "transfer", "due_days": 30})
	changed := billingRun(t, f, billing, run.ID)
	require.Equal(t, current.PreviewCents, changed.PreviewCents)
	refusedAsChanged(t, f, billing, run.ID, current)

	setTerms(t, f.traeger, billing, "holiday", map[string]any{"payment_method": "direct_debit", "due_days": 30})
	debit := billingRun(t, f, billing, run.ID)
	require.Equal(t, changed.PreviewCents, debit.PreviewCents)
	refusedAsChanged(t, f, billing, run.ID, changed)
	setTerms(t, f.traeger, billing, "holiday", map[string]any{"payment_method": "transfer", "due_days": 30})
	changed = billingRun(t, f, billing, run.ID)

	rec = f.editDetails(t, f.siblings, map[string]any{
		"guardian": map[string]any{"first_name": "Eva", "last_name": "Muster", "email": "eva.neu@example.org", "phone": "0151 1234567"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	recipient := billingRun(t, f, billing, run.ID)
	require.Equal(t, changed.PreviewCents, recipient.PreviewCents)
	require.Equal(t, "eva.neu@example.org", recipient.Documents[0].RecipientEmail)
	refusedAsChanged(t, f, billing, run.ID, changed)
	assertNothingIssued(t, f, run.ID)
	changed = recipient

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(changed))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}
