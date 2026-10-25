package parent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// holidayLinksStub resolves one known link.
type holidayLinksStub struct {
	token          string
	organizationID int64
	schoolIDs      []int64
}

func (s holidayLinksStub) ResolveHolidayRegistrationSchools(_ context.Context, token string) (int64, []int64, error) {
	if token != s.token {
		return 0, nil, ErrHolidayRegistrationLinkNotFound
	}
	return s.organizationID, s.schoolIDs, nil
}

func serveHolidayRegistrationFamily(t *testing.T, rs *Resource, token string) *httptest.ResponseRecorder {
	t.Helper()
	router := chi.NewRouter()
	router.Get("/parent/holiday-registrations/{token}/family", rs.getHolidayRegistrationFamily)
	req := httptest.NewRequest(http.MethodGet, "/parent/holiday-registrations/"+token+"/family", nil)
	ctx := context.WithValue(testpkg.WithPackageTenantRuntime(req.Context()), sessiontoken.CtxClaims, sessiontoken.AppClaims{
		ID: 7301, Sub: "eltern@example.org", FirstName: "Konto", LastName: "Name",
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req.WithContext(ctx))
	return w
}

func TestGetHolidayRegistrationFamily_UnknownLinkReadsNoChild(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	parent := &fakeParentService{}
	rs := &Resource{ParentService: parent, HolidayLinks: holidayLinksStub{token: "known"}}

	w := serveHolidayRegistrationFamily(t, rs, "unknown")

	require.Equal(t, http.StatusNotFound, w.Code, w.Body.String())
	var refusal struct {
		Code string `json:"code"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &refusal))
	assert.Equal(t, "enrollment.holiday_registration_not_found", refusal.Code)
	assert.Zero(t, parent.gotHolidayAccount, "no child is read without a published link")
}

// The Träger and its schools come from the link only; the parent fields fall
// back to the account where the school knows no value.
func TestGetHolidayRegistrationFamily_AnswersTheLinkedTraegersChildren(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	birthDate := timezone.NewDate(2018, 3, 1)
	parent := &fakeParentService{holidayFamily: &parentService.HolidayRegistrationFamily{
		Guardian: &parentService.HolidayRegistrationGuardian{FirstName: "Sabine", LastName: "Schneider", Phone: "0151 1"},
		Children: []parentService.HolidayRegistrationChild{{StudentID: 11, SchoolID: 21, FirstName: "Felix", LastName: "Schneider", BirthDate: &birthDate}},
	}}
	rs := &Resource{ParentService: parent, HolidayLinks: holidayLinksStub{token: "known", organizationID: 5, schoolIDs: []int64{21, 22}}}

	w := serveHolidayRegistrationFamily(t, rs, "known")

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, int64(7301), parent.gotHolidayAccount, "the account comes from the claims")
	assert.Equal(t, int64(5), parent.gotHolidayOrganization)
	assert.Equal(t, []int64{21, 22}, parent.gotHolidaySchools)
	var body struct {
		Data HolidayRegistrationFamilyResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, HolidayRegistrationGuardianResponse{FirstName: "Sabine", LastName: "Schneider", Email: "eltern@example.org", Phone: "0151 1"}, body.Data.Guardian)
	require.Len(t, body.Data.Children, 1)
	assert.Equal(t, "11", body.Data.Children[0].StudentID)
	assert.Equal(t, "21", body.Data.Children[0].SchoolID)
	require.NotNil(t, body.Data.Children[0].BirthDate)
	assert.Equal(t, birthDate, *body.Data.Children[0].BirthDate)
}
