package activity

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

// TestAgCategoryLifecycle tests the complete lifecycle of an activity group category
func TestAgCategoryLifecycle(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	// 2. Define test category
	newCategory := &models2.AgCategory{
		Name: "New Test Category",
	}

	// Category after creation
	createdCategory := &models2.AgCategory{
		ID:        1,
		Name:      "New Test Category",
		CreatedAt: now,
	}

	// Category after update
	updatedCategory := &models2.AgCategory{
		ID:        1,
		Name:      "Updated Category Name",
		CreatedAt: now,
	}

	// 3. Set up expectations for creation
	mockAgStore.On(
		"CreateAgCategory",
		mock.Anything,
		mock.MatchedBy(func(c *models2.AgCategory) bool {
			return c.Name == "New Test Category"
		}),
	).Return(nil).Once()

	// 4. Set up expectations for retrieval after creation
	mockAgStore.On("GetAgCategoryByID", mock.Anything, int64(1)).Return(createdCategory, nil).Once()

	// 5. Set up expectations for update
	mockAgStore.On("UpdateAgCategory", mock.Anything, mock.MatchedBy(func(c *models2.AgCategory) bool {
		return c.ID == 1 && c.Name == "Updated Category Name"
	})).Return(nil).Once()

	// 6. Set up expectations for retrieval after update
	mockAgStore.On("GetAgCategoryByID", mock.Anything, int64(1)).Return(updatedCategory, nil).Once()

	// 7. Set up expectations for listing activity groups in this category
	mockAgStore.On("ListAgs", mock.Anything, map[string]interface{}{"category_id": int64(1)}).Return([]models2.Ag{}, nil).Once()

	// 8. Set up expectations for deletion
	mockAgStore.On("DeleteAgCategory", mock.Anything, int64(1)).Return(nil).Once()

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Category
	t.Run("1. Create Category", func(t *testing.T) {
		// Create test request
		categoryReq := &AgCategoryRequest{AgCategory: newCategory}
		body, _ := json.Marshal(categoryReq)
		r := httptest.NewRequest("POST", "/categories", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createCategory(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseCategory models2.AgCategory
		err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseCategory.ID)
		assert.Equal(t, "New Test Category", responseCategory.Name)
	})

	// PHASE 2: Get Category
	t.Run("2. Get Category", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/categories/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getCategory(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseCategory models2.AgCategory
		err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseCategory.ID)
		assert.Equal(t, "New Test Category", responseCategory.Name)
	})

	// PHASE 3: Update Category
	t.Run("3. Update Category", func(t *testing.T) {
		// Create updated category request
		updateData := &models2.AgCategory{
			ID:   1,
			Name: "Updated Category Name",
		}

		categoryReq := &AgCategoryRequest{AgCategory: updateData}
		body, _ := json.Marshal(categoryReq)
		r := httptest.NewRequest("PUT", "/categories/1", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.updateCategory(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseCategory models2.AgCategory
		err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Category Name", responseCategory.Name)
	})

	// PHASE 4: Get Category AGs
	t.Run("4. Get Category AGs", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/categories/1/ags", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getCategoryAgs(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseAgs []models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(responseAgs)) // No AGs in this category yet
	})

	// PHASE 5: Delete Category
	t.Run("5. Delete Category", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/categories/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteCategory(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockAgStore.AssertExpectations(t)
}

// TestEnrollStudentHandler tests the enrollStudent handler in isolation
func TestEnrollStudentHandler(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	ag := &models2.Ag{
		ID:             1,
		Name:           "Test Activity Group",
		MaxParticipant: 10,
		Students:       []*models2.Student{},
	}

	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(ag, nil)
	mockAgStore.On("EnrollStudent", mock.Anything, int64(1), int64(1)).Return(nil)

	r := httptest.NewRequest("POST", "/1/enroll/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	rctx.URLParams.Add("studentId", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	rs.enrollStudent(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, float64(1), response["ag_id"])
	assert.Equal(t, float64(1), response["student_id"])

	mockAgStore.AssertExpectations(t)
}

// TestPublicEndpoints tests the public endpoints for activity groups
func TestPublicEndpoints(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	specialist := &models2.PedagogicalSpecialist{
		ID:   1,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Doe",
		},
	}

	category1 := &models2.AgCategory{
		ID:   1,
		Name: "Sports",
	}

	category2 := &models2.AgCategory{
		ID:   2,
		Name: "Arts",
	}

	// Sample activity groups
	ag1 := models2.Ag{
		ID:             1,
		Name:           "Basketball",
		MaxParticipant: 10,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		Supervisor:     specialist,
		AgCategoryID:   category1.ID,
		AgCategory:     category1,
		Students:       []*models2.Student{}, // Use pointer slice
	}

	ag2 := models2.Ag{
		ID:             2,
		Name:           "Painting",
		MaxParticipant: 15,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		Supervisor:     specialist,
		AgCategoryID:   category2.ID,
		AgCategory:     category2,
		Students:       []*models2.Student{}, // Use pointer slice
	}

	// Set up expectations
	mockAgStore.On("ListAgs", mock.Anything, map[string]interface{}{"is_open": true, "active": true}).Return([]models2.Ag{ag1, ag2}, nil).Once()
	mockAgStore.On("ListAgCategories", mock.Anything).Return([]models2.AgCategory{*category1, *category2}, nil).Once()

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: List Public AGs
	t.Run("1. List Public AGs", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/public", nil)
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.listPublicAgs(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// The response will be transformed into PublicAg objects
		var responseAgs []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseAgs))
		assert.Equal(t, "Basketball", responseAgs[0]["name"])
		assert.Equal(t, "Painting", responseAgs[1]["name"])
	})

	// PHASE 2: List Public Categories
	t.Run("2. List Public Categories", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/public/categories", nil)
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.listPublicCategories(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		// The response will be transformed into PublicCategory objects
		var responseCategories []map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &responseCategories)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(responseCategories))
		assert.Equal(t, "Sports", responseCategories[0]["name"])
		assert.Equal(t, "Arts", responseCategories[1]["name"])
	})

	// Verify all expectations were met
	mockAgStore.AssertExpectations(t)
}

