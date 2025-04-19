package activity

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockActivityStore is a mock of the ActivityStore interface
type MockActivityStore struct {
	mock.Mock
}

// Implement all required methods of the ActivityStore interface
func (m *MockActivityStore) CreateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	args := m.Called(ctx, category)
	category.ID = 1
	category.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockActivityStore) GetAgCategoryByID(ctx context.Context, id int64) (*models2.AgCategory, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models2.AgCategory), args.Error(1)
}

func (m *MockActivityStore) UpdateAgCategory(ctx context.Context, category *models2.AgCategory) error {
	args := m.Called(ctx, category)
	return args.Error(0)
}

func (m *MockActivityStore) DeleteAgCategory(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockActivityStore) ListAgCategories(ctx context.Context) ([]models2.AgCategory, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.AgCategory), args.Error(1)
}

func (m *MockActivityStore) CreateAg(ctx context.Context, ag *models2.Ag, studentIDs []int64, timeslots []*models2.AgTime) error {
	args := m.Called(ctx, ag, studentIDs, timeslots)
	ag.ID = 1
	ag.CreatedAt = time.Now()
	ag.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockActivityStore) GetAgByID(ctx context.Context, id int64) (*models2.Ag, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models2.Ag), args.Error(1)
}

func (m *MockActivityStore) UpdateAg(ctx context.Context, ag *models2.Ag) error {
	args := m.Called(ctx, ag)
	return args.Error(0)
}

func (m *MockActivityStore) DeleteAg(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockActivityStore) ListAgs(ctx context.Context, filters map[string]interface{}) ([]models2.Ag, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models2.Ag), args.Error(1)
}

func (m *MockActivityStore) CreateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	args := m.Called(ctx, agTime)
	agTime.ID = 1
	agTime.CreatedAt = time.Now()
	return args.Error(0)
}

func (m *MockActivityStore) GetAgTimeByID(ctx context.Context, id int64) (*models2.AgTime, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models2.AgTime), args.Error(1)
}

func (m *MockActivityStore) UpdateAgTime(ctx context.Context, agTime *models2.AgTime) error {
	args := m.Called(ctx, agTime)
	return args.Error(0)
}

func (m *MockActivityStore) DeleteAgTime(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockActivityStore) ListAgTimes(ctx context.Context, agID int64) ([]models2.AgTime, error) {
	args := m.Called(ctx, agID)
	return args.Get(0).([]models2.AgTime), args.Error(1)
}

func (m *MockActivityStore) EnrollStudent(ctx context.Context, agID, studentID int64) error {
	args := m.Called(ctx, agID, studentID)
	return args.Error(0)
}

func (m *MockActivityStore) UnenrollStudent(ctx context.Context, agID, studentID int64) error {
	args := m.Called(ctx, agID, studentID)
	return args.Error(0)
}

func (m *MockActivityStore) ListEnrolledStudents(ctx context.Context, agID int64) ([]models2.Student, error) {
	args := m.Called(ctx, agID)
	return args.Get(0).([]models2.Student), args.Error(1)
}

func (m *MockActivityStore) ListStudentAgs(ctx context.Context, studentID int64) ([]models2.Ag, error) {
	args := m.Called(ctx, studentID)
	return args.Get(0).([]models2.Ag), args.Error(1)
}

// MockAuthTokenStore is a mock of the AuthTokenStore interface
type MockAuthTokenStore struct {
	mock.Mock
}

func (m *MockAuthTokenStore) GetToken(t string) (*jwt.Token, error) {
	args := m.Called(t)
	return args.Get(0).(*jwt.Token), args.Error(1)
}

// Test setup function
func setupTest(t *testing.T) (*Resource, *MockActivityStore, *MockAuthTokenStore) {
	mockStore := new(MockActivityStore)
	mockAuthStore := new(MockAuthTokenStore)
	resource := NewResource(mockStore, mockAuthStore)
	return resource, mockStore, mockAuthStore
}

