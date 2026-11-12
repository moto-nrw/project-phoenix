// Router-level tests for the corrections a reissued invoice carries over
// (#5663, ADR 0119 addendum): the preview of the next run proposes the
// corrections of the cancelled invoice with their origin, the Abrechnung
// changes or removes each before the release, and a Gutschrift of the
// cancelled invoice is named but never carried.
package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
)

const carriedCorrectionCents = int64(2000)

type carriedCorrectionBody struct {
	PositionID        int64  `json:"position_id"`
	SourceNumber      string `json:"source_number"`
	AmountCents       int64  `json:"amount_cents"`
	SourceAmountCents int64  `json:"source_amount_cents"`
	Reason            string `json:"reason"`
}

type carriedChildBody struct {
	ChildID            int64                   `json:"child_id"`
	TotalCents         int64                   `json:"total_cents"`
	CarriedCorrections []carriedCorrectionBody `json:"carried_corrections"`
	Corrections        []struct {
		AmountCents       int64  `json:"amount_cents"`
		TotalBeforeCents  int64  `json:"total_before_cents"`
		TotalAfterCents   int64  `json:"total_after_cents"`
		Reason            string `json:"reason"`
		CarriedFromNumber string `json:"carried_from_number"`
	} `json:"corrections"`
	Positions []struct {
		Kind        string `json:"kind"`
		AmountCents int64  `json:"amount_cents"`
		Note        string `json:"note"`
	} `json:"positions"`
}

type carriedDocumentBody struct {
	ID              int64              `json:"id"`
	Number          string             `json:"number"`
	RegistrationID  int64              `json:"registration_id"`
	ReferenceNumber string             `json:"reference_number"`
	TotalCents      int64              `json:"total_cents"`
	Children        []carriedChildBody `json:"children"`
	ReplacedCredits []struct {
		Number     string `json:"number"`
		TotalCents int64  `json:"total_cents"`
		Reason     string `json:"reason"`
	} `json:"replaced_credits"`
}

type carriedRunBody struct {
	ID           int64                 `json:"id"`
	Status       string                `json:"status"`
	PreviewCount int                   `json:"preview_count"`
	PreviewCents int64                 `json:"preview_cents"`
	Fingerprint  string                `json:"fingerprint"`
	Documents    []carriedDocumentBody `json:"documents"`
}

func carriedRun(t *testing.T, f reviewFixture, token string, id int64) carriedRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
}

func carriedCorrectionPath(runID, positionID int64) string {
	return billingRunPath(runID, fmt.Sprintf("/carried-corrections/%d", positionID))
}

func releaseCarriedRun(t *testing.T, f reviewFixture, token string, run carriedRunBody) carriedDocumentBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunPath(run.ID, "/release"),
		map[string]any{"document_count": run.PreviewCount, "total_cents": run.PreviewCents, "fingerprint": run.Fingerprint})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := carriedRun(t, f, token, run.ID)
	require.Len(t, released.Documents, 1)
	return released.Documents[0]
}

// correctedAndCancelled releases a run in which one child's amount is lowered
// by carriedCorrectionCents, cancels that invoice and returns the preview of
// the next run of the period, with the child's amount before the correction.
func correctedAndCancelled(t *testing.T, f reviewFixture, billing string) (carriedRunBody, int64) {
	t.Helper()
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := carriedRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	require.Len(t, run.Documents, 1)
	document, child := run.Documents[0], run.Documents[0].Children[0]
	require.Greater(t, child.TotalCents, carriedCorrectionCents)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"), map[string]any{
		"registration_id": document.RegistrationID, "child_id": child.ChildID,
		"total_cents": child.TotalCents - carriedCorrectionCents, "reason": "Erster Tag krank",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	run = decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	issued := releaseCarriedRun(t, f, billing, run)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, issued.ID), map[string]any{"reason": "Anschrift falsch"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return carriedRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID), child.TotalCents
}

