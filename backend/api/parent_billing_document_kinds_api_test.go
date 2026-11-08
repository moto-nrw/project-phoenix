// Router-level tests for the kind of document per service (#4688): the
// Abrechnung chooses invoice, fee notice or confirmation letter; the release
// freezes the kind on every document and issues no fee notice without the
// legal remedy of the Träger; a later choice changes no issued document; a
// Storno of a fee notice is an Aufhebungsbescheid.
package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const documentKindsPath = "/traeger/billing-document-kinds"

type documentKindsBody struct {
	Services []struct {
		Service      string   `json:"service"`
		DocumentKind string   `json:"document_kind"`
		Choices      []string `json:"choices"`
		UpdatedAt    *string  `json:"updated_at"`
	} `json:"services"`
	LegalRemedySet           bool `json:"legal_remedy_set"`
	FeeNoticeTemplateVersion int  `json:"fee_notice_template_version"`
}

type kindRunBody struct {
	Status             string `json:"status"`
	DocumentKind       string `json:"document_kind"`
	LegalRemedyMissing bool   `json:"legal_remedy_missing"`
	PreviewCount       int    `json:"preview_count"`
	PreviewCents       int64  `json:"preview_cents"`
	Documents          []struct {
		ID           int64  `json:"id"`
		Kind         string `json:"kind"`
		DocumentKind string `json:"document_kind"`
		Number       string `json:"number"`
	} `json:"documents"`
}

// noticeTemplateBody is a fee notice template with its legal remedy.
type noticeTemplateBody struct {
	documentTemplateBody
	LegalRemedy string `json:"legal_remedy"`
}

func documentKinds(t *testing.T, router http.Handler, token string) documentKindsBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodGet, documentKindsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[documentKindsBody](t, rec.Body.Bytes())
}

func setDocumentKind(t *testing.T, router http.Handler, token, service, kind string) documentKindsBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodPut, documentKindsPath+"/"+service, map[string]any{"document_kind": kind})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[documentKindsBody](t, rec.Body.Bytes())
}

func kindRun(t *testing.T, router http.Handler, token, path string) kindRunBody {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[kindRunBody](t, rec.Body.Bytes())
}

