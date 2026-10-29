// Router-level tests for the proof of benefits of the public holiday
// registration (#5238): parents upload it with the checks of the online
// registration's uploads (#4783), the submission claims it, only the detail
// of the registration names it, the Träger-Büro opens it with an entry in the
// audit trail, and unclaimed proofs expire after a day.
package api

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

var (
	proofPDF = []byte("%PDF-1.4\n% Bescheid Bürgergeld\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n")
	proofPNG = append([]byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}, bytes.Repeat([]byte{0}, 64)...)
)

type proofReceipt struct {
	Token       string `json:"token"`
	Filename    string `json:"filename"`
	SizeBytes   int64  `json:"size_bytes"`
	ContentType string `json:"content_type"`
}

type proofDetail struct {
	ID               int64 `json:"id"`
	BenefitsDeclared bool  `json:"benefits_declared"`
	BenefitsProofs   []struct {
		ID          int64  `json:"id"`
		Filename    string `json:"filename"`
		SizeBytes   int64  `json:"size_bytes"`
		ContentType string `json:"content_type"`
		UploadedAt  string `json:"uploaded_at"`
	} `json:"benefits_proofs"`
}

// uploadProof posts one file to the public upload of the link.
func uploadProof(t *testing.T, f holidayRegistrationFixture, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, registrationPath(token)+"/uploads", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	f.public.ServeHTTP(rec, req)
	return rec
}

func uploadedProof(t *testing.T, f holidayRegistrationFixture, token, filename string, content []byte) proofReceipt {
	t.Helper()
	rec := uploadProof(t, f, token, filename, content)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[proofReceipt](t, rec.Body.Bytes())
}

// withProofs declares benefits and names the uploaded proofs.
func withProofs(registration map[string]any, receipts ...proofReceipt) map[string]any {
	tokens := make([]string, len(receipts))
	for i, receipt := range receipts {
		tokens[i] = receipt.Token
	}
	registration["benefits_declared"] = true
	registration["benefits_proof_tokens"] = tokens
	return registration
}

