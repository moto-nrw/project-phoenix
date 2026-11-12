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

	"github.com/moto-ogs/project-phoenix/modules/identityaccess/sessiontoken"
	timezone "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	parentService "github.com/moto-ogs/project-phoenix/workflows/parentportal"
)

// holidayLinksStub resolves one known link.
type holidayLinksStub struct {
	token          string
	organizationID int64
	schoolIDs      []int64
}

func (s holidayLinksStub) ResolveHolidayRegistrationSchools(_ context.Context, token string) (parentService.HolidayRegistrationScope, error) {
	if token != s.token {
		return parentService.HolidayRegistrationScope{}, ErrHolidayRegistrationLinkNotFound
	}
	return parentService.HolidayRegistrationScope{OrganizationID: s.organizationID, SchoolIDs: s.schoolIDs}, nil
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

// The prefill of #6682 on the wire: the parent's address and the child's
// other guardians with string IDs; redacted contact data stays absent.
func TestHolidayRegistrationFamilyResponse_CarriesAddressAndContacts(t *testing.T) {
	t.Parallel()
	response := holidayRegistrationFamilyResponse(sessiontoken.AppClaims{Sub: "eltern@example.org"}, &parentService.HolidayRegistrationFamily{
		Guardian: &parentService.HolidayRegistrationGuardian{
			FirstName: "Sabine", Address: &parentService.HolidayRegistrationAddress{Street: "Lindenweg 4", PostalCode: "49143", City: "Bissendorf"},
		},
		Children: []parentService.HolidayRegistrationChild{{StudentID: 11, SchoolID: 21, Contacts: []parentService.HolidayRegistrationContact{
			{GuardianProfileID: 31, FirstName: "Erika", LastName: "Brandt", Phone: "+49 5402 9876", RelationshipType: "relative", Emergency: true, Pickup: true},
			{GuardianProfileID: 32, FirstName: "Lena", LastName: "Vogt", RelationshipType: "parent", FullGuardian: true},
		}}},
	})
	raw, err := json.Marshal(response)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"guardian": {"first_name": "Sabine", "last_name": "", "email": "eltern@example.org", "phone": "",
			"address": {"street": "Lindenweg 4", "postal_code": "49143", "city": "Bissendorf"}},
		"children": [{"student_id": "11", "school_id": "21", "first_name": "", "last_name": "", "contacts": [
			{"guardian_profile_id": "31", "first_name": "Erika", "last_name": "Brandt", "phone": "+49 5402 9876",
				"relationship_type": "relative", "full_guardian": false, "emergency": true, "pickup": true},
			{"guardian_profile_id": "32", "first_name": "Lena", "last_name": "Vogt",
				"relationship_type": "parent", "full_guardian": true, "emergency": false, "pickup": false}]}],
		"account_proof": ""}`, string(raw))
}
