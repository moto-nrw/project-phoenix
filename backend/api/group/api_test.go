package group

import (
	"context"
	"encoding/json"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock GroupStore
type MockGroupStore struct {
	mock.Mock
}

func (m *MockGroupStore) GetGroupByID(ctx context.Context, id int64) (*models.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Group), args.Error(1)
}

func (m *MockGroupStore) CreateGroup(ctx context.Context, group *models.Group, supervisorIDs []int64) error {
	args := m.Called(ctx, group, supervisorIDs)
	group.ID = 1 // Simulate auto-increment
	group.CreatedAt = time.Now()
	group.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockGroupStore) UpdateGroup(ctx context.Context, group *models.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupStore) DeleteGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupStore) ListGroups(ctx context.Context, filters map[string]interface{}) ([]models.Group, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models.Group), args.Error(1)
}

func (m *MockGroupStore) UpdateGroupSupervisors(ctx context.Context, groupID int64, supervisorIDs []int64) error {
	args := m.Called(ctx, groupID, supervisorIDs)
	return args.Error(0)
}

func (m *MockGroupStore) CreateCombinedGroup(ctx context.Context, combinedGroup *models.CombinedGroup, groupIDs []int64, specialistIDs []int64) error {
	args := m.Called(ctx, combinedGroup, groupIDs, specialistIDs)
	combinedGroup.ID = 1 // Simulate auto-increment
	combinedGroup.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockGroupStore) GetCombinedGroupByID(ctx context.Context, id int64) (*models.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CombinedGroup), args.Error(1)
}

func (m *MockGroupStore) ListCombinedGroups(ctx context.Context) ([]models.CombinedGroup, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.CombinedGroup), args.Error(1)
}

func (m *MockGroupStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64) (*models.CombinedGroup, error) {
	args := m.Called(ctx, sourceRoomID, targetRoomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CombinedGroup), args.Error(1)
}

// Mock AuthTokenStore
type MockAuthTokenStore struct {
	mock.Mock
}

func (m *MockAuthTokenStore) GetToken(t string) (*jwt.Token, error) {
	args := m.Called(t)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.Token), args.Error(1)
}

func setupTestAPI() (*Resource, *MockGroupStore, *MockAuthTokenStore) {
	mockGroupStore := new(MockGroupStore)
	mockAuthStore := new(MockAuthTokenStore)
	resource := NewResource(mockGroupStore, mockAuthStore)
	return resource, mockGroupStore, mockAuthStore
}

func TestListGroups(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	testGroups := []models.Group{
		{
			ID:   1,
			Name: "Group 1",
		},
		{
			ID:   2,
			Name: "Group 2",
		},
	}

	mockGroupStore.On("ListGroups", mock.Anything, mock.Anything).Return(testGroups, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listGroups(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseGroups []models.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroups)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseGroups))
	assert.Equal(t, "Group 1", responseGroups[0].Name)
	assert.Equal(t, "Group 2", responseGroups[1].Name)

	mockGroupStore.AssertExpectations(t)
}

func TestGetGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	testGroup := &models.Group{
		ID:   1,
		Name: "Test Group",
	}

	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(testGroup, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseGroup models.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.Equal(t, "Test Group", responseGroup.Name)

	mockGroupStore.AssertExpectations(t)
}

func TestRouter(t *testing.T) {
	rs, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
