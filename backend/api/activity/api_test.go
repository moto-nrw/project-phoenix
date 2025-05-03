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
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgStore implements the AgStore interface for testing
type MockAgStore struct {
	mock.Mock
}

// Category operations
func (m *MockAgStore) CreateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	args := m.Called(ctx, category)
	category.ID = 1 // Simulate auto-increment
	category.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockAgStore) GetAgCategoryByID(ctx context.Context, id int64) (*models2.AgCategory, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.AgCategory), args.Error(1)
}

func (m *MockAgStore) UpdateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockAgStore) DeleteAgCategory(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAgStore) ListAgCategories(ctx context.Context) ([]models2.AgCategory, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.AgCategory), args.Error(1)
}

// Activity Group operations
func (m *MockAgStore) CreateAg(ctx context.Context, ag *models2.Ag, studentIDs []int64, timeslots []*models2.AgTime) error {
	args := m.Called(ctx, ag, studentIDs, timeslots)
	ag.ID = 1 // Simulate auto-increment
	ag.CreatedAt = time.Now()
	ag.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockAgStore) GetAgByID(ctx context.Context, id int64) (*models2.Ag, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Ag), args.Error(1)
}

func (m *MockAgStore) UpdateAg(ctx context.Context, ag *models2.Ag) error {
	args := m.Called(ctx, ag)
	ag.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockAgStore) DeleteAg(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAgStore) ListAgs(ctx context.Context, filters map[string]interface{}) ([]models2.Ag, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models2.Ag), args.Error(1)
}

// Time slot operations
func (m *MockAgStore) CreateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	args := m.Called(ctx, agTime)
	agTime.ID = 1 // Simulate auto-increment
	agTime.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockAgStore) GetAgTimeByID(ctx context.Context, id int64) (*models2.AgTime, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.AgTime), args.Error(1)
}

func (m *MockAgStore) UpdateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	args := m.Called(ctx, agTime)
	return args.Error(0)
}

func (m *MockAgStore) DeleteAgTime(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockAgStore) ListAgTimes(ctx context.Context, agID int64) ([]models2.AgTime, error) {
	args := m.Called(ctx, agID)
	return args.Get(0).([]models2.AgTime), args.Error(1)
}

// Enrollment operations
func (m *MockAgStore) EnrollStudent(ctx context.Context, agID, studentID int64) error {
	args := m.Called(ctx, agID, studentID)
	return args.Error(0)
}

func (m *MockAgStore) UnenrollStudent(ctx context.Context, agID, studentID int64) error {
	args := m.Called(ctx, agID, studentID)
	return args.Error(0)
}

// ListEnrolledStudents modifies the mock to return pointers to students
func (m *MockAgStore) ListEnrolledStudents(ctx context.Context, agID int64) ([]*models2.Student, error) {
	args := m.Called(ctx, agID)
	return args.Get(0).([]*models2.Student), args.Error(1)
}

func (m *MockAgStore) ListStudentAgs(ctx context.Context, studentID int64) ([]models2.Ag, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models2.Ag), args.Error(1)
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

// MockTimespanStore implements the TimespanStore interface for testing
type MockTimespanStore struct {
	mock.Mock
}

func (m *MockTimespanStore) CreateTimespan(ctx context.Context, startTime time.Time, endTime *time.Time) (*models2.Timespan, error) {
	args := m.Called(ctx, startTime, endTime)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Timespan), args.Error(1)
}

func (m *MockTimespanStore) GetTimespan(ctx context.Context, id int64) (*models2.Timespan, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Timespan), args.Error(1)
}

func (m *MockTimespanStore) UpdateTimespanEndTime(ctx context.Context, id int64, endTime time.Time) error {
	args := m.Called(ctx, id, endTime)
	return args.Error(0)
}

