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
	"github.com/moto-nrw/project-phoenix/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestRoomLifecycle tests the complete lifecycle of a room
func TestRoomLifecycle(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	// 2. Define test room
	newRoom := &models.Room{
		RoomName: "Integration Test Room",
		Building: "Test Building",
		Floor:    2,
		Capacity: 40,
		Category: "Test",
		Color:    "#FF5500",
	}

	// Room after creation
	createdRoom := &models.Room{
		ID:         1,
		RoomName:   "Integration Test Room",
		Building:   "Test Building",
		Floor:      2,
		Capacity:   40,
		Category:   "Test",
		Color:      "#FF5500",
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Room after update
	updatedRoom := &models.Room{
		ID:         1,
		RoomName:   "Updated Test Room", // Changed
		Building:   "Test Building",
		Floor:      3,              // Changed
		Capacity:   45,             // Changed
		Category:   "Test Updated", // Changed
		Color:      "#0055FF",      // Changed
		CreatedAt:  now,
		ModifiedAt: now.Add(time.Minute),
	}

	// Timespan for occupancy
	timespan := &models.Timespan{
		ID:        1,
		StartTime: now,
		EndTime:   nil,
		CreatedAt: now,
	}

	// Group ID for registration
	groupID := int64(1)

	// Room occupancy
	roomOccupancy := &models.RoomOccupancy{
		ID:         1,
		DeviceID:   "TEST-DEVICE-001",
		RoomID:     1,
		GroupID:    &groupID,
		TimespanID: 1,
		Timespan:   timespan,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Combined group after merging
	combinedGroup := &models.CombinedGroup{
		ID:           1,
		Name:         "Test Combined Group",
		IsActive:     true,
		AccessPolicy: "all",
		CreatedAt:    now,
	}

	// Room history
	roomHistory := []models.RoomHistory{
		{
			ID:             1,
			RoomID:         1,
			AgName:         "Test Activity",
			Day:            now,
			TimespanID:     1,
			SupervisorID:   1,
			MaxParticipant: 10,
			CreatedAt:      now,
		},
	}

	// 3. Set up expectations for creation
	mockStore.On("CreateRoom", mock.Anything, newRoom).Return(nil)
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(createdRoom, nil)

	// 4. Set up expectations for update
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(createdRoom, nil).Once()
	mockStore.On("UpdateRoom", mock.Anything, mock.MatchedBy(func(r *models.Room) bool {
		return r.ID == 1 && r.RoomName == "Updated Test Room"
	})).Return(nil)
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()

	// 5. Set up expectations for tablet registration
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()
	mockStore.On("RegisterTablet", mock.Anything, int64(1), "TEST-DEVICE-001", (*int64)(nil), &groupID).Return(roomOccupancy, nil)

	// 6. Set up expectations for tablet unregistration
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()
	mockStore.On("UnregisterTablet", mock.Anything, int64(1), "TEST-DEVICE-001").Return(nil)

	// 7. Set up expectations for room merging
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()
	mockStore.On("GetRoomByID", mock.Anything, int64(2)).Return(&models.Room{ID: 2, RoomName: "Second Test Room"}, nil)
	mockStore.On("MergeRooms", mock.Anything, int64(1), int64(2), "Test Merged Rooms", mock.Anything, "all").Return(combinedGroup, nil)

	// 8. Set up expectations for room history
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()
	mockStore.On("GetRoomHistoryByRoom", mock.Anything, int64(1)).Return(roomHistory, nil)

	// 9. Set up expectations for room deletion
	mockStore.On("GetRoomByID", mock.Anything, int64(1)).Return(updatedRoom, nil).Once()
	mockStore.On("DeleteRoom", mock.Anything, int64(1)).Return(nil)

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Room
	t.Run("1. Create Room", func(t *testing.T) {
		// Create test request
		roomReq := &RoomRequest{Room: newRoom}
		body, _ := json.Marshal(roomReq)
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createRoom(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseRoom models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRoom)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseRoom.ID)
		assert.Equal(t, "Integration Test Room", responseRoom.RoomName)
	})

	// PHASE 2: Update Room
	t.Run("2. Update Room", func(t *testing.T) {
		// Create updated room request
		updateData := &models.Room{
			ID:       1,
			RoomName: "Updated Test Room",
			Building: "Test Building",
			Floor:    3,
			Capacity: 45,
			Category: "Test Updated",
			Color:    "#0055FF",
		}

		roomReq := &RoomRequest{Room: updateData}
		body, _ := json.Marshal(roomReq)
		r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateRoom(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseRoom models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRoom)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Test Room", responseRoom.RoomName)
		assert.Equal(t, 3, responseRoom.Floor)
		assert.Equal(t, 45, responseRoom.Capacity)
		assert.Equal(t, "Test Updated", responseRoom.Category)
		assert.Equal(t, "#0055FF", responseRoom.Color)
	})

	// PHASE 3: Register Tablet
	t.Run("3. Register Tablet", func(t *testing.T) {
		// Create tablet registration request
		regReq := &RegisterTabletRequest{
			DeviceID: "TEST-DEVICE-001",
			GroupID:  &groupID,
		}

		body, _ := json.Marshal(regReq)
		r := httptest.NewRequest("POST", "/1/register", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.registerTablet(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseOccupancy models.RoomOccupancy
		err := json.Unmarshal(w.Body.Bytes(), &responseOccupancy)
		assert.NoError(t, err)
		assert.Equal(t, "TEST-DEVICE-001", responseOccupancy.DeviceID)
		assert.Equal(t, int64(1), responseOccupancy.RoomID)
		assert.Equal(t, &groupID, responseOccupancy.GroupID)
	})

	// PHASE 4: Unregister Tablet
	t.Run("4. Unregister Tablet", func(t *testing.T) {
		// Create tablet unregistration request
		unregReq := &UnregisterTabletRequest{
			DeviceID: "TEST-DEVICE-001",
		}

		body, _ := json.Marshal(unregReq)
		r := httptest.NewRequest("POST", "/1/unregister", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

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
	})

	// PHASE 5: Merge Rooms
	t.Run("5. Merge Rooms", func(t *testing.T) {
		// Create merge request
		validUntil := now.Add(7 * 24 * time.Hour) // 1 week later
		mergeReq := &MergeRoomsRequest{
			SourceRoomID: 1,
			TargetRoomID: 2,
			Name:         "Test Merged Rooms",
			ValidUntil:   &validUntil,
			AccessPolicy: "all",
		}

		body, _ := json.Marshal(mergeReq)
		r := httptest.NewRequest("POST", "/combined-groups/merge", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

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
	})

	// PHASE 6: Get Room History
	t.Run("6. Get Room History", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1/history", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getRoomHistory(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var history []models.RoomHistory
		err := json.Unmarshal(w.Body.Bytes(), &history)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(history))
		assert.Equal(t, int64(1), history[0].ID)
		assert.Equal(t, "Test Activity", history[0].AgName)
		assert.Equal(t, 10, history[0].MaxParticipant)
	})

	// PHASE 7: Delete Room
	t.Run("7. Delete Room", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteRoom(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockStore.AssertExpectations(t)
}

// TestRoomFilteringOperations tests the room filtering operations
func TestRoomFilteringOperations(t *testing.T) {
	rs, mockStore := setupTestAPI()

	// Setup test rooms
	classroomRooms := []models.Room{
		{ID: 1, RoomName: "Room 101", Building: "Main Building", Floor: 1, Category: "Classroom"},
		{ID: 2, RoomName: "Room 102", Building: "Main Building", Floor: 1, Category: "Classroom"},
	}

	labRooms := []models.Room{
		{ID: 3, RoomName: "Lab 1", Building: "Science Wing", Floor: 2, Category: "Laboratory"},
		{ID: 4, RoomName: "Lab 2", Building: "Science Wing", Floor: 2, Category: "Laboratory"},
	}

	mainBuildingRooms := []models.Room{
		{ID: 1, RoomName: "Room 101", Building: "Main Building", Floor: 1, Category: "Classroom"},
		{ID: 2, RoomName: "Room 102", Building: "Main Building", Floor: 1, Category: "Classroom"},
		{ID: 5, RoomName: "Room 201", Building: "Main Building", Floor: 2, Category: "Classroom"},
	}

	floor2Rooms := []models.Room{
		{ID: 3, RoomName: "Lab 1", Building: "Science Wing", Floor: 2, Category: "Laboratory"},
		{ID: 4, RoomName: "Lab 2", Building: "Science Wing", Floor: 2, Category: "Laboratory"},
		{ID: 5, RoomName: "Room 201", Building: "Main Building", Floor: 2, Category: "Classroom"},
	}

	occupiedRooms := []models.Room{
		{ID: 1, RoomName: "Room 101", Building: "Main Building", Floor: 1, Category: "Classroom"},
		{ID: 3, RoomName: "Lab 1", Building: "Science Wing", Floor: 2, Category: "Laboratory"},
	}

	groupedRooms := map[string][]models.Room{
		"Classroom":  classroomRooms,
		"Laboratory": labRooms,
	}

	// Set up expectations
	mockStore.On("GetRoomsByCategory", mock.Anything, "Classroom").Return(classroomRooms, nil)
	mockStore.On("GetRoomsByBuilding", mock.Anything, "Main Building").Return(mainBuildingRooms, nil)
	mockStore.On("GetRoomsByFloor", mock.Anything, 2).Return(floor2Rooms, nil)
	mockStore.On("GetRoomsByOccupied", mock.Anything, true).Return(occupiedRooms, nil)
	mockStore.On("GetRoomsGroupedByCategory", mock.Anything).Return(groupedRooms, nil)

	standardContext := context.Background()

	// Test getting rooms by category
	t.Run("Get Rooms By Category", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/by-category", nil)
		r.URL.RawQuery = "category=Classroom"
		r = r.WithContext(standardContext)
		w := httptest.NewRecorder()

		rs.getRoomsByCategory(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseRooms []models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseRooms))
		assert.Equal(t, "Classroom", responseRooms[0].Category)
	})

	// Test getting rooms by building
	t.Run("Get Rooms By Building", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/by-building", nil)
		r.URL.RawQuery = "building=Main Building"
		r = r.WithContext(standardContext)
		w := httptest.NewRecorder()

		rs.getRoomsByBuilding(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseRooms []models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(responseRooms))
		assert.Equal(t, "Main Building", responseRooms[0].Building)
	})

	// Test getting rooms by floor
	t.Run("Get Rooms By Floor", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/by-floor", nil)
		r.URL.RawQuery = "floor=2"
		r = r.WithContext(standardContext)
		w := httptest.NewRecorder()

		rs.getRoomsByFloor(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseRooms []models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(responseRooms))
		assert.Equal(t, 2, responseRooms[0].Floor)
	})

	// Test getting rooms by occupied status
	t.Run("Get Rooms By Occupied", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/by-occupied", nil)
		r.URL.RawQuery = "occupied=true"
		r = r.WithContext(standardContext)
		w := httptest.NewRecorder()

		rs.getRoomsByOccupied(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var responseRooms []models.Room
		err := json.Unmarshal(w.Body.Bytes(), &responseRooms)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseRooms))
	})

	// Test getting rooms grouped by category
	t.Run("Get Rooms Grouped By Category", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/grouped", nil)
		r = r.WithContext(standardContext)
		w := httptest.NewRecorder()

		rs.getRoomsGroupedByCategory(w, r)

		assert.Equal(t, http.StatusOK, w.Code)

		var response []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(response))

		// Check that each category has the right rooms
		for _, category := range response {
			categoryName := category["category"].(string)
			rooms := category["rooms"].([]interface{})

			if categoryName == "Classroom" {
				assert.Equal(t, 2, len(rooms))
			} else if categoryName == "Laboratory" {
				assert.Equal(t, 2, len(rooms))
			}
		}
	})

	// Verify all expectations were met
	mockStore.AssertExpectations(t)
}

// TestRoomBusinessLogic tests the business logic functions
func TestRoomBusinessLogic(t *testing.T) {
	// Test room validation
	t.Run("ValidateRoom", func(t *testing.T) {
		// Valid room
		validRoom := &models.Room{
			RoomName: "Test Room",
			Building: "Test Building",
			Floor:    1,
			Capacity: 30,
			Category: "Classroom",
			Color:    "#FF5500",
		}
		err := ValidateRoom(validRoom)
		assert.NoError(t, err)

		// Invalid room - missing room name
		invalidRoom1 := &models.Room{
			RoomName: "",
			Building: "Test Building",
			Floor:    1,
			Capacity: 30,
			Category: "Classroom",
		}
		err = ValidateRoom(invalidRoom1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "room_name is required")

		// Invalid room - negative capacity
		invalidRoom2 := &models.Room{
			RoomName: "Test Room",
			Building: "Test Building",
			Floor:    1,
			Capacity: -10,
			Category: "Classroom",
		}
		err = ValidateRoom(invalidRoom2)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "capacity must be non-negative")
	})
}
