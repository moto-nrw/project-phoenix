package student

import (
	"context"
	"encoding/json"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	models2 "github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock StudentStore
type MockStudentStore struct {
	mock.Mock
}

func (m *MockStudentStore) GetStudentByID(ctx context.Context, id int64) (*models2.Student, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Student), args.Error(1)
}

func (m *MockStudentStore) GetStudentByCustomUserID(ctx context.Context, customUserID int64) (*models2.Student, error) {
	args := m.Called(ctx, customUserID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Student), args.Error(1)
}

func (m *MockStudentStore) CreateStudent(ctx context.Context, student *models2.Student) error {
	args := m.Called(ctx, student)
	student.ID = 1 // Simulate auto-increment
	student.CreatedAt = time.Now()
	student.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockStudentStore) UpdateStudent(ctx context.Context, student *models2.Student) error {
	args := m.Called(ctx, student)
	return args.Error(0)
}

func (m *MockStudentStore) DeleteStudent(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStudentStore) ListStudents(ctx context.Context, filters map[string]interface{}) ([]models2.Student, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models2.Student), args.Error(1)
}

func (m *MockStudentStore) UpdateStudentLocation(ctx context.Context, id int64, locations map[string]bool) error {
	args := m.Called(ctx, id, locations)
	return args.Error(0)
}

func (m *MockStudentStore) CreateStudentVisit(ctx context.Context, studentID, roomID, timespanID int64) (*models2.Visit, error) {
	args := m.Called(ctx, studentID, roomID, timespanID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Visit), args.Error(1)
}

func (m *MockStudentStore) GetStudentVisits(ctx context.Context, studentID int64, date *time.Time) ([]models2.Visit, error) {
	args := m.Called(ctx, studentID, date)
	return args.Get(0).([]models2.Visit), args.Error(1)
}

func (m *MockStudentStore) GetRoomVisits(ctx context.Context, roomID int64, date *time.Time, active bool) ([]models2.Visit, error) {
	args := m.Called(ctx, roomID, date, active)
	return args.Get(0).([]models2.Visit), args.Error(1)
}

func (m *MockStudentStore) GetStudentAsList(ctx context.Context, id int64) (*models2.StudentList, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.StudentList), args.Error(1)
}

func (m *MockStudentStore) CreateFeedback(ctx context.Context, studentID int64, feedbackValue string, mensaFeedback bool) (*models2.Feedback, error) {
	args := m.Called(ctx, studentID, feedbackValue, mensaFeedback)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.Feedback), args.Error(1)
}

func (m *MockStudentStore) GetCombinedGroupVisits(ctx context.Context, combinedGroupID int64, date *time.Time, active bool) ([]models2.Visit, error) {
	args := m.Called(ctx, combinedGroupID, date, active)
	return args.Get(0).([]models2.Visit), args.Error(1)
}

func (m *MockStudentStore) GetRoomOccupancyByDeviceID(ctx context.Context, deviceID string) (*models2.RoomOccupancyDetail, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.RoomOccupancyDetail), args.Error(1)
}

// Mock AuthTokenStore
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

func setupTestAPI() (*Resource, *MockStudentStore, *MockAuthTokenStore) {
	mockStudentStore := new(MockStudentStore)
	mockAuthStore := new(MockAuthTokenStore)
	resource := NewResource(mockStudentStore, mockAuthStore)
	return resource, mockStudentStore, mockAuthStore
}

func TestListStudents(t *testing.T) {
	rs, mockStudentStore, _ := setupTestAPI()

	// Setup test data
	customUser1 := &models2.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
	}

	customUser2 := &models2.CustomUser{
		ID:         2,
		FirstName:  "Jane",
		SecondName: "Smith",
	}

	group1 := &models2.Group{
		ID:   1,
		Name: "Group 1",
	}

	testStudents := []models2.Student{
		{
			ID:           1,
			SchoolClass:  "1A",
			CustomUserID: 1,
			CustomUser:   customUser1,
			GroupID:      1,
			Group:        group1,
		},
		{
			ID:           2,
			SchoolClass:  "1B",
			CustomUserID: 2,
			CustomUser:   customUser2,
			GroupID:      1,
			Group:        group1,
		},
	}

	mockStudentStore.On("ListStudents", mock.Anything, mock.Anything).Return(testStudents, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listStudents(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseStudents []models2.Student
	err := json.Unmarshal(w.Body.Bytes(), &responseStudents)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseStudents))
	assert.Equal(t, "1A", responseStudents[0].SchoolClass)
	assert.Equal(t, "John", responseStudents[0].CustomUser.FirstName)
	assert.Equal(t, "1B", responseStudents[1].SchoolClass)

	mockStudentStore.AssertExpectations(t)
}

func TestGetStudent(t *testing.T) {
	rs, mockStudentStore, _ := setupTestAPI()

	// Setup test data
	customUser := &models2.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
	}

	group := &models2.Group{
		ID:   1,
		Name: "Group 1",
	}

	testStudent := &models2.Student{
		ID:           1,
		SchoolClass:  "1A",
		CustomUserID: 1,
		CustomUser:   customUser,
		GroupID:      1,
		Group:        group,
	}

	mockStudentStore.On("GetStudentByID", mock.Anything, int64(1)).Return(testStudent, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getStudent(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseStudent models2.Student
	err := json.Unmarshal(w.Body.Bytes(), &responseStudent)
	assert.NoError(t, err)
	assert.Equal(t, "1A", responseStudent.SchoolClass)
	assert.Equal(t, int64(1), responseStudent.CustomUserID)

	mockStudentStore.AssertExpectations(t)
}

func TestRouter(t *testing.T) {
	rs, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
