package rfid

import (
	"bytes"
	"encoding/json"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Define a comprehensive integration test suite for the RFID system
func TestRFIDIntegrationFlow(t *testing.T) {
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

	// Create test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Test data
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
	room := &models2.Room{
		ID:       roomID,
		RoomName: "Classroom 101",
		Capacity: 30,
	}

	// Mock tag for tag read
	mockTag := &Tag{
		ID:        1,
		TagID:     tagID,
		ReaderID:  readerID,
		ReadAt:    now,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Setup mock for timespan
	timespan := &models2.Timespan{
		ID:        789,
		StartTime: now,
		EndTime:   nil,
		CreatedAt: now,
	}

	// Setup mock for visit
	visit := &models2.Visit{
		ID:         101,
		Day:        now,
		StudentID:  student.ID,
		RoomID:     roomID,
		TimespanID: timespan.ID,
		CreatedAt:  now,
	}

	// Setup mock for room occupancy
	roomOccupancy := &RoomOccupancyData{
		RoomID:       roomID,
		RoomName:     room.RoomName,
		Capacity:     room.Capacity,
		StudentCount: 1,
		Students: []RoomOccupancyStudent{
			{
				ID:        student.ID,
				Name:      user.FirstName + " " + user.SecondName,
				EnteredAt: now,
			},
		},
	}

	// PHASE 1: Student enters the building (tag read at entrance)
	t.Run("Phase 1: Student enters building", func(t *testing.T) {
		// Setup expectations for tag read
		mockRFIDStore.On("SaveTag", mock.Anything, tagID, readerID).Return(mockTag, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil).Once()

		// Expect student location update to "in-house"
		mockStudentStore.On("UpdateStudentLocation", mock.Anything, student.ID, mock.MatchedBy(func(locations map[string]bool) bool {
			return locations["in_house"] == true &&
				locations["wc"] == false &&
				locations["school_yard"] == false
		})).Return(nil).Once()

		// Make request for student tracking
		payload := map[string]string{
			"tag_id":        tagID,
			"reader_id":     readerID,
			"location_type": "entry",
		}
		jsonData, err := json.Marshal(payload)
		require.NoError(t, err)

		// Perform the request
		resp := performRequest(t, server, "POST", "/track-student", bytes.NewBuffer(jsonData))
		defer resp.Body.Close()

		// Assert the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var trackResp StudentTrackingResponse
		err = json.NewDecoder(resp.Body).Decode(&trackResp)
		require.NoError(t, err)

		assert.True(t, trackResp.Success)
		assert.Equal(t, "Jane Doe", trackResp.Name)
		assert.Equal(t, "in-house", trackResp.Location)
	})

	// PHASE 2: Student enters a classroom
	t.Run("Phase 2: Student enters classroom", func(t *testing.T) {
		// Setup expectations
		mockRFIDStore.On("SaveTag", mock.Anything, tagID, "ROOM_READER").Return(mockTag, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil).Once()

		// Timespan creation
		mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan, nil).Once()

		// Visit record creation
		mockStudentStore.On("CreateStudentVisit", mock.Anything, student.ID, roomID, timespan.ID).Return(visit, nil).Once()

		// Room entry record
		mockRFIDStore.On("RecordRoomEntry", mock.Anything, student.ID, roomID).Return(nil).Once()

		// Student location update
		mockStudentStore.On("UpdateStudentLocation", mock.Anything, student.ID, mock.MatchedBy(func(locations map[string]bool) bool {
			return locations["in_house"] == true &&
				locations["wc"] == false &&
				locations["school_yard"] == false
		})).Return(nil).Once()

		// Room occupancy data
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, roomID).Return(roomOccupancy, nil).Once()

		// Make request for room entry
		payload := map[string]interface{}{
			"tag_id":    tagID,
			"room_id":   roomID,
			"reader_id": "ROOM_READER",
		}
		jsonData, err := json.Marshal(payload)
		require.NoError(t, err)

		// Perform the request
		resp := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData))
		defer resp.Body.Close()

		// Assert the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var entryResp OccupancyResponse
		err = json.NewDecoder(resp.Body).Decode(&entryResp)
		require.NoError(t, err)

		assert.True(t, entryResp.Success)
		assert.Equal(t, student.ID, entryResp.StudentID)
		assert.Equal(t, roomID, entryResp.RoomID)
		assert.Equal(t, 1, entryResp.StudentCount)
	})

	// PHASE 3: Query room occupancy
	t.Run("Phase 3: Query room occupancy", func(t *testing.T) {
		// Setup expectations for getting room occupancy
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, roomID).Return(roomOccupancy, nil).Once()

		// Perform the request
		resp := performRequest(t, server, "GET", "/room-occupancy?room_id=101", nil)
		defer resp.Body.Close()

		// Assert the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var occupancyResp RoomOccupancyData
		err := json.NewDecoder(resp.Body).Decode(&occupancyResp)
		require.NoError(t, err)

		assert.Equal(t, roomID, occupancyResp.RoomID)
		assert.Equal(t, room.RoomName, occupancyResp.RoomName)
		assert.Equal(t, 1, occupancyResp.StudentCount)
		assert.Len(t, occupancyResp.Students, 1)
		assert.Equal(t, student.ID, occupancyResp.Students[0].ID)
	})

	// PHASE 4: Student leaves the classroom
	t.Run("Phase 4: Student exits classroom", func(t *testing.T) {
		// Setup expectations
		mockRFIDStore.On("SaveTag", mock.Anything, tagID, "EXIT_READER").Return(mockTag, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID).Return(user, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user.ID).Return(student, nil).Once()

		// Mock active visits query
		visitWithTimespan := &models2.Visit{
			ID:         visit.ID,
			Day:        visit.Day,
			StudentID:  visit.StudentID,
			RoomID:     visit.RoomID,
			TimespanID: visit.TimespanID,
			Timespan:   timespan, // Original timespan without end time
			CreatedAt:  visit.CreatedAt,
		}

		activeVisits := []models2.Visit{*visitWithTimespan}
		mockStudentStore.On("GetRoomVisits", mock.Anything, roomID, mock.AnythingOfType("*time.Time"), true).Return(activeVisits, nil).Once()

		// Mock timespan update
		mockTimespanStore.On("UpdateTimespanEndTime", mock.Anything, timespan.ID, mock.AnythingOfType("time.Time")).Return(nil).Once()

		// Room exit record
		mockRFIDStore.On("RecordRoomExit", mock.Anything, student.ID, roomID).Return(nil).Once()

		// Updated room occupancy after exit
		emptyRoomOccupancy := &RoomOccupancyData{
			RoomID:       roomID,
			RoomName:     room.RoomName,
			Capacity:     room.Capacity,
			StudentCount: 0,
			Students:     []RoomOccupancyStudent{},
		}
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, roomID).Return(emptyRoomOccupancy, nil).Once()

		// Make request for room exit
		payload := map[string]interface{}{
			"tag_id":    tagID,
			"room_id":   roomID,
			"reader_id": "EXIT_READER",
		}
		jsonData, err := json.Marshal(payload)
		require.NoError(t, err)

		// Perform the request
		resp := performRequest(t, server, "POST", "/room-exit", bytes.NewBuffer(jsonData))
		defer resp.Body.Close()

		// Assert the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var exitResp OccupancyResponse
		err = json.NewDecoder(resp.Body).Decode(&exitResp)
		require.NoError(t, err)

		assert.True(t, exitResp.Success)
		assert.Equal(t, student.ID, exitResp.StudentID)
		assert.Equal(t, roomID, exitResp.RoomID)
		assert.Equal(t, 0, exitResp.StudentCount) // Room should be empty
	})

	// PHASE 5: Query student visits
	t.Run("Phase 5: Query student visit history", func(t *testing.T) {
		// Setup expectations
		completedVisit := &models2.Visit{
			ID:         visit.ID,
			Day:        visit.Day,
			StudentID:  visit.StudentID,
			RoomID:     visit.RoomID,
			TimespanID: visit.TimespanID,
			Timespan: &models2.Timespan{
				ID:        timespan.ID,
				StartTime: timespan.StartTime,
				EndTime:   ptTime(timespan.StartTime.Add(30 * time.Minute)),
				CreatedAt: timespan.CreatedAt,
			},
			CreatedAt: visit.CreatedAt,
		}
		visits := []models2.Visit{*completedVisit}

		mockStudentStore.On("GetStudentVisits", mock.Anything, student.ID, mock.Anything).Return(visits, nil).Once()

		// Perform the request
		resp := performRequest(t, server, "GET", "/student/456/visits", nil)
		defer resp.Body.Close()

		// Assert the response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var visitsResp []models2.Visit
		err := json.NewDecoder(resp.Body).Decode(&visitsResp)
		require.NoError(t, err)

		assert.Len(t, visitsResp, 1)
		assert.Equal(t, student.ID, visitsResp[0].StudentID)
		assert.Equal(t, roomID, visitsResp[0].RoomID)
		assert.NotNil(t, visitsResp[0].Timespan)
		assert.NotNil(t, visitsResp[0].Timespan.EndTime)
	})

	// Verify all expectations were met
	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}