func countProofs(t *testing.T, f holidayRegistrationFixture, organization int64, where string) int {
	t.Helper()
	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_proofs
		WHERE organization_id = ? AND `+where, organization).Scan(context.Background(), &rows))
	return rows
}

func TestHolidayProofIsClaimedAndOpenedOnlyFromTheRegistrationDetail(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Nachweis Ferien")
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Fremd Nachweis", identityaccess.TraegerRoleAdministration)

	pdf := uploadedProof(t, f.holidayRegistrationFixture, f.period.PublicToken, "Bescheid Bürgergeld.pdf", proofPDF)
	assert.Equal(t, "Bescheid Bürgergeld.pdf", pdf.Filename)
	assert.Equal(t, "application/pdf", pdf.ContentType)
	assert.Equal(t, int64(len(proofPDF)), pdf.SizeBytes)
	assert.NotEmpty(t, pdf.Token)
	png := uploadedProof(t, f.holidayRegistrationFixture, f.period.PublicToken, "foto.PNG", proofPNG)
	assert.Equal(t, 2, countProofs(t, f.holidayRegistrationFixture, f.office.organization, "holiday_registration_id IS NULL"),
		"an upload waits unclaimed for its registration")

	registration := withProofs(siblingsRegistration(f.offer, "lena.nachweis@example.org"), pdf, png)
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), registration)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	id := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes()).ID
	assert.Equal(t, 2, countProofs(t, f.holidayRegistrationFixture, f.office.organization,
		fmt.Sprintf("holiday_registration_id = %d AND token_hash IS NULL AND claimed_at IS NOT NULL", id)),
		"the registration claims both proofs and the tokens are gone")

	// A token claims once: a second registration with it is refused whole.
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		withProofs(siblingsRegistration(f.offer, "zweiter.versuch@example.org"), pdf))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.upload_expired", code)
	assert.Equal(t, 3, f.list(t, "").Total, "the refused registration was not stored")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.registrationPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[proofDetail](t, rec.Body.Bytes())
	assert.True(t, detail.BenefitsDeclared)
	require.Len(t, detail.BenefitsProofs, 2)
	assert.Equal(t, "Bescheid Bürgergeld.pdf", detail.BenefitsProofs[0].Filename)
	assert.Equal(t, "image/png", detail.BenefitsProofs[1].ContentType)
	assert.Equal(t, "foto.PNG", detail.BenefitsProofs[1].Filename)
	assert.NotContains(t, rec.Body.String(), "%PDF", "the detail carries no bytes")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path(""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"benefits_declared":true`, "the list still shows the statement on benefits")
	assert.NotContains(t, rec.Body.String(), "benefits_proofs", "the list names no proof")
	assert.NotContains(t, rec.Body.String(), "Bescheid Bürgergeld", "the list names no proof")
	req := httptest.NewRequest(http.MethodGet, f.path("/export?format=csv"), nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	export := httptest.NewRecorder()
	f.traeger.ServeHTTP(export, req)
	require.Equal(t, http.StatusOK, export.Code, export.Body.String())
	assert.NotContains(t, export.Body.String(), "Bescheid Bürgergeld", "the export names no proof")

	open := func(token string, registrationID, proofID int64) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, f.registrationPath(registrationID, fmt.Sprintf("/proofs/%d", proofID)), nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.traeger.ServeHTTP(rec, req)
		return rec
	}
	rec = open(f.office.token, id, detail.BenefitsProofs[0].ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, proofPDF, rec.Body.Bytes())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Equal(t, `inline; filename*=utf-8''Bescheid%20B%C3%BCrgergeld.pdf`, rec.Header().Get("Content-Disposition"))
	assert.Contains(t, rec.Header().Get("Content-Security-Policy"), "sandbox", "the parents' file runs without scripts on our origin")
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	assert.Contains(t, f.auditActions(t), fmt.Sprintf("proof:%d", id), "every opened proof stands in the audit trail")

	rec = open(f.office.token, f.siblings, detail.BenefitsProofs[0].ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a proof opens only from its own registration")
	code, _ = registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.upload_not_found", code)
	rec = open(other.token, id, detail.BenefitsProofs[0].ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger opens nothing")

	// The hosting OGS works in the school transaction of its school; the
	// Träger policy shows it no proof.
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	school, err := tenant.NewTenantID(f.office.schools[0])
	require.NoError(t, err)
	var visible int
	require.NoError(t, tenant.WithinTenant(unit, school, func(txCtx context.Context) error {
		tx, txErr := tenant.CurrentTransaction[bun.Tx](txCtx)
		if txErr != nil {
			return txErr
		}
		return tx.NewRaw(`SELECT COUNT(*) FROM enrollment.holiday_registration_proofs`).Scan(txCtx, &visible)
	}))
	assert.Zero(t, visible, "a school sees no proof of the Träger")
}

func TestHolidayProofUploadRefusesWrongFilesAndForeignTokens(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Nachweis Prüfung")
	public := f.holidayRegistrationFixture
	refusal := func(rec *httptest.ResponseRecorder) string {
		t.Helper()
		code, _ := registrationRefusalOf(t, rec)
		return code
	}

	rec := uploadProof(t, public, f.period.PublicToken, "notiz.txt", []byte("Wir bekommen Bürgergeld."))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_invalid", refusal(rec), "only PDF, JPEG and PNG by their content")
	rec = uploadProof(t, public, f.period.PublicToken, "bescheid.png", proofPDF)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_invalid", refusal(rec), "the extension must name the content")
	rec = uploadProof(t, public, f.period.PublicToken, "gross.pdf", append(bytes.Clone(proofPDF), bytes.Repeat([]byte{' '}, 10*1024*1024)...))
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_invalid", refusal(rec), "at most 10 MB per file")
	rec = uploadProof(t, public, "unbekannter-link", "bescheid.pdf", proofPDF)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Zero(t, countProofs(t, public, f.office.organization, "TRUE"), "no refused file is stored")

	// A proof needs the statement on benefits.
	pdf := uploadedProof(t, public, f.period.PublicToken, "bescheid.pdf", proofPDF)
	registration := withProofs(siblingsRegistration(f.offer, "ohne.leistungen@example.org"), pdf)
	registration["benefits_declared"] = false
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), registration)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.holiday_registration_invalid", code)
	assert.Contains(t, fields, "benefits_proofs")

	// A token of another period claims nothing.
	second := openHolidays(t, public, f.office)
	foreign := uploadedProof(t, public, second.PublicToken, "bescheid.pdf", proofPDF)
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		withProofs(siblingsRegistration(f.offer, "falscher.zeitraum@example.org"), foreign))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "enrollment.upload_expired", refusal(rec))
	assert.Equal(t, 2, f.list(t, "").Total, "neither refused registration was stored")

	// Without a proof the registration goes through as before.
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		withProofs(siblingsRegistration(f.offer, "spaeter.nachweis@example.org")))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestHolidayProofUploadsCountPerAddress(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Nachweis Limit", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	for i := range 30 {
		rec := uploadProof(t, f, period.PublicToken, "bescheid.pdf", proofPDF)
		require.Equal(t, http.StatusCreated, rec.Code, "upload %d: %s", i+1, rec.Body.String())
	}
	rec := uploadProof(t, f, period.PublicToken, "bescheid.pdf", proofPDF)
	require.Equal(t, http.StatusTooManyRequests, rec.Code, rec.Body.String())
	assert.Equal(t, "3600", rec.Header().Get("Retry-After"))
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.upload_rate_limited", code)
	assert.Equal(t, 30, countProofs(t, f, office.organization, "TRUE"), "the refused upload stored nothing")
}