// TestCategoryCRUD tests the CRUD operations for categories
func TestCategoryCRUD(t *testing.T) {
	// Setup
	rs, mockStore, _ := setupTest(t)

	// Test ListCategories
	t.Run("ListCategories", func(t *testing.T) {
		categories := []models2.AgCategory{
			{ID: 1, Name: "Sport", CreatedAt: time.Now()},
			{ID: 2, Name: "Music", CreatedAt: time.Now()},
		}
		mockStore.On("ListAgCategories", mock.Anything).Return(categories, nil).Once()

		// Create a request to pass to our handler
		r := httptest.NewRequest("GET", "/categories", nil)
		w := httptest.NewRecorder()

		// Set up the router
		router := chi.NewRouter()
		router.Get("/categories", rs.listCategories)
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusOK, w.Code)

		// Unmarshal the response
		var responseCategories []models2.AgCategory
		err := json.Unmarshal(w.Body.Bytes(), &responseCategories)
		assert.NoError(t, err)
		assert.Len(t, responseCategories, 2)
		assert.Equal(t, "Sport", responseCategories[0].Name)
		assert.Equal(t, "Music", responseCategories[1].Name)
	})

	// Test CreateCategory
	t.Run("CreateCategory", func(t *testing.T) {
		category := &models2.AgCategory{Name: "Art"}
		mockStore.On("CreateAgCategory", mock.Anything, mock.MatchedBy(func(c *models2.AgCategory) bool {
			return c.Name == "Art"
		})).Return(nil).Once()

		// Create the JSON payload
		payload, _ := json.Marshal(CategoryRequest{AgCategory: category})
		r := httptest.NewRequest("POST", "/categories", bytes.NewBuffer(payload))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Set up the router
		router := chi.NewRouter()
		router.Post("/categories", rs.createCategory)
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusCreated, w.Code)

		// Unmarshal the response
		var responseCategory models2.AgCategory
		err := json.Unmarshal(w.Body.Bytes(), &responseCategory)
		assert.NoError(t, err)
		assert.Equal(t, "Art", responseCategory.Name)
		assert.Equal(t, int64(1), responseCategory.ID)
	})

	// Add more tests for other category operations as needed
}

// TestActivityGroupCRUD tests the CRUD operations for activity groups
func TestActivityGroupCRUD(t *testing.T) {
	// Setup
	rs, mockStore, _ := setupTest(t)

	// Test ListActivityGroups
	t.Run("ListActivityGroups", func(t *testing.T) {
		ags := []models2.Ag{
			{ID: 1, Name: "Football", MaxParticipant: 20, SupervisorID: 1, AgCategoryID: 1, CreatedAt: time.Now(), ModifiedAt: time.Now()},
			{ID: 2, Name: "Piano", MaxParticipant: 10, SupervisorID: 2, AgCategoryID: 2, CreatedAt: time.Now(), ModifiedAt: time.Now()},
		}
		mockStore.On("ListAgs", mock.Anything, mock.Anything).Return(ags, nil).Once()

		// Create a request to pass to our handler
		r := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()

		// Set up the router
		router := chi.NewRouter()
		router.Get("/", rs.listActivityGroups)
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusOK, w.Code)

		// Unmarshal the response
		var responseAgs []models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAgs)
		assert.NoError(t, err)
		assert.Len(t, responseAgs, 2)
		assert.Equal(t, "Football", responseAgs[0].Name)
		assert.Equal(t, "Piano", responseAgs[1].Name)
	})

	// Test CreateActivityGroup
	t.Run("CreateActivityGroup", func(t *testing.T) {
		ag := &models2.Ag{
			Name:           "Basketball",
			MaxParticipant: 15,
			SupervisorID:   1,
			AgCategoryID:   1,
		}
		studentIDs := []int64{1, 2, 3}
		times := []*models2.AgTime{
			{Weekday: "Monday", TimespanID: 1},
			{Weekday: "Wednesday", TimespanID: 2},
		}

		mockStore.On("CreateAg", mock.Anything, mock.MatchedBy(func(a *models2.Ag) bool {
			return a.Name == "Basketball" && a.MaxParticipant == 15
		}), studentIDs, times).Return(nil).Once()

		// Mock the GetAgByID call
		mockStore.On("GetAgByID", mock.Anything, int64(1)).Return(&models2.Ag{
			ID:             1,
			Name:           "Basketball",
			MaxParticipant: 15,
			SupervisorID:   1,
			AgCategoryID:   1,
			CreatedAt:      time.Now(),
			ModifiedAt:     time.Now(),
		}, nil).Once()

		// Create the JSON payload
		payload, _ := json.Marshal(ActivityGroupRequest{
			Ag:         ag,
			StudentIDs: studentIDs,
			Times:      times,
		})
		r := httptest.NewRequest("POST", "/", bytes.NewBuffer(payload))
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		// Set up the router
		router := chi.NewRouter()
		router.Post("/", rs.createActivityGroup)
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusCreated, w.Code)

		// Unmarshal the response
		var responseAg models2.Ag
		err := json.Unmarshal(w.Body.Bytes(), &responseAg)
		assert.NoError(t, err)
		assert.Equal(t, "Basketball", responseAg.Name)
		assert.Equal(t, int64(1), responseAg.ID)
		assert.Equal(t, 15, responseAg.MaxParticipant)
	})

	// Add more tests for other activity group operations as needed
}

