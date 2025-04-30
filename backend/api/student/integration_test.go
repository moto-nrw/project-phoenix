package student

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

// TestStudentLifecycle tests the complete lifecycle of a student
func TestStudentLifecycle(t *testing.T) {
	rs, mockStudentStore, mockUserStore, _ := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	customUser := &models2.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
		CreatedAt:  now,
		ModifiedAt: now,
	}

	group := &models2.Group{
		ID:         1,
		Name:       "Group 1",
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// 2. Define test student
	newStudent := &models2.Student{
		SchoolClass:  "2A",
		Bus:          true,
		CustomUserID: 1,
		GroupID:      1,
		NameLG:       "Parent Name",
		ContactLG:    "parent@example.com",
	}

	// Student after creation
	createdStudent := &models2.Student{
		ID:           1,
		SchoolClass:  "2A",
		Bus:          true,
		CustomUserID: 1,
		CustomUser:   customUser,
		GroupID:      1,
		Group:        group,
		NameLG:       "Parent Name",
		ContactLG:    "parent@example.com",
		InHouse:      false,
		WC:           false,
		SchoolYard:   false,
		CreatedAt:    now,
		ModifiedAt:   now,
	}

	// Student after update
	updatedStudent := &models2.Student{
		ID:           1,
		SchoolClass:  "3B",  // Changed
		Bus:          false, // Changed
		CustomUserID: 1,
		CustomUser:   customUser,
		GroupID:      1,
		Group:        group,
		NameLG:       "Updated Parent Name", // Changed
		ContactLG:    "updated@example.com", // Changed
		InHouse:      false,
		WC:           false,
		SchoolYard:   false,
		CreatedAt:    now,
		ModifiedAt:   now.Add(time.Minute),
	}

	// Timespan for visits
	timespan := &models2.Timespan{
		ID:        1,
		StartTime: now,
		EndTime:   nil,
		CreatedAt: now,
	}

	// Room for visits
	room := &models2.Room{
		ID:         1,
		RoomName:   "Classroom 101",
		Capacity:   30,
		CreatedAt:  now,
		ModifiedAt: now,
	}

	// Visit information
	visit := &models2.Visit{
		ID:         1,
		Day:        now,
		StudentID:  1,
		RoomID:     1,
		Room:       room,
		TimespanID: 1,
		Timespan:   timespan,
		CreatedAt:  now,
	}

	// Mock Room occupancy detail for device ID
	roomOccupancy := &models2.RoomOccupancyDetail{
		RoomID:     1,
		TimespanID: 1,
	}

	// 3. Set up expectations for creation
	mockStudentStore.On("CreateStudent", mock.Anything, newStudent).Return(nil)
	mockStudentStore.On("GetStudentByID", mock.Anything, int64(1)).Return(createdStudent, nil)

	// 4. Set up expectations for update
	mockStudentStore.On("GetStudentByID", mock.Anything, int64(1)).Return(createdStudent, nil)
	mockStudentStore.On("UpdateStudent", mock.Anything, mock.MatchedBy(func(s *models2.Student) bool {
		return s.ID == 1 && s.SchoolClass == "3B" && s.NameLG == "Updated Parent Name"
	})).Return(nil)
	mockStudentStore.On("GetStudentByID", mock.Anything, int64(1)).Return(updatedStudent, nil)

	// 5. Set up expectations for room registration
	mockStudentStore.On("GetRoomOccupancyByDeviceID", mock.Anything, "DEVICE-TEST-001").Return(roomOccupancy, nil)
	mockStudentStore.On("CreateStudentVisit", mock.Anything, int64(1), int64(1), int64(1)).Return(visit, nil)
	mockStudentStore.On("UpdateStudentLocation", mock.Anything, int64(1), mock.MatchedBy(func(locations map[string]bool) bool {
		return locations["in_house"] == true
	})).Return(nil)

	// 6. Set up expectations for student visit retrieval
	visitWithTimespan := visit
	visitList := []models2.Visit{*visitWithTimespan}
	mockStudentStore.On("GetStudentVisits", mock.Anything, int64(1), mock.Anything).Return(visitList, nil)

	// 7. Set up expectations for feedback
	feedback := &models2.Feedback{
		ID:            1,
		FeedbackValue: "Great class!",
		Day:           now,
		Time:          now,
		StudentID:     1,
		MensaFeedback: false,
		CreatedAt:     now,
	}
	mockStudentStore.On("CreateFeedback", mock.Anything, int64(1), "Great class!", false).Return(feedback, nil)

	// 8. Set up expectations for student deletion
	mockStudentStore.On("DeleteStudent", mock.Anything, int64(1)).Return(nil)

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Student
	t.Run("1. Create Student", func(t *testing.T) {
		// Create test request
		studentReq := &StudentRequest{Student: newStudent}
		body, _ := json.Marshal(studentReq)
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseStudent models2.Student
		err := json.Unmarshal(w.Body.Bytes(), &responseStudent)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseStudent.ID)
		assert.Equal(t, "2A", responseStudent.SchoolClass)
		assert.Equal(t, "Parent Name", responseStudent.NameLG)
	})

	// PHASE 2: Update Student
	t.Run("2. Update Student", func(t *testing.T) {
		// Create updated student request
		updateData := &models2.Student{
			ID:           1,
			SchoolClass:  "3B",
			Bus:          false,
			CustomUserID: 1,
			GroupID:      1,
			NameLG:       "Updated Parent Name",
			ContactLG:    "updated@example.com",
		}

		// Set up expectations for user name update in case FirstName/SecondName is provided
		mockUserStore.On("GetCustomUserByID", mock.Anything, int64(1)).Return(customUser, nil).Maybe()
		mockUserStore.On("UpdateCustomUser", mock.Anything, mock.Anything).Return(nil).Maybe()

		studentReq := &StudentRequest{Student: updateData}
		body, _ := json.Marshal(studentReq)
		r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseStudent models2.Student
		err := json.Unmarshal(w.Body.Bytes(), &responseStudent)
		assert.NoError(t, err)
		assert.Equal(t, "3B", responseStudent.SchoolClass)
		assert.Equal(t, "Updated Parent Name", responseStudent.NameLG)
		assert.Equal(t, "updated@example.com", responseStudent.ContactLG)
		assert.False(t, responseStudent.Bus)
	})

	// PHASE 3: Register Student In Room
	t.Run("3. Register Student In Room", func(t *testing.T) {
		// Create room registration request
		regReq := &RoomRegistrationRequest{
			StudentID: 1,
			DeviceID:  "DEVICE-TEST-001",
		}

		body, _ := json.Marshal(regReq)
		r := httptest.NewRequest("POST", "/register-in-room", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.registerStudentInRoom(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseVisit models2.Visit
		err := json.Unmarshal(w.Body.Bytes(), &responseVisit)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseVisit.StudentID)
		assert.Equal(t, int64(1), responseVisit.RoomID)
		assert.Equal(t, int64(1), responseVisit.TimespanID)
	})

	// PHASE 4: Get Student Visits
	t.Run("4. Get Student Visits", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1/visits", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getStudentVisits(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseVisits []models2.Visit
		err := json.Unmarshal(w.Body.Bytes(), &responseVisits)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseVisits))
		assert.Equal(t, int64(1), responseVisits[0].StudentID)
		assert.Equal(t, int64(1), responseVisits[0].RoomID)
		assert.Equal(t, "Classroom 101", responseVisits[0].Room.RoomName)
	})

	// PHASE 5: Give Feedback
	t.Run("5. Give Feedback", func(t *testing.T) {
		// Create feedback request
		feedbackReq := &FeedbackRequest{
			StudentID:     1,
			FeedbackValue: "Great class!",
			MensaFeedback: false,
		}

		body, _ := json.Marshal(feedbackReq)
		r := httptest.NewRequest("POST", "/give-feedback", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.giveFeedback(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseFeedback models2.Feedback
		err := json.Unmarshal(w.Body.Bytes(), &responseFeedback)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseFeedback.ID)
		assert.Equal(t, int64(1), responseFeedback.StudentID)
		assert.Equal(t, "Great class!", responseFeedback.FeedbackValue)
		assert.False(t, responseFeedback.MensaFeedback)
	})

	// PHASE 6: Delete Student
	t.Run("6. Delete Student", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockStudentStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
}

// TestStudentBusinessLogic tests the business logic functions
func TestStudentBusinessLogic(t *testing.T) {
	// Test data
	student := &models2.Student{
		ID:           1,
		SchoolClass:  "3A",
		Bus:          false,
		CustomUserID: 1,
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Doe",
		},
		GroupID: 1,
		Group: &models2.Group{
			ID:     1,
			Name:   "Group 1",
			RoomID: func() *int64 { id := int64(101); return &id }(),
		},
		InHouse:    true,
		WC:         false,
		SchoolYard: false,
	}

	// Test status determination
	t.Run("GetStudentCurrentStatus", func(t *testing.T) {
		// Student in school
		status := GetStudentCurrentStatus(student)
		assert.Equal(t, "In school", status)

		// Student in bathroom
		studentInWC := *student
		studentInWC.WC = true
		status = GetStudentCurrentStatus(&studentInWC)
		assert.Equal(t, "In bathroom", status)

		// Student in schoolyard
		studentInYard := *student
		studentInYard.InHouse = false
		studentInYard.SchoolYard = true
		status = GetStudentCurrentStatus(&studentInYard)
		assert.Equal(t, "In school yard", status)

		// Student not present
		studentNotPresent := *student
		studentNotPresent.InHouse = false
		status = GetStudentCurrentStatus(&studentNotPresent)
		assert.Equal(t, "Not in school", status)
	})

	// Test room access verification
	t.Run("VerifyRoomAccess", func(t *testing.T) {
		// Student's assigned room
		roomID := int64(101)
		hasAccess := VerifyRoomAccess(student, roomID)
		assert.True(t, hasAccess)

		// Different room
		otherRoomID := int64(102)
		hasAccess = VerifyRoomAccess(student, otherRoomID)
		assert.False(t, hasAccess)
	})

	// Test processing visits
	t.Run("ProcessStudentVisits", func(t *testing.T) {
		now := time.Now()
		endTime := now.Add(2 * time.Hour)

		visits := []models2.Visit{
			{
				ID:        1,
				Day:       now,
				StudentID: 1,
				RoomID:    101,
				Room: &models2.Room{
					ID:       101,
					RoomName: "Classroom 101",
				},
				TimespanID: 1,
				Timespan: &models2.Timespan{
					ID:        1,
					StartTime: now,
					EndTime:   &endTime,
				},
			},
			{
				ID:        2,
				Day:       now,
				StudentID: 1,
				RoomID:    102,
				Room: &models2.Room{
					ID:       102,
					RoomName: "Library",
				},
				TimespanID: 2,
				Timespan: &models2.Timespan{
					ID:        2,
					StartTime: now.Add(3 * time.Hour),
					EndTime:   nil, // Still active
				},
			},
		}

		summary := ProcessStudentVisits(visits)

		assert.Equal(t, 2, summary["total_visits"])
		assert.NotNil(t, summary["rooms_visited"])
		roomsVisited, ok := summary["rooms_visited"].(map[int64]string)
		assert.True(t, ok)
		assert.Equal(t, "Classroom 101", roomsVisited[101])
		assert.Equal(t, "Library", roomsVisited[102])
	})

	// Test attendance rate calculation
	t.Run("GetStudentAttendanceRate", func(t *testing.T) {
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		dayBefore := now.AddDate(0, 0, -2)

		visits := []models2.Visit{
			{
				ID:        1,
				Day:       now,
				StudentID: 1,
			},
			{
				ID:        2,
				Day:       now, // Same day, should count as 1
				StudentID: 1,
			},
			{
				ID:        3,
				Day:       yesterday,
				StudentID: 1,
			},
			// No visit on day before
		}

		// 2 days attended out of 3
		rate := GetStudentAttendanceRate(visits, 3)
		assert.Equal(t, float64(2)/float64(3), rate)

		// Add visit for day before
		visits = append(visits, models2.Visit{
			ID:        4,
			Day:       dayBefore,
			StudentID: 1,
		})

		// Now 3 days attended out of 3
		rate = GetStudentAttendanceRate(visits, 3)
		assert.Equal(t, float64(1.0), rate)
	})
}
