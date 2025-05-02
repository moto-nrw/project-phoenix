package room

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	models2 "github.com/moto-nrw/project-phoenix/models"
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
	room.CreatedAt = time.Now()
	room.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockRoomStore) UpdateRoom(ctx context.Context, room *models2.Room) error {
	args := m.Called(ctx, room)
	room.ModifiedAt = time.Now()
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

func (m *MockRoomStore) GetCurrentRoomOccupancy(ctx context.Context, roomID int64) (*models2.RoomOccupancy, error) {
	args := m.Called(ctx, roomID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.RoomOccupancy), args.Error(1)
}

func (m *MockRoomStore) RegisterTablet(ctx context.Context, roomID int64, deviceID string, agID *int64, groupID *int64) (*models2.RoomOccupancy, error) {
	args := m.Called(ctx, roomID, deviceID, agID, groupID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.RoomOccupancy), args.Error(1)
}

func (m *MockRoomStore) UnregisterTablet(ctx context.Context, roomID int64, deviceID string) error {
	args := m.Called(ctx, roomID, deviceID)
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

// setupTestAPI creates a new Resource with a mock store for testing
func setupTestAPI() (*Resource, *MockRoomStore) {
	mockStore := new(MockRoomStore)
	resource := NewResource(mockStore)
	return resource, mockStore
}

func TestListRooms(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	testRooms := []models2.Room{
		{
			ID:       1,
			RoomName: "Room 101",
			Building: "Main Building",
			Floor:    1,
			Capacity: 30,
			Category: "Classroom",
		},
		{
			ID:       2,
			RoomName: "Room 102",
			Building: "Main Building",
			Floor:    1,
			Capacity: 25,
			Category: "Classroom",
		},
	}

	mockStore.On("GetRooms", mock.Anything).Return(testRooms, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listRooms(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseRooms []models2.Room
	err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseRooms))
	assert.Equal(t, "Room 101", responseRooms[0].RoomName)
	assert.Equal(t, "Room 102", responseRooms[1].RoomName)

	mockStore.AssertExpectations(t)
}

func TestGetRoom(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	testRoom := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(testRoom, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getRoom(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseRoom models2.Room
	err := json.Unmarshal(w.Body.Bytes(), &responseRoom)
	assert.NoError(t, err)
	assert.Equal(t, "Room 101", responseRoom.RoomName)
	assert.Equal(t, int64(1), responseRoom.ID)

	mockStore.AssertExpectations(t)
}

func TestCreateRoom(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	newRoom := &models2.Room{
		RoomName: "Room 103",
		Building: "Main Building",
		Floor:    1,
		Capacity: 35,
		Category: "Classroom",
	}

	mockStore.On("CreateRoom", mock.Anything, newRoom).Return(nil)

	// Create test request
	roomReq := &RoomRequest{Room: newRoom}
	body, _ := json.Marshal(roomReq)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createRoom(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseRoom models2.Room
	err := json.Unmarshal(w.Body.Bytes(), &responseRoom)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseRoom.ID)
	assert.Equal(t, "Room 103", responseRoom.RoomName)

	mockStore.AssertExpectations(t)
}

func TestUpdateRoom(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	existingRoom := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	updatedRoom := &models2.Room{
		ID:       1,
		RoomName: "Room 101 Updated",
		Building: "Main Building",
		Floor:    1,
		Capacity: 35,
		Category: "Classroom",
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(existingRoom, nil)
	mockStore.On("UpdateRoom", mock.Anything, mock.MatchedBy(func(r *models2.Room) bool {
		return r.ID == 1 && r.RoomName == "Room 101 Updated" && r.Capacity == 35
	})).Return(nil)

	// Create test request
	roomReq := &RoomRequest{Room: updatedRoom}
	body, _ := json.Marshal(roomReq)
	r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.updateRoom(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseRoom models2.Room
	err := json.Unmarshal(w.Body.Bytes(), &responseRoom)
	assert.NoError(t, err)
	assert.Equal(t, "Room 101 Updated", responseRoom.RoomName)
	assert.Equal(t, 35, responseRoom.Capacity)

	mockStore.AssertExpectations(t)
}

func TestDeleteRoom(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	existingRoom := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(existingRoom, nil)
	mockStore.On("DeleteRoom", mock.Anything, int64(1)).Return(nil)

	// Create test request
	r := httptest.NewRequest("DELETE", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.deleteRoom(w, r)

	// Check response
	assert.Equal(t, http.StatusNoContent, w.Code)

	mockStore.AssertExpectations(t)
}

func TestGetRoomsByCategory(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	testRooms := []models2.Room{
		{
			ID:       1,
			RoomName: "Room 101",
			Building: "Main Building",
			Floor:    1,
			Capacity: 30,
			Category: "Classroom",
		},
		{
			ID:       2,
			RoomName: "Room 102",
			Building: "Main Building",
			Floor:    1,
			Capacity: 25,
			Category: "Classroom",
		},
	}

	mockStore.On("GetRoomsByCategory", mock.Anything, "Classroom").Return(testRooms, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/by-category?category=Classroom", nil)
	w := httptest.NewRecorder()

	// Add query parameter
	r.URL.RawQuery = "category=Classroom"

	// Call the handler directly
	rs.getRoomsByCategory(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseRooms []models2.Room
	err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseRooms))
	assert.Equal(t, "Classroom", responseRooms[0].Category)
	assert.Equal(t, "Classroom", responseRooms[1].Category)

	mockStore.AssertExpectations(t)
}

func TestMergeRooms(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	sourceRoom := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	targetRoom := &models2.Room{
		ID:       2,
		RoomName: "Room 102",
		Building: "Main Building",
		Floor:    1,
		Capacity: 25,
		Category: "Classroom",
	}

	combinedGroup := &models2.CombinedGroup{
		ID:           1,
		Name:         "Room 101 + Room 102",
		IsActive:     true,
		AccessPolicy: "all",
		CreatedAt:    time.Now(),
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(sourceRoom, nil)
	mockStore.On("GetRoomByID", mock.Anything, int64(2)).Return(targetRoom, nil)
	mockStore.On("MergeRooms", mock.Anything, int64(1), int64(2), "", (*time.Time)(nil), "all").Return(combinedGroup, nil)

	// Create test request
	mergeReq := &MergeRoomsRequest{
		SourceRoomID: 1,
		TargetRoomID: 2,
		AccessPolicy: "all",
	}
	body, _ := json.Marshal(mergeReq)
	r := httptest.NewRequest("POST", "/combined-groups/merge", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.mergeRooms(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Rooms merged successfully", response["message"])
	assert.NotNil(t, response["combined_group"])

	mockStore.AssertExpectations(t)
}

func TestRegisterTablet(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	timespan := &models2.Timespan{
		ID:        1,
		StartTime: time.Now(),
	}

	groupID := int64(2)
	occupancy := &models2.RoomOccupancy{
		ID:         1,
		DeviceID:   "device-123",
		RoomID:     1,
		GroupID:    &groupID,
		TimespanID: 1,
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
		Timespan:   timespan,
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(room, nil)
	mockStore.On("RegisterTablet", mock.Anything, int64(1), "device-123", (*int64)(nil), &groupID).Return(occupancy, nil)

	// Create test request
	registerReq := &RegisterTabletRequest{
		DeviceID: "device-123",
		GroupID:  &groupID,
	}
	body, _ := json.Marshal(registerReq)
	r := httptest.NewRequest("POST", "/1/register", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.registerTablet(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseOccupancy models2.RoomOccupancy
	err := json.Unmarshal(w.Body.Bytes(), &responseOccupancy)
	assert.NoError(t, err)
	assert.Equal(t, "device-123", responseOccupancy.DeviceID)
	assert.Equal(t, int64(1), responseOccupancy.RoomID)

	mockStore.AssertExpectations(t)
}

func TestUnregisterTablet(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(room, nil)
	mockStore.On("UnregisterTablet", mock.Anything, int64(1), "device-123").Return(nil)

	// Create test request
	unregisterReq := &UnregisterTabletRequest{
		DeviceID: "device-123",
	}
	body, _ := json.Marshal(unregisterReq)
	r := httptest.NewRequest("POST", "/1/unregister", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.unregisterTablet(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Tablet unregistered successfully", response["message"])

	mockStore.AssertExpectations(t)
}

func TestGetActiveCombinedGroups(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	combinedGroups := []models2.CombinedGroup{
		{
			ID:           1,
			Name:         "Combined Group 1",
			IsActive:     true,
			AccessPolicy: "all",
			CreatedAt:    time.Now(),
		},
		{
			ID:           2,
			Name:         "Combined Group 2",
			IsActive:     true,
			AccessPolicy: "all",
			CreatedAt:    time.Now(),
		},
	}

	mockStore.On("FindActiveCombinedGroups", mock.Anything).Return(combinedGroups, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/combined-groups", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getActiveCombinedGroups(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseCombinedGroups []models2.CombinedGroup
	err := json.Unmarshal(w.Body.Bytes(), &responseCombinedGroups)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseCombinedGroups))
	assert.Equal(t, "Combined Group 1", responseCombinedGroups[0].Name)
	assert.Equal(t, "Combined Group 2", responseCombinedGroups[1].Name)

	mockStore.AssertExpectations(t)
}

func TestDeactivateCombinedGroup(t *testing.T) {
	rs, mockStore := setupTestAPI()

	mockStore.On("DeactivateCombinedGroup", mock.Anything, int64(1)).Return(nil)

	// Create test request
	r := httptest.NewRequest("DELETE", "/combined-groups/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.deactivateCombinedGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, true, response["success"])
	assert.Equal(t, "Combined group deactivated successfully", response["message"])

	mockStore.AssertExpectations(t)
}

func TestGetRoomHistory(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	room := &models2.Room{
		ID:       1,
		RoomName: "Room 101",
		Building: "Main Building",
		Floor:    1,
		Capacity: 30,
		Category: "Classroom",
	}

	history := []models2.RoomHistory{
		{
			ID:           1,
			RoomID:       1,
			AgName:       "Activity Group 1",
			Day:          time.Now(),
			TimespanID:   1,
			SupervisorID: 1,
			CreatedAt:    time.Now(),
		},
		{
			ID:           2,
			RoomID:       1,
			AgName:       "Activity Group 2",
			Day:          time.Now().Add(-24 * time.Hour),
			TimespanID:   2,
			SupervisorID: 2,
			CreatedAt:    time.Now().Add(-24 * time.Hour),
		},
	}

	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(room, nil)
	mockStore.On("GetRoomHistoryByRoom", mock.Anything, int64(1)).Return(history, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/1/history", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getRoomHistory(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseHistory []models2.RoomHistory
	err := json.Unmarshal(w.Body.Bytes(), &responseHistory)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseHistory))
	assert.Equal(t, "Activity Group 1", responseHistory[0].AgName)
	assert.Equal(t, "Activity Group 2", responseHistory[1].AgName)

	mockStore.AssertExpectations(t)
}

func TestGetRoomHistoryByDateRange(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test data
	startDate, _ := time.Parse("2006-01-02", "2023-01-01")
	endDate, _ := time.Parse("2006-01-02", "2023-01-31")
	endDatePlusOneDay := endDate.Add(24 * time.Hour)

	history := []models2.RoomHistory{
		{
			ID:           1,
			RoomID:       1,
			AgName:       "Activity Group 1",
			Day:          startDate.Add(24 * time.Hour),
			TimespanID:   1,
			SupervisorID: 1,
			CreatedAt:    startDate.Add(24 * time.Hour),
		},
		{
			ID:           2,
			RoomID:       2,
			AgName:       "Activity Group 2",
			Day:          startDate.Add(48 * time.Hour),
			TimespanID:   2,
			SupervisorID: 2,
			CreatedAt:    startDate.Add(48 * time.Hour),
		},
	}

	mockStore.On("GetRoomHistoryByDateRange", mock.Anything, startDate, endDatePlusOneDay).Return(history, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/history/by-date-range?start_date=2023-01-01&end_date=2023-01-31", nil)
	r.URL.RawQuery = "start_date=2023-01-01&end_date=2023-01-31"
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getRoomHistoryByDateRange(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseHistory []models2.RoomHistory
	err := json.Unmarshal(w.Body.Bytes(), &responseHistory)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseHistory))

	mockStore.AssertExpectations(t)
}

func TestRouter(t *testing.T) {
	rs, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
