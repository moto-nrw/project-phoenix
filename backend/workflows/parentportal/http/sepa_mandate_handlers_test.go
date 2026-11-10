package parent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func (f *fakeParentService) SEPAMandate(_ context.Context, accountID, organizationID, contractID int64) (parentService.SEPAMandateView, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID, f.gotSEPACall = accountID, organizationID, contractID, "read"
	return f.sepaView, f.sepaErr
}

func (f *fakeParentService) PreviewSEPAMandate(_ context.Context, accountID, organizationID, contractID int64, input parentService.SEPAMandateInput) (parentService.SEPAMandateText, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID, f.gotSEPACall = accountID, organizationID, contractID, "preview"
	f.gotSEPAInput = input
	return f.sepaText, f.sepaErr
}

func (f *fakeParentService) GrantSEPAMandate(_ context.Context, accountID, organizationID, contractID int64, input parentService.SEPAMandateInput) (parentService.SEPAMandateView, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID, f.gotSEPACall = accountID, organizationID, contractID, "grant"
	f.gotSEPAInput = input
	return f.sepaView, f.sepaErr
}

func (f *fakeParentService) RevokeSEPAMandate(_ context.Context, accountID, organizationID, contractID int64) (parentService.SEPAMandateView, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID, f.gotSEPACall = accountID, organizationID, contractID, "revoke"
	return f.sepaView, f.sepaErr
}

func serveSEPAMandate(t *testing.T, rs *Resource, method, path, body string, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Route("/parent", func(r chi.Router) { rs.mountSEPAMandateRoutes(r, nil) })
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: accountID, Sub: "eltern@example.org",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

const testIBAN = "DE89370400440532013000"

// The grant names the account of the claims and the contract of the path,
// never one of the body, and answers 201 with the mask only.
func TestGrantSEPAMandate_UsesTheClaimsAndThePath(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	parent := &fakeParentService{sepaView: parentService.SEPAMandateView{Enabled: true, CanManage: true,
		Mandate: &parentService.SEPAMandateSummary{ID: 5, IBANMasked: "DE•• •••• •••• •••• ••30 00"}}}

	w := serveSEPAMandate(t, &Resource{ParentService: parent}, http.MethodPost, "/parent/me/documents/4/contracts/31/sepa-mandate",
		`{"account_holder":"Eva Muster","iban":"`+testIBAN+`","holder_confirmed":true,"account_id":99,"contract_id":7}`, 7301)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "grant", parent.gotSEPACall)
	assert.Equal(t, int64(7301), parent.gotDocumentsAccount, "the account comes from the claims")
	assert.EqualValues(t, 4, parent.gotDocumentOrganization)
	assert.EqualValues(t, 31, parent.gotDocumentID, "the contract comes from the path")
	assert.Equal(t, testIBAN, parent.gotSEPAInput.IBAN)
	assert.True(t, parent.gotSEPAInput.HolderConfirmed)
	assert.NotContains(t, w.Body.String(), testIBAN, "the answer never carries the IBAN")
	assert.Contains(t, w.Body.String(), `"id":"5"`, "backend IDs travel as strings")
}

// Each route reaches its own operation.
func TestSEPAMandateRoutes_ReachTheirOperation(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	for _, tc := range []struct{ method, suffix, call string }{
		{http.MethodGet, "", "read"}, {http.MethodPost, "/preview", "preview"}, {http.MethodPost, "/revoke", "revoke"},
	} {
		parent := &fakeParentService{}
		w := serveSEPAMandate(t, &Resource{ParentService: parent}, tc.method, "/parent/me/documents/4/contracts/31/sepa-mandate"+tc.suffix, `{}`, 7301)
		require.Equal(t, http.StatusOK, w.Code, tc.suffix)
		assert.Equal(t, tc.call, parent.gotSEPACall)
	}
}

// Every refusal answers its registered code, and none repeats the IBAN of
// the request.
func TestSEPAMandate_RefusesWithItsCodes(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{fmt.Errorf("%w: iban: must be a valid IBAN", parentService.ErrSEPAMandateInvalid), http.StatusBadRequest, "parent_billing.sepa_mandate_invalid"},
		{parentService.ErrOnlineMandateDisabled, http.StatusForbidden, "parent_billing.online_mandate_disabled"},
		{parentService.ErrGuardianPermissionDenied, http.StatusForbidden, "care.guardian_permission_denied"},
		{parentService.ErrBillingDocumentNotFound, http.StatusNotFound, "parent_billing.parent_document_not_found"},
		{parentService.ErrSEPAMandateNotFound, http.StatusNotFound, "parent_billing.sepa_mandate_not_found"},
		{parentService.ErrSEPAMandateInForce, http.StatusConflict, "parent_billing.sepa_mandate_in_force"},
		{parentService.ErrSEPAMandateTextChanged, http.StatusConflict, "parent_billing.sepa_mandate_text_changed"},
		{parentService.ErrSEPACreditorMissing, http.StatusConflict, "parent_billing.sepa_creditor_missing"},
	}
	for _, tc := range cases {
		parent := &fakeParentService{sepaErr: tc.err}
		w := serveSEPAMandate(t, &Resource{ParentService: parent}, http.MethodPost, "/parent/me/documents/4/contracts/31/sepa-mandate",
			`{"iban":"`+testIBAN+`"}`, 7301)
		assert.Equal(t, tc.status, w.Code, tc.code)
		assert.Contains(t, w.Body.String(), `"code":"`+tc.code+`"`)
		assert.NotContains(t, w.Body.String(), testIBAN, tc.code)
	}

	w := serveSEPAMandate(t, &Resource{ParentService: &fakeParentService{}}, http.MethodPost,
		"/parent/me/documents/4/contracts/31/sepa-mandate", `{"iban":`, 7301)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `"code":"parent_billing.sepa_mandate_invalid"`)

	w = serveSEPAMandate(t, &Resource{ParentService: &fakeParentService{}}, http.MethodGet,
		"/parent/me/documents/4/contracts/31/sepa-mandate", "", 0)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "no claims, no mandate")
}

// The writes of the mandate take bank data and run behind the auth limiter
// of the parents router; the read does not. A limiter that refuses every
// request proves the wiring of the production router, not of a test mount.
func TestSEPAMandateWritesRunBehindTheAuthLimiter(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	limited := 0
	limiter := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			limited++
			w.WriteHeader(http.StatusTooManyRequests)
		})
	}
	parent := &fakeParentService{}
	router := (&Resource{ParentService: parent}).RouterWithAuthRateLimiter(limiter, func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims,
					sessiontoken.AppClaims{ID: 7301, Sub: "eltern@example.org"})
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
	})
	const base = "/me/documents/4/contracts/31/sepa-mandate"
	for _, path := range []string{base, base + "/preview", base + "/revoke"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		assert.Equal(t, http.StatusTooManyRequests, w.Code, path)
	}
	assert.Equal(t, 3, limited, "every write passes the limiter")
	assert.Empty(t, parent.gotSEPACall, "a refused write never reaches the service")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, base, nil))
	assert.Equal(t, http.StatusOK, w.Code, "the read is not limited")
	assert.Equal(t, "read", parent.gotSEPACall)
	assert.Equal(t, 3, limited)
}
