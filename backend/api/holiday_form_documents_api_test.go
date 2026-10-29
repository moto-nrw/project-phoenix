package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const holidayTestPDF = "%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF\n"

func holidayPDFUpload(t *testing.T, f holidayRegistrationFixture, token string, period int64, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("document", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, holidayFormPath(period)+"/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	return rec
}

func holidayPDF(t *testing.T, f holidayRegistrationFixture, office traegerOffice, period int64) string {
	t.Helper()
	rec := holidayPDFUpload(t, f, office.token, period, "Hinweise.pdf", []byte(holidayTestPDF))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[enrollment.HolidayFormDocument](t, rec.Body.Bytes()).ID
}
func holidayLegalBlocks(ids ...string) []enrollment.FormLegalBlock {
	blocks := []enrollment.FormLegalBlock{}
	for i, id := range ids {
		blocks = append(blocks, enrollment.FormLegalBlock{Key: fmt.Sprintf("hinweis_%d", i), Kind: "notice", Source: "custom", Title: fmt.Sprintf("Hinweis %d", i+1), Label: "Ich habe den Hinweis gelesen und zur Kenntnis genommen.", Enabled: true, Required: true, DisplayMode: "pdf", Attachments: []enrollment.FormAttachment{{Label: "PDF-Hinweis", DocumentID: id}}})
	}
	return append(blocks, enrollment.FormLegalBlock{Key: "foto_ferien", Kind: "consent", Source: "custom", Title: "Fotos", Label: "Mein Kind darf fotografiert werden.", Text: "Fotos bleiben in der OGS.", Enabled: true})
}
func saveHolidayLegalBlocks(t *testing.T, f holidayRegistrationFixture, office traegerOffice, period int64, blocks []enrollment.FormLegalBlock) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, office.token, http.MethodPut, holidayFormPath(period), map[string]any{"fields": []any{}, "legal_blocks": blocks})
}

func TestHolidayLegalDocumentsAndPinnedAcceptanceEvidence(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger PDF", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	first, second := holidayPDF(t, f, office, period.ID), holidayPDF(t, f, office, period.ID)
	blocks := holidayLegalBlocks(first, second)
	rec := saveHolidayLegalBlocks(t, f, office, period.ID, blocks)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	saved := decodeSchoolPortalData[enrollment.HolidayPeriodForm](t, rec.Body.Bytes())
	require.Equal(t, 2, saved.Version)
	require.Len(t, saved.LegalBlocks, 3)
	publicPath := registrationPath(period.PublicToken) + "/documents/" + first
	rec = publicJSON(t, f.public, http.MethodGet, publicPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, holidayTestPDF, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Disposition"), "inline;"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	body := siblingsRegistration(holidayOffer(t, f, period.PublicToken), "pdf@example.org")
	body["form_version"] = 2
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, paths := registrationRefusalOf(t, rec)
	assert.ElementsMatch(t, []string{"legal_acceptances.hinweis_0", "legal_acceptances.hinweis_1"}, paths)
	body["legal_acceptances"] = map[string]bool{"hinweis_0": true, "hinweis_1": true, "unknown": true}
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	_, paths = registrationRefusalOf(t, rec)
	assert.Contains(t, paths, "legal_acceptances.unknown")
	body["legal_acceptances"] = map[string]bool{"hinweis_0": true, "hinweis_1": true, "foto_ferien": false}
	before := time.Now().Add(-time.Second)
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	receipt := decodeSchoolPortalData[struct {
		ID int64 `json:"id"`
	}](t, rec.Body.Bytes())
	detailPath := fmt.Sprintf("%s/registrations/%d", holidayPeriodPath(period.ID), receipt.ID)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, detailPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	require.Len(t, detail.LegalAcceptances, 2)
	assert.True(t, detail.LegalAcceptances["hinweis_0"].After(before))
	assert.Equal(t, 2, detail.Form.Version)
	// Replacement publishes a new version. The registration and private PDF
	// keep the prior version, while the public link no longer exposes it.
	replacement := holidayPDF(t, f, office, period.ID)
	rec = saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks(replacement))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = publicJSON(t, f.public, http.MethodGet, publicPath, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, detailPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	pinned := decodeSchoolPortalData[enrollment.HolidayRegistrationDetail](t, rec.Body.Bytes())
	assert.Equal(t, detail.Form, pinned.Form)
	assert.Equal(t, detail.LegalAcceptances, pinned.LegalAcceptances)
	privatePath := holidayFormPath(period.ID) + "/documents/" + first
	rec = traegerJSON(t, f.traeger, office.token, http.MethodDelete, privatePath, nil)
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, office.token, http.MethodGet, privatePath, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, holidayTestPDF, rec.Body.String())
	draft := holidayPDF(t, f, office, period.ID)
	rec = traegerJSON(t, f.traeger, office.token, http.MethodDelete, holidayFormPath(period.ID)+"/documents/"+draft, nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)
}

