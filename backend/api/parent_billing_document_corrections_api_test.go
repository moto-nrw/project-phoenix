// Router-level tests for the Storno, the Gutschrift and the Neuausstellung
// of an issued invoice (#4690, ADR 0119 addendum): each is a document of
// its own with a number of the same number range, a reason and a reference
// to the invoice; a cancelled invoice is issued again by the next run of
// the period; the history of every invoice names them; the letters and the
// overview for the Kämmerei list them with their minus; the right
// "Abrechnung" and the Träger boundary hold.
package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type correctedDocumentBody struct {
	ID                  int64  `json:"id"`
	RunID               int64  `json:"run_id"`
	Kind                string `json:"kind"`
	Number              string `json:"number"`
	RegistrationID      int64  `json:"registration_id"`
	ReferenceDocumentID *int64 `json:"reference_document_id"`
	ReferenceNumber     string `json:"reference_number"`
	Reason              string `json:"reason"`
	TotalCents          int64  `json:"total_cents"`
	Children            []struct {
		ChildID    int64 `json:"child_id"`
		TotalCents int64 `json:"total_cents"`
		Positions  []struct {
			Kind        string `json:"kind"`
			AmountCents int64  `json:"amount_cents"`
			Note        string `json:"note"`
		} `json:"positions"`
	} `json:"children"`
	History []struct {
		ID         int64  `json:"id"`
		RunID      int64  `json:"run_id"`
		Kind       string `json:"kind"`
		Number     string `json:"number"`
		TotalCents int64  `json:"total_cents"`
		Reason     string `json:"reason"`
	} `json:"history"`
}

type correctedRunBody struct {
	ID           int64                   `json:"id"`
	Status       string                  `json:"status"`
	PreviewCount int                     `json:"preview_count"`
	PreviewCents int64                   `json:"preview_cents"`
	Documents    []correctedDocumentBody `json:"documents"`
}

func cancellationPath(runID, documentID int64) string {
	return billingRunPath(runID, fmt.Sprintf("/documents/%d/cancellation", documentID))
}

func creditPath(runID, documentID int64) string {
	return billingRunPath(runID, fmt.Sprintf("/documents/%d/credits", documentID))
}

func correctedRun(t *testing.T, f reviewFixture, token string, id int64) correctedRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[correctedRunBody](t, rec.Body.Bytes())
}

func documentByNumber(t *testing.T, run correctedRunBody, number string) correctedDocumentBody {
	t.Helper()
	for _, document := range run.Documents {
		if document.Number == number {
			return document
		}
	}
	require.Failf(t, "document missing", "%s not in run %d", number, run.ID)
	return correctedDocumentBody{}
}