func (m *MockTimespanStore) DeleteTimespan(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// setupTestAPI creates a test API with mocked dependencies
func setupTestAPI() (*Resource, *MockAgStore, *MockAuthTokenStore, *MockTimespanStore) {
	mockAgStore := new(MockAgStore)
	mockAuthStore := new(MockAuthTokenStore)
	mockTimespanStore := new(MockTimespanStore)
	resource := NewResource(mockAgStore, mockAuthStore, mockTimespanStore)
	return resource, mockAgStore, mockAuthStore, mockTimespanStore
}

// TestListCategories tests the listCategories handler
func TestListCategories(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	testCategories := []models2.AgCategory{
		{
			ID:   1,
			Name: "Category 1",
		},
		{
			ID:   2,
			Name: "Category 2",
		},
	}

	mockAgStore.On("ListAgCategories", mock.Anything).Return(testCategories, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/categories", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listCategories(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseCategories []models2.AgCategory
	err := json.Unmarshal(w.Body.Bytes(), &responseCategories)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseCategories))
	assert.Equal(t, "Category 1", responseCategories[0].Name)
	assert.Equal(t, "Category 2", responseCategories[1].Name)

	mockAgStore.AssertExpectations(t)
}

// TestCreateCategory tests the createCategory handler
func TestCreateCategory(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	newCategory := &models2.AgCategory{
		Name: "New Category",
	}

	mockAgStore.On("CreateAgCategory", mock.Anything, mock.MatchedBy(func(c *models2.AgCategory) bool {
		return c.Name == "New Category"
	})).Return(nil)

	// Create test request
	categoryReq := &AgCategoryRequest{AgCategory: newCategory}
	body, _ := json.Marshal(categoryReq)
	r := httptest.NewRequest("POST", "/categories", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createCategory(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseCategory models2.AgCategory
	err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
	assert.NoError(t, err)
	assert.Equal(t, "New Category", responseCategory.Name)
	assert.Equal(t, int64(1), responseCategory.ID) // Auto-increment ID in mock

	mockAgStore.AssertExpectations(t)
}

// TestGetCategory tests the getCategory handler
func TestGetCategory(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	category := &models2.AgCategory{
		ID:   1,
		Name: "Test Category",
	}

	mockAgStore.On("GetAgCategoryByID", mock.Anything, int64(1)).Return(category, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/categories/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getCategory(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseCategory models2.AgCategory
	err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
	assert.NoError(t, err)
	assert.Equal(t, "Test Category", responseCategory.Name)
	assert.Equal(t, int64(1), responseCategory.ID)

	mockAgStore.AssertExpectations(t)
}

// TestListAgs tests the listAgs handler
func TestListAgs(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	testAgs := []models2.Ag{
		{
			ID:             1,
			Name:           "Activity Group 1",
			MaxParticipant: 10,
			AgCategoryID:   1,
			SupervisorID:   1,
		},
		{
			ID:             2,
			Name:           "Activity Group 2",
			MaxParticipant: 15,
			AgCategoryID:   2,
			SupervisorID:   2,
		},
	}

	mockAgStore.On("ListAgs", mock.Anything, mock.Anything).Return(testAgs, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listAgs(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseAgs []models2.Ag
	err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseAgs))
	assert.Equal(t, "Activity Group 1", responseAgs[0].Name)
	assert.Equal(t, "Activity Group 2", responseAgs[1].Name)

	mockAgStore.AssertExpectations(t)
}

// TestCreateAg tests the createAg handler
func TestCreateAg(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	now := time.Now()
	timespan := &models2.Timespan{
		ID:        1,
		StartTime: now,
	}

	// Important: Set AgID to 0 for the test - it will be set to the AG's ID after creation
	timeslot := &models2.AgTime{
		Weekday:    "Monday",
		TimespanID: 1,
		Timespan:   timespan,
		AgID:       0, // Set this to 0 instead of nil
	}

	newAg := &models2.Ag{
		Name:           "New Activity Group",
		MaxParticipant: 10,
		SupervisorID:   1,
		AgCategoryID:   1,
	}

	createdAg := &models2.Ag{
		ID:             1,
		Name:           "New Activity Group",
		MaxParticipant: 10,
		SupervisorID:   1,
		AgCategoryID:   1,
		Times:          []*models2.AgTime{timeslot},
		CreatedAt:      now,
		ModifiedAt:     now,
	}

	// Use empty slices, not nil
	studentIDs := []int64{}
	timeslots := []*models2.AgTime{}

	// Set up expectation without timeslots for now
	mockAgStore.On("CreateAg",
		mock.Anything,
		mock.MatchedBy(func(a *models2.Ag) bool {
			return a.Name == "New Activity Group" && a.MaxParticipant == 10
		}),
		studentIDs,
		timeslots,
	).Return(nil)

	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(createdAg, nil)

	// Create test request
	agReq := &AgRequest{
		Ag:         newAg,
		StudentIDs: studentIDs,
		Timeslots:  timeslots,
	}

	body, _ := json.Marshal(agReq)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createAg(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseAg models2.Ag
	err := json.Unmarshal(w.Body.Bytes(), &responseAg)
	assert.NoError(t, err)
	assert.Equal(t, "New Activity Group", responseAg.Name)
	assert.Equal(t, int64(1), responseAg.ID)
	assert.Equal(t, 1, len(responseAg.Times))

	mockAgStore.AssertExpectations(t)
}

// TestEnrollStudent tests the enrollStudent handler
func TestEnrollStudent(t *testing.T) {
	rs, mockAgStore, _, _ := setupTestAPI()

	// Setup test data
	ag := &models2.Ag{
		ID:             1,
		Name:           "Test Activity Group",
		MaxParticipant: 10,
		Students:       []*models2.Student{}, // Use pointer slice
	}

	mockAgStore.On("GetAgByID", mock.Anything, int64(1)).Return(ag, nil)
	mockAgStore.On("EnrollStudent", mock.Anything, int64(1), int64(1)).Return(nil)

	// Create test request
	r := httptest.NewRequest("POST", "/1/enroll/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	rctx.URLParams.Add("studentId", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
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

	mockAgStore.AssertExpectations(t)
}

// TestAddAgTime tests the addAgTime handler
func TestAddAgTime(t *testing.T) {
	rs, mockAgStore, _, mockTimespanStore := setupTestAPI()

	// Setup test data
	now := time.Now()
	timespan := &models2.Timespan{
		ID:        1,
		StartTime: now,
	}

	// Use the new request format with direct timespan creation
	createReq := &AgTimeCreateRequest{
		Weekday:   "Monday",
		StartTime: now,
		EndTime:   nil,
	}

	createdTime := &models2.AgTime{
		ID:         1,
		Weekday:    "Monday",
		TimespanID: 1,
		AgID:       1,
		Timespan:   timespan,
		CreatedAt:  now,
	}

	// Set up mock expectation for creating a timespan
	// Using mock.AnythingOfType instead of direct time value due to monotonic clock differences
	mockTimespanStore.On("CreateTimespan", mock.Anything, mock.AnythingOfType("time.Time"), mock.Anything).Return(timespan, nil)

	// Set up mock expectation for creating a time slot
	mockAgStore.On("CreateAgTime", mock.Anything, mock.MatchedBy(func(at *models2.AgTime) bool {
		return at.Weekday == "Monday" && at.TimespanID == 1 && at.AgID == 1
	})).Return(nil)

	// Set up mock expectation for getting the created time slot
	mockAgStore.On("GetAgTimeByID", mock.Anything, int64(1)).Return(createdTime, nil)

	// Create test request
	body, _ := json.Marshal(createReq)
	r := httptest.NewRequest("POST", "/1/times", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.addAgTime(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseTime models2.AgTime
	err := json.Unmarshal(w.Body.Bytes(), &responseTime)
	assert.NoError(t, err)
	assert.Equal(t, "Monday", responseTime.Weekday)
	assert.Equal(t, int64(1), responseTime.ID)
	assert.Equal(t, int64(1), responseTime.AgID)

	mockAgStore.AssertExpectations(t)
	mockTimespanStore.AssertExpectations(t)
}

// TestRouter tests the router configuration
func TestRouter(t *testing.T) {
	rs, _, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
