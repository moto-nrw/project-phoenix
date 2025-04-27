package room

import (
	"bytes"
	"context"
	"encoding/json"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockRoomStore is a mock implementation of RoomStore
type MockRoomStore struct {
	mock.Mock
}

// Implement all RoomStore interface methods
func (m *MockRoomStore) GetRooms(ctx context.Context) ([]models2.Room, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetRoomsByCategory(ctx context.Context, category string) ([]models2.Room, error) {
	args := m.Called(ctx, category)
	return args.Get(0).([]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetRoomsByBuilding(ctx context.Context, building string) ([]models2.Room, error) {
	args := m.Called(ctx, building)
	return args.Get(0).([]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetRoomsByFloor(ctx context.Context, floor int) ([]models2.Room, error) {
	args := m.Called(ctx, floor)
	return args.Get(0).([]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetRoomsByOccupied(ctx context.Context, occupied bool) ([]models2.Room, error) {
	args := m.Called(ctx, occupied)
	return args.Get(0).([]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetRoomByID(ctx context.Context, id int64) (*models2.Room, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Room), args.Error(1)
}

func (m *MockRoomStore) CreateRoom(ctx context.Context, room *models2.Room) error {
	args := m.Called(ctx, room)
	room.ID = 1 // Mock ID assignment
	return args.Error(0)
}

func (m *MockRoomStore) UpdateRoom(ctx context.Context, room *models2.Room) error {
	args := m.Called(ctx, room)
	return args.Error(0)
}

func (m *MockRoomStore) DeleteRoom(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoomStore) GetRoomsGroupedByCategory(ctx context.Context) (map[string][]models2.Room, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string][]models2.Room), args.Error(1)
}

func (m *MockRoomStore) GetAllRoomOccupancies(ctx context.Context) ([]RoomOccupancyDetail, error) {
	args := m.Called(ctx)
	return args.Get(0).([]RoomOccupancyDetail), args.Error(1)
}

func (m *MockRoomStore) GetRoomOccupancyByID(ctx context.Context, id int64) (*RoomOccupancyDetail, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RoomOccupancyDetail), args.Error(1)
}

func (m *MockRoomStore) GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*RoomOccupancyDetail, error) {
	args := m.Called(ctx, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RoomOccupancyDetail), args.Error(1)
}

func (m *MockRoomStore) RegisterTablet(ctx context.Context, roomID int64, req *RegisterTabletRequest) (*models2.RoomOccupancy, error) {
	args := m.Called(ctx, roomID, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.RoomOccupancy), args.Error(1)
}

func (m *MockRoomStore) UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error {
	args := m.Called(ctx, roomID, deviceID)
	return args.Error(0)
}

func (m *MockRoomStore) AddSupervisorToRoomOccupancy(ctx context.Context, roomOccupancyID, supervisorID int64) error {
	args := m.Called(ctx, roomOccupancyID, supervisorID)
	return args.Error(0)
}

func (m *MockRoomStore) MergeRooms(ctx context.Context, sourceRoomID, targetRoomID int64, name string, validUntil *time.Time, accessPolicy string) (*models2.CombinedGroup, error) {
	args := m.Called(ctx, sourceRoomID, targetRoomID, name, validUntil, accessPolicy)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.CombinedGroup), args.Error(1)
}

func (m *MockRoomStore) GetCombinedGroupForRoom(ctx context.Context, roomID int64) (*models2.CombinedGroup, error) {
	args := m.Called(ctx, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.CombinedGroup), args.Error(1)
}

func (m *MockRoomStore) FindActiveCombinedGroups(ctx context.Context) ([]models2.CombinedGroup, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.CombinedGroup), args.Error(1)
}

func (m *MockRoomStore) DeactivateCombinedGroup(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockRoomStore) GetRoomHistoryByRoom(ctx context.Context, roomID int64) ([]models2.RoomHistory, error) {
	args := m.Called(ctx, roomID)
	return args.Get(0).([]models2.RoomHistory), args.Error(1)
}

func (m *MockRoomStore) GetRoomHistoryByDateRange(ctx context.Context, startDate, endDate time.Time) ([]models2.RoomHistory, error) {
	args := m.Called(ctx, startDate, endDate)
	return args.Get(0).([]models2.RoomHistory), args.Error(1)
}

func (m *MockRoomStore) GetRoomHistoryBySupervisor(ctx context.Context, supervisorID int64) ([]models2.RoomHistory, error) {
	args := m.Called(ctx, supervisorID)
	return args.Get(0).([]models2.RoomHistory), args.Error(1)
}

// setupAPI creates a new API with a mock store
func setupAPI(t *testing.T) (*API, *MockRoomStore) {
	mockStore := new(MockRoomStore)
	api := &API{store: mockStore}
	return api, mockStore
}

// TestMergeRooms tests the room merging functionality
func TestMergeRooms(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Create test data
	sourceRoom := &models2.Room{ID: 1, RoomName: "Room A"}
	targetRoom := &models2.Room{ID: 2, RoomName: "Room B"}
	combinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Room A + Room B",
		IsActive:     true,
		AccessPolicy: "all",
		CreatedAt:    time.Now(),
	}

	// Configure mock store behaviors
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(sourceRoom, nil)
	mockStore.On("GetRoomByID", mock.Anything, int64(2)).Return(targetRoom, nil)
	mockStore.On("MergeRooms", mock.Anything, int64(1), int64(2), "", (*time.Time)(nil), "").Return(combinedGroup, nil)

	// Create request payload
	mergeRequest := MergeRoomsRequest{
		SourceRoomID: 1,
		TargetRoomID: 2,
	}
	requestBody, _ := json.Marshal(mergeRequest)

	// Create HTTP request
	r := httptest.NewRequest("POST", "/combined_groups/merge", bytes.NewBuffer(requestBody))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Set up chi router with the merge endpoint
	router := chi.NewRouter()
	router.Post("/combined_groups/merge", api.handleMergeRooms)
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Rooms merged successfully", response["message"])
	assert.NotNil(t, response["combined_group"])

	// Verify method calls
	mockStore.AssertExpectations(t)
}

// TestGetCombinedGroupForRoom tests getting a combined group for a room
func TestGetCombinedGroupForRoom(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Create test data
	room := &models2.Room{ID: 1, RoomName: "Room A"}
	combinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		CreatedAt:    time.Now(),
	}

	// Configure mock store behaviors
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(room, nil)
	mockStore.On("GetCombinedGroupForRoom", mock.Anything, int64(1)).Return(combinedGroup, nil)

	// Create HTTP request
	r := httptest.NewRequest("GET", "/1/combined_group", nil)
	w := httptest.NewRecorder()

	// Set up chi router with the endpoint
	router := chi.NewRouter()
	router.Get("/{id}/combined_group", api.handleGetCombinedGroupForRoom)
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response models2.CombinedGroup
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Combined Group", response.Name)

	// Verify method calls
	mockStore.AssertExpectations(t)
}

// TestGetActiveCombinedGroups tests getting all active combined groups
func TestGetActiveCombinedGroups(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Create test data
	combinedGroups := []models2.CombinedGroup{
		{ID: 1, Name: "Group A", IsActive: true},
		{ID: 2, Name: "Group B", IsActive: true},
	}

	// Configure mock store behaviors
	mockStore.On("FindActiveCombinedGroups", mock.Anything).Return(combinedGroups, nil)

	// Create HTTP request
	r := httptest.NewRequest("GET", "/combined_groups", nil)
	w := httptest.NewRecorder()

	// Set up chi router with the endpoint
	router := chi.NewRouter()
	router.Get("/combined_groups", api.handleGetActiveCombinedGroups)
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response []models2.CombinedGroup
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Len(t, response, 2)

	// Verify method calls
	mockStore.AssertExpectations(t)
}

// TestDeactivateCombinedGroup tests deactivating a combined group
func TestDeactivateCombinedGroup(t *testing.T) {
	api, mockStore := setupAPI(t)

	// Configure mock store behaviors
	mockStore.On("DeactivateCombinedGroup", mock.Anything, int64(1)).Return(nil)

	// Create HTTP request
	r := httptest.NewRequest("DELETE", "/combined_groups/1", nil)
	w := httptest.NewRecorder()

	// Set up chi router with the endpoint
	router := chi.NewRouter()
	router.Delete("/combined_groups/{id}", api.handleDeactivateCombinedGroup)
	router.ServeHTTP(w, r)

	// Assert response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Combined group deactivated successfully", response["message"])

	// Verify method calls
	mockStore.AssertExpectations(t)
}