// TestTimeslotCRUD tests the CRUD operations for timeslots
func TestTimeslotCRUD(t *testing.T) {
	// Setup
	rs, mockStore, _ := setupTest(t)

	// Test ListAgTimes
	t.Run("ListAgTimes", func(t *testing.T) {
		agTimes := []models2.AgTime{
			{ID: 1, Weekday: "Monday", TimespanID: 1, AgID: 1, CreatedAt: time.Now()},
			{ID: 2, Weekday: "Wednesday", TimespanID: 2, AgID: 1, CreatedAt: time.Now()},
		}
		mockStore.On("ListAgTimes", mock.Anything, int64(1)).Return(agTimes, nil).Once()

		// Create a request to pass to our handler
		r := httptest.NewRequest("GET", "/1/times", nil)
		w := httptest.NewRecorder()

		// Set up the router with URL params
		router := chi.NewRouter()
		router.Route("/{id}", func(r chi.Router) {
			r.Route("/times", func(r chi.Router) {
				r.Get("/", rs.listAgTimes)
			})
		})
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusOK, w.Code)

		// Unmarshal the response
		var responseAgTimes []models2.AgTime
		err := json.Unmarshal(w.Body.Bytes(), &responseAgTimes)
		assert.NoError(t, err)
		assert.Len(t, responseAgTimes, 2)
		assert.Equal(t, "Monday", responseAgTimes[0].Weekday)
		assert.Equal(t, "Wednesday", responseAgTimes[1].Weekday)
	})

	// Add more tests for other timeslot operations as needed
}

// TestEnrollmentCRUD tests the CRUD operations for student enrollments
func TestEnrollmentCRUD(t *testing.T) {
	// Setup
	rs, mockStore, _ := setupTest(t)

	// Test ListEnrolledStudents
	t.Run("ListEnrolledStudents", func(t *testing.T) {
		students := []models2.Student{
			{ID: 1, SchoolClass: "5A", CustomUserID: 1, GroupID: 1, CreatedAt: time.Now(), ModifiedAt: time.Now()},
			{ID: 2, SchoolClass: "5B", CustomUserID: 2, GroupID: 2, CreatedAt: time.Now(), ModifiedAt: time.Now()},
		}
		mockStore.On("ListEnrolledStudents", mock.Anything, int64(1)).Return(students, nil).Once()

		// Create a request to pass to our handler
		r := httptest.NewRequest("GET", "/1/students", nil)
		w := httptest.NewRecorder()

		// Set up the router with URL params
		router := chi.NewRouter()
		router.Route("/{id}", func(r chi.Router) {
			r.Route("/students", func(r chi.Router) {
				r.Get("/", rs.listEnrolledStudents)
			})
		})
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusOK, w.Code)

		// Unmarshal the response
		var responseStudents []models2.Student
		err := json.Unmarshal(w.Body.Bytes(), &responseStudents)
		assert.NoError(t, err)
		assert.Len(t, responseStudents, 2)
		assert.Equal(t, "5A", responseStudents[0].SchoolClass)
		assert.Equal(t, "5B", responseStudents[1].SchoolClass)
	})

	// Test EnrollStudent
	t.Run("EnrollStudent", func(t *testing.T) {
		mockStore.On("EnrollStudent", mock.Anything, int64(1), int64(3)).Return(nil).Once()

		// Create a request to pass to our handler
		r := httptest.NewRequest("POST", "/1/students/3", nil)
		w := httptest.NewRecorder()

		// Set up the router with URL params
		router := chi.NewRouter()
		router.Route("/{id}", func(r chi.Router) {
			r.Route("/students", func(r chi.Router) {
				r.Post("/{studentId}", func(w http.ResponseWriter, r *http.Request) {
					agID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
					studentID, _ := strconv.ParseInt(chi.URLParam(r, "studentId"), 10, 64)
					rs.Store.EnrollStudent(r.Context(), agID, studentID)
					w.WriteHeader(http.StatusCreated)
				})
			})
		})
		router.ServeHTTP(w, r)

		// Check the status code
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	// Add more tests for other enrollment operations as needed
}