func TestReissuedInvoiceProposesTheCorrectionsOfTheCancelledInvoice(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur übernehmen")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next, frozen := correctedAndCancelled(t, f, billing)

	require.Len(t, next.Documents, 1)
	draft := next.Documents[0]
	assert.Equal(t, "FE-0001", draft.ReferenceNumber)
	assert.Equal(t, frozen-carriedCorrectionCents, draft.Children[0].TotalCents, "the draft bills the corrected amount")
	child := draft.Children[0]
	require.Len(t, child.CarriedCorrections, 1)
	carried := child.CarriedCorrections[0]
	assert.Equal(t, -carriedCorrectionCents, carried.AmountCents)
	assert.Equal(t, -carriedCorrectionCents, carried.SourceAmountCents)
	assert.Equal(t, "Erster Tag krank", carried.Reason)
	assert.Equal(t, "FE-0001", carried.SourceNumber)
	assert.Positive(t, carried.PositionID)
	last := child.Positions[len(child.Positions)-1]
	assert.Equal(t, "correction", last.Kind)
	assert.Equal(t, -carriedCorrectionCents, last.AmountCents)
	assert.Equal(t, "Erster Tag krank", last.Note)

	issued := releaseCarriedRun(t, f, billing, next)
	assert.Equal(t, frozen-carriedCorrectionCents, issued.Children[0].TotalCents, "the release issues the corrected amount")
	issuedLast := issued.Children[0].Positions[len(issued.Children[0].Positions)-1]
	assert.Equal(t, -carriedCorrectionCents, issuedLast.AmountCents)
	assert.Equal(t, "Erster Tag krank", issuedLast.Note)

	// The issued invoice keeps the correction in its history with its origin.
	require.Len(t, issued.Children[0].Corrections, 1)
	history := issued.Children[0].Corrections[0]
	assert.Equal(t, "FE-0001", history.CarriedFromNumber)
	assert.Equal(t, frozen, history.TotalBeforeCents)
	assert.Equal(t, frozen-carriedCorrectionCents, history.TotalAfterCents)
	assert.Equal(t, "Erster Tag krank", history.Reason)
}

func TestRemovedCarriedCorrectionIssuesTheFrozenAmount(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur entfernen")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next, frozen := correctedAndCancelled(t, f, billing)
	carried := next.Documents[0].Children[0].CarriedCorrections[0]

	rec := traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried.PositionID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	removed := decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	assert.Equal(t, frozen, removed.Documents[0].Children[0].TotalCents)
	assert.Empty(t, removed.Documents[0].Children[0].CarriedCorrections)
	assert.Contains(t, runActions(t, f, next.ID), "correct", "the removal stands in the trail of the run")

	// It stays removed on the next read and cannot be removed twice.
	assert.Equal(t, frozen, carriedRun(t, f, billing, next.ID).Documents[0].Children[0].TotalCents)
	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried.PositionID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.carried_correction_not_found", code)

	issued := releaseCarriedRun(t, f, billing, removed)
	assert.Equal(t, frozen, issued.Children[0].TotalCents, "the release issues the frozen amount")
	for _, position := range issued.Children[0].Positions {
		assert.NotEqual(t, "correction", position.Kind)
	}
	assert.Empty(t, issued.Children[0].Corrections)

	// A released run keeps what it issued.
	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried.PositionID), nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

