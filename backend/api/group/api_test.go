package group

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGroupStore implements the GroupStore interface for testing
type MockGroupStore struct {
	mock.Mock
}

func (m *MockGroupStore) GetGroupByID(ctx context.Context, id int64) (*models2.Group, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Group), args.Error(1)
}

func (m *MockGroupStore) CreateGroup(ctx context.Context, group *models2.Group, supervisorIDs []int64) error {
	args := m.Called(ctx, group, supervisorIDs)
	group.ID = 1 // Simulate auto-increment
	group.CreatedAt = time.Now()
	group.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockGroupStore) UpdateGroup(ctx context.Context, group *models2.Group) error {
	args := m.Called(ctx, group)
	return args.Error(0)
}

func (m *MockGroupStore) DeleteGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockGroupStore) ListGroups(ctx context.Context, filters map[string]interface{}) ([]models2.Group, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models2.Group), args.Error(1)
}

func (m *MockGroupStore) UpdateGroupSupervisors(ctx context.Context, groupID int64, supervisorIDs []int64) error {
	args := m.Called(ctx, groupID, supervisorIDs)
	return args.Error(0)
}

func (m *MockGroupStore) CreateCombinedGroup(ctx context.Context, combinedGroup *models2.CombinedGroup, groupIDs []int64, specialistIDs []int64) error {
	args := m.Called(ctx, combinedGroup, groupIDs, specialistIDs)
	combinedGroup.ID = 1 // Simulate auto-increment
	combinedGroup.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockGroupStore) GetCombinedGroupByID(ctx context.Context, id int64) (*models2.CombinedGroup, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.CombinedGroup), args.Error(1)
}

func (m *MockGroupStore) ListCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.CombinedGroup), args.Error(1)
}

func (m *MockGroupStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64) (*models2.CombinedGroup, error) {
	args := m.Called(ctx, sourceRoomID, targetRoomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.CombinedGroup), args.Error(1)
}

// MockAuthTokenStore implements the AuthTokenStore interface for testing
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

// setupTestAPI creates a test API with mocked dependencies
func setupTestAPI() (*Resource, *MockGroupStore, *MockAuthTokenStore) {
	mockGroupStore := new(MockGroupStore)
	mockAuthStore := new(MockAuthTokenStore)
	resource := NewResource(mockGroupStore, mockAuthStore)
	return resource, mockGroupStore, mockAuthStore
}

// TestListGroups tests the listGroups handler
func TestListGroups(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	testGroups := []models2.Group{
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

	var responseGroups []models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroups)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseGroups))
	assert.Equal(t, "Group 1", responseGroups[0].Name)
	assert.Equal(t, "Group 2", responseGroups[1].Name)

	mockGroupStore.AssertExpectations(t)
}

// TestGetGroup tests the getGroup handler
func TestGetGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       101,
		RoomName: "Room 101",
	}

	testGroup := &models2.Group{
		ID:       1,
		Name:     "Test Group",
		RoomID:   &room.ID,
		Room:     room,
		Students: []models2.Student{},
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

	var responseGroup models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.Equal(t, "Test Group", responseGroup.Name)
	assert.Equal(t, int64(101), *responseGroup.RoomID)

	mockGroupStore.AssertExpectations(t)
}

// TestCreateGroup tests the createGroup handler
func TestCreateGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       101,
		RoomName: "Room 101",
	}

	newGroup := &models2.Group{
		Name:   "New Group",
		RoomID: &room.ID,
	}

	createdGroup := &models2.Group{
		ID:         1,
		Name:       "New Group",
		RoomID:     &room.ID,
		Room:       room,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	mockGroupStore.On("CreateGroup", mock.Anything, mock.MatchedBy(func(g *models2.Group) bool {
		return g.Name == "New Group" && *g.RoomID == 101
	}), []int64{1, 2}).Return(nil)

	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(createdGroup, nil)

	// Create test request
	groupReq := &GroupRequest{
		Group:         newGroup,
		SupervisorIDs: []int64{1, 2},
	}
	body, _ := json.Marshal(groupReq)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseGroup models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseGroup.ID)
	assert.Equal(t, "New Group", responseGroup.Name)

	mockGroupStore.AssertExpectations(t)
}

