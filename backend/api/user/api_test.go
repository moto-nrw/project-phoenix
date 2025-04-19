package user

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock UserStore
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetCustomUserByID(ctx context.Context, id int64) (*models.CustomUser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CustomUser), args.Error(1)
}

func (m *MockUserStore) GetCustomUserByTagID(ctx context.Context, tagID string) (*models.CustomUser, error) {
	args := m.Called(ctx, tagID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.CustomUser), args.Error(1)
}

func (m *MockUserStore) CreateCustomUser(ctx context.Context, user *models.CustomUser) error {
	args := m.Called(ctx, user)
	user.ID = 1 // Simulate auto-increment
	return args.Error(0)
}

func (m *MockUserStore) UpdateCustomUser(ctx context.Context, user *models.CustomUser) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserStore) DeleteCustomUser(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserStore) UpdateTagID(ctx context.Context, userID int64, tagID string) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

func (m *MockUserStore) ListCustomUsers(ctx context.Context) ([]models.CustomUser, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.CustomUser), args.Error(1)
}

func (m *MockUserStore) GetSpecialistByID(ctx context.Context, id int64) (*models.PedagogicalSpecialist, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PedagogicalSpecialist), args.Error(1)
}

func (m *MockUserStore) GetSpecialistByUserID(ctx context.Context, userID int64) (*models.PedagogicalSpecialist, error) {
	args := m.Called(ctx, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.PedagogicalSpecialist), args.Error(1)
}

func (m *MockUserStore) CreateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error {
	args := m.Called(ctx, specialist)
	specialist.ID = 1 // Simulate auto-increment
	return args.Error(0)
}

func (m *MockUserStore) UpdateSpecialist(ctx context.Context, specialist *models.PedagogicalSpecialist) error {
	args := m.Called(ctx, specialist)
	return args.Error(0)
}

func (m *MockUserStore) DeleteSpecialist(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserStore) ListSpecialists(ctx context.Context) ([]models.PedagogicalSpecialist, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.PedagogicalSpecialist), args.Error(1)
}

func (m *MockUserStore) ListSpecialistsWithoutSupervision(ctx context.Context) ([]models.PedagogicalSpecialist, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.PedagogicalSpecialist), args.Error(1)
}

func (m *MockUserStore) CreateDevice(ctx context.Context, device *models.Device) error {
	args := m.Called(ctx, device)
	device.ID = 1 // Simulate auto-increment
	return args.Error(0)
}

func (m *MockUserStore) GetDeviceByID(ctx context.Context, id int64) (*models.Device, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockUserStore) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*models.Device, error) {
	args := m.Called(ctx, deviceID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Device), args.Error(1)
}

func (m *MockUserStore) DeleteDevice(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockUserStore) ListDevicesByUserID(ctx context.Context, userID int64) ([]models.Device, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]models.Device), args.Error(1)
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

func setupTestAPI() (*Resource, *MockUserStore, *MockAuthTokenStore) {
	mockUserStore := new(MockUserStore)
	mockAuthStore := new(MockAuthTokenStore)
	resource := NewResource(mockUserStore, mockAuthStore)
	return resource, mockUserStore, mockAuthStore
}