func TestFeeNoticeNeedsTheLegalRemedyAndKeepsItsKindAfterTheRelease(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bescheid")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	brandTraeger(t, f)
	for _, id := range []int64{f.siblings, f.again} {
		require.Equal(t, http.StatusOK, f.act(t, id, "confirm").Code)
	}
	setNumberRange(t, f, billing, "GB-{NNNN}")

	kinds := documentKinds(t, f.traeger, billing)
	require.Len(t, kinds.Services, 2)
	assert.Equal(t, "confirmation", kinds.Services[0].DocumentKind, "the holiday care issues the confirmation letter until the Träger chooses")
	assert.Equal(t, "invoice", kinds.Services[1].DocumentKind)
	assert.Nil(t, kinds.Services[0].UpdatedAt)
	assert.False(t, kinds.LegalRemedySet)
	kinds = setDocumentKind(t, f.traeger, billing, "holiday", "fee_notice")
	assert.Equal(t, "fee_notice", kinds.Services[0].DocumentKind)
	assert.NotNil(t, kinds.Services[0].UpdatedAt)

	run := startBillingRun(t, f, billing, f.period.ID)
	preview := kindRun(t, f.traeger, billing, billingRunPath(run.ID, ""))
	assert.Equal(t, "fee_notice", preview.DocumentKind)
	assert.True(t, preview.LegalRemedyMissing, "the run shows before the release that the legal remedy is missing")
	for _, document := range preview.Documents {
		assert.Equal(t, "fee_notice", document.DocumentKind, "every draft names the kind its release would freeze")
	}
	release := map[string]any{"document_count": preview.PreviewCount, "total_cents": preview.PreviewCents, "document_kind": "fee_notice"}
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), release)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.legal_remedy_missing")
	var issued int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.documents WHERE run_id = ?`, run.ID).Scan(context.Background(), &issued))
	assert.Zero(t, issued, "no fee notice without the legal remedy, and no number used")

	notice := noticeTemplateBody{
		documentTemplateBody: documentTemplateBody{
			Subject: "Gebührenbescheid {{bescheidnummer}}", Salutation: "Sehr geehrte Familie {{familienname}},",
			Body:    "für {{kinder}} setzen wir für {{zeitraum}} fest:\n{{positionen}}\n\n**{{zahlungshinweis}}**",
			Closing: "Mit freundlichen Grüßen\n{{traeger}}",
		},
		LegalRemedy: "Gegen diesen Bescheid kann innerhalb eines Monats nach Bekanntgabe Widerspruch bei {{traeger}} erhoben werden.",
	}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, documentTemplatesPath+"/bescheid/versions", notice)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.True(t, documentKinds(t, f.traeger, billing).LegalRemedySet)
	assert.False(t, kindRun(t, f.traeger, billing, billingRunPath(run.ID, "")).LegalRemedyMissing)

	// The Abrechnung saw a fee notice; a release that would issue another
	// kind refuses.
	setDocumentKind(t, f.traeger, billing, "holiday", "invoice")
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), release)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_changed")
	setDocumentKind(t, f.traeger, billing, "holiday", "fee_notice")

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/release"), release)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[kindRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "fee_notice", released.DocumentKind)
	require.Len(t, released.Documents, 2)

	// A later choice changes no issued document (snapshot).
	setDocumentKind(t, f.traeger, billing, "holiday", "confirmation")
	after := kindRun(t, f.traeger, billing, billingRunPath(run.ID, ""))
	assert.Equal(t, "fee_notice", after.DocumentKind, "the released run keeps the kind it froze")
	for _, document := range after.Documents {
		assert.Equal(t, "fee_notice", document.DocumentKind)
	}
	var stored []string
	require.NoError(t, f.db.NewRaw(`SELECT document_kind FROM parent_billing.documents WHERE run_id = ? ORDER BY id`, run.ID).
		Scan(context.Background(), &stored))
	assert.Equal(t, []string{"fee_notice", "fee_notice"}, stored)

	// The letters are fee notices; their Storno is an Aufhebungsbescheid.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, after.Documents[0].ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `filename="gebuehrenbescheid-gb-0001.pdf"`)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, cancellationPath(run.ID, after.Documents[0].ID),
		map[string]any{"reason": "Falsche Woche"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	cancellation := decodeSchoolPortalData[struct {
		ID           int64  `json:"id"`
		DocumentKind string `json:"document_kind"`
	}](t, rec.Body.Bytes())
	assert.Equal(t, "fee_notice", cancellation.DocumentKind, "the Storno keeps the kind of its notice")
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, letterPath(run.ID, cancellation.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `filename="aufhebungsbescheid-gb-0003.pdf"`)
}

func TestDocumentKindsNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Dokumentart Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Dokumentart Fremd", identityaccess.TraegerRoleBilling)
	setDocumentKind(t, f.traeger, billing, "care", "fee_notice")

	for _, request := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, documentKindsPath, nil},
		{http.MethodPut, documentKindsPath + "/care", map[string]any{"document_kind": "invoice"}},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right Abrechnung: %s", request.method, request.path, rec.Body.String())
	}
	assert.Equal(t, "fee_notice", documentKinds(t, f.traeger, billing).Services[1].DocumentKind)

	rec := traegerJSON(t, f.traeger, foreign.token, http.MethodGet, documentKindsPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "invoice", decodeSchoolPortalData[documentKindsBody](t, rec.Body.Bytes()).Services[1].DocumentKind,
		"another Träger keeps its own default")

	for path, body := range map[string]map[string]any{
		documentKindsPath + "/care":  {"document_kind": "confirmation"},
		documentKindsPath + "/mensa": {"document_kind": "invoice"},
	} {
		rec := traegerJSON(t, f.traeger, billing, http.MethodPut, path, body)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.document_kind_invalid")
	}
}

func TestDocumentKindListQueryBudget(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Budget Dokumentart")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	list := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, documentKindsPath, nil)
		req.Header.Set("Authorization", "Bearer "+billing)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	empty := list()
	setDocumentKind(t, f.traeger, billing, "holiday", "fee_notice")
	setDocumentKind(t, f.traeger, billing, "care", "fee_notice")
	assert.Len(t, list().Statements, len(empty.Statements), "the list reads the same statements for any number of choices")
	querybudgets.AssertQueryBudget(t, "api.traeger.billing_document_kinds", empty)
}

// TestCareFeeNoticeNeedsTheLegalRemedyAndItsKindChangesTheReviewedState: the
// monthly run issues fee notices only with the legal remedy, and a choice
// made after the review makes the release refuse (#4688).
func TestCareFeeNoticeNeedsTheLegalRemedyAndItsKindChangesTheReviewedState(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatsbescheid")
	school := f.office.schools[0]
	careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	reviewed := f.run(t, run.ID)

	setDocumentKind(t, f.traeger, f.office.token, "care", "fee_notice")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed))
	require.Equal(t, http.StatusConflict, rec.Code, "the reviewer saw invoices: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_changed")

	preview := kindRun(t, f.traeger, f.office.token, careRunPath(run.ID, ""))
	assert.Equal(t, "fee_notice", preview.DocumentKind)
	assert.True(t, preview.LegalRemedyMissing)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(f.run(t, run.ID)))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.legal_remedy_missing")

	// The administration writes the legal remedy into the fee notice.
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	testpkg.OwnTraegerMasterData(t, f.db, f.office.organization)
	notice := noticeTemplateBody{
		documentTemplateBody: documentTemplateBody{Subject: "Gebührenbescheid {{bescheidnummer}}",
			Body: "für {{kinder}} setzen wir für {{zeitraum}} fest:\n{{positionen}}"},
		LegalRemedy: "Gegen diesen Bescheid ist innerhalb eines Monats Widerspruch möglich.",
	}
	rec = traegerJSON(t, f.traeger, answer.AccessToken, http.MethodPost, documentTemplatesPath+"/bescheid/versions", notice)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(f.run(t, run.ID)))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setDocumentKind(t, f.traeger, f.office.token, "care", "invoice")
	released := kindRun(t, f.traeger, f.office.token, careRunPath(run.ID, ""))
	assert.Equal(t, "fee_notice", released.DocumentKind, "a later choice changes no issued notice")
	var kinds []string
	require.NoError(t, f.db.NewRaw(`SELECT document_kind FROM parent_billing.care_invoices WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &kinds))
	assert.Equal(t, []string{"fee_notice"}, kinds)
}
