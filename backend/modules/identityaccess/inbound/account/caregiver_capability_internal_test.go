package account

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type caregiverCapabilityServiceStub struct {
	getFn     func(context.Context, int64) (json.RawMessage, error)
	enableFn  func(ctx context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error)
	disableFn func(context.Context, int64) (json.RawMessage, error)
}

func (s caregiverCapabilityServiceStub) GetCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	if s.getFn != nil {
		return s.getFn(ctx, accountID)
	}
	return nil, nil
}

func (s caregiverCapabilityServiceStub) EnableCaregiverCapability(ctx context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
	if s.enableFn != nil {
		return s.enableFn(ctx, accountID, firstName, lastName, position)
	}
	return nil, nil
}

func (s caregiverCapabilityServiceStub) DisableCaregiverCapability(ctx context.Context, accountID int64) (json.RawMessage, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, accountID)
	}
	return nil, nil
}

// Local stand-ins for the failure behaviours the root's caregiver capability
// binding attaches to People Directory's errors. The mapping from the
// People Directory error types to these behaviours is covered where it
// lives (services/users.CaregiverCapabilityViews).
type (
	fakeCaregiverBlocked struct{ blockers []string }
	fakeCaregiverMissing struct{}
	fakeCaregiverInvalid struct{ cause error }
	fakeCaregiverFailure struct{ cause error }
)

func (e fakeCaregiverBlocked) Error() string {
	return "caregiver capability cannot be removed while active bindings exist"
}
func (e fakeCaregiverBlocked) CaregiverCapabilityBlockers() []string { return e.blockers }

func (fakeCaregiverMissing) Error() string                 { return "account not assigned to tenant" }
func (fakeCaregiverMissing) CaregiverAccountMissing() bool { return true }

func (e fakeCaregiverInvalid) Error() string {
	return "enable caregiver capability: " + e.cause.Error()
}
func (e fakeCaregiverInvalid) CaregiverRequestInvalid() error { return e.cause }

func (e fakeCaregiverFailure) Error() string {
	return "disable caregiver capability: " + e.cause.Error()
}
func (e fakeCaregiverFailure) CaregiverFailure() error { return e.cause }

func withAccountRouteParam(req *http.Request, accountID string) *http.Request {
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("accountId", accountID)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
}

func decodeResponseBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	var body map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	return body
}

// renderCaregiverCapabilityError renders err through caregiverCapabilityErrorRenderer
// the way the routes do and returns the recorded response.
func renderCaregiverCapabilityError(t *testing.T, err error) *httptest.ResponseRecorder {
	t.Helper()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{
			getFn: func(context.Context, int64) (json.RawMessage, error) {
				return nil, err
			},
		},
	}
	req := withAccountRouteParam(httptest.NewRequest(http.MethodGet, "/accounts/7/caregiver-capability", nil), "7")
	rr := httptest.NewRecorder()

	resource.getCaregiverCapability(rr, req)

	return rr
}

func TestEnableCaregiverCapabilityRequestBind_TrimsFields(t *testing.T) {
	t.Parallel()

	req := &EnableCaregiverCapabilityRequest{
		FirstName: "  Ada  ",
		LastName:  "  Lovelace  ",
		Position:  "  Betreuung  ",
	}

	err := req.Bind(nil)

	require.NoError(t, err)
	assert.Equal(t, "Ada", req.FirstName)
	assert.Equal(t, "Lovelace", req.LastName)
	assert.Equal(t, "Betreuung", req.Position)
}