// Helper function to create a pointer to a time.Time
func ptTime(t time.Time) *time.Time {
	return &t
}

// Test API with multiple students and rooms
func TestMultipleStudentsAndRooms(t *testing.T) {
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

	// Create test server with mock authentication
	router := setupTestRouter(api)
	server := httptest.NewServer(router)
	defer server.Close()

	// Test data for two students
	now := time.Now()

	// Student 1
	tagID1 := "STUDENT0001"
	student1ID := int64(456)
	user1 := &models2.CustomUser{
		ID:         123,
		FirstName:  "Jane",
		SecondName: "Doe",
		TagID:      &tagID1,
	}
	student1 := &models2.Student{
		ID:           student1ID,
		SchoolClass:  "4A",
		CustomUserID: user1.ID,
	}

	// Student 2
	tagID2 := "STUDENT0002"
	student2ID := int64(789)
	user2 := &models2.CustomUser{
		ID:         124,
		FirstName:  "John",
		SecondName: "Smith",
		TagID:      &tagID2,
	}
	student2 := &models2.Student{
		ID:           student2ID,
		SchoolClass:  "4B",
		CustomUserID: user2.ID,
	}

	// Rooms
	classroom := int64(101)
	library := int64(102)

	// First phase: Both students enter different rooms
	t.Run("Multiple students entering different rooms", func(t *testing.T) {
		// Student 1 enters classroom
		mockTag1 := &Tag{
			ID:        1,
			TagID:     tagID1,
			ReaderID:  "CLASSROOM_READER",
			ReadAt:    now,
			CreatedAt: now,
			UpdatedAt: now,
		}

		timespan1 := &models2.Timespan{
			ID:        1001,
			StartTime: now,
			EndTime:   nil,
			CreatedAt: now,
		}

		visit1 := &models2.Visit{
			ID:         2001,
			Day:        now,
			StudentID:  student1ID,
			RoomID:     classroom,
			TimespanID: timespan1.ID,
			CreatedAt:  now,
		}

		roomOccupancy1 := &RoomOccupancyData{
			RoomID:       classroom,
			RoomName:     "Classroom 101",
			Capacity:     30,
			StudentCount: 1,
			Students: []RoomOccupancyStudent{
				{
					ID:        student1ID,
					Name:      user1.FirstName + " " + user1.SecondName,
					EnteredAt: now,
				},
			},
		}

		// Setup expectations for student 1
		mockRFIDStore.On("SaveTag", mock.Anything, tagID1, "CLASSROOM_READER").Return(mockTag1, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID1).Return(user1, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user1.ID).Return(student1, nil).Once()
		mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan1, nil).Once()
		mockStudentStore.On("CreateStudentVisit", mock.Anything, student1ID, classroom, timespan1.ID).Return(visit1, nil).Once()
		mockRFIDStore.On("RecordRoomEntry", mock.Anything, student1ID, classroom).Return(nil).Once()
		mockStudentStore.On("UpdateStudentLocation", mock.Anything, student1ID, mock.MatchedBy(func(locations map[string]bool) bool {
			return locations["in_house"] == true
		})).Return(nil).Once()
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, classroom).Return(roomOccupancy1, nil).Once()

		// Make request for room entry - student 1
		payload1 := map[string]interface{}{
			"tag_id":    tagID1,
			"room_id":   classroom,
			"reader_id": "CLASSROOM_READER",
		}
		jsonData1, err := json.Marshal(payload1)
		require.NoError(t, err)

		// Perform request for student 1
		resp1 := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData1))
		defer resp1.Body.Close()

		// Assert response for student 1
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Student 2 enters library
		mockTag2 := &Tag{
			ID:        2,
			TagID:     tagID2,
			ReaderID:  "LIBRARY_READER",
			ReadAt:    now.Add(5 * time.Minute),
			CreatedAt: now.Add(5 * time.Minute),
			UpdatedAt: now.Add(5 * time.Minute),
		}

		timespan2 := &models2.Timespan{
			ID:        1002,
			StartTime: now.Add(5 * time.Minute),
			EndTime:   nil,
			CreatedAt: now.Add(5 * time.Minute),
		}

		visit2 := &models2.Visit{
			ID:         2002,
			Day:        now,
			StudentID:  student2ID,
			RoomID:     library,
			TimespanID: timespan2.ID,
			CreatedAt:  now.Add(5 * time.Minute),
		}

		roomOccupancy2 := &RoomOccupancyData{
			RoomID:       library,
			RoomName:     "Library",
			Capacity:     50,
			StudentCount: 1,
			Students: []RoomOccupancyStudent{
				{
					ID:        student2ID,
					Name:      user2.FirstName + " " + user2.SecondName,
					EnteredAt: now.Add(5 * time.Minute),
				},
			},
		}

		// Setup expectations for student 2
		mockRFIDStore.On("SaveTag", mock.Anything, tagID2, "LIBRARY_READER").Return(mockTag2, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID2).Return(user2, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user2.ID).Return(student2, nil).Once()
		mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan2, nil).Once()
		mockStudentStore.On("CreateStudentVisit", mock.Anything, student2ID, library, timespan2.ID).Return(visit2, nil).Once()
		mockRFIDStore.On("RecordRoomEntry", mock.Anything, student2ID, library).Return(nil).Once()
		mockStudentStore.On("UpdateStudentLocation", mock.Anything, student2ID, mock.MatchedBy(func(locations map[string]bool) bool {
			return locations["in_house"] == true
		})).Return(nil).Once()
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, library).Return(roomOccupancy2, nil).Once()

		// Make request for room entry - student 2
		payload2 := map[string]interface{}{
			"tag_id":    tagID2,
			"room_id":   library,
			"reader_id": "LIBRARY_READER",
		}
		jsonData2, err := json.Marshal(payload2)
		require.NoError(t, err)

		// Perform request for student 2
		resp2 := performRequest(t, server, "POST", "/room-entry", bytes.NewBuffer(jsonData2))
		defer resp2.Body.Close()

		// Assert response for student 2
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})

	// Second phase: Query active visits for each room
	t.Run("Query active visits by room", func(t *testing.T) {
		// Setup expectations for classroom visits
		timespan1 := &models2.Timespan{
			ID:        1001,
			StartTime: now,
			EndTime:   nil,
			CreatedAt: now,
		}

		visit1 := models2.Visit{
			ID:         2001,
			Day:        now,
			StudentID:  student1ID,
			RoomID:     classroom,
			TimespanID: timespan1.ID,
			Timespan:   timespan1,
			CreatedAt:  now,
		}

		classroomVisits := []models2.Visit{visit1}
		mockStudentStore.On("GetRoomVisits", mock.Anything, classroom, mock.Anything, true).Return(classroomVisits, nil).Once()

		// Get active visits for classroom
		resp1 := performRequest(t, server, "GET", "/room/101/visits?active=true", nil)
		defer resp1.Body.Close()

		// Assert response
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		var visits1 []models2.Visit
		err := json.NewDecoder(resp1.Body).Decode(&visits1)
		require.NoError(t, err)
		assert.Len(t, visits1, 1)
		assert.Equal(t, student1ID, visits1[0].StudentID)

		// Setup expectations for library visits
		timespan2 := &models2.Timespan{
			ID:        1002,
			StartTime: now.Add(5 * time.Minute),
			EndTime:   nil,
			CreatedAt: now.Add(5 * time.Minute),
		}

		visit2 := models2.Visit{
			ID:         2002,
			Day:        now,
			StudentID:  student2ID,
			RoomID:     library,
			TimespanID: timespan2.ID,
			Timespan:   timespan2,
			CreatedAt:  now.Add(5 * time.Minute),
		}

		libraryVisits := []models2.Visit{visit2}
		mockStudentStore.On("GetRoomVisits", mock.Anything, library, mock.Anything, true).Return(libraryVisits, nil).Once()

		// Get active visits for library
		resp2 := performRequest(t, server, "GET", "/room/102/visits?active=true", nil)
		defer resp2.Body.Close()

		// Assert response
		assert.Equal(t, http.StatusOK, resp2.StatusCode)

		var visits2 []models2.Visit
		err = json.NewDecoder(resp2.Body).Decode(&visits2)
		require.NoError(t, err)
		assert.Len(t, visits2, 1)
		assert.Equal(t, student2ID, visits2[0].StudentID)
	})

	// Phase 3: Both students exit
	t.Run("Multiple students exiting rooms", func(t *testing.T) {
		// Setup for student 1 exit
		mockTag1Exit := &Tag{
			ID:        3,
			TagID:     tagID1,
			ReaderID:  "CLASSROOM_EXIT",
			ReadAt:    now.Add(60 * time.Minute),
			CreatedAt: now.Add(60 * time.Minute),
			UpdatedAt: now.Add(60 * time.Minute),
		}

		timespan1NoEnd := &models2.Timespan{
			ID:        1001,
			StartTime: now,
			EndTime:   nil,
			CreatedAt: now,
		}

		visit1 := models2.Visit{
			ID:         2001,
			Day:        now,
			StudentID:  student1ID,
			RoomID:     classroom,
			TimespanID: timespan1NoEnd.ID,
			Timespan:   timespan1NoEnd,
			CreatedAt:  now,
		}

		classroomVisits := []models2.Visit{visit1}

		// Expectations for student 1 exit
		mockRFIDStore.On("SaveTag", mock.Anything, tagID1, "CLASSROOM_EXIT").Return(mockTag1Exit, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID1).Return(user1, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user1.ID).Return(student1, nil).Once()
		mockStudentStore.On("GetRoomVisits", mock.Anything, classroom, mock.Anything, true).Return(classroomVisits, nil).Once()
		mockTimespanStore.On("UpdateTimespanEndTime", mock.Anything, timespan1NoEnd.ID, mock.AnythingOfType("time.Time")).Return(nil).Once()
		mockRFIDStore.On("RecordRoomExit", mock.Anything, student1ID, classroom).Return(nil).Once()

		roomOccupancy1Empty := &RoomOccupancyData{
			RoomID:       classroom,
			RoomName:     "Classroom 101",
			Capacity:     30,
			StudentCount: 0,
			Students:     []RoomOccupancyStudent{},
		}
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, classroom).Return(roomOccupancy1Empty, nil).Once()

		// Make request for student 1 exit
		payload1 := map[string]interface{}{
			"tag_id":    tagID1,
			"room_id":   classroom,
			"reader_id": "CLASSROOM_EXIT",
		}
		jsonData1, err := json.Marshal(payload1)
		require.NoError(t, err)

		// Perform request for student 1 exit
		resp1 := performRequest(t, server, "POST", "/room-exit", bytes.NewBuffer(jsonData1))
		defer resp1.Body.Close()

		// Assert response
		assert.Equal(t, http.StatusOK, resp1.StatusCode)

		// Setup for student 2 exit
		mockTag2Exit := &Tag{
			ID:        4,
			TagID:     tagID2,
			ReaderID:  "LIBRARY_EXIT",
			ReadAt:    now.Add(90 * time.Minute),
			CreatedAt: now.Add(90 * time.Minute),
			UpdatedAt: now.Add(90 * time.Minute),
		}

		timespan2NoEnd := &models2.Timespan{
			ID:        1002,
			StartTime: now.Add(5 * time.Minute),
			EndTime:   nil,
			CreatedAt: now.Add(5 * time.Minute),
		}

		visit2 := models2.Visit{
			ID:         2002,
			Day:        now,
			StudentID:  student2ID,
			RoomID:     library,
			TimespanID: timespan2NoEnd.ID,
			Timespan:   timespan2NoEnd,
			CreatedAt:  now.Add(5 * time.Minute),
		}

		libraryVisits := []models2.Visit{visit2}

		// Expectations for student 2 exit
		mockRFIDStore.On("SaveTag", mock.Anything, tagID2, "LIBRARY_EXIT").Return(mockTag2Exit, nil).Once()
		mockUserStore.On("GetCustomUserByTagID", mock.Anything, tagID2).Return(user2, nil).Once()
		mockStudentStore.On("GetStudentByCustomUserID", mock.Anything, user2.ID).Return(student2, nil).Once()
		mockStudentStore.On("GetRoomVisits", mock.Anything, library, mock.Anything, true).Return(libraryVisits, nil).Once()
		mockTimespanStore.On("UpdateTimespanEndTime", mock.Anything, timespan2NoEnd.ID, mock.AnythingOfType("time.Time")).Return(nil).Once()
		mockRFIDStore.On("RecordRoomExit", mock.Anything, student2ID, library).Return(nil).Once()

		roomOccupancy2Empty := &RoomOccupancyData{
			RoomID:       library,
			RoomName:     "Library",
			Capacity:     50,
			StudentCount: 0,
			Students:     []RoomOccupancyStudent{},
		}
		mockRFIDStore.On("GetRoomOccupancy", mock.Anything, library).Return(roomOccupancy2Empty, nil).Once()

		// Make request for student 2 exit
		payload2 := map[string]interface{}{
			"tag_id":    tagID2,
			"room_id":   library,
			"reader_id": "LIBRARY_EXIT",
		}
		jsonData2, err := json.Marshal(payload2)
		require.NoError(t, err)

		// Perform request for student 2 exit
		resp2 := performRequest(t, server, "POST", "/room-exit", bytes.NewBuffer(jsonData2))
		defer resp2.Body.Close()

		// Assert response
		assert.Equal(t, http.StatusOK, resp2.StatusCode)
	})

	// Phase 4: Query all visits for today
	t.Run("Query all visits for today", func(t *testing.T) {
		// Set up expectations for getting all rooms
		classroomOccupancy := &RoomOccupancyData{
			RoomID:       classroom,
			RoomName:     "Classroom 101",
			Capacity:     30,
			StudentCount: 0,
			Students:     []RoomOccupancyStudent{},
		}

		libraryOccupancy := &RoomOccupancyData{
			RoomID:       library,
			RoomName:     "Library",
			Capacity:     50,
			StudentCount: 0,
			Students:     []RoomOccupancyStudent{},
		}

		allRooms := []RoomOccupancyData{*classroomOccupancy, *libraryOccupancy}
		mockRFIDStore.On("GetCurrentRooms", mock.Anything).Return(allRooms, nil).Once()

		// Expectations for visits from each room
		timespan1WithEnd := &models2.Timespan{
			ID:        1001,
			StartTime: now,
			EndTime:   ptTime(now.Add(60 * time.Minute)),
			CreatedAt: now,
		}

		visit1WithEnd := models2.Visit{
			ID:         2001,
			Day:        now,
			StudentID:  student1ID,
			RoomID:     classroom,
			TimespanID: timespan1WithEnd.ID,
			Timespan:   timespan1WithEnd,
			CreatedAt:  now,
		}

		// Create classroom visits data
		completedClassroomVisits := []models2.Visit{visit1WithEnd}
		mockStudentStore.On("GetRoomVisits", mock.Anything, classroom, mock.AnythingOfType("*time.Time"), false).Return(completedClassroomVisits, nil).Once()

		// Create library visits data
		timespan2WithEnd := &models2.Timespan{
			ID:        1002,
			StartTime: now.Add(5 * time.Minute),
			EndTime:   ptTime(now.Add(90 * time.Minute)),
			CreatedAt: now.Add(5 * time.Minute),
		}

		visit2WithEnd := models2.Visit{
			ID:         2002,
			Day:        now,
			StudentID:  student2ID,
			RoomID:     library,
			TimespanID: timespan2WithEnd.ID,
			Timespan:   timespan2WithEnd,
			CreatedAt:  now.Add(5 * time.Minute),
		}

		completedLibraryVisits := []models2.Visit{visit2WithEnd}
		mockStudentStore.On("GetRoomVisits", mock.Anything, library, mock.AnythingOfType("*time.Time"), false).Return(completedLibraryVisits, nil).Once()

		// Get all visits for today
		resp := performRequest(t, server, "GET", "/visits/today", nil)
		defer resp.Body.Close()

		// Assert response
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var allVisits []models2.Visit
		err := json.NewDecoder(resp.Body).Decode(&allVisits)
		require.NoError(t, err)

		assert.Len(t, allVisits, 2)

		// Verify that we have visits from both students
		studentIDs := make(map[int64]bool)
		for _, visit := range allVisits {
			studentIDs[visit.StudentID] = true
		}

		assert.True(t, studentIDs[student1ID])
		assert.True(t, studentIDs[student2ID])
	})

	// Verify all expectations were met
	mockRFIDStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
	mockStudentStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}
