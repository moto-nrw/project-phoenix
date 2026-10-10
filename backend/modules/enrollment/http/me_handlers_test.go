package enrollmenthttp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil/routetest"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// guardianProfilesStub answers the autofill port and records the account and
// school the route asked for.
type guardianProfilesStub struct {
	accountID int64
	tenantID  int64
	profile   *capability.GuardianAutofill
	err       error
	called    bool
}

func (s *guardianProfilesStub) LoadForTenant(_ context.Context, accountID, tenantID int64) (*capability.GuardianAutofill, error) {
	s.called = true
	s.accountID, s.tenantID = accountID, tenantID
	return s.profile, s.err
}

func serveMyProfile(t *testing.T, rs *Resource, claims routetest.Claims, tenantID int64) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/me/profile", nil)
	ctx := routetest.WithAuthenticatedContext(req.Context(), claims, nil)
	if tenantID > 0 {
		ctx = tenant.WithTenantID(ctx, tenantID)
	}
	w := httptest.NewRecorder()
	rs.getMyProfile(w, req.WithContext(ctx))
	return w
}

func TestGetMyProfile_UnwiredPortIsServerError(t *testing.T) {
	t.Parallel()

	w := serveMyProfile(t, &Resource{}, routetest.Claims{ID: 4321}, 4242)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// The profile of the school in context replaces the claims as the guardian
// fields, and its children are offered for reuse.
func TestGetMyProfile_RendersTheGuardianProfileOfTheSchool(t *testing.T) {
	t.Parallel()

	email := "anna@example.test"
	stub := &guardianProfilesStub{profile: &capability.GuardianAutofill{
		FirstName: "Anna", LastName: "Beispiel", Email: &email, PrimaryPhone: "0201 123",
		Children: []capability.GuardianAutofillChild{{StudentID: 77, FirstName: "Lara", LastName: "Beispiel", SchoolClass: "2a", EnrollmentSubmit: true, Status: "active"}},
	}}

	w := serveMyProfile(t, &Resource{GuardianProfiles: stub}, routetest.Claims{ID: 4321, FirstName: "Claim", Sub: "claim@example.test"}, 4242)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(4321), stub.accountID)
	assert.Equal(t, int64(4242), stub.tenantID)
	var body struct {
		Data struct {
			Guardian struct {
				FirstName string  `json:"first_name"`
				Email     string  `json:"email"`
				Phone     *string `json:"phone"`
			} `json:"guardian"`
			Children []struct {
				ID          string `json:"id"`
				SchoolClass string `json:"school_class"`
			} `json:"children"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Anna", body.Data.Guardian.FirstName)
	assert.Equal(t, "anna@example.test", body.Data.Guardian.Email)
	require.NotNil(t, body.Data.Guardian.Phone)
	assert.Equal(t, "0201 123", *body.Data.Guardian.Phone)
	require.Len(t, body.Data.Children, 1)
	assert.Equal(t, "77", body.Data.Children[0].ID)
	assert.Equal(t, "2a", body.Data.Children[0].SchoolClass)
}

// An account without a profile in this school gets its claims as the
// guardian fields.
func TestGetMyProfile_NoProfileFallsBackToClaims(t *testing.T) {
	t.Parallel()

	w := serveMyProfile(t, &Resource{GuardianProfiles: &guardianProfilesStub{}}, routetest.Claims{ID: 4321, FirstName: "Claim", Sub: "claim@example.test"}, 4242)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"first_name":"Claim"`)
	assert.Contains(t, w.Body.String(), `"email":"claim@example.test"`)
}

func TestGetMyProfile_LoadFailureIsServerError(t *testing.T) {
	t.Parallel()

	w := serveMyProfile(t, &Resource{GuardianProfiles: &guardianProfilesStub{err: errors.New("profile read failed")}}, routetest.Claims{ID: 4321}, 4242)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestGetMyProfile_RequiresAccountAndTenant(t *testing.T) {
	t.Parallel()

	stub := &guardianProfilesStub{}
	rs := &Resource{GuardianProfiles: stub}

	assert.Equal(t, http.StatusForbidden, serveMyProfile(t, rs, routetest.Claims{}, 4242).Code)
	assert.Equal(t, http.StatusForbidden, serveMyProfile(t, rs, routetest.Claims{ID: 4321}, 0).Code)
	assert.False(t, stub.called, "the port must not be asked without account and tenant")
}