func TestHolidayPDFUploadAndCrossTraegerBoundaries(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger PDF Grenze", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	reader := newTraegerOffice(t, f.db, f.traeger, "Träger PDF Lesend", identityaccess.TraegerRoleReadOnly)
	for _, tc := range []struct {
		name    string
		content []byte
		token   string
		status  int
	}{
		{"falsch.pdf", []byte("not a PDF"), office.token, 400},
		{"falsch.txt", []byte(holidayTestPDF), office.token, 400},
		{"gross.pdf", append([]byte(holidayTestPDF), make([]byte, 10<<20)...), office.token, 400},
		{"Hinweis.pdf", []byte(holidayTestPDF), reader.token, 403},
	} {
		rec := holidayPDFUpload(t, f, tc.token, period.ID, tc.name, tc.content)
		assert.Equal(t, tc.status, rec.Code, rec.Body.String())
	}
	own := holidayPDF(t, f, office, period.ID)
	otherPeriod := openHolidays(t, f, office)
	otherDoc := holidayPDF(t, f, office, otherPeriod.ID)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger PDF Fremd", identityaccess.TraegerRoleAdministration)
	foreignPeriod := openHolidays(t, f, foreign)
	foreignDoc := holidayPDF(t, f, foreign, foreignPeriod.ID)
	for _, id := range []string{own, otherDoc, foreignDoc} {
		rec := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken)+"/documents/"+id, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "unreferenced or foreign document %s: %s", id, rec.Body.String())
	}
	for _, id := range []string{otherDoc, foreignDoc} {
		rec := saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks(id))
		assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	}
	rec := saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks(own))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, token := range []string{otherPeriod.PublicToken, foreignPeriod.PublicToken, "unknown"} {
		rec = publicJSON(t, f.public, http.MethodGet, registrationPath(token)+"/documents/"+own, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	}
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, holidayFormPath(period.ID)+"/documents/"+own, nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	rec = traegerJSON(t, f.traeger, reader.token, http.MethodGet, holidayFormPath(period.ID)+"/documents/"+own, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestHolidayPDFSaveAndDeleteSerialize(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger PDF Parallel", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	id := holidayPDF(t, f, office, period.ID)
	start := make(chan struct{})
	var saved, deleted *httptest.ResponseRecorder
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		saved = saveHolidayLegalBlocks(t, f, office, period.ID, holidayLegalBlocks(id))
	}()
	go func() {
		defer wg.Done()
		<-start
		deleted = traegerJSON(t, f.traeger, office.token, http.MethodDelete, holidayFormPath(period.ID)+"/documents/"+id, nil)
	}()
	close(start)
	wg.Wait()
	switch saved.Code {
	case http.StatusOK:
		assert.Equal(t, http.StatusConflict, deleted.Code)
	case http.StatusBadRequest:
		assert.Equal(t, http.StatusNoContent, deleted.Code)
	default:
		t.Fatalf("save: %d %s; delete: %d %s", saved.Code, saved.Body.String(), deleted.Code, deleted.Body.String())
	}
}

func TestHolidayLegalBlocksDefaultPreservesPublishedPeriod(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger PDF Bestand", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	rec := publicJSON(t, f.public, http.MethodGet, registrationPath(period.PublicToken), nil)
	require.Equal(t, http.StatusOK, rec.Code)
	offer := decodeSchoolPortalData[struct {
		Form enrollment.HolidayPeriodForm `json:"form"`
	}](t, rec.Body.Bytes())
	assert.Empty(t, offer.Form.LegalBlocks)
	rec = publicJSON(t, f.public, http.MethodPost, registrationPath(period.PublicToken), siblingsRegistration(holidayOffer(t, f, period.PublicToken), "pdf-bestand@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var accepted string
	require.NoError(t, f.db.NewRaw(`SELECT legal_acceptances FROM enrollment.holiday_registrations WHERE holiday_period_id = ?`, period.ID).Scan(context.Background(), &accepted))
	assert.JSONEq(t, `{}`, string(accepted))
}

func TestHolidayLegalBlocksRejectSchoolSettingsAndInvalidAcknowledgements(t *testing.T) {
	t.Parallel()
	f := newHolidayRegistrationFixture(t)
	office := newTraegerOffice(t, f.db, f.traeger, "Träger Rechtsblöcke", identityaccess.TraegerRoleAdministration)
	period := openHolidays(t, f, office)
	for _, tc := range []struct {
		name   string
		blocks []enrollment.FormLegalBlock
		path   string
	}{
		{"school standard", []enrollment.FormLegalBlock{{Key: "agb", Source: "standard", Kind: "terms", Enabled: true, Title: "AGB", Label: "AGB"}}, "legal_blocks.0.source"},
		{"reserved key", []enrollment.FormLegalBlock{{Key: "agb", Source: "custom", Kind: "terms", Enabled: true, Title: "AGB", Label: "AGB"}}, "legal_blocks.0.key"},
		{"missing title", []enrollment.FormLegalBlock{{Key: "notice", Source: "custom", Kind: "notice", Enabled: true, Label: "Hinweis"}}, "legal_blocks.0.title"},
		{"required consent", []enrollment.FormLegalBlock{{Key: "fotos", Source: "custom", Kind: "consent", Enabled: true, Required: true, Title: "Fotos", Label: "Fotos"}}, "legal_blocks.0.required"},
		{"school document URL", []enrollment.FormLegalBlock{{Key: "notice", Source: "custom", Kind: "notice", Enabled: true, Title: "Hinweis", Label: "Hinweis", Attachments: []enrollment.FormAttachment{{Label: "Schule", DocumentURL: "/uploads/enrollment-form-legal-documents/1_school.pdf"}}}}, "legal_blocks.0.attachments.0.document_url"},
	} {
		rec := saveHolidayLegalBlocks(t, f, office, period.ID, tc.blocks)
		require.Equal(t, http.StatusBadRequest, rec.Code, "%s: %s", tc.name, rec.Body.String())
		var problem struct {
			Errors []struct {
				Field string `json:"field"`
			} `json:"errors"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &problem))
		var paths []string
		for _, field := range problem.Errors {
			paths = append(paths, field.Field)
		}
		assert.Contains(t, paths, tc.path, tc.name)
	}
}
