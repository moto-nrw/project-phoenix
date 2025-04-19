package rfid

import (
	"bytes"
	"encoding/json"
	"errors"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestUnknownTagID tests handling of unknown RFID tags
func TestUnknownTagID(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Common test data
	now := time.Now()
	unknownTagID := "UNKNOWN_TAG"
	readerID := "ENTRANCE_READER"
	roomID := int64(101)

	// Setup expectations
	mockRFIDStore.On("SaveTag", mock.Anything, unknownTagID, readerID).Return(&Tag{
		ID:        1,
		TagID:     unknownTagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil)

	mockUserStore.On("GetCustomUserByTagID", mock.Anything, unknownTagID).Return(nil, errors.New("user not found"))

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	payload := map[string]interface{}{
		"tag_id":    unknownTagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Assert response shows failure due to unknown tag
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response OccupancyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "No user found with this tag ID")

	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
}

// TestDatabaseFailureWhenSavingTag tests handling of database failures
func TestDatabaseFailureWhenSavingTag(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Common test data
	now := time.Now()
	tagID := "STUDENT0001"
	readerID := "ENTRANCE_READER"
	roomID := int64(101)
	user := &models2.CustomUser{
		ID:         123,
		FirstName:  "Jane",
		SecondName: "Doe",
		TagID:      &tagID,
	}
	student := &models2.Student{
		ID:           456,
		SchoolClass:  "4A",
		CustomUserID: user.ID,
		InHouse:      false,
		WC:           false,
		SchoolYard:   false,
	}

	// Setup expectations for database failure
	mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(nil, errors.New("database connection failed"))

	// Continue with other operations despite tag saving failure
	mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil)
	mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil)

	// Mock timespan creation
	mockTimespan := &models2.Timespan{
		ID:        1,
		StartTime: now,
		EndTime:   nil,
		CreatedAt: now,
	}
	mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(mockTimespan, nil)

	// Mock visit creation
	mockVisit := &models2.Visit{
		ID:         1,
		Day:        now,
		StudentID:  student.ID,
		RoomID:     roomID,
		TimespanID: int64(1),
		CreatedAt:  now,
	}
	mockStudentStore.On("CreateStudentVisit", mock.Anything, student.ID, roomID, int64(1)).Return(mockVisit, nil)

	mockRFIDStore.On("RecordRoomEntry", mock.Anything, student.ID, roomID).Return(nil)

	// Expect location update
	mockStudentStore.On("UpdateStudentLocation", mock.Anything, student.ID, mock.MatchedBy(func(locations map[string]bool) bool {
		return locations["in_house"] == true
	})).Return(nil)

	// Mock for GetRoomOccupancy
	mockOccupancy := &RoomOccupancyData{
		RoomID:       roomID,
		RoomName:     "Test Room",
		Capacity:     30,
		StudentCount: 1,
		Students: []RoomOccupancyStudent{
			{
				ID:        student.ID,
				Name:      user.FirstName + " " + user.SecondName,
				EnteredAt: now,
			},
		},
	}
	mockRFIDStore.On("GetRoomOccupancy", mock.Anything, roomID).Return(mockOccupancy, nil)

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	payload := map[string]interface{}{
		"tag_id":    tagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Should still succeed despite tag saving failure
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response OccupancyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)
	assert.Equal(t, student.ID, response.StudentID)

	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}

// TestErrorCreatingTimespan tests handling of timespan creation errors
func TestErrorCreatingTimespan(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Common test data
	now := time.Now()
	tagID := "STUDENT0001"
	readerID := "ENTRANCE_READER"
	roomID := int64(101)
	user := &models2.CustomUser{
		ID:         123,
		FirstName:  "Jane",
		SecondName: "Doe",
		TagID:      &tagID,
	}
	student := &models2.Student{
		ID:           456,
		SchoolClass:  "4A",
		CustomUserID: user.ID,
		InHouse:      false,
		WC:           false,
		SchoolYard:   false,
	}

	// Setup expectations
	mockTag := &Tag{
		ID:        3,
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(mockTag, nil)
	mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil)
	mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil)

	// Error creating timespan
	timespanErr := errors.New("failed to create timespan")
	mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(nil, timespanErr)

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	payload := map[string]interface{}{
		"tag_id":    tagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Should return 500 internal server error
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}