// TestUnclaimedHolidayProofsGoAfterADay runs on its own database: the
// retention deletes across every Träger.
func TestUnclaimedHolidayProofsGoAfterADay(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	f := newReviewFixture(t, "Träger Nachweis Frist")
	public := f.holidayRegistrationFixture
	ctx := context.Background()
	claimed := uploadedProof(t, public, f.period.PublicToken, "bescheid.pdf", proofPDF)
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		withProofs(siblingsRegistration(f.offer, "frist.nachweis@example.org"), claimed))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	fresh := uploadedProof(t, public, f.period.PublicToken, "neu.pdf", proofPDF)
	stale := uploadedProof(t, public, f.period.PublicToken, "alt.pdf", proofPDF)
	_, err := f.db.ExecContext(ctx, `UPDATE enrollment.holiday_registration_proofs SET created_at = NOW() - INTERVAL '25 hours'
		WHERE organization_id = ? AND filename IN ('alt.pdf', 'bescheid.pdf')`, f.office.organization)
	require.NoError(t, err)

	// A day-old token claims nothing any more.
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken),
		withProofs(siblingsRegistration(f.offer, "zu.spaet@example.org"), stale))
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "enrollment.upload_expired", code)

	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		_, retainErr := enrollmentCompose.NewHolidayRegistrationRetention(func() calendar.Date {
			return calendar.NewDate(2027, time.January, 1)
		}).DeleteExpiredHolidayRegistrations(adminCtx)
		return retainErr
	}))
	var kept []string
	require.NoError(t, f.db.NewRaw(`SELECT filename FROM enrollment.holiday_registration_proofs
		WHERE organization_id = ? ORDER BY filename`, f.office.organization).Scan(ctx, &kept))
	assert.Equal(t, []string{"bescheid.pdf", "neu.pdf"}, kept,
		"the unclaimed day-old proof goes; the claimed one stays with its registration, the fresh one waits")
	assert.NotEmpty(t, fresh.Token)

	// A claimed proof goes with its registration.
	_, err = f.db.ExecContext(ctx, `DELETE FROM enrollment.holiday_registrations WHERE organization_id = ? AND guardian_email = 'frist.nachweis@example.org'`,
		f.office.organization)
	require.NoError(t, err)
	assert.Zero(t, countProofs(t, public, f.office.organization, "filename = 'bescheid.pdf'"))
}
