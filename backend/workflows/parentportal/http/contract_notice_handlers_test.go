package parent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func (f *fakeParentService) ContractNoticeOptions(_ context.Context, accountID, organizationID, contractID int64) (parentService.ContractNoticeOptions, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID = accountID, organizationID, contractID
	return f.noticeOptions, f.documentsErr
}

func (f *fakeParentService) DeclareContractNotice(_ context.Context, accountID, organizationID, contractID int64,
	input parentService.ContractNoticeInput) (parentService.ContractNoticeOptions, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID = accountID, organizationID, contractID
	f.gotNoticeInput = input
	return f.noticeOptions, f.documentsErr
}

func serveContractNotice(t *testing.T, rs *Resource, method, path, body string, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/me/documents/{organizationId}/contracts/{contractId}/notice", rs.contractNoticeOptions)
	router.Post("/parent/me/documents/{organizationId}/contracts/{contractId}/notice", rs.declareContractNotice)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: accountID, Sub: "eltern@example.org",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

// The declaration names the account of the claims and the contract of the
// path, never one of the body, and answers the confirmation.
func TestDeclareContractNotice_UsesTheClaimsAndThePath(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	endsOn := timezone.NewDate(2026, 12, 31)
	parent := &fakeParentService{noticeOptions: parentService.ContractNoticeOptions{
		ContractID: 31, Status: "gekuendigt", Notice: &parentService.ContractNotice{
			Kind: "kuendigung", TerminationKind: "ordentlich", EndsOn: &endsOn, ConfirmationText: "wir bestätigen Ihre Kündigung",
		},
	}}

	w := serveContractNotice(t, &Resource{ParentService: parent}, http.MethodPost, "/parent/me/documents/4/contracts/31/notice",
		`{"kind":"kuendigung","termination_kind":"ordentlich","account_id":99}`, 7301)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(7301), parent.gotDocumentsAccount, "the account comes from the claims")
	assert.EqualValues(t, 4, parent.gotDocumentOrganization)
	assert.EqualValues(t, 31, parent.gotDocumentID)
	assert.Equal(t, parentService.ContractNoticeInput{Kind: "kuendigung", TerminationKind: "ordentlich"}, parent.gotNoticeInput)
	assert.Contains(t, w.Body.String(), `"ends_on":"2026-12-31"`)
}

// Without the relationship right 403, outside the inbox 404, a refusal of
// the contract 409 with its code, and no claims 401.
func TestContractNotice_RefusesWithItsCodes(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	path := "/parent/me/documents/4/contracts/31/notice"
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{parentService.ErrGuardianPermissionDenied, http.StatusForbidden, "care.guardian_permission_denied"},
		{fmt.Errorf("read: %w", parentService.ErrBillingDocumentNotFound), http.StatusNotFound, "parent_billing.parent_document_not_found"},
		{parentService.ErrContractNotEndable, http.StatusConflict, "care_contract.contract_not_endable"},
		{parentService.ErrContractWithdrawalClosed, http.StatusConflict, "care_contract.withdrawal_closed"},
		{parentService.ErrContractSpecialTerminationClosed, http.StatusConflict, "care_contract.special_termination_closed"},
		{parentService.ErrContractNoticeTemplateMissing, http.StatusConflict, "care_contract.notice_template_missing"},
		{parentService.ErrContractNoticeInvalid, http.StatusBadRequest, "care_contract.field_invalid"},
	}
	for _, tc := range cases {
		w := serveContractNotice(t, &Resource{ParentService: &fakeParentService{documentsErr: tc.err}}, http.MethodPost, path,
			`{"kind":"widerruf"}`, 7301)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		assert.Equal(t, tc.code, refusalCode(t, w))
	}
	w := serveContractNotice(t, &Resource{ParentService: &fakeParentService{documentsErr: parentService.ErrGuardianPermissionDenied}},
		http.MethodGet, path, "", 7301)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	anonymous := &fakeParentService{}
	w = serveContractNotice(t, &Resource{ParentService: anonymous}, http.MethodPost, path, `{"kind":"widerruf"}`, 0)
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Zero(t, anonymous.gotDocumentsAccount, "no claims, no declaration")

	var body map[string]any
	w = serveContractNotice(t, &Resource{ParentService: &fakeParentService{}}, http.MethodPost, "/parent/me/documents/x/contracts/31/notice", `{}`, 7301)
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
}