// TestUpdateGroup tests the updateGroup handler
func TestUpdateGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       101,
		RoomName: "Room 101",
	}

	existingGroup := &models2.Group{
		ID:       1,
		Name:     "Existing Group",
		RoomID:   &room.ID,
		Room:     room,
		Students: []models2.Student{},
	}

	updatedGroup := &models2.Group{
		ID:       1,
		Name:     "Updated Group", // Changed name
		RoomID:   &room.ID,
		Room:     room,
		Students: []models2.Student{},
	}

	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(existingGroup, nil).Once()
	mockGroupStore.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *models2.Group) bool {
		return g.ID == 1 && g.Name == "Updated Group"
	})).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Once()

	// Create test request
	groupReq := &GroupRequest{
		Group: &models2.Group{
			ID:     1,
			Name:   "Updated Group",
			RoomID: &room.ID,
		},
	}
	body, _ := json.Marshal(groupReq)
	r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.updateGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseGroup models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.Equal(t, "Updated Group", responseGroup.Name)

	mockGroupStore.AssertExpectations(t)
}

// TestDeleteGroup tests the deleteGroup handler
func TestDeleteGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	mockGroupStore.On("DeleteGroup", mock.Anything, int64(1)).Return(nil)

	// Create test request
	r := httptest.NewRequest("DELETE", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.deleteGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusNoContent, w.Code)

	mockGroupStore.AssertExpectations(t)
}

// TestUpdateGroupSupervisors tests the updateGroupSupervisors handler
func TestUpdateGroupSupervisors(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	existingGroup := &models2.Group{
		ID:   1,
		Name: "Test Group",
		Supervisors: []*models2.PedagogicalSpecialist{
			{ID: 1},
		},
	}

	updatedGroup := &models2.Group{
		ID:   1,
		Name: "Test Group",
		Supervisors: []*models2.PedagogicalSpecialist{
			{ID: 2},
			{ID: 3},
		},
	}

	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(existingGroup, nil).Once()
	mockGroupStore.On("UpdateGroupSupervisors", mock.Anything, int64(1), []int64{2, 3}).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Once()

	// Create test request
	supervisorReq := &SupervisorRequest{
		SupervisorIDs: []int64{2, 3},
	}
	body, _ := json.Marshal(supervisorReq)
	r := httptest.NewRequest("POST", "/1/supervisors", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.updateGroupSupervisors(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseGroup models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseGroup.Supervisors))
	assert.Equal(t, int64(2), responseGroup.Supervisors[0].ID)
	assert.Equal(t, int64(3), responseGroup.Supervisors[1].ID)

	mockGroupStore.AssertExpectations(t)
}