func TestListUsersPublic(t *testing.T) {
	rs, mockUserStore, _ := setupTestAPI()

	// Setup test data
	users := []models.CustomUser{
		{ID: 1, FirstName: "John", SecondName: "Doe"},
		{ID: 2, FirstName: "Jane", SecondName: "Smith"},
	}

	mockUserStore.On("ListCustomUsers", mock.Anything).Return(users, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/public/users", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listUsersPublic(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseUsers []map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseUsers)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseUsers))
	assert.Equal(t, float64(1), responseUsers[0]["id"])
	assert.Equal(t, "John", responseUsers[0]["first_name"])
	assert.Equal(t, "Doe", responseUsers[0]["second_name"])

	mockUserStore.AssertExpectations(t)
}

func TestCreateUser(t *testing.T) {
	rs, mockUserStore, _ := setupTestAPI()

	// Create test user
	user := models.CustomUser{
		FirstName:  "New",
		SecondName: "User",
	}

	mockUserStore.On("CreateCustomUser", mock.Anything, mock.MatchedBy(func(u *models.CustomUser) bool {
		return u.FirstName == "New" && u.SecondName == "User"
	})).Return(nil)

	// Create test request
	body, _ := json.Marshal(UserRequest{CustomUser: &user})
	r := httptest.NewRequest("POST", "/users", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createUser(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseUser models.CustomUser
	err := json.Unmarshal(w.Body.Bytes(), &responseUser)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseUser.ID)
	assert.Equal(t, "New", responseUser.FirstName)
	assert.Equal(t, "User", responseUser.SecondName)

	mockUserStore.AssertExpectations(t)
}

func TestChangeTagID(t *testing.T) {
	rs, mockUserStore, _ := setupTestAPI()

	// Setup test data
	req := ChangeTagIDRequest{
		UserID: 1,
		TagID:  "ABCDE12345",
	}

	mockUserStore.On("UpdateTagID", mock.Anything, int64(1), "ABCDE12345").Return(nil)

	// Create test request
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/change-tag-id", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.changeTagID(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]bool
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"])

	mockUserStore.AssertExpectations(t)
}

func TestCreateSpecialist(t *testing.T) {
	rs, mockUserStore, _ := setupTestAPI()

	// Create user for linking
	user := &models.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
	}

	// Create test specialist
	specialist := models.PedagogicalSpecialist{
		Role:          "Teacher",
		CustomUserID:  1,
		UserID:        123,
		IsPasswordOTP: true,
	}

	mockUserStore.On("CreateSpecialist", mock.Anything, mock.MatchedBy(func(s *models.PedagogicalSpecialist) bool {
		return s.Role == "Teacher" && s.CustomUserID == 1
	})).Return(nil)

	mockUserStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(&models.PedagogicalSpecialist{
		ID:            1,
		Role:          "Teacher",
		CustomUserID:  1,
		UserID:        123,
		IsPasswordOTP: true,
		CustomUser:    user,
	}, nil)

	// Create test request
	body, _ := json.Marshal(SpecialistRequest{PedagogicalSpecialist: &specialist})
	r := httptest.NewRequest("POST", "/specialists", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createSpecialist(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseSpecialist models.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), responseSpecialist.ID)
	assert.Equal(t, "Teacher", responseSpecialist.Role)
	assert.Equal(t, int64(1), responseSpecialist.CustomUserID)
	assert.Equal(t, "John", responseSpecialist.CustomUser.FirstName)

	mockUserStore.AssertExpectations(t)
}

func TestProcessTagScan(t *testing.T) {
	rs, mockUserStore, _ := setupTestAPI()

	// Setup test data
	req := TagScanRequest{
		TagID:    "ABCDE12345",
		DeviceID: "DEVICE001",
	}

	user := &models.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
		TagID:      strPtr("ABCDE12345"),
	}

	mockUserStore.On("GetCustomUserByTagID", mock.Anything, "ABCDE12345").Return(user, nil)

	// Create test request
	body, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/process-tag-scan", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.processTagScan(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, float64(1), response["user_id"].(float64))

	mockUserStore.AssertExpectations(t)
}

func TestRouter(t *testing.T) {
	rs, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)

	// Check routes by testing if they match the expected patterns
	routes := getRoutes(router)

	// Check public routes
	assert.Contains(t, routes, "/public/users")
	assert.Contains(t, routes, "/public/supervisors")

	// Check protected routes
	assert.Contains(t, routes, "/users/")
	assert.Contains(t, routes, "/specialists/")
	assert.Contains(t, routes, "/change-tag-id")
	assert.Contains(t, routes, "/process-tag-scan")
}

// Helper function to extract routes from a chi router
func getRoutes(r chi.Router) []string {
	var routes []string
	walkFunc := func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
		routes = append(routes, route)
		return nil
	}
	if err := chi.Walk(r, walkFunc); err != nil {
		return nil
	}
	return routes
}

// Helper function to create string pointer
func strPtr(s string) *string {
	return &s
}
