// Router-level tests for the cost takeover of the holiday contribution
// (#5627, ADR 0115 addendum): the cost bearers of a Träger behind the right
// "Abrechnung", the confirmation with a cost bearer, which freezes the rest
// after the sibling discount as the cost bearer's line, the refusals, the
// overview for the Kämmerei, the list for the reimbursement and the proof
// the Träger-Büro uploads.
package api

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const costBearersPath = "/traeger/cost-bearers"

type costBearerBody struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Address       string `json:"address"`
	FileReference string `json:"file_reference"`
	Active        bool   `json:"active"`
	InUse         bool   `json:"in_use"`
}

type takeoverLineBody struct {
	Kind           string `json:"kind"`
	AmountCents    int64  `json:"amount_cents"`
	CostBearerID   *int64 `json:"cost_bearer_id"`
	CostBearerName string `json:"cost_bearer_name"`
}

type takeoverContributionBody struct {
	Children []struct {
		Lines      []takeoverLineBody `json:"lines"`
		TotalCents int64              `json:"total_cents"`
	} `json:"children"`
	TotalCents        int64  `json:"total_cents"`
	BenefitsExemption bool   `json:"benefits_exemption"`
	CostBearerID      *int64 `json:"cost_bearer_id"`
	CostBearerName    string `json:"cost_bearer_name"`
	Frozen            bool   `json:"frozen"`
}

func youthOffice() map[string]any {
	return map[string]any{
		"name": "Fachdienst Jugend des Landkreises Musterstadt", "address": "Am Kreishaus 1\n12345 Musterstadt",
		"file_reference": "FJ 51-2027", "active": true,
	}
}

func createCostBearer(t *testing.T, f reviewFixture, token string, body map[string]any) costBearerBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, costBearersPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[costBearerBody](t, rec.Body.Bytes())
}

func costBearerPath(id int64) string { return fmt.Sprintf("%s/%d", costBearersPath, id) }

func (f reviewFixture) takeoverPreview(t *testing.T, registrationID int64, query string) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, f.office.token, http.MethodGet, holidayContributionPath(f.period.ID, registrationID)+query, nil)
}

func refusalCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	code, _ := registrationRefusalOf(t, rec)
	return code
}

// confirmSiblingsWithTakeover confirms the siblings with the cost bearer
// under the sibling rule: Ben (2 × 45,00 €) is the first child, Lia
// (67,50 €) the second and pays half. The cost bearer pays 90,00 € and
// 33,75 €.
func confirmSiblingsWithTakeover(t *testing.T, f reviewFixture, bearer costBearerBody) takeoverContributionBody {
	t.Helper()
	rec := f.confirmWith(t, f.siblings, map[string]any{"cost_bearer_id": bearer.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Contribution takeoverContributionBody `json:"contribution"`
	}](t, rec.Body.Bytes()).Contribution
}

func TestCostBearersAreKeptPerTraegerAndNamedOnesOnlySwitchOff(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Kostenträger")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Kostenträger Fremd", identityaccess.TraegerRoleBilling)

	office := createCostBearer(t, f, billing, youthOffice())
	assert.Equal(t, "Fachdienst Jugend des Landkreises Musterstadt", office.Name)
	assert.Equal(t, "Am Kreishaus 1\n12345 Musterstadt", office.Address)
	assert.True(t, office.Active)
	unused := createCostBearer(t, f, billing, map[string]any{"name": "  Sozialamt Musterstadt  ", "active": true})
	assert.Equal(t, "Sozialamt Musterstadt", unused.Name, "the name is trimmed")

	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, costBearersPath, map[string]any{"name": " ", "file_reference": "A\nB"})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.cost_bearer_invalid", code)
	assert.ElementsMatch(t, []string{"name", "file_reference"}, fields)

	// The administration confirms with a cost bearer but keeps none; it
	// reads the active ones with the confirmation settings.
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec = traegerJSON(t, f.traeger, f.office.token, method, costBearersPath, youthOffice())
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right: %s", method, rec.Body.String())
	}
	// Another Träger sees none of them and changes none.
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, costBearersPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, decodeSchoolPortalData[[]costBearerBody](t, rec.Body.Bytes()))
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		rec = traegerJSON(t, f.traeger, foreign.token, method, costBearerPath(office.ID), youthOffice())
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s of a foreign cost bearer: %s", method, rec.Body.String())
	}

	confirmSiblingsWithTakeover(t, f, office)
	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, costBearerPath(office.ID), nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "parent_billing.cost_bearer_in_use", refusalCode(t, rec), "a frozen contribution names it")
	switchedOff := youthOffice()
	switchedOff["active"] = false
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, costBearerPath(office.ID), switchedOff)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.False(t, decodeSchoolPortalData[costBearerBody](t, rec.Body.Bytes()).Active)

	// A PUT changes an existing cost bearer and never creates one, also not
	// with ID 0 and also not when repeated.
	for range 2 {
		rec = traegerJSON(t, f.traeger, billing, http.MethodPut, costBearerPath(0), youthOffice())
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Equal(t, "parent_billing.cost_bearer_not_found", refusalCode(t, rec))
	}
	var bearers int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.cost_bearers WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &bearers))
	assert.Equal(t, 2, bearers, "PUT /0 created no cost bearer")

	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, costBearerPath(unused.ID), nil)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodDelete, costBearerPath(unused.ID), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, costBearersPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []costBearerBody{{ID: office.ID, Name: office.Name, Address: office.Address, FileReference: office.FileReference, InUse: true}},
		decodeSchoolPortalData[[]costBearerBody](t, rec.Body.Bytes()), "the switched-off cost bearer stays in the list, marked as named")
}