// TestErrorUpdatingStudentLocation tests handling of student location update errors
func TestErrorUpdatingStudentLocation(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Common test data
	now := time.Now()
	tagID := "STUDENT0001"
	readerID := "ENTRANCE_READER"
	roomID := int64(101)
	user := &models2.CustomUser{
		ID:         123,
		FirstName:  "Jane",
		SecondName: "Doe",
		TagID:      &tagID,
	}
	student := &models2.Student{
		ID:           456,
		SchoolClass:  "4A",
		CustomUserID: user.ID,
		InHouse:      false,
		WC:           false,
		SchoolYard:   false,
	}

	// Setup expectations
	mockTag := &Tag{
		ID:        4,
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(mockTag, nil)
	mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil)
	mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil)

	// Success creating timespan
	timespan := &models2.Timespan{
		ID:        4,
		StartTime: now,
		EndTime:   nil,
		CreatedAt: now,
	}
	mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan, nil)

	// Success creating visit
	visit := &models2.Visit{
		ID:         4,
		Day:        now,
		StudentID:  student.ID,
		RoomID:     roomID,
		TimespanID: timespan.ID,
		CreatedAt:  now,
	}
	mockStudentStore.On("CreateStudentVisit", mock.Anything, student.ID, roomID, timespan.ID).Return(visit, nil)

	// Success recording room entry
	mockRFIDStore.On("RecordRoomEntry", mock.Anything, student.ID, roomID).Return(nil)

	// Error updating student location
	locationErr := errors.New("failed to update student location")
	mockStudentStore.On("UpdateStudentLocation", mock.Anything, student.ID, mock.MatchedBy(func(locations map[string]bool) bool {
		return locations["in_house"] == true
	})).Return(locationErr)

	// Success getting room occupancy despite location update error
	mockOccupancy := &RoomOccupancyData{
		RoomID:       roomID,
		RoomName:     "Test Room",
		Capacity:     30,
		StudentCount: 1,
		Students: []RoomOccupancyStudent{
			{
				ID:        student.ID,
				Name:      user.FirstName + " " + user.SecondName,
				EnteredAt: now,
			},
		},
	}
	mockRFIDStore.On("GetRoomOccupancy", mock.Anything, roomID).Return(mockOccupancy, nil)

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	payload := map[string]interface{}{
		"tag_id":    tagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Should still succeed, just with a warning in the logs
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response OccupancyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.True(t, response.Success)

	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}

// TestMalformedRequest tests handling of malformed requests
func TestMalformedRequest(t *testing.T) {
	// Setup API with minimal dependencies since we'll fail at request binding
	mockRFIDStore := new(MockRFIDStore)
	api := &API{
		store: mockRFIDStore,
	}

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request with invalid JSON
	badPayload := "this is not json"

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBufferString(badPayload))
	defer resp.Body.Close()

	// Should return 422 Unprocessable Entity for malformed JSON
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
}

// TestUserFoundButNoStudentRecord tests handling of missing student records
func TestUserFoundButNoStudentRecord(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Common test data
	now := time.Now()
	tagID := "STUDENT0001"
	readerID := "ENTRANCE_READER"
	roomID := int64(101)
	user := &models2.CustomUser{
		ID:         123,
		FirstName:  "Jane",
		SecondName: "Doe",
		TagID:      &tagID,
	}

	// Setup expectations
	mockTag := &Tag{
		ID:        6,
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(mockTag, nil)
	mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil)
	mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(nil, errors.New("student not found"))

	// Create a router and test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Make request
	payload := map[string]interface{}{
		"tag_id":    tagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Perform request
	resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Should fail gracefully with appropriate message
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var response OccupancyResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	require.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Message, "User found but no student record")

	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
}

