package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	authService "github.com/moto-nrw/project-phoenix/services/auth"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type caregiverCapabilityServiceStub struct {
	getFn     func(context.Context, int64) (*userModel.CaregiverCapabilityState, error)
	enableFn  func(context.Context, int64, userModel.EnableCaregiverCapabilityInput) (*userModel.CaregiverCapabilityState, error)
	disableFn func(context.Context, int64) (*userModel.CaregiverCapabilityState, error)
}

func (s caregiverCapabilityServiceStub) GetCaregiverCapability(ctx context.Context, accountID int64) (*userModel.CaregiverCapabilityState, error) {
	if s.getFn != nil {
		return s.getFn(ctx, accountID)
	}
	return nil, nil
}

func (s caregiverCapabilityServiceStub) EnableCaregiverCapability(ctx context.Context, accountID int64, input userModel.EnableCaregiverCapabilityInput) (*userModel.CaregiverCapabilityState, error) {
	if s.enableFn != nil {
		return s.enableFn(ctx, accountID, input)
	}
	return nil, nil
}

func (s caregiverCapabilityServiceStub) DisableCaregiverCapability(ctx context.Context, accountID int64) (*userModel.CaregiverCapabilityState, error) {
	if s.disableFn != nil {
		return s.disableFn(ctx, accountID)
	}
	return nil, nil
}

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
			getFn: func(_ context.Context, accountID int64) (*userModel.CaregiverCapabilityState, error) {
				assert.Equal(t, int64(7), accountID)
				return &userModel.CaregiverCapabilityState{
					AccountID:           accountID,
					HasTeacher:          true,
					HasUserRole:         true,
					IsActiveCaregiver:   true,
					HasCaregiverProfile: true,
				}, nil
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
			enableFn: func(_ context.Context, accountID int64, input userModel.EnableCaregiverCapabilityInput) (*userModel.CaregiverCapabilityState, error) {
				assert.Equal(t, int64(7), accountID)
				assert.Equal(t, userModel.EnableCaregiverCapabilityInput{
					FirstName: "Ada",
					LastName:  "Lovelace",
					Position:  "Betreuung",
				}, input)
				return &userModel.CaregiverCapabilityState{AccountID: accountID, HasTeacher: true}, nil
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
			enableFn: func(context.Context, int64, userModel.EnableCaregiverCapabilityInput) (*userModel.CaregiverCapabilityState, error) {
				return nil, &usersService.UsersError{
					Op:  "enable caregiver capability",
					Err: &usersService.ValidationError{Err: fmt.Errorf("invalid caregiver capability request")},
				}
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
			disableFn: func(context.Context, int64) (*userModel.CaregiverCapabilityState, error) {
				return nil, &usersService.CaregiverCapabilityBlockedError{
					Reasons: []userModel.CaregiverCapabilityBlockerCode{
						userModel.CaregiverCapabilityBlockerActiveGroupSupervisions,
					},
				}
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

	renderer := caregiverCapabilityErrorRenderer(authService.ErrAccountNotFound)
	errResponse, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusNotFound, errResponse.HTTPStatusCode)
}

func TestCaregiverCapabilityErrorRenderer_MapsUsersErrorToBadRequest(t *testing.T) {
	t.Parallel()

	renderer := caregiverCapabilityErrorRenderer(&usersService.UsersError{
		Op:  "enable caregiver capability",
		Err: &usersService.ValidationError{Err: fmt.Errorf("invalid caregiver capability request")},
	})
	errResponse, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, errResponse.HTTPStatusCode)
}

func TestCaregiverCapabilityErrorRenderer_MapsUsersOperationalErrorToInternalServerError(t *testing.T) {
	t.Parallel()

	renderer := caregiverCapabilityErrorRenderer(&usersService.UsersError{
		Op:  "disable caregiver capability",
		Err: fmt.Errorf("lock activities.supervisors: database offline"),
	})
	errResponse, ok := renderer.(*common.ErrResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, errResponse.HTTPStatusCode)
}

func TestCaregiverCapabilityErrorRenderer_MapsBlockedToSharedRenderer(t *testing.T) {
	t.Parallel()

	renderer := caregiverCapabilityErrorRenderer(&usersService.CaregiverCapabilityBlockedError{
		Reasons: []userModel.CaregiverCapabilityBlockerCode{
			userModel.CaregiverCapabilityBlockerGroupAssignments,
		},
	})
	blocked, ok := renderer.(*common.CaregiverCapabilityBlockedResponse)
	require.True(t, ok)
	assert.Equal(t, http.StatusConflict, blocked.HTTPStatusCode)
	assert.Equal(t, []userModel.CaregiverCapabilityBlockerCode{
		userModel.CaregiverCapabilityBlockerGroupAssignments,
	}, blocked.Blockers)
}