func TestCostTakeoverFreezesTheRestAfterTheSiblingDiscount(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Kostenübernahme")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Kostenübernahme Fremd", identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	createContributionRule(t, f, billing, benefitsExemption())
	office := createCostBearer(t, f, billing, youthOffice())
	inactive := youthOffice()
	inactive["name"], inactive["active"] = "Altes Sozialamt", false
	switchedOff := createCostBearer(t, f, billing, inactive)
	stranger := createCostBearer(t, f, foreign.token, youthOffice())

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/holiday-contributions/confirmation-settings", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	settings := decodeSchoolPortalData[struct {
		CostBearers []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"cost_bearers"`
	}](t, rec.Body.Bytes())
	require.Len(t, settings.CostBearers, 1, "the Träger-Büro chooses among the active cost bearers of its Träger")
	assert.Equal(t, office.ID, settings.CostBearers[0].ID)

	rec = f.takeoverPreview(t, f.siblings, fmt.Sprintf("?cost_bearer_id=%d", office.ID))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[takeoverContributionBody](t, rec.Body.Bytes())
	assert.False(t, preview.Frozen)
	assert.Equal(t, int64(0), preview.TotalCents)
	assert.Equal(t, office.Name, preview.CostBearerName)

	for _, refused := range []struct {
		name   string
		body   map[string]any
		status int
		code   string
	}{
		{"exemption and takeover", map[string]any{"benefits_exemption": true, "cost_bearer_id": office.ID}, http.StatusConflict, "parent_billing.cost_takeover_with_exemption"},
		{"switched-off cost bearer", map[string]any{"cost_bearer_id": switchedOff.ID}, http.StatusConflict, "parent_billing.cost_bearer_inactive"},
		{"cost bearer of another Träger", map[string]any{"cost_bearer_id": stranger.ID}, http.StatusNotFound, "parent_billing.cost_bearer_not_found"},
	} {
		rec = f.confirmWith(t, f.siblings, refused.body)
		require.Equal(t, refused.status, rec.Code, "%s: %s", refused.name, rec.Body.String())
		assert.Equal(t, refused.code, refusalCode(t, rec), refused.name)
		query := fmt.Sprintf("?cost_bearer_id=%v", refused.body["cost_bearer_id"])
		if refused.body["benefits_exemption"] == true {
			query += "&benefits_exemption=true"
		}
		rec = f.takeoverPreview(t, f.siblings, query)
		assert.Equal(t, refused.status, rec.Code, "the preview refuses the %s too: %s", refused.name, rec.Body.String())
	}
	assert.Equal(t, "received", f.list(t, "").Registrations[0].Status, "every refusal was rolled back")

	frozen := confirmSiblingsWithTakeover(t, f, office)
	require.Len(t, frozen.Children, 2)
	assert.Equal(t, int64(0), frozen.TotalCents, "the family pays nothing")
	assert.False(t, frozen.BenefitsExemption)
	require.NotNil(t, frozen.CostBearerID)
	assert.Equal(t, office.ID, *frozen.CostBearerID)
	kinds := func(lines []takeoverLineBody) []string {
		out := make([]string, len(lines))
		for i, line := range lines {
			out[i] = fmt.Sprintf("%s:%d", line.Kind, line.AmountCents)
		}
		return out
	}
	assert.Equal(t, []string{"base:6750", "sibling:-3375", "cost_takeover:-3375"}, kinds(frozen.Children[0].Lines),
		"Lia is the second child: the cost bearer pays what is left after the sibling discount")
	assert.Equal(t, []string{"base:4500", "base:4500", "cost_takeover:-9000"}, kinds(frozen.Children[1].Lines))
	takeover := frozen.Children[0].Lines[2]
	assert.Equal(t, office.Name, takeover.CostBearerName)

	renamed := youthOffice()
	renamed["name"] = "Jugendamt Musterstadt"
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, costBearerPath(office.ID), renamed)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.takeoverPreview(t, f.siblings, "")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, office.Name, decodeSchoolPortalData[takeoverContributionBody](t, rec.Body.Bytes()).CostBearerName,
		"a rename changes no frozen contribution")
}

func TestCostTakeoverShowsInTheRunTheOverviewAndTheReimbursementList(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Erstattung")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	createContributionRule(t, f, billing, siblingHalf())
	office := createCostBearer(t, f, billing, youthOffice())
	confirmSiblingsWithTakeover(t, f, office)
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	proof := uploadOfficeProof(t, f, f.office.token, f.siblings, "bescheid.pdf", proofPDF)
	require.Equal(t, http.StatusCreated, proof.Code, proof.Body.String())

	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	assert.Equal(t, int64(4500), run.PreviewCents, "the run bills only what the families pay")
	listPath := billingRunPath(run.ID, fmt.Sprintf("/cost-takeovers/%d/export", office.ID))
	rec := traegerJSON(t, f.traeger, billing, http.MethodGet, listPath+"?format=csv", nil)
	require.Equal(t, http.StatusConflict, rec.Code, "the list comes with the release: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	renamed := youthOffice()
	renamed["name"] = "Jugendamt Musterstadt"
	rec = traegerJSON(t, f.traeger, billing, http.MethodPut, costBearerPath(office.ID), renamed)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	overview := rec.Body.String()
	assert.Contains(t, overview, "Kostenübernahme: Fachdienst Jugend des Landkreises Musterstadt;-90,00",
		"the issued invoice keeps the name it copied")
	assert.Contains(t, overview, "Gesamtsumme des Laufs;45,00", "the total counts what the families pay")
	assert.Contains(t, overview, "Davon übernimmt Fachdienst Jugend des Landkreises Musterstadt (1 Rechnung);123,75")

	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, listPath+"?format=csv", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(rec.Body.String(), "\xEF\xBB\xBF")), "\n")
	assert.Equal(t, "Rechnungsnummer;Kind;Wochen und Zeitmodell;Grundbetrag in Euro;Geschwisterermäßigung in Euro;"+
		"Übernommener Betrag in Euro;Nachweis;Bestätigt am", strings.TrimSpace(lines[0]))
	list := rec.Body.String()
	assert.Contains(t, list, "FE-0001;Lia Muster;")
	assert.Contains(t, list, ";67,50;-33,75;33,75;Ja;")
	assert.Contains(t, list, ";90,00;0,00;90,00;Ja;")
	assert.Contains(t, list, "Summe;;;123,75", "the sum of what the cost bearer pays")
	assert.Contains(t, list, ";Kostenträger;Jugendamt Musterstadt;", "the list names the cost bearer as it is now")
	assert.Contains(t, list, ";Anschrift;Am Kreishaus 1, 12345 Musterstadt;")
	assert.Contains(t, list, ";Aktenzeichen;FJ 51-2027;")
	assert.NotContains(t, list, "FE-0002", "only the invoices the cost bearer pays")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, listPath+"?format=xlsx", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, text := range []string{"Übernommener Betrag in Euro", "Lia Muster", "Jugendamt Musterstadt", "FJ 51-2027", "123,75"} {
		assert.Contains(t, xlsxText(t, rec.Body.Bytes()), text)
	}
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, listPath+"?format=pdf", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, strings.HasPrefix(rec.Body.String(), "%PDF"), "the PDF renders the same table")
	var exports []string
	require.NoError(t, f.db.NewRaw(`SELECT details->>'cost_bearer_id' FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'export' AND details ? 'cost_bearer_id' ORDER BY id`, run.ID).Scan(context.Background(), &exports))
	assert.Equal(t, []string{fmt.Sprint(office.ID), fmt.Sprint(office.ID), fmt.Sprint(office.ID)}, exports,
		"each download of the list is in the trail of the run")

	// The list needs the right "Abrechnung" and a cost bearer of the Träger.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, listPath+"?format=csv", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/cost-takeovers/999999999/export?format=csv"), nil)
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "parent_billing.cost_bearer_not_found", refusalCode(t, rec))
}

// uploadOfficeProof posts one file as the Träger-Büro's proof of a
// registration.
func uploadOfficeProof(t *testing.T, f reviewFixture, token string, registrationID int64, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, f.registrationPath(registrationID, "/proofs"), &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	return rec
}

func TestTraegerBueroUploadsAProofTheFamilyBroughtOnPaper(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Nachweis Büro")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)

	rec := uploadOfficeProof(t, f, f.office.token, f.siblings, "notiz.txt", []byte("Bescheid kommt per Post."))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_invalid", refusalCode(t, rec), "only PDF, JPEG and PNG by their content")
	rec = uploadOfficeProof(t, f, f.office.token, f.siblings, "bescheid.png", proofPDF)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_content_mismatch", refusalCode(t, rec))
	rec = uploadOfficeProof(t, f, f.office.token, f.siblings, "gross.pdf", append(bytes.Clone(proofPDF), bytes.Repeat([]byte{' '}, 10*1024*1024)...))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_invalid", refusalCode(t, rec), "at most 10 MB per file")
	rec = uploadOfficeProof(t, f, billing, f.siblings, "bescheid.pdf", proofPDF)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the upload needs the right to manage registrations: %s", rec.Body.String())
	assert.Zero(t, countProofs(t, f.holidayRegistrationFixture, f.office.organization, "TRUE"), "no refused file is stored")

	rec = uploadOfficeProof(t, f, f.office.token, f.siblings, "bescheid.png", proofPNG)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[proofDetail](t, rec.Body.Bytes())
	require.Len(t, detail.BenefitsProofs, 1, "the answer is the registration with its proofs")
	assert.Equal(t, "image/png", detail.BenefitsProofs[0].ContentType)
	assert.Contains(t, f.auditActions(t), fmt.Sprintf("proof_upload:%d", f.siblings), "the upload is in the registration's audit trail")
	history := f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)).History
	require.NotEmpty(t, history)
	assert.Equal(t, "proof_upload", history[len(history)-1].Action, "the history of the registration names the upload")
	open := traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		f.registrationPath(f.siblings, fmt.Sprintf("/proofs/%d", detail.BenefitsProofs[0].ID)), nil)
	require.Equal(t, http.StatusOK, open.Code, open.Body.String())
	assert.Equal(t, proofPNG, open.Body.Bytes(), "the office opens it like a proof of the parents")
}

// TestTraegerBueroCannotUploadAProofToACancelledRegistration pins that a
// cancelled registration stays as it is (ADR 0114): the upload of a proof
// is refused with the conflict of the other review actions and writes
// neither a file nor an entry in the trail.
func TestTraegerBueroCannotUploadAProofToACancelledRegistration(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Nachweis storniert")
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, f.registrationPath(f.again, "/cancel"), map[string]any{})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = uploadOfficeProof(t, f, f.office.token, f.again, "bescheid.pdf", proofPDF)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.holiday_registration_state_conflict", refusalCode(t, rec))
	assert.Zero(t, countProofs(t, f.holidayRegistrationFixture, f.office.organization, "TRUE"), "no file is stored")
	assert.NotContains(t, f.auditActions(t), fmt.Sprintf("proof_upload:%d", f.again), "the trail records no upload")
}

func TestCostBearerListAndReimbursementListQueryBudgetsAreFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Kostenträger")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	get := func(path string, all bool) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+billing)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		if all {
			return counter.Queries()
		}
		return counter.Selects("")
	}
	office := createCostBearer(t, f, billing, youthOffice())
	one := get(costBearersPath, false)
	createCostBearer(t, f, billing, map[string]any{"name": "Sozialamt Musterstadt", "active": true})
	assert.Len(t, get(costBearersPath, false), len(one), "the list reads the same statements for any number of cost bearers")
	querybudgets.AssertQueryBudget(t, "api.traeger.cost_bearers", one)

	confirmSiblingsWithTakeover(t, f, office)
	rec := f.confirmWith(t, f.again, map[string]any{"cost_bearer_id": office.ID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setNumberRange(t, f, billing, "FE-{NNNN}")
	run := billingRun(t, f, billing, startBillingRun(t, f, billing, f.period.ID).ID)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), releaseBody(run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	list := get(billingRunPath(run.ID, fmt.Sprintf("/cost-takeovers/%d/export?format=csv", office.ID)), true)
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_run_cost_takeovers", list)
}

// requestInBackground runs a request of the Träger-Büro and hands its answer
// over once the handler returns.
func requestInBackground(t *testing.T, f reviewFixture, token, method, path string, body any) <-chan *httptest.ResponseRecorder {
	t.Helper()
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- traegerJSON(t, f.traeger, token, method, path, body) }()
	return done
}

// assertWaits checks that the request is still held by the row lock.
func assertWaits(t *testing.T, done <-chan *httptest.ResponseRecorder, what string) {
	t.Helper()
	select {
	case rec := <-done:
		t.Fatalf("%s did not wait for the lock on the cost bearer: %d %s", what, rec.Code, rec.Body.String())
	case <-time.After(300 * time.Millisecond):
	}
}

// TestConfirmationAndDeletionOfACostBearerWaitForEachOther pins the row lock
// of the cost bearer (#5627): a deletion that holds it makes a confirmation
// wait and then find no cost bearer; a confirmation that holds it makes a
// deletion wait and then see the line that names it. No line ever names a
// deleted cost bearer.
func TestConfirmationAndDeletionOfACostBearerWaitForEachOther(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Kostenträger Sperre")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	ctx := context.Background()

	// A deletion holds the row: the confirmation waits, then the cost
	// bearer is gone and the registration stays received.
	deleted := createCostBearer(t, f, billing, map[string]any{"name": "Sozialamt Musterstadt", "active": true})
	deletion, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = deletion.ExecContext(ctx, `SELECT id FROM parent_billing.cost_bearers WHERE id = ? FOR UPDATE`, deleted.ID)
	require.NoError(t, err)
	confirmation := requestInBackground(t, f, f.office.token, http.MethodPost, holidayConfirmPath(f.period.ID, f.siblings),
		map[string]any{"cost_bearer_id": deleted.ID})
	assertWaits(t, confirmation, "the confirmation")
	_, err = deletion.ExecContext(ctx, `DELETE FROM parent_billing.cost_bearers WHERE id = ?`, deleted.ID)
	require.NoError(t, err)
	require.NoError(t, deletion.Commit())
	rec := <-confirmation
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, "parent_billing.cost_bearer_not_found", refusalCode(t, rec))
	assert.Equal(t, "received", f.detail(t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(f.siblings, ""), nil)).Status)

	// A confirmation holds the row while it writes the line: the deletion
	// waits, then sees the line and refuses.
	office := createCostBearer(t, f, billing, youthOffice())
	require.Equal(t, http.StatusOK, f.confirmWith(t, f.again, nil).Code)
	var contribution int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.holiday_contributions WHERE holiday_registration_id = ? ORDER BY id LIMIT 1`,
		f.again).Scan(ctx, &contribution))
	confirming, err := f.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = confirming.ExecContext(ctx, `SELECT id FROM parent_billing.cost_bearers WHERE id = ? FOR SHARE`, office.ID)
	require.NoError(t, err)
	deletionRequest := requestInBackground(t, f, billing, http.MethodDelete, costBearerPath(office.ID), nil)
	assertWaits(t, deletionRequest, "the deletion")
	_, err = confirming.ExecContext(ctx, `INSERT INTO parent_billing.holiday_contribution_lines
		(organization_id, holiday_contribution_id, position, kind, amount_cents, cost_bearer_id, cost_bearer_name)
		VALUES (?, ?, 99, 'cost_takeover', 0, ?, ?)`, f.office.organization, contribution, office.ID, office.Name)
	require.NoError(t, err)
	require.NoError(t, confirming.Commit())
	rec = <-deletionRequest
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Equal(t, "parent_billing.cost_bearer_in_use", refusalCode(t, rec))
}
