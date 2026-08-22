package parent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
)

func parentRequestWithStudentID(method, path, body, studentID string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req = withClaims(req, 1234)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("studentId", studentID)
	return req.WithContext(
		contextWithChiRoute(req.Context(), rctx),
	)
}

func contextWithChiRoute(ctx context.Context, route *chi.Context) context.Context {
	return context.WithValue(ctx, chi.RouteCtxKey, route)
}

func sampleMasterData() *parentService.ChildMasterData {
	birthday := timezone.NewDate(2018, time.March, 4)
	health := "Allergie"
	email := "parent@example.test"
	return &parentService.ChildMasterData{
		StudentID:              42,
		FirstName:              "Lara",
		LastName:               "Beispiel",
		Birthday:               &birthday,
		SchoolClass:            "2a",
		Status:                 "active",
		HealthInfo:             &health,
		GuardianProfileID:      77,
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
		AllowedDepartureModes: usersModels.AllowedDepartureModes{
			"mon": []usersModels.DepartureMode{usersModels.DeparturePickup},
		},
		PendingChanges: []*usersModels.StudentDataChangeRequest{
			{
				Model:    modelBase.Model{ID: 900, CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)},
				Target:   usersModels.DataChangeTargetPerson,
				FieldKey: "first_name",
				NewValue: json.RawMessage(`"Lea"`),
				Status:   usersModels.DataChangeStatusPending,
			},
		},
	}
}

func TestGetMasterData_ReturnsProjectedChildData(t *testing.T) {
	t.Parallel()

	fake := &fakeParentService{masterData: sampleMasterData()}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(http.MethodGet, "/children/42/master-data", "", "42")
	w := httptest.NewRecorder()

	rs.getMasterData(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, `"student_id":"42"`)
	assert.Contains(t, body, `"first_name":"Lara"`)
	assert.Contains(t, body, `"birthday":"2018-03-04"`)
	assert.Contains(t, body, `"guardian_profile_id":"77"`)
	assert.Contains(t, body, `"pending_changes"`)
}

func TestUpdateMasterDataField_ForwardsPathAndJSONValue(t *testing.T) {
	t.Parallel()

	fake := &fakeParentService{updateMasterData: sampleMasterData()}
	rs := &Resource{ParentService: fake}
	req := parentRequestWithStudentID(
		http.MethodPatch,
		"/children/42/master-data/student/health_info",
		`{"value":"Neue Info"}`,
		"42",
	)
	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("target", usersModels.DataChangeTargetStudent)
	rctx.URLParams.Add("field", "health_info")
	w := httptest.NewRecorder()

	rs.updateMasterDataField(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, int64(1234), fake.gotMasterAccount)
	assert.Equal(t, int64(42), fake.gotMasterStudent)
	assert.Equal(t, usersModels.DataChangeTargetStudent, fake.gotMasterTarget)
	assert.Equal(t, "health_info", fake.gotMasterField)
	assert.JSONEq(t, `"Neue Info"`, string(fake.gotMasterValue))
}

func TestUpdateMasterDataField_RejectsMissingTargetOrBadBody(t *testing.T) {
	t.Parallel()

	rs := &Resource{ParentService: &fakeParentService{}}

	req := parentRequestWithStudentID(http.MethodPatch, "/children/42/master-data//", `{}`, "42")
	w := httptest.NewRecorder()
	rs.updateMasterDataField(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	req = parentRequestWithStudentID(http.MethodPatch, "/children/42/master-data/student/health_info", `{`, "42")
	rctx := chi.RouteContext(req.Context())
	rctx.URLParams.Add("target", usersModels.DataChangeTargetStudent)
	rctx.URLParams.Add("field", "health_info")
	w = httptest.NewRecorder()
	rs.updateMasterDataField(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSubmitAndListMasterDataRequests(t *testing.T) {
	t.Parallel()

	row := &usersModels.StudentDataChangeRequest{
		Model:    modelBase.Model{ID: 901, CreatedAt: time.Date(2026, 6, 24, 12, 0, 0, 0, time.UTC)},
		Target:   usersModels.DataChangeTargetPerson,
		FieldKey: "first_name",
		NewValue: json.RawMessage(`"Lea"`),
		Status:   usersModels.DataChangeStatusPending,
	}
	fake := &fakeParentService{submitRows: []*usersModels.StudentDataChangeRequest{row}, listRows: []*usersModels.StudentDataChangeRequest{row}}
	rs := &Resource{ParentService: fake}

	req := parentRequestWithStudentID(
		http.MethodPost,
		"/children/42/master-data/requests",
		`{"changes":[{"target":"person","field_key":"first_name","value":"Lea"}]}`,
		"42",
	)
	w := httptest.NewRecorder()
	rs.submitMasterDataRequest(w, req)

	require.Equal(t, http.StatusCreated, w.Code)
	require.Len(t, fake.gotSubmitChanges, 1)
	assert.Equal(t, usersModels.DataChangeTargetPerson, fake.gotSubmitChanges[0].Target)
	assert.JSONEq(t, `"Lea"`, string(fake.gotSubmitChanges[0].Value))
	assert.Contains(t, w.Body.String(), `"id":"901"`)

	req = parentRequestWithStudentID(http.MethodGet, "/children/42/master-data/requests", "", "42")
	w = httptest.NewRecorder()
	rs.listMasterDataRequests(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"field_key":"first_name"`)
}

func TestSubmitMasterDataRequest_MapsServiceErrors(t *testing.T) {
	t.Parallel()

	rs := &Resource{ParentService: &fakeParentService{submitErr: parentService.ErrMasterDataDuplicatePending}}
	req := parentRequestWithStudentID(
		http.MethodPost,
		"/children/42/master-data/requests",
		`{"changes":[{"target":"person","field_key":"first_name","value":"Lea"}]}`,
		"42",
	)
	w := httptest.NewRecorder()

	rs.submitMasterDataRequest(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Contains(t, w.Body.String(), "master_data_duplicate_pending")
}