func TestChangedCarriedCorrectionReplacesTheProposal(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur ändern")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next, frozen := correctedAndCancelled(t, f, billing)
	carried := next.Documents[0].Children[0].CarriedCorrections[0]
	path := carriedCorrectionPath(next.ID, carried.PositionID)

	// The amount without the correction is what removing it does.
	rec := traegerJSON(t, f.traeger, billing, http.MethodPut, path, map[string]any{"total_cents": frozen, "reason": "Doch nicht krank"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_correction_invalid", code)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, path, map[string]any{"total_cents": frozen - 500, "reason": " "})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Len(t, carriedRun(t, f, billing, next.ID).Documents[0].Children[0].CarriedCorrections, 1, "a refusal keeps the proposal")

	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, path, map[string]any{"total_cents": frozen - 500, "reason": "Nur ein halber Tag krank"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	changed := decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	child := changed.Documents[0].Children[0]
	assert.Equal(t, frozen-500, child.TotalCents)
	assert.Empty(t, child.CarriedCorrections)
	require.Len(t, child.Corrections, 1)
	assert.Equal(t, frozen, child.Corrections[0].TotalBeforeCents)
	assert.Equal(t, int64(-500), child.Corrections[0].AmountCents)
	assert.Empty(t, child.Corrections[0].CarriedFromNumber, "the changed correction is the office's own")

	issued := releaseCarriedRun(t, f, billing, changed)
	assert.Equal(t, frozen-500, issued.Children[0].TotalCents)
	last := issued.Children[0].Positions[len(issued.Children[0].Positions)-1]
	assert.Equal(t, int64(-500), last.AmountCents)
	assert.Equal(t, "Nur ein halber Tag krank", last.Note)
}

// raisedAndZeroedCancelled releases an invoice with two corrections of one
// child, a raise and then down to zero, cancels it and returns the next run.
func raisedAndZeroedCancelled(t *testing.T, f reviewFixture, billing string) carriedRunBody {
	t.Helper()
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	first := carriedRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	document, child := first.Documents[0], first.Documents[0].Children[0]
	for _, total := range []int64{child.TotalCents + 5000, 0} {
		rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(first.ID, "/corrections"), map[string]any{
			"registration_id": document.RegistrationID, "child_id": child.ChildID, "total_cents": total, "reason": "Nachbuchung",
		})
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		first = decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	}
	issued := releaseCarriedRun(t, f, billing, first)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(first.ID, issued.ID), map[string]any{"reason": "Anschrift falsch"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return carriedRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
}

// TestCarriedCorrectionStopsAtZero removes the raise: the second proposal then
// would take more than the child's amount. It lowers the amount only to zero
// and keeps what the cancelled invoice had, so the release is not negative.
func TestCarriedCorrectionStopsAtZero(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur Grenze")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next := raisedAndZeroedCancelled(t, f, billing)
	carried := next.Documents[0].Children[0].CarriedCorrections
	require.Len(t, carried, 2)

	rec := traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried[0].PositionID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	after := decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	assert.Equal(t, int64(0), after.Documents[0].Children[0].TotalCents)
	left := after.Documents[0].Children[0].CarriedCorrections
	require.Len(t, left, 1)
	assert.Equal(t, carried[1].SourceAmountCents, left[0].SourceAmountCents)
	assert.Greater(t, left[0].AmountCents, left[0].SourceAmountCents, "the proposal stops at zero and keeps what the invoice had")
	assert.Equal(t, int64(0), releaseCarriedRun(t, f, billing, after).Children[0].TotalCents)
}

// TestRemovingACarriedCorrectionCannotPushTheChildBelowZero corrects the child
// on top of a proposed raise: without the raise that correction would leave
// less than nothing, so the removal is refused and changes nothing.
func TestRemovingACarriedCorrectionCannotPushTheChildBelowZero(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur Rest")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next := raisedAndZeroedCancelled(t, f, billing)
	carried := next.Documents[0].Children[0].CarriedCorrections
	rec := traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried[1].PositionID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	raised := decodeSchoolPortalData[carriedRunBody](t, rec.Body.Bytes())
	document, child := raised.Documents[0], raised.Documents[0].Children[0]
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(next.ID, "/corrections"), map[string]any{
		"registration_id": document.RegistrationID, "child_id": child.ChildID, "total_cents": 0, "reason": "Doch nicht gebucht",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried[0].PositionID), nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_correction_invalid", code)
	after := carriedRun(t, f, billing, next.ID).Documents[0]
	assert.Equal(t, int64(0), after.Children[0].TotalCents)
	assert.Len(t, after.Children[0].CarriedCorrections, 1, "the refusal rolled the removal back")
}

func TestGutschriftOfTheCancelledInvoiceIsNamedButNotCarried(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Gutschrift Hinweis")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	next, frozen := correctedAndCancelled(t, f, billing)
	cancelled := next.Documents[0].ReferenceNumber

	// The routes refuse a Gutschrift of a cancelled invoice and a Storno of a
	// credited one, so only a document from before that rule has this state;
	// it is written directly.
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `INSERT INTO parent_billing.documents (
			organization_id, run_id, service, number, number_year, number_sequence, issued_on, holiday_period_id,
			holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email, service_from, service_until,
			total_cents, kind, reference_document_id, reason)
		SELECT organization_id, run_id, service, 'FE-9000', number_year, 9000, issued_on, holiday_period_id,
			holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email, service_from, service_until,
			-500, 'credit', id, 'Ausflug fiel aus'
		FROM parent_billing.documents WHERE organization_id = ? AND number = ?`, f.office.organization, cancelled)
	require.NoError(t, err)

	draft := carriedRun(t, f, billing, next.ID).Documents[0]
	require.Len(t, draft.ReplacedCredits, 1)
	assert.Equal(t, "FE-9000", draft.ReplacedCredits[0].Number)
	assert.Equal(t, int64(-500), draft.ReplacedCredits[0].TotalCents)
	assert.Equal(t, "Ausflug fiel aus", draft.ReplacedCredits[0].Reason)
	assert.Equal(t, frozen-carriedCorrectionCents, draft.Children[0].TotalCents, "the Gutschrift does not lower the draft")
	require.Len(t, draft.Children[0].CarriedCorrections, 1, "only the correction is proposed")
	assert.Equal(t, "Erster Tag krank", draft.Children[0].CarriedCorrections[0].Reason)
}

func TestCarriedCorrectionsNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Korrektur Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Korrektur Fremd", identityaccess.TraegerRoleBilling)
	next, frozen := correctedAndCancelled(t, f, billing)
	carried := next.Documents[0].Children[0].CarriedCorrections[0]
	path := carriedCorrectionPath(next.ID, carried.PositionID)
	change := map[string]any{"total_cents": frozen - 100, "reason": "Fremd"}

	for _, request := range []struct {
		method string
		body   any
	}{{http.MethodDelete, nil}, {http.MethodPut, change}} {
		rec := traegerJSON(t, f.traeger, f.office.token, request.method, path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right Abrechnung: %s", request.method, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, request.method, path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s of another Träger: %s", request.method, rec.Body.String())
	}
	// A position that is no proposal of this run is not found.
	rec := traegerJSON(t, f.traeger, billing, http.MethodDelete, carriedCorrectionPath(next.ID, carried.PositionID+100000), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	assert.Equal(t, frozen-carriedCorrectionCents, carriedRun(t, f, billing, next.ID).Documents[0].Children[0].TotalCents, "refused requests change nothing")
}
