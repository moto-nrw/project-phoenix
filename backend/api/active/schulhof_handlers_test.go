package active

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/supervisiondashboard"
)

// =============================================================================
// Mock Services
// =============================================================================

// mockSchulhofService implements the courtyard status query for testing.
type mockSchulhofService struct {
	getStatusFunc func(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error)
}

func (m *mockSchulhofService) Status(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
	if m.getStatusFunc != nil {
		return m.getStatusFunc(ctx, staffID)
	}
	return nil, errors.New("not implemented")
}

type mockUserContextService struct {
	getCurrentStaffFunc func(ctx context.Context) (*StaffIdentity, error)
}

func (m *mockUserContextService) GetCurrentStaff(ctx context.Context) (*StaffIdentity, error) {
	if m.getCurrentStaffFunc != nil {
		return m.getCurrentStaffFunc(ctx)
	}
	return nil, errors.New("not implemented")
}

func (m *mockUserContextService) HasCurrentStaff(ctx context.Context) (bool, error) {
	staff, err := m.GetCurrentStaff(ctx)
	return err == nil && staff != nil, err
}

// =============================================================================
// Test Setup
// =============================================================================

func setupSchulhofTestRouter(resource *SchulhofResource) testutil.Router {
	router := testutil.NewJSONRouter()
	router.Get("/status", resource.getSchulhofStatus)
	return router
}

func executeSchulhofRequest(router testutil.Router, req *testutil.Request, claims testutil.Claims, permissions []string) *httptest.ResponseRecorder {
	ctx := testutil.WithAuthenticatedContext(req.Context(), claims, permissions)
	req = req.WithContext(ctx)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// =============================================================================
// getSchulhofStatus Handler Tests
// =============================================================================

func TestGetSchulhofStatus_Success(t *testing.T) {
	t.Parallel()

	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
			roomID := int64(100)
			activityGroupID := int64(200)
			activeGroupID := int64(300)
			supervisionID := int64(400)

			return &supervisiondashboard.SchulhofStatus{
				Exists:            true,
				RoomID:            &roomID,
				RoomName:          "Schulhof",
				ActivityGroupID:   &activityGroupID,
				ActiveGroupID:     &activeGroupID,
				IsUserSupervising: true,
				SupervisionID:     &supervisionID,
				SupervisorCount:   2,
				StudentCount:      15,
				Supervisors: []supervisiondashboard.Supervisor{
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
		getCurrentStaffFunc: func(ctx context.Context) (*StaffIdentity, error) {
			return &StaffIdentity{}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	assert.Equal(t, testutil.StatusOK, rr.Code, "Expected 200 OK. Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"exists":true`)
	assert.Contains(t, rr.Body.String(), `"room_name":"Schulhof"`)
	assert.Contains(t, rr.Body.String(), `"is_user_supervising":true`)
	assert.Contains(t, rr.Body.String(), `"supervisor_count":2`)
	assert.Contains(t, rr.Body.String(), `"student_count":15`)
	assert.Contains(t, rr.Body.String(), `"John Doe"`)
	assert.Contains(t, rr.Body.String(), `"Jane Smith"`)
}

func TestGetSchulhofStatus_SchulhofDoesNotExist(t *testing.T) {
	t.Parallel()

	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
			return &supervisiondashboard.SchulhofStatus{
				Exists:            false,
				RoomName:          "",
				IsUserSupervising: false,
				SupervisorCount:   0,
				StudentCount:      0,
				Supervisors:       []supervisiondashboard.Supervisor{},
			}, nil
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*StaffIdentity, error) {
			return &StaffIdentity{}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	assert.Equal(t, testutil.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"exists":false`)
	assert.Contains(t, rr.Body.String(), `"supervisor_count":0`)
}

func TestGetSchulhofStatus_UserNotStaff(t *testing.T) {
	t.Parallel()

	mockSchulhof := &mockSchulhofService{}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*StaffIdentity, error) {
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
	t.Parallel()

	mockSchulhof := &mockSchulhofService{
		getStatusFunc: func(ctx context.Context, staffID int64) (*supervisiondashboard.SchulhofStatus, error) {
			return nil, errors.New("database connection failed")
		},
	}

	mockUserContext := &mockUserContextService{
		getCurrentStaffFunc: func(ctx context.Context) (*StaffIdentity, error) {
			return &StaffIdentity{}, nil
		},
	}

	resource := NewSchulhofResource(mockSchulhof, mockUserContext)
	router := setupSchulhofTestRouter(resource)
	req := testutil.NewRequest("GET", "/status", nil)

	rr := executeSchulhofRequest(router, req, testutil.AdminTestClaims(1), []string{"schulhof:read"})

	testutil.AssertErrorResponse(t, rr, testutil.StatusInternalServerError)
	assert.Contains(t, rr.Body.String(), "failed to get Schulhof status")
}
