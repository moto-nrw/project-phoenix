package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func serveBillingDocuments(t *testing.T, rs *Resource, path string, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/me/documents", rs.listBillingDocuments)
	router.Get("/parent/me/documents/{organizationId}/{documentId}/pdf", rs.billingDocumentPDF)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: accountID, Sub: "eltern@example.org",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

func refusalCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var refusal struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &refusal))
	return refusal.Code
}

// The inbox answers the account of the claims with string IDs and dates.
func TestListBillingDocuments_AnswersTheAccountsDocuments(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	sentAt := time.Date(2026, 10, 2, 9, 30, 0, 0, time.UTC)
	parent := &fakeParentService{documents: []parentService.BillingDocument{{
		ID: 31, OrganizationID: 4, TraegerName: "Ganztag im Quartier e.V.", Kind: "confirmation_letter", Number: "FE-0007",
		PeriodName: "Herbstferien 2026", ServiceFrom: timezone.NewDate(2026, 10, 12), ServiceUntil: timezone.NewDate(2026, 10, 16),
		IssuedOn: timezone.NewDate(2026, 10, 1), TotalCents: 4500, ChildNames: []string{"Felix"}, SentAt: sentAt,
	}}}

	w := serveBillingDocuments(t, &Resource{ParentService: parent}, "/parent/me/documents", 7301)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(7301), parent.gotDocumentsAccount, "the account comes from the claims")
	var body struct {
		Data []BillingDocumentResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, BillingDocumentResponse{
		ID: "31", OrganizationID: "4", TraegerName: "Ganztag im Quartier e.V.", Kind: "confirmation_letter", Number: "FE-0007",
		PeriodName: "Herbstferien 2026", ServiceFrom: timezone.NewDate(2026, 10, 12), ServiceUntil: timezone.NewDate(2026, 10, 16),
		IssuedOn: timezone.NewDate(2026, 10, 1), TotalCents: 4500, ChildNames: []string{"Felix"}, SentAt: sentAt,
	}, body.Data[0])
	assert.Contains(t, w.Body.String(), `"service_from":"2026-10-12"`, "calendar dates travel as YYYY-MM-DD")
}

// A relationship without the inbox right is 403, a document outside the
// inbox 404, and both read nothing.
func TestBillingDocuments_RefuseWithoutRightOrOutsideTheInbox(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	forbidden := &fakeParentService{documentsErr: parentService.ErrGuardianPermissionDenied}
	w := serveBillingDocuments(t, &Resource{ParentService: forbidden}, "/parent/me/documents", 7301)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "care.guardian_permission_denied", refusalCode(t, w))

	w = serveBillingDocuments(t, &Resource{ParentService: forbidden}, "/parent/me/documents/4/31/pdf", 7301)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	missing := &fakeParentService{documentsErr: fmt.Errorf("lookup: %w", parentService.ErrBillingDocumentNotFound)}
	w = serveBillingDocuments(t, &Resource{ParentService: missing}, "/parent/me/documents/4/31/pdf", 7301)
	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	assert.Equal(t, int64(4), missing.gotDocumentOrganization)
	assert.Equal(t, int64(31), missing.gotDocumentID)

	broken := &fakeParentService{documentsErr: errors.New("database gone")}
	w = serveBillingDocuments(t, &Resource{ParentService: broken}, "/parent/me/documents", 7301)
	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())

	w = serveBillingDocuments(t, &Resource{ParentService: &fakeParentService{}}, "/parent/me/documents/x/31/pdf", 7301)
	require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())

	anonymous := &fakeParentService{}
	w = serveBillingDocuments(t, &Resource{ParentService: anonymous}, "/parent/me/documents", 0)
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Zero(t, anonymous.gotDocumentsAccount, "no claims, no read")
}

// The PDF is a download that no cache keeps and no browser sniffs.
func TestBillingDocumentPDF_IsAnUncachedDownload(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	parent := &fakeParentService{documentFile: parentService.BillingDocumentFile{
		Filename: "bestaetigung-fe-0007.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4 FE-0007"),
	}}

	w := serveBillingDocuments(t, &Resource{ParentService: parent}, "/parent/me/documents/4/31/pdf", 7301)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(7301), parent.gotDocumentsAccount)
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="bestaetigung-fe-0007.pdf"`, w.Header().Get("Content-Disposition"))
	assert.Equal(t, "no-store", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "%PDF-1.4 FE-0007", w.Body.String())
}
