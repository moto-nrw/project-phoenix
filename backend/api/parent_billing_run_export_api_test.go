// Router-level tests for the overview of a released billing run for the
// Kämmerei (#4672, ADR 0122): the positions and sums per family as PDF,
// Excel and CSV, only for a released run, behind the right "Abrechnung",
// within the Träger and recorded in the run's trail.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// euroText writes cents as the overview does, for amounts below 1.000 Euro.
func euroText(cents int64) string {
	return fmt.Sprintf("%d,%02d", cents/100, cents%100)
}

func TestReleasedBillingRunExportsTheOverviewForTheKaemmerei(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Lauf Kämmerei")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Lauf Kämmerei Fremd", identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	run := startBillingRun(t, f, billing, f.period.ID)
	setNumberRange(t, f, billing, "FE-{NNNN}")
	preview := billingRun(t, f, billing, run.ID)
	lia := preview.Documents[0].Children[0]
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/corrections"),
		map[string]any{"registration_id": f.siblings, "child_id": lia.ChildID, "total_cents": 1000, "reason": "Härtefall"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// A run in preview has issued nothing the Kämmerei could collect.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_run_not_released", code)

	checked := billingRun(t, f, billing, run.ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(checked))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[billingRunBody](t, rec.Body.Bytes())
	require.Len(t, released.Documents, 1)
	invoice := released.Documents[0]

	// The CSV lists every position of the family, the sum of its invoice
	// and the sum of the run; the mail address of the parent stays out.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "text/csv; charset=utf-8", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `attachment; filename="abrechnungsanordnung-`)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")), "\n")
	assert.Equal(t, "Rechnungsnummer;Empfänger;Kind;Position;Betrag in Euro", strings.TrimSpace(lines[0]))
	positions := 0
	for _, child := range invoice.Children {
		positions += len(child.Positions)
	}
	require.Len(t, lines, 1+positions+2, "a line per position, the sum of the invoice and the sum of the run")
	assert.True(t, strings.HasPrefix(lines[1], "FE-0001;Muster, Eva;Lia "), lines[1])
	assert.Contains(t, rec.Body.String(), ";Korrektur: Härtefall;", "a correction is a position with its reason")
	assert.Equal(t, "FE-0001;Muster, Eva;;Summe der Rechnung;"+euroText(invoice.TotalCents), strings.TrimSpace(lines[len(lines)-2]))
	assert.Equal(t, ";;;Gesamtsumme des Laufs;"+euroText(*released.TotalCents), strings.TrimSpace(lines[len(lines)-1]))
	assert.NotContains(t, rec.Body.String(), invoice.RecipientEmail)
	assert.Contains(t, rec.Body.String(), ";Geschwister: 2. Kind, 50 % weniger;-", "a discount is a negative amount")
	assert.NotContains(t, rec.Body.String(), ";'-", "a negative amount stays a number, not text")

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=xlsx"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", rec.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(rec.Body.String(), "PK"), "an Excel workbook")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=pdf"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(rec.Body.String(), "%PDF"), "a PDF")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=docx"), nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	// Without the right Abrechnung 403, another Träger 404; neither is in
	// the trail. Every download is.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	var formats []string
	require.NoError(t, f.db.NewRaw(`SELECT details->>'format' FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'export' AND actor_account_id IS NOT NULL ORDER BY id`, run.ID).
		Scan(context.Background(), &formats))
	assert.Equal(t, []string{"csv", "xlsx", "pdf"}, formats)
}
