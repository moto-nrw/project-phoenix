package active

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/active"
	activityModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/facilities"
)

// =============================================================================
// Mock Services
// =============================================================================

// mockSchulhofService implements facilities.SchulhofService for testing
type mockSchulhofService struct {
	getStatusFunc            func(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error)
	toggleSupervisionFunc    func(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error)
	ensureInfrastructureFunc func(ctx context.Context, createdBy int64) (*activityModels.Group, error)
	getOrCreateActiveFunc    func(ctx context.Context, createdBy int64) (*active.Group, error)
}

func (m *mockSchulhofService) GetSchulhofStatus(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error) {
	if m.getStatusFunc != nil {
		return m.getStatusFunc(ctx, staffID)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSchulhofService) ToggleSupervision(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error) {
	if m.toggleSupervisionFunc != nil {
		return m.toggleSupervisionFunc(ctx, staffID, action)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSchulhofService) EnsureInfrastructure(ctx context.Context, createdBy int64) (*activityModels.Group, error) {
	if m.ensureInfrastructureFunc != nil {
		return m.ensureInfrastructureFunc(ctx, createdBy)
	}
	return nil, errors.New("not implemented")
}

func (m *mockSchulhofService) GetOrCreateActiveGroup(ctx context.Context, createdBy int64) (*active.Group, error) {
	if m.getOrCreateActiveFunc != nil {
		return m.getOrCreateActiveFunc(ctx, createdBy)
	}
	return nil, errors.New("not implemented")
}

// mockUserContextService implements usercontext.UserContextService for testing
type mockUserContextService struct {
	getCurrentStaffFunc   func(ctx context.Context) (*users.Staff, error)
	getCurrentProfileFunc func(ctx context.Context) (map[string]interface{}, error)
}

func (m *mockUserContextService) GetCurrentStaff(ctx context.Context) (*users.Staff, error) {
	if m.getCurrentStaffFunc != nil {
		return m.getCurrentStaffFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetCurrentUser(ctx context.Context) (*auth.Account, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetCurrentPerson(ctx context.Context) (*users.Person, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetCurrentTeacher(ctx context.Context) (*users.Teacher, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetMyGroups(ctx context.Context) ([]*education.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetMyActivityGroups(ctx context.Context) ([]*activityModels.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetMyActiveGroups(ctx context.Context) ([]*active.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetMySupervisedGroups(ctx context.Context) ([]*active.Group, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetGroupStudents(ctx context.Context, groupID int64) ([]*users.Student, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetGroupVisits(ctx context.Context, groupID int64) ([]*active.Visit, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) GetCurrentProfile(ctx context.Context) (map[string]interface{}, error) {
	if m.getCurrentProfileFunc != nil {
		return m.getCurrentProfileFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) UpdateCurrentProfile(ctx context.Context, updates map[string]interface{}) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) UpdateAvatar(ctx context.Context, avatarURL string) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) WithTx(tx bun.Tx) interface{} {
	return m
}

// =============================================================================
// Test Setup
// =============================================================================

func setupSchulhofTestRouter(resource *SchulhofResource) chi.Router {
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Get("/status", resource.getSchulhofStatus)
	router.Post("/supervise", resource.toggleSchulhofSupervision)
	return router
}

func executeSchulhofRequest(router chi.Router, req *http.Request, claims jwt.AppClaims, permissions []string) *httptest.ResponseRecorder {
	ctx := context.WithValue(req.Context(), jwt.CtxClaims, claims)
	ctx = context.WithValue(ctx, jwt.CtxPermissions, permissions)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// =============================================================================
// getSchulhofStatus Handler Tests
// =============================================================================

func TestGetSchulhofStatus_Success(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error) {
			roomID := int64(100)
			activityGroupID := int64(200)
			activeGroupID := int64(300)
			supervisionID := int64(400)

			return &facilities.SchulhofStatus{
				Exists:            true,
				RoomID:            &roomID,
				RoomName:          "Schulhof",
				ActivityGroupID:   &activityGroupID,
				ActiveGroupID:     &activeGroupID,
				IsUserSupervising: true,
				SupervisionID:     &supervisionID,
				SupervisorCount:   2,
				StudentCount:      15,
				Supervisors: []facilities.SupervisorInfo{
					{
						ID:            1,
						StaffID:       10,
						Name:          "John Doe",
						IsCurrentUser: true,
					},
					{
						ID:            2,
						StaffID:       20,
						Name:          "Jane Smith",
						IsCurrentUser: false,
					},
				},
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"exists":true`)
	assert.Contains(t, rr.Body.String(), `"room_name":"Schulhof"`)
	assert.Contains(t, rr.Body.String(), `"is_user_supervising":true`)
	assert.Contains(t, rr.Body.String(), `"supervisor_count":2`)
	assert.Contains(t, rr.Body.String(), `"student_count":15`)
	assert.Contains(t, rr.Body.String(), `"John Doe"`)
	assert.Contains(t, rr.Body.String(), `"Jane Smith"`)
}

func TestGetSchulhofStatus_SchulhofDoesNotExist(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error) {
			return &facilities.SchulhofStatus{
				Exists:            false,
				RoomName:          "",
				IsUserSupervising: false,
				SupervisorCount:   0,
				StudentCount:      0,
				Supervisors:       []facilities.SupervisorInfo{},
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"exists":false`)
	assert.Contains(t, rr.Body.String(), `"supervisor_count":0`)
}

func TestGetSchulhofStatus_UserNotStaff(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return nil, errors.New("user is not staff")
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.DefaultTestClaims(), []string{"schulhof:read"})

	testutil.AssertForbidden(t, rr)
	assert.Contains(t, rr.Body.String(), "user must be a staff member")
}

func TestGetSchulhofStatus_ServiceError(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error) {
			return nil, errors.New("database connection failed")
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "failed to get Schulhof status")
}

// =============================================================================
// toggleSchulhofSupervision Handler Tests
// =============================================================================

func TestToggleSchulhofSupervision_StartSuccess(t *testing.T) {
	supervisionID := int64(500)
	mockSchulhof := &mockSchulhofService{
		toggleSupervisionFunc: func(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error) {
			assert.Equal(t, "start", action)
			return &facilities.SupervisionResult{
				Action:        "started",
				SupervisionID: &supervisionID,
				ActiveGroupID: 300,
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{
		"action": "start",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	assert.Equal(t, http.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"action":"started"`)
	assert.Contains(t, rr.Body.String(), `"supervision_id":500`)
	assert.Contains(t, rr.Body.String(), `"active_group_id":300`)
	assert.Contains(t, rr.Body.String(), "Schulhof supervision started successfully")
}

func TestToggleSchulhofSupervision_StopSuccess(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		toggleSupervisionFunc: func(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error) {
			assert.Equal(t, "stop", action)
			return &facilities.SupervisionResult{
				Action:        "stopped",
				SupervisionID: nil, // No supervision ID when stopping
				ActiveGroupID: 300,
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{
		"action": "stop",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"action":"stopped"`)
	assert.Contains(t, rr.Body.String(), "Schulhof supervision stopped successfully")
}

func TestToggleSchulhofSupervision_UserNotStaff(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return nil, errors.New("user is not staff")
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{
		"action": "start",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.DefaultTestClaims(), []string{"schulhof:write"})

	testutil.AssertForbidden(t, rr)
	assert.Contains(t, rr.Body.String(), "user must be a staff member")
}

func TestToggleSchulhofSupervision_InvalidRequestBody(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := httptest.NewRequest("POST", "/supervise", nil)
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	testutil.AssertBadRequest(t, rr)
	assert.Contains(t, rr.Body.String(), "invalid request body")
}

func TestToggleSchulhofSupervision_MissingAction(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{} // Empty body
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	testutil.AssertBadRequest(t, rr)
	assert.Contains(t, rr.Body.String(), "action must be 'start' or 'stop'")
}

func TestToggleSchulhofSupervision_InvalidAction(t *testing.T) {
	testCases := []struct {
		name   string
		action string
	}{
		{"empty string", ""},
		{"invalid value", "pause"},
		{"uppercase", "START"},
		{"mixed case", "Start"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockSchulhof := &mockSchulhofService{}

			mockUserContext := &mockUserContextService{
				getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
					return &users.Staff{PersonID: 10}, nil
				},
			}

			resource := NewSchulhofResource(mockSchulhof, mockUserContext)
			router := setupSchulhofTestRouter(resource)
			body := map[string]interface{}{
				"action": tc.action,
			}
			bodyBytes, _ := json.Marshal(body)
			req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")

			rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

			testutil.AssertBadRequest(t, rr)
			assert.Contains(t, rr.Body.String(), "action must be 'start' or 'stop'")
		})
	}
}

func TestToggleSchulhofSupervision_NotCurrentlySupervisingError(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		toggleSupervisionFunc: func(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error) {
			return nil, errors.New("user is not currently supervising the Schulhof")
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{
		"action": "stop",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Body.String(), "user is not currently supervising the Schulhof")
}

func TestToggleSchulhofSupervision_ServiceError(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		toggleSupervisionFunc: func(ctx context.Context, staffID int64, action string) (*facilities.SupervisionResult, error) {
			return nil, errors.New("database connection failed")
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	body := map[string]interface{}{
		"action": "start",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	testutil.AssertErrorResponse(t, rr, http.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "database connection failed")
}

// =============================================================================
// Edge Cases and Integration
// =============================================================================

func TestSchulhofHandlers_NilStaffFromContext(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return nil, nil // Returns nil staff without error
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)

	t.Run("getSchulhofStatus", func(t *testing.T) {
		req := testutil.NewRequest("GET", "/status", nil)
		rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})
		testutil.AssertForbidden(t, rr)
	})

	t.Run("toggleSchulhofSupervision", func(t *testing.T) {
		body := map[string]interface{}{
			"action": "start",
		}
		bodyBytes, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/supervise", bytes.NewReader(bodyBytes))
		req.Header.Set("Content-Type", "application/json")

		rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})
		testutil.AssertForbidden(t, rr)
	})
}

func TestSchulhofHandlers_EmptySupervisorsList(t *testing.T) {
	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*facilities.SchulhofStatus, error) {
			roomID := int64(100)
			return &facilities.SchulhofStatus{
				Exists:            true,
				RoomID:            &roomID,
				RoomName:          "Schulhof",
				IsUserSupervising: false,
				SupervisorCount:   0,
				StudentCount:      5,
				Supervisors:       []facilities.SupervisorInfo{}, // Empty list
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"supervisors":[]`)
	assert.Contains(t, rr.Body.String(), `"supervisor_count":0`)
}

func TestToggleSchulhofSupervision_MalformedJSON(t *testing.T) {
	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*users.Staff, error) {
			return &users.Staff{PersonID: 10}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)

	// Malformed JSON
	req := httptest.NewRequest("POST", "/supervise", bytes.NewBufferString("{invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:write"})

	testutil.AssertBadRequest(t, rr)
}
