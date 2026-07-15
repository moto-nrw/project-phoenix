package enrollment

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type fakeLegalDocumentReferenceRepository struct {
	referenced bool
	err        error
	checkCtx   func(context.Context)
}

func (f fakeLegalDocumentReferenceRepository) HasLegalDocumentReference(ctx context.Context, _, _ string) (bool, error) {
	if f.checkCtx != nil {
		f.checkCtx(ctx)
	}
	return f.referenced, f.err
}

func TestUploadLegalDocument_SavesTenantPrefixedPDF(t *testing.T) {
	req := multipartPDFRequest(t, "document", "terms.pdf", []byte("%PDF-1.4\n%test\n"))
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	(&Resource{}).uploadLegalDocument(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Data struct {
			DocumentURL string `json:"document_url"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.True(t, strings.HasPrefix(body.Data.DocumentURL, "/uploads/enrollment-form-legal-documents/42_"))
	assert.True(t, strings.HasSuffix(body.Data.DocumentURL, ".pdf"))

	path, err := common.ResolveStoredPath("public", body.Data.DocumentURL, "/uploads/enrollment-form-legal-documents/")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(path) })
	_, err = os.Stat(path)
	require.NoError(t, err)
}

func TestUploadLegalDocument_RejectsNonPDF(t *testing.T) {
	req := multipartPDFRequest(t, "document", "terms.pdf", []byte("not a pdf"))
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	(&Resource{}).uploadLegalDocument(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDeleteLegalDocument_RemovesUnreferencedTenantPDF(t *testing.T) {
	path, url := writeEnrollmentLegalDocument(t, "42_delete-me.pdf")
	req := httptest.NewRequest(http.MethodDelete, "/enrollment/legal-documents/42_delete-me.pdf", nil)
	req = withFilenameParam(req, "42_delete-me.pdf")
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	res := &Resource{legalDocumentRefs: fakeLegalDocumentReferenceRepository{}}
	res.deleteLegalDocument(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	_, err := os.Stat(path)
	assert.True(t, os.IsNotExist(err), "expected %s to be removed", url)
}

func TestDeleteLegalDocument_KeepsReferencedTenantPDF(t *testing.T) {
	path, _ := writeEnrollmentLegalDocument(t, "42_keep-me.pdf")
	req := httptest.NewRequest(http.MethodDelete, "/enrollment/legal-documents/42_keep-me.pdf", nil)
	req = withFilenameParam(req, "42_keep-me.pdf")
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	res := &Resource{legalDocumentRefs: fakeLegalDocumentReferenceRepository{referenced: true}}
	res.deleteLegalDocument(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	_, err := os.Stat(path)
	require.NoError(t, err)
}

func TestDeleteLegalDocument_ChecksReferencesInTenantTx(t *testing.T) {
	_, _ = writeEnrollmentLegalDocument(t, "42_tx-check.pdf")
	req := httptest.NewRequest(http.MethodDelete, "/enrollment/legal-documents/42_tx-check.pdf", nil)
	req = withFilenameParam(req, "42_tx-check.pdf")
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	type txContextKey struct{}
	sawTenantTxContext := false
	res := &Resource{
		legalDocumentRefs: fakeLegalDocumentReferenceRepository{
			referenced: true,
			checkCtx: func(ctx context.Context) {
				sawTenantTxContext = ctx.Value(txContextKey{}) == "tenant-tx"
			},
		},
		runInTenantTxForTest: func(r *http.Request, fn func(ctx context.Context) error) error {
			return fn(context.WithValue(r.Context(), txContextKey{}, "tenant-tx"))
		},
	}

	res.deleteLegalDocument(w, req)

	require.Equal(t, http.StatusNoContent, w.Code)
	assert.True(t, sawTenantTxContext)
}

func TestDeleteLegalDocument_RejectsOtherTenantPDF(t *testing.T) {
	_, _ = writeEnrollmentLegalDocument(t, "7_other-tenant.pdf")
	req := httptest.NewRequest(http.MethodDelete, "/enrollment/legal-documents/7_other-tenant.pdf", nil)
	req = withFilenameParam(req, "7_other-tenant.pdf")
	req = req.WithContext(tenant.WithTenantID(req.Context(), 42))
	w := httptest.NewRecorder()

	res := &Resource{legalDocumentRefs: fakeLegalDocumentReferenceRepository{}}
	res.deleteLegalDocument(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func withFilenameParam(req *http.Request, filename string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("filename", filename)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func writeEnrollmentLegalDocument(t *testing.T, filename string) (string, string) {
	t.Helper()
	dir := filepath.Join("public", "uploads", "enrollment-form-legal-documents")
	require.NoError(t, os.MkdirAll(dir, 0755))
	path := filepath.Join(dir, filename)
	require.NoError(t, os.WriteFile(path, []byte("%PDF-1.4\n%test\n"), 0600))
	t.Cleanup(func() { _ = os.Remove(path) })
	return path, "/uploads/enrollment-form-legal-documents/" + filename
}

func multipartPDFRequest(t *testing.T, fieldName, fileName string, content []byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/enrollment/legal-documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
