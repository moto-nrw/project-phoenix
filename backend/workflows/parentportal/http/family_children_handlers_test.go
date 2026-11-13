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

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

func (f *fakeParentService) FamilyChildren(_ context.Context, accountID, organizationID, contractID int64) (parentService.FamilyChildren, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID = accountID, organizationID, contractID
	return f.familyChildren, f.documentsErr
}

func (f *fakeParentService) StateFamilyChildren(_ context.Context, accountID, organizationID, contractID int64, children int) (parentService.FamilyChildren, error) {
	f.gotDocumentsAccount, f.gotDocumentOrganization, f.gotDocumentID = accountID, organizationID, contractID
	f.gotFamilyChildren = children
	return f.familyChildren, f.documentsErr
}

func serveFamilyChildren(t *testing.T, rs *Resource, method, path, body string, accountID int) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/me/documents/{organizationId}/contracts/{contractId}/family-children", rs.familyChildren)
	router.Put("/parent/me/documents/{organizationId}/contracts/{contractId}/family-children", rs.stateFamilyChildren)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: accountID, Sub: "eltern@example.org",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

// The statement names the account of the claims and the contract of the
// path, never one of the body.
func TestStateFamilyChildren_UsesTheClaimsAndThePath(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	three := 3
	parent := &fakeParentService{familyChildren: parentService.FamilyChildren{StatedChildren: &three, CanState: true}}

	w := serveFamilyChildren(t, &Resource{ParentService: parent}, http.MethodPut, "/parent/me/documents/4/contracts/31/family-children",
		`{"children":3,"account_id":99,"contract_id":7}`, 7301)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(7301), parent.gotDocumentsAccount, "the account comes from the claims")
	assert.EqualValues(t, 4, parent.gotDocumentOrganization)
	assert.EqualValues(t, 31, parent.gotDocumentID, "the contract comes from the path")
	assert.Equal(t, 3, parent.gotFamilyChildren)
	assert.Contains(t, w.Body.String(), `"stated_children":3`)
	assert.Contains(t, w.Body.String(), `"can_state":true`)
}

// An invalid number 400, without the relationship right 403, outside the
// inbox 404, no claims 401.
func TestFamilyChildren_RefusesWithItsCodes(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	path := "/parent/me/documents/4/contracts/31/family-children"
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{parentService.ErrFamilyChildrenInvalid, http.StatusBadRequest, "parent_billing.family_children_invalid"},
		{parentService.ErrGuardianPermissionDenied, http.StatusForbidden, "care.guardian_permission_denied"},
		{fmt.Errorf("read: %w", parentService.ErrBillingDocumentNotFound), http.StatusNotFound, "parent_billing.parent_document_not_found"},
	}
	for _, tc := range cases {
		w := serveFamilyChildren(t, &Resource{ParentService: &fakeParentService{documentsErr: tc.err}}, http.MethodPut, path,
			`{"children":2}`, 7301)
		require.Equal(t, tc.status, w.Code, w.Body.String())
		assert.Equal(t, tc.code, refusalCode(t, w))
	}
	w := serveFamilyChildren(t, &Resource{ParentService: &fakeParentService{documentsErr: parentService.ErrGuardianPermissionDenied}},
		http.MethodGet, path, "", 7301)
	require.Equal(t, http.StatusForbidden, w.Code, w.Body.String())

	anonymous := &fakeParentService{}
	w = serveFamilyChildren(t, &Resource{ParentService: anonymous}, http.MethodPut, path, `{"children":2}`, 0)
	require.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
	assert.Zero(t, anonymous.gotDocumentsAccount, "no claims, no statement")

	w = serveFamilyChildren(t, &Resource{ParentService: &fakeParentService{}}, http.MethodPut, path, `{"children":"drei"}`, 7301)
	require.Equal(t, http.StatusBadRequest, w.Code, "a number that is not whole is refused")
	assert.Equal(t, "parent_billing.family_children_invalid", refusalCode(t, w))
}