// TestAgLifecycle tests the complete lifecycle of an activity group
func TestAgLifecycle(t *testing.T) {
	rs, mockAgStore, _, mockTimespanStore := setupTestAPI()

	// 1. Setup test data
	now := time.Now()

	// Create test data
	specialist := &models2.PedagogicalSpecialist{
		ID:   1,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Doe",
		},
	}

	category := &models2.AgCategory{
		ID:   1,
		Name: "Test Category",
	}

	timespan := &models2.Timespan{
		ID:        1,
		StartTime: now,
	}

	// Create sample timeslot
	timeslot := &models2.AgTime{
		Weekday:    "Monday",
		TimespanID: 1,
		Timespan:   timespan,
		AgID:       1, // Will be set when linked to AG
	}

	// Create sample student
	student := &models2.Student{
		ID: 1,
		CustomUser: &models2.CustomUser{
			ID:         2,
			FirstName:  "Jane",
			SecondName: "Smith",
		},
		SchoolClass: "5A",
	}

	// 2. Define test activity group
	newAg := &models2.Ag{
		Name:           "New Test AG",
		MaxParticipant: 10,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		AgCategoryID:   category.ID,
	}

	// AG after creation
	createdAg := &models2.Ag{
		ID:             1,
		Name:           "New Test AG",
		MaxParticipant: 10,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		Supervisor:     specialist,
		AgCategoryID:   category.ID,
		AgCategory:     category,
		CreatedAt:      now,
		ModifiedAt:     now,
		Times:          []*models2.AgTime{},
		Students:       []*models2.Student{},
	}

	// AG after adding a timeslot
	agWithTime := *createdAg
	timeslotWithAgID := *timeslot
	timeslotWithAgID.ID = 1
	timeslotWithAgID.AgID = 1
	timeslotWithAgID.CreatedAt = now
	agWithTime.Times = []*models2.AgTime{&timeslotWithAgID}

	// AG after enrolling a student
	agWithStudent := agWithTime
	agWithStudent.Students = []*models2.Student{student}

	// 3. Set up expectations for creation
	mockAgStore.On("CreateAg", mock.Anything, mock.MatchedBy(func(a *models2.Ag) bool {
		return a.Name == "New Test AG" && a.MaxParticipant == 10
	}), []int64{}, []*models2.AgTime{}).Return(nil).Once()

	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(createdAg, nil).Once()

	// 4. Set up expectations for adding a time slot
	// Add expectation for timespan creation
	mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan, nil).Once()

	// Add expectation for time slot creation
	mockAgStore.On("CreateAgTime", mock.Anything, mock.MatchedBy(func(at *models2.AgTime) bool {
		return at.Weekday == "Monday" && at.TimespanID == 1 && at.AgID == 1
	})).Return(nil).Once()

	mockAgStore.On("GetAgTimeByID", mock.Anything, int64(1)).Return(&timeslotWithAgID, nil).Once()

	// 5. Set up expectations for listing time slots
	mockAgStore.On("ListAgTimes", mock.Anything, int64(1)).Return([]models2.AgTime{timeslotWithAgID}, nil).Once()

	// 6. Set up expectations for enrolling a student
	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(&agWithTime, nil).Once()
	mockAgStore.On("EnrollStudent", mock.Anything, int64(1), int64(1)).Return(nil).Once()

	// 7. Set up expectations for listing enrolled students
	mockAgStore.On("ListEnrolledStudents", mock.Anything, int64(1)).Return([]*models2.Student{student}, nil).Once()

	// 8. Set up expectations for unenrolling a student
	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(&agWithStudent, nil).Once()
	mockAgStore.On("UnenrollStudent", mock.Anything, int64(1), int64(1)).Return(nil).Once()

	// 9. Set up expectations for deleting a time slot
	mockAgStore.On("GetAgTimeByID", mock.Anything, int64(1)).Return(&timeslotWithAgID, nil).Once()
	mockAgStore.On("DeleteAgTime", mock.Anything, int64(1)).Return(nil).Once()

	// 10. Set up expectations for deletion
	mockAgStore.On("DeleteAg", mock.Anything, int64(1)).Return(nil).Once()

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Create Activity Group
	t.Run("1. Create Activity Group", func(t *testing.T) {
		// Create test request
		agReq := &AgRequest{
			Ag:         newAg,
			StudentIDs: []int64{},           // Initialize as empty slice instead of nil
			Timeslots:  []*models2.AgTime{}, // Initialize as empty slice instead of nil
		}
		body, _ := json.Marshal(agReq)
		r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Use the standard context
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.createAg(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseAg models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAg)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseAg.ID)
		assert.Equal(t, "New Test AG", responseAg.Name)
		assert.Equal(t, 10, responseAg.MaxParticipant)
		assert.True(t, responseAg.IsOpenAg)
	})

	// PHASE 2: Add Time Slot
	t.Run("2. Add Time Slot", func(t *testing.T) {
		// Create time slot with the new request format
		createReq := &AgTimeCreateRequest{
			Weekday:   "Monday",
			StartTime: now,
			EndTime:   nil,
		}
		body, _ := json.Marshal(createReq)
		r := httptest.NewRequest("POST", "/1/times", bytes.NewReader(body))
		r.Header.Set("Content-Type", "application/json")

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.addAgTime(w, r)

		// Check response
		assert.Equal(t, http.StatusCreated, w.Code)

		var responseTime models2.AgTime
		err := json.Unmarshal(w.Body.Bytes(), &responseTime)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), responseTime.ID)
		assert.Equal(t, "Monday", responseTime.Weekday)
		assert.Equal(t, int64(1), responseTime.AgID)
	})

	// PHASE 3: Get Time Slots
	t.Run("3. Get Time Slots", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1/times", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getAgTimes(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseTimes []models2.AgTime
		err := json.Unmarshal(w.Body.Bytes(), &responseTimes)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseTimes))
		assert.Equal(t, "Monday", responseTimes[0].Weekday)
	})

	// PHASE 4: Enroll Student
	t.Run("4. Enroll Student", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/1/enroll/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("studentId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.enrollStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(1), response["ag_id"])
		assert.Equal(t, float64(1), response["student_id"])
	})

	// PHASE 5: Get Enrolled Students
	t.Run("5. Get Enrolled Students", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/1/students", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getAgStudents(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseStudents []*models2.Student
		err := json.Unmarshal(w.Body.Bytes(), &responseStudents)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseStudents))
		assert.Equal(t, int64(1), responseStudents[0].ID)
		assert.Equal(t, "5A", responseStudents[0].SchoolClass)
	})

	// PHASE 6: Unenroll Student
	t.Run("6. Unenroll Student", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1/enroll/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("studentId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.unenrollStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// PHASE 7: Delete Time Slot
	t.Run("7. Delete Time Slot", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1/times/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("timeId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteAgTime(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// PHASE 8: Delete Activity Group
	t.Run("8. Delete Activity Group", func(t *testing.T) {
		r := httptest.NewRequest("DELETE", "/1", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.deleteAg(w, r)

		// Check response
		assert.Equal(t, http.StatusNoContent, w.Code)
	})

	// Verify all expectations were met
	mockAgStore.AssertExpectations(t)
}

// TestStudentEnrollmentFlow tests the flow for a student to view and enroll in activity groups
func TestStudentEnrollmentFlow(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	now := time.Now()

	// Create test data
	specialist := &models2.PedagogicalSpecialist{
		ID:   1,
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Doe",
		},
	}

	student := &models2.Student{
		ID: 1,
		CustomUser: &models2.CustomUser{
			ID:         2,
			FirstName:  "Jane",
			SecondName: "Smith",
		},
		SchoolClass: "5A",
	}

	category := &models2.AgCategory{
		ID:   1,
		Name: "Sports",
	}

	// Sample activity groups
	ag1 := models2.Ag{
		ID:             1,
		Name:           "Basketball",
		MaxParticipant: 10,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		Supervisor:     specialist,
		AgCategoryID:   category.ID,
		AgCategory:     category,
		CreatedAt:      now,
		ModifiedAt:     now,
		Students:       []*models2.Student{}, // Use pointer slice
	}

	ag2 := models2.Ag{
		ID:             2,
		Name:           "Soccer",
		MaxParticipant: 15,
		IsOpenAg:       true,
		SupervisorID:   specialist.ID,
		Supervisor:     specialist,
		AgCategoryID:   category.ID,
		AgCategory:     category,
		CreatedAt:      now,
		ModifiedAt:     now,
		Students:       []*models2.Student{student}, // Use pointer slice
	}

	// Set up expectations - notice we're setting up ListStudentAgs to be called twice with the same parameters
	mockAgStore.On("ListStudentAgs", mock.Anything, int64(1)).Return([]models2.Ag{ag2}, nil).Twice()
	mockAgStore.On("ListAgs", mock.Anything, map[string]interface{}{"is_open": true, "active": true}).Return([]models2.Ag{ag1, ag2}, nil).Once()
	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(&ag1, nil).Once()
	mockAgStore.On("EnrollStudent", mock.Anything, int64(1), int64(1)).Return(nil).Once()

	// Use a standard context for testing
	standardContext := context.Background()

	// PHASE 1: Get Student's Enrolled AGs
	t.Run("1. Get Student's Enrolled AGs", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/student/1/ags", nil)

		// Set URL parameter with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getStudentAgs(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseAgs []models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseAgs))
		assert.Equal(t, "Soccer", responseAgs[0].Name)
	})

	// PHASE 2: Get Available AGs for Enrollment
	t.Run("2. Get Available AGs for Enrollment", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/student/available?student_id=1", nil)
		r = r.WithContext(standardContext)

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.getAvailableAgs(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var responseAgs []models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(responseAgs))
		assert.Equal(t, "Basketball", responseAgs[0].Name)
	})

	// PHASE 3: Enroll in a New AG
	t.Run("3. Enroll in a New AG", func(t *testing.T) {
		r := httptest.NewRequest("POST", "/1/enroll/1", nil)

		// Set URL parameters with standard context
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "1")
		rctx.URLParams.Add("studentId", "1")
		r = r.WithContext(context.WithValue(standardContext, chi.RouteCtxKey, rctx))

		w := httptest.NewRecorder()

		// Call the handler directly
		rs.enrollStudent(w, r)

		// Check response
		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)
		assert.True(t, response["success"].(bool))
		assert.Equal(t, float64(1), response["ag_id"])
		assert.Equal(t, float64(1), response["student_id"])
	})

	// Verify all expectations were met
	mockAgStore.AssertExpectations(t)
}