func TestGetCaregiverCapability_ServiceNotConfigured(t *testing.T) {
	t.Parallel()

	resource := &Resource{}
	req := withAccountRouteParam(httptest.NewRequest(http.MethodGet, "/accounts/7/caregiver-capability", nil), "7")
	rr := httptest.NewRecorder()

	resource.getCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetCaregiverCapability_InvalidAccountID(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{},
	}
	req := withAccountRouteParam(httptest.NewRequest(http.MethodGet, "/accounts/nope/caregiver-capability", nil), "nope")
	rr := httptest.NewRecorder()

	resource.getCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetCaregiverCapability_Success(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{
			getFn: func(_ context.Context, accountID int64) (json.RawMessage, error) {
				assert.Equal(t, int64(7), accountID)
				return json.RawMessage(fmt.Sprintf(
					`{"account_id":%d,"has_teacher":true,"has_user_role":true,"is_active_caregiver":true,"has_caregiver_profile":true}`,
					accountID,
				)), nil
			},
		},
	}
	req := withAccountRouteParam(httptest.NewRequest(http.MethodGet, "/accounts/7/caregiver-capability", nil), "7")
	rr := httptest.NewRecorder()

	resource.getCaregiverCapability(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	responseBody := decodeResponseBody(t, rr)
	data := responseBody["data"].(map[string]any)
	assert.Equal(t, float64(7), data["account_id"])
	assert.Equal(t, true, data["has_user_role"])
	assert.Equal(t, true, data["is_active_caregiver"])
}

func TestEnableCaregiverCapability_Success(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{
			enableFn: func(_ context.Context, accountID int64, firstName, lastName, position string) (json.RawMessage, error) {
				assert.Equal(t, int64(7), accountID)
				assert.Equal(t, "Ada", firstName)
				assert.Equal(t, "Lovelace", lastName)
				assert.Equal(t, "Betreuung", position)
				return json.RawMessage(fmt.Sprintf(`{"account_id":%d,"has_teacher":true}`, accountID)), nil
			},
		},
	}

	body := bytes.NewBufferString(`{"first_name":" Ada ","last_name":" Lovelace ","position":" Betreuung "}`)
	req := withAccountRouteParam(httptest.NewRequest(http.MethodPost, "/accounts/7/caregiver-capability", body), "7")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.enableCaregiverCapability(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	responseBody := decodeResponseBody(t, rr)
	data := responseBody["data"].(map[string]any)
	assert.Equal(t, float64(7), data["account_id"])
	assert.Equal(t, true, data["has_teacher"])
}

func TestEnableCaregiverCapability_InvalidBody(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{},
	}

	req := withAccountRouteParam(httptest.NewRequest(http.MethodPost, "/accounts/7/caregiver-capability", bytes.NewBufferString("{")), "7")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.enableCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestEnableCaregiverCapability_EmptyBody(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{},
	}

	req := withAccountRouteParam(
		httptest.NewRequest(http.MethodPost, "/accounts/7/caregiver-capability", http.NoBody),
		"7",
	)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.enableCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	responseBody := decodeResponseBody(t, rr)
	assert.Equal(t, "error", responseBody["status"])
	assert.Equal(t, "EOF", responseBody["error"])
}

func TestEnableCaregiverCapability_RendersUsersValidationError(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{
			enableFn: func(context.Context, int64, string, string, string) (json.RawMessage, error) {
				return nil, fakeCaregiverInvalid{cause: fmt.Errorf("invalid caregiver capability request")}
			},
		},
	}

	body := bytes.NewBufferString(`{"first_name":"Ada"}`)
	req := withAccountRouteParam(httptest.NewRequest(http.MethodPost, "/accounts/7/caregiver-capability", body), "7")
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	resource.enableCaregiverCapability(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestDisableCaregiverCapability_RendersBlockedConflict(t *testing.T) {
	t.Parallel()

	resource := &Resource{
		CaregiverCapabilityService: caregiverCapabilityServiceStub{
			disableFn: func(context.Context, int64) (json.RawMessage, error) {
				return nil, fakeCaregiverBlocked{blockers: []string{"active_group_supervisions"}}
			},
		},
	}

	req := withAccountRouteParam(httptest.NewRequest(http.MethodDelete, "/accounts/7/caregiver-capability", nil), "7")
	rr := httptest.NewRecorder()

	resource.disableCaregiverCapability(rr, req)

	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	responseBody := decodeResponseBody(t, rr)
	assert.Equal(t, "error", responseBody["status"])
	assert.Equal(t, "caregiver capability cannot be removed while active bindings exist", responseBody["error"])
	assert.Equal(t, []any{"active_group_supervisions"}, responseBody["blockers"])
}

func TestCaregiverCapabilityErrorRenderer_MapsNotFound(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, identityaccess.ErrAccountNotFound)
	assert.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
}

func TestCaregiverCapabilityErrorRenderer_MapsMissingAccountToNotFound(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, fmt.Errorf("get caregiver capability: %w", fakeCaregiverMissing{}))
	require.Equal(t, http.StatusNotFound, rr.Code, rr.Body.String())
	assert.Equal(t, "account not found", decodeResponseBody(t, rr)["error"])
}

func TestCaregiverCapabilityErrorRenderer_MapsUsersErrorToBadRequest(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, fakeCaregiverInvalid{cause: fmt.Errorf("invalid caregiver capability request")})
	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
	assert.Equal(t, "invalid caregiver capability request", decodeResponseBody(t, rr)["error"])
}

func TestCaregiverCapabilityErrorRenderer_MapsUsersOperationalErrorToInternalServerError(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, fakeCaregiverFailure{cause: fmt.Errorf("lock activities.supervisors: database offline")})
	assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
}

func TestCaregiverCapabilityErrorRenderer_MapsUnclassifiedErrorToInternalServerError(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, errors.New("unexpected"))
	assert.Equal(t, http.StatusInternalServerError, rr.Code, rr.Body.String())
}

func TestCaregiverCapabilityErrorRenderer_MapsBlockedToSharedRenderer(t *testing.T) {
	t.Parallel()

	rr := renderCaregiverCapabilityError(t, fakeCaregiverBlocked{blockers: []string{"group_assignments"}})
	require.Equal(t, http.StatusConflict, rr.Code, rr.Body.String())
	responseBody := decodeResponseBody(t, rr)
	assert.Equal(t, []any{"group_assignments"}, responseBody["blockers"])
}