// TestSetGroupRepresentative tests the setGroupRepresentative handler
func TestSetGroupRepresentative(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	specialistID := int64(101)

	existingGroup := &models2.Group{
		ID:   1,
		Name: "Test Group",
	}

	updatedGroup := &models2.Group{
		ID:               1,
		Name:             "Test Group",
		RepresentativeID: &specialistID,
	}

	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(existingGroup, nil).Once()
	mockGroupStore.On("UpdateGroup", mock.Anything, mock.MatchedBy(func(g *models2.Group) bool {
		return g.ID == 1 && g.RepresentativeID != nil && *g.RepresentativeID == 101
	})).Return(nil)
	mockGroupStore.On("GetGroupByID", mock.Anything, int64(1)).Return(updatedGroup, nil).Once()

	// Create test request
	repReq := &RepresentativeRequest{
		SpecialistID: 101,
	}
	body, _ := json.Marshal(repReq)
	r := httptest.NewRequest("POST", "/1/representative", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.setGroupRepresentative(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseGroup models2.Group
	err := json.Unmarshal(w.Body.Bytes(), &responseGroup)
	assert.NoError(t, err)
	assert.NotNil(t, responseGroup.RepresentativeID)
	assert.Equal(t, int64(101), *responseGroup.RepresentativeID)

	mockGroupStore.AssertExpectations(t)
}

// TestCreateCombinedGroup tests the createCombinedGroup handler
func TestCreateCombinedGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	newCombinedGroup := &models2.CombinedGroup{
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
	}

	createdCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "New Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		Groups: []*models2.Group{
			{ID: 1, Name: "Group 1"},
			{ID: 2, Name: "Group 2"},
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			{ID: 1},
		},
		CreatedAt: time.Now(),
	}

	mockGroupStore.On("CreateCombinedGroup", mock.Anything, mock.MatchedBy(func(cg *models2.CombinedGroup) bool {
		return cg.Name == "New Combined Group" && cg.AccessPolicy == "all"
	}), []int64{1, 2}, []int64{1}).Return(nil)

	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(createdCombinedGroup, nil)

	// Create test request
	combinedGroupReq := &CombinedGroupRequest{
		CombinedGroup: newCombinedGroup,
		GroupIDs:      []int64{1, 2},
		SpecialistIDs: []int64{1},
	}
	body, _ := json.Marshal(combinedGroupReq)
	r := httptest.NewRequest("POST", "/combined", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createCombinedGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseCombinedGroup models2.CombinedGroup
	err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseCombinedGroup.ID)
	assert.Equal(t, "New Combined Group", responseCombinedGroup.Name)
	assert.Equal(t, 2, len(responseCombinedGroup.Groups))
	assert.Equal(t, 1, len(responseCombinedGroup.AccessSpecialists))

	mockGroupStore.AssertExpectations(t)
}

// TestGetCombinedGroup tests the getCombinedGroup handler
func TestGetCombinedGroup(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	combinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Test Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		Groups: []*models2.Group{
			{ID: 1, Name: "Group 1"},
			{ID: 2, Name: "Group 2"},
		},
		AccessSpecialists: []*models2.PedagogicalSpecialist{
			{ID: 1},
		},
		CreatedAt: time.Now(),
	}

	mockGroupStore.On("GetCombinedGroupByID", mock.Anything, int64(1)).Return(combinedGroup, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/combined/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getCombinedGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseCombinedGroup models2.CombinedGroup
	err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroup)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseCombinedGroup.ID)
	assert.Equal(t, "Test Combined Group", responseCombinedGroup.Name)
	assert.Equal(t, 2, len(responseCombinedGroup.Groups))
	assert.Equal(t, 1, len(responseCombinedGroup.AccessSpecialists))

	mockGroupStore.AssertExpectations(t)
}

// TestMergeRooms tests the mergeRooms handler
func TestMergeRooms(t *testing.T) {
	rs, mockGroupStore, _ := setupTestAPI()

	// Setup test data
	mergedCombinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Room 101 + Room 102",
		IsActive:     true,
		AccessPolicy: "all",
		Groups: []*models2.Group{
			{ID: 1, Name: "Group 1"},
			{ID: 2, Name: "Group 2"},
		},
		CreatedAt: time.Now(),
	}

	mockGroupStore.On("MergeRooms", mock.Anything, int64(101), int64(102)).Return(mergedCombinedGroup, nil)

	// Create test request
	mergeReq := &MergeRoomsRequest{
		SourceRoomID: 101,
		TargetRoomID: 102,
	}
	body, _ := json.Marshal(mergeReq)
	r := httptest.NewRequest("POST", "/merge-rooms", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.mergeRooms(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.NotNil(t, response["combined_group"])

	mockGroupStore.AssertExpectations(t)
}

// TestRouter tests the router configuration
func TestRouter(t *testing.T) {
	rs, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
