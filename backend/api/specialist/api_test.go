package specialist

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

// MockSpecialistStore implements the SpecialistStore interface for testing
type MockSpecialistStore struct {
	mock.Mock
}

// Specialist operations
func (m *MockSpecialistStore) CreateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist, tagID *string, accountID *int64) error {
	args := m.Called(ctx, specialist, tagID, accountID)
	specialist.ID = 1 // Simulate auto-increment
	specialist.CreatedAt = time.Now()
	specialist.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockSpecialistStore) GetSpecialistByID(ctx context.Context, id int64) (*models2.PedagogicalSpecialist, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.PedagogicalSpecialist), args.Error(1)
}

func (m *MockSpecialistStore) UpdateSpecialist(ctx context.Context, specialist *models2.PedagogicalSpecialist) error {
	args := m.Called(ctx, specialist)
	specialist.ModifiedAt = time.Now()
	return args.Error(0)
}

func (m *MockSpecialistStore) DeleteSpecialist(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockSpecialistStore) ListSpecialists(ctx context.Context, filters map[string]interface{}) ([]models2.PedagogicalSpecialist, error) {
	args := m.Called(ctx, filters)
	return args.Get(0).([]models2.PedagogicalSpecialist), args.Error(1)
}

func (m *MockSpecialistStore) ListSpecialistsWithoutSupervision(ctx context.Context) ([]models2.PedagogicalSpecialist, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models2.PedagogicalSpecialist), args.Error(1)
}

// Group supervision operations
func (m *MockSpecialistStore) AssignToGroup(ctx context.Context, specialistID, groupID int64) error {
	args := m.Called(ctx, specialistID, groupID)
	return args.Error(0)
}

func (m *MockSpecialistStore) RemoveFromGroup(ctx context.Context, specialistID, groupID int64) error {
	args := m.Called(ctx, specialistID, groupID)
	return args.Error(0)
}

func (m *MockSpecialistStore) ListAssignedGroups(ctx context.Context, specialistID int64) ([]models2.Group, error) {
	args := m.Called(ctx, specialistID)
	return args.Get(0).([]models2.Group), args.Error(1)
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

// MockUserStore implements the UserStore interface for testing
type MockUserStore struct {
	mock.Mock
}

func (m *MockUserStore) GetCustomUserByID(ctx context.Context, id int64) (*models2.CustomUser, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models2.CustomUser), args.Error(1)
}

func (m *MockUserStore) CreateCustomUser(ctx context.Context, user *models2.CustomUser) error {
	args := m.Called(ctx, user)
	user.ID = 1 // Simulate auto-increment
	return args.Error(0)
}

func (m *MockUserStore) UpdateCustomUser(ctx context.Context, user *models2.CustomUser) error {
	args := m.Called(ctx, user)
	return args.Error(0)
}

func (m *MockUserStore) UpdateTagID(ctx context.Context, userID int64, tagID string) error {
	args := m.Called(ctx, userID, tagID)
	return args.Error(0)
}

// setupTestAPI creates a test API with mocked dependencies
func setupTestAPI() (*Resource, *MockSpecialistStore, *MockAuthTokenStore, *MockUserStore) {
	mockSpecialistStore := new(MockSpecialistStore)
	mockAuthStore := new(MockAuthTokenStore)
	mockUserStore := new(MockUserStore)
	resource := NewResource(mockSpecialistStore, mockAuthStore, mockUserStore)
	return resource, mockSpecialistStore, mockAuthStore, mockUserStore
}

// TestListSpecialists tests the listSpecialists handler
func TestListSpecialists(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	// Setup test data
	testSpecialists := []models2.PedagogicalSpecialist{
		{
			ID:     1,
			Role:   "Teacher",
			UserID: 1,
			CustomUser: &models2.CustomUser{
				ID:         1,
				FirstName:  "John",
				SecondName: "Doe",
			},
		},
		{
			ID:     2,
			Role:   "Principal",
			UserID: 2,
			CustomUser: &models2.CustomUser{
				ID:         2,
				FirstName:  "Jane",
				SecondName: "Smith",
			},
		},
	}

	mockSpecialistStore.On("ListSpecialists", mock.Anything, mock.Anything).Return(testSpecialists, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.listSpecialists(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseSpecialists []models2.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialists)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(responseSpecialists))
	assert.Equal(t, "Teacher", responseSpecialists[0].Role)
	assert.Equal(t, "Principal", responseSpecialists[1].Role)

	mockSpecialistStore.AssertExpectations(t)
}