// TestConcurrentRFIDTagReads tests the system's handling of concurrent tag reads
func TestConcurrentRFIDTagReads(t *testing.T) {
	// Setup API and all its dependencies
	mockRFIDStore := new(MockRFIDStore)
	mockUserStore := new(MockUserStore)
	mockStudentStore := new(MockStudentStore)
	mockTimespanStore := new(MockTimespanStore)

	api := &API{
		store:         mockRFIDStore,
		userStore:     mockUserStore,
		studentStore:  mockStudentStore,
		timespanStore: mockTimespanStore,
	}

	// Create test server
	router := api.Router()
	server := httptest.NewServer(router)
	defer server.Close()

	// Test data
	now := time.Now()

	// Multiple tag IDs and reader IDs
	tagIDs := []string{"TAG001", "TAG002", "TAG003"}
	readerIDs := []string{"READER1", "READER2", "READER3"}

	// Setup mock expectations for each tag
	for i := 0; i < len(tagIDs); i++ {
		// Mock tag
		mockTag := &Tag{
			ID:        int64(i + 1),
			TagID:     tagIDs[i],
			ReaderID:  readerIDs[i],
			ReadAt:    now,
			CreatedAt: now,
			UpdatedAt: now,
		}
		mockRFIDStore.On("SaveTag", mock.Anything, tagIDs[i], readerIDs[i]).Return(mockTag, nil)
	}

	// Run test: Simulate concurrent tag reads
	done := make(chan bool)

	// Make concurrent requests for tag reads
	for i := 0; i < len(tagIDs); i++ {
		idx := i // Capture loop variable
		go func() {
			payload := map[string]string{
				"tag_id":    tagIDs[idx],
				"reader_id": readerIDs[idx],
			}
			jsonData, err := json.Marshal(payload)
			require.NoError(t, err)

			// Perform request
			resp := performRequest(t, server, "POST", "/tag", bytes.NewBuffer(jsonData))
			defer resp.Body.Close()

			// Check response
			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var response Tag
			err = json.NewDecoder(resp.Body).Decode(&response)
			require.NoError(t, err)
			assert.Equal(t, tagIDs[idx], response.TagID)

			done <- true
		}()
	}

	// Wait for all requests to complete
	for i := 0; i < len(tagIDs); i++ {
		<-done
	}

	mockRFIDStore.AssertExpectations(t)
}

// TestMissingStores tests behavior when required stores are missing
func TestMissingStores(t *testing.T) {
	// Create API without userStore
	mockRFIDStore := new(MockRFIDStore)
	apiWithoutUserStore := &API{
		store:         mockRFIDStore,
		userStore:     nil, // Deliberately nil
		studentStore:  nil,
		timespanStore: nil,
	}

	routerWithoutUserStore := apiWithoutUserStore.Router()
	serverWithoutUserStore := httptest.NewServer(routerWithoutUserStore)
	defer serverWithoutUserStore.Close()

	tagID := "TAG001"
	roomID := int64(101)
	readerID := "READER1"

	// Make request for room entry which requires userStore
	payload := map[string]interface{}{
		"tag_id":    tagID,
		"room_id":   roomID,
		"reader_id": readerID,
	}
	jsonData, err := json.Marshal(payload)
	require.NoError(t, err)

	// Setup tag saving expectation
	mockTag := &Tag{
		ID:        1,
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(mockTag, nil)

	// Perform request
	resp := performRequest(t, serverWithoutUserStore, "POST", "/room-entry", bytes.NewBuffer(jsonData))
	defer resp.Body.Close()

	// Should fail due to missing userStore
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	mockRFIDStore.AssertExpectations(t)
}