func exportCSV(t *testing.T, f reviewFixture, token string, runID int64) string {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(runID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")
}

func TestIssuedInvoicesAreCancelledCreditedAndIssuedAgainAsDocumentsOfTheirOwn(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Storno Ferien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	released := correctedRun(t, f, billing, run.ID)
	require.Len(t, released.Documents, 2)
	first, second := documentByNumber(t, released, "FE-0001"), documentByNumber(t, released, "FE-0002")
	assert.Equal(t, "invoice", first.Kind)
	assert.Empty(t, first.History, "an invoice starts without history")

	// A Storno needs a reason.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, first.ID), map[string]any{"reason": "  "})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_document_correction_invalid", code)

	// The Storno is its own document: the next number of the same range,
	// every position with its minus and the reference to the invoice.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, first.ID),
		map[string]any{"reason": "Anschrift falsch, neue Rechnung folgt."})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	cancellation := decodeSchoolPortalData[correctedDocumentBody](t, rec.Body.Bytes())
	assert.Equal(t, "cancellation", cancellation.Kind)
	assert.Equal(t, "FE-0003", cancellation.Number)
	assert.Equal(t, "FE-0001", cancellation.ReferenceNumber)
	require.NotNil(t, cancellation.ReferenceDocumentID)
	assert.Equal(t, first.ID, *cancellation.ReferenceDocumentID)
	assert.Equal(t, -first.TotalCents, cancellation.TotalCents)
	assert.Equal(t, "Anschrift falsch, neue Rechnung folgt.", cancellation.Reason)
	for _, child := range cancellation.Children {
		for _, position := range child.Positions {
			assert.LessOrEqual(t, position.AmountCents, int64(0), "every position of a Storno has its minus")
		}
	}

	// The run has its letters, so the Storno has its letter at once.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, cancellation.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, `attachment; filename="storno-fe-0003.pdf"`, rec.Header().Get("Content-Disposition"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))

	// A cancelled invoice is neither cancelled nor credited again.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, first.ID), map[string]any{"reason": "Doppelt"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.invoice_cancelled", code)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, creditPath(run.ID, first.ID),
		map[string]any{"child_id": first.Children[0].ChildID, "amount_cents": 100, "reason": "Krank"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.invoice_cancelled", code)
	// A Storno is no invoice to cancel.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, cancellation.ID), map[string]any{"reason": "Storno"})
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_document_not_found", code)

	// A Gutschrift lowers one child's amount, at most to zero.
	child := second.Children[0]
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, creditPath(run.ID, second.ID),
		map[string]any{"child_id": child.ChildID, "amount_cents": child.TotalCents + 1, "reason": "Krank"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_document_correction_invalid", code)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, creditPath(run.ID, second.ID),
		map[string]any{"child_id": child.ChildID, "amount_cents": 500, "reason": "Erster Tag krank"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	credit := decodeSchoolPortalData[correctedDocumentBody](t, rec.Body.Bytes())
	assert.Equal(t, "credit", credit.Kind)
	assert.Equal(t, "FE-0004", credit.Number)
	assert.Equal(t, "FE-0002", credit.ReferenceNumber)
	assert.Equal(t, int64(-500), credit.TotalCents)
	require.Len(t, credit.Children, 1)
	require.Len(t, credit.Children[0].Positions, 1)
	assert.Equal(t, "credit", credit.Children[0].Positions[0].Kind)
	assert.Equal(t, "Erster Tag krank", credit.Children[0].Positions[0].Note)
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, credit.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, `attachment; filename="gutschrift-fe-0004.pdf"`, rec.Header().Get("Content-Disposition"))

	// A credited invoice is not cancelled; the rest is credited instead.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, second.ID), map[string]any{"reason": "Abgemeldet"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.invoice_credited", code)

	// The run lists every document; the history of each invoice names its
	// Storno and Gutschrift. The sum is what the families still owe.
	detail := correctedRun(t, f, billing, run.ID)
	require.Len(t, detail.Documents, 4)
	assert.Equal(t, 2, detail.PreviewCount, "two invoices")
	assert.Equal(t, first.TotalCents+second.TotalCents-first.TotalCents-500, detail.PreviewCents)
	history := documentByNumber(t, detail, "FE-0001").History
	require.Len(t, history, 1)
	assert.Equal(t, "cancellation", history[0].Kind)
	assert.Equal(t, "FE-0003", history[0].Number)
	history = documentByNumber(t, detail, "FE-0002").History
	require.Len(t, history, 1)
	assert.Equal(t, "credit", history[0].Kind)
	assert.Equal(t, int64(-500), history[0].TotalCents)

	// The overview for the Kämmerei lists Storno and Gutschrift as lines of
	// their own with their minus and the invoice they belong to.
	csv := exportCSV(t, f, billing, run.ID)
	lines := strings.Split(strings.TrimSpace(csv), "\n")
	assert.Equal(t, "Nummer;Art;Bezug;Empfänger;Kind;Position;Betrag in Euro", strings.TrimSpace(lines[0]))
	assert.Contains(t, csv, "FE-0001;Rechnung;;")
	assert.Contains(t, csv, "FE-0003;Storno;zu FE-0001;")
	assert.Contains(t, csv, ";Summe des Stornos;-"+euroText(first.TotalCents))
	assert.Contains(t, csv, "FE-0004;Gutschrift;zu FE-0002;")
	assert.Contains(t, csv, ";Gutschrift: Erster Tag krank;-5,00")
	assert.Contains(t, csv, ";Summe der Gutschrift;-5,00")
	assert.Equal(t, ";;;;;Gesamtsumme des Laufs;"+euroText(detail.PreviewCents), strings.TrimSpace(lines[len(lines)-1]))

	// The cancelled invoice no longer bills its registration: the next run
	// of the period issues it again with a new number and names the invoice
	// it replaces.
	next := startBillingRun(t, f, billing, f.period.ID)
	preview := correctedRun(t, f, billing, next.ID)
	require.Len(t, preview.Documents, 1, "only the cancelled family")
	assert.Equal(t, first.RegistrationID, preview.Documents[0].RegistrationID)
	assert.Equal(t, "FE-0001", preview.Documents[0].ReferenceNumber)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(next.ID, "/release"),
		map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissued := correctedRun(t, f, billing, next.ID).Documents[0]
	assert.Equal(t, "FE-0005", reissued.Number)
	assert.Equal(t, "invoice", reissued.Kind)
	require.NotNil(t, reissued.ReferenceDocumentID)
	assert.Equal(t, first.ID, *reissued.ReferenceDocumentID)
	assert.Contains(t, exportCSV(t, f, billing, next.ID), "FE-0005;Rechnung;ersetzt FE-0001;")

	history = documentByNumber(t, correctedRun(t, f, billing, run.ID), "FE-0001").History
	require.Len(t, history, 2, "the Storno and the new invoice")
	assert.Equal(t, "invoice", history[1].Kind)
	assert.Equal(t, "FE-0005", history[1].Number)
	assert.Equal(t, next.ID, history[1].RunID, "the new invoice belongs to the next run")

	// A third run of the period has nothing left to bill.
	third := startBillingRun(t, f, billing, f.period.ID)
	assert.Empty(t, correctedRun(t, f, billing, third.ID).Documents)

	actions := runActions(t, f, run.ID)
	assert.Contains(t, actions, "cancel")
	assert.Contains(t, actions, "credit")
	var changed error
	_, changed = f.db.ExecContext(context.Background(), `UPDATE parent_billing.documents SET reason = 'x' WHERE id = ?`, cancellation.ID)
	assert.ErrorContains(t, changed, "never changes", "a Storno never changes either")
}

func TestStornoAndGutschriftNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Storno Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Storno Fremd", identityaccess.TraegerRoleBilling)
	run := releasedBillingRun(t, f, billing)
	invoice := correctedRun(t, f, billing, run.ID).Documents[0]
	credit := map[string]any{"child_id": invoice.Children[0].ChildID, "amount_cents": 100, "reason": "Krank"}
	cancel := map[string]any{"reason": "Abgemeldet"}

	for _, request := range []struct {
		path string
		body any
	}{{cancellationPath(run.ID, invoice.ID), cancel}, {creditPath(run.ID, invoice.ID), credit}} {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right Abrechnung: %s", request.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPost, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s of another Träger: %s", request.path, rec.Body.String())
	}
	// An invoice of the Träger in another run is not part of this one.
	other := startBillingRun(t, f, billing, f.period.ID)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(other.ID, invoice.ID), cancel)
	assert.Equal(t, http.StatusConflict, rec.Code, "a run in preview has no invoice: %s", rec.Body.String())

	var documents int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.documents WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &documents))
	assert.Equal(t, 2, documents, "refused requests issued nothing")
}

// TestStornoBeforeTheLettersGetsItsLetterWithTheLettersOfTheRun cancels an
// invoice before the run has its letters: the Storno has no letter yet and
// gets it when the letters of the run are created, after its invoice.
func TestStornoBeforeTheLettersGetsItsLetterWithTheLettersOfTheRun(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Storno Schreiben")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	invoice := correctedRun(t, f, billing, run.ID).Documents[0]
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, invoice.ID), map[string]any{"reason": "Abgemeldet"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	cancellation := decodeSchoolPortalData[correctedDocumentBody](t, rec.Body.Bytes())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, cancellation.ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, "no letter before the letters of the run: %s", rec.Body.String())

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, cancellation.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, `attachment; filename="storno-fe-0003.pdf"`, rec.Header().Get("Content-Disposition"))
}