// TestCreateSpecialist tests the createSpecialist handler
func TestCreateSpecialist(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	// Setup test data
	newSpecialist := &models2.PedagogicalSpecialist{
		Role: "Teacher",
		CustomUser: &models2.CustomUser{
			FirstName:  "New",
			SecondName: "Teacher",
		},
	}

	createdSpecialist := &models2.PedagogicalSpecialist{
		ID:     1,
		Role:   "Teacher",
		UserID: 1,
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "New",
			SecondName: "Teacher",
		},
		CreatedAt:  time.Now(),
		ModifiedAt: time.Now(),
	}

	tagID := "NEWTAG001"

	mockSpecialistStore.On("CreateSpecialist", mock.Anything, mock.MatchedBy(func(s *models2.PedagogicalSpecialist) bool {
		return s.Role == "Teacher"
	}), mock.MatchedBy(func(t *string) bool {
		return *t == tagID
	}), mock.Anything).Return(nil)

	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(createdSpecialist, nil)

	// Create test request
	specialistReq := &SpecialistRequest{
		PedagogicalSpecialist: newSpecialist,
		TagID:                 tagID,
	}
	body, _ := json.Marshal(specialistReq)
	r := httptest.NewRequest("POST", "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.createSpecialist(w, r)

	// Check response
	assert.Equal(t, http.StatusCreated, w.Code)

	var responseSpecialist models2.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
	assert.NoError(t, err)
	assert.Equal(t, "Teacher", responseSpecialist.Role)
	assert.Equal(t, int64(1), responseSpecialist.ID)

	mockSpecialistStore.AssertExpectations(t)
}

// TestGetSpecialist tests the getSpecialist handler
func TestGetSpecialist(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	// Setup test data
	specialist := &models2.PedagogicalSpecialist{
		ID:     1,
		Role:   "Teacher",
		UserID: 1,
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Doe",
		},
	}

	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(specialist, nil)

	// Create test request
	r := httptest.NewRequest("GET", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.getSpecialist(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseSpecialist models2.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
	assert.NoError(t, err)
	assert.Equal(t, "Teacher", responseSpecialist.Role)
	assert.Equal(t, int64(1), responseSpecialist.ID)

	mockSpecialistStore.AssertExpectations(t)
}

// TestUpdateSpecialist tests the updateSpecialist handler
func TestUpdateSpecialist(t *testing.T) {
	rs, mockSpecialistStore, _, mockUserStore := setupTestAPI()

	// Setup test data
	updateData := &models2.PedagogicalSpecialist{
		ID:     1,
		Role:   "Principal",
		UserID: 1,
		CustomUser: &models2.CustomUser{
			ID: 1,
		},
	}

	updatedSpecialist := &models2.PedagogicalSpecialist{
		ID:     1,
		Role:   "Principal",
		UserID: 1,
		CustomUser: &models2.CustomUser{
			ID:         1,
			FirstName:  "John",
			SecondName: "Updated",
		},
		ModifiedAt: time.Now(),
	}

	customUser := &models2.CustomUser{
		ID:         1,
		FirstName:  "John",
		SecondName: "Doe",
	}

	mockSpecialistStore.On("UpdateSpecialist", mock.Anything, mock.MatchedBy(func(s *models2.PedagogicalSpecialist) bool {
		return s.ID == 1 && s.Role == "Principal"
	})).Return(nil)

	mockUserStore.On("GetCustomUserByID", mock.Anything, int64(1)).Return(customUser, nil)
	mockUserStore.On("UpdateCustomUser", mock.Anything, mock.Anything).Return(nil)

	mockSpecialistStore.On("GetSpecialistByID", mock.Anything, int64(1)).Return(updatedSpecialist, nil)

	// Create test request
	specialistReq := &SpecialistRequest{
		PedagogicalSpecialist: updateData,
		SecondName:            "Updated",
	}
	body, _ := json.Marshal(specialistReq)
	r := httptest.NewRequest("PUT", "/1", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.updateSpecialist(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var responseSpecialist models2.PedagogicalSpecialist
	err := json.Unmarshal(w.Body.Bytes(), &responseSpecialist)
	assert.NoError(t, err)
	assert.Equal(t, "Principal", responseSpecialist.Role)
	assert.Equal(t, "Updated", responseSpecialist.CustomUser.SecondName)

	mockSpecialistStore.AssertExpectations(t)
	mockUserStore.AssertExpectations(t)
}

// TestDeleteSpecialist tests the deleteSpecialist handler
func TestDeleteSpecialist(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	mockSpecialistStore.On("DeleteSpecialist", mock.Anything, int64(1)).Return(nil)

	// Create test request
	r := httptest.NewRequest("DELETE", "/1", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.deleteSpecialist(w, r)

	// Check response
	assert.Equal(t, http.StatusNoContent, w.Code)

	mockSpecialistStore.AssertExpectations(t)
}

// TestAssignToGroup tests the assignToGroup handler
func TestAssignToGroup(t *testing.T) {
	rs, mockSpecialistStore, _, _ := setupTestAPI()

	mockSpecialistStore.On("AssignToGroup", mock.Anything, int64(1), int64(2)).Return(nil)

	// Create test request
	r := httptest.NewRequest("POST", "/1/groups/2", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "1")
	rctx.URLParams.Add("groupId", "2")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	// Call the handler directly
	rs.assignToGroup(w, r)

	// Check response
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response["success"].(bool))
	assert.Equal(t, float64(1), response["specialist_id"])
	assert.Equal(t, float64(2), response["group_id"])

	mockSpecialistStore.AssertExpectations(t)
}

// TestRouter tests the router configuration
func TestRouter(t *testing.T) {
	rs, _, _, _ := setupTestAPI()
	router := rs.Router()

	// Test if the router is created correctly
	assert.NotNil(t, router)
}
