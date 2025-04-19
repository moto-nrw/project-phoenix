package settings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockSettingsStore is a mock implementation of the SettingsStore interface
type MockSettingsStore struct {
	mock.Mock
}

func (m *MockSettingsStore) Create(ctx context.Context, setting *models.Setting) error {
	args := m.Called(ctx, setting)
	setting.ID = 1 // Assign an ID to simulate database insert
	return args.Error(0)
}

func (m *MockSettingsStore) Update(ctx context.Context, id int64, setting *models.Setting) error {
	args := m.Called(ctx, id, setting)
	return args.Error(0)
}

func (m *MockSettingsStore) UpdateByKey(ctx context.Context, key string, value string) error {
	args := m.Called(ctx, key, value)
	return args.Error(0)
}

func (m *MockSettingsStore) Get(ctx context.Context, id int64) (*models.Setting, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Setting), args.Error(1)
}

func (m *MockSettingsStore) GetByKey(ctx context.Context, key string) (*models.Setting, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Setting), args.Error(1)
}

func (m *MockSettingsStore) GetByCategory(ctx context.Context, category string) ([]*models.Setting, error) {
	args := m.Called(ctx, category)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Setting), args.Error(1)
}

func (m *MockSettingsStore) List(ctx context.Context) ([]*models.Setting, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*models.Setting), args.Error(1)
}

func (m *MockSettingsStore) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockAuth is a mock implementation of jwt.Auth
type MockAuth struct {
	mock.Mock
}

func (m *MockAuth) CreateToken(userID int64, role string, email string) (string, error) {
	args := m.Called(userID, role, email)
	return args.String(0), args.Error(1)
}

// setupTestRouter sets up a test router with the settings resource
func setupTestRouter(store *MockSettingsStore) chi.Router {
	r := chi.NewRouter()
	logger := logging.NewNoopLogger()
	return setupTestResource(r, store, nil, logger)
}

// contextKey is a custom type for context keys
type contextKey string

// Claims is a simplified version of JWT claims for testing
type Claims struct {
	Role string
}

// ContextKey is the context key for JWT claims
const ContextKey = contextKey("jwt")

// setupTestResource sets up a test router with the settings resource and custom middleware
func setupTestResource(r chi.Router, store *MockSettingsStore, _ interface{}, logger *logrus.Logger) chi.Router {
	// Create mock auth
	mockAuth := new(MockAuth)

	// Create settings resource
	resource := NewResource(store, mockAuth)

	// Add test auth middleware that sets admin role
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ContextKey, Claims{Role: "admin"})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	// Set up router with resource routes - mount at /settings for tests
	r.Mount("/settings", resource.Router())

	return r
}

func TestSettingsResource_Create(t *testing.T) {
	// Setup
	store := new(MockSettingsStore)
	router := setupTestRouter(store)

	// Test data
	validSetting := models.SettingRequest{
		Key:      "test_key",
		Value:    "test_value",
		Category: "test_category",
	}

	// Test case: successful creation
	t.Run("Success", func(t *testing.T) {
		// Mock store behavior
		store.On("GetByKey", mock.Anything, "test_key").Return(nil, fmt.Errorf("not found")).Once()
		store.On("Create", mock.Anything, mock.AnythingOfType("*models.Setting")).Return(nil).Once()

		// Create request
		body, _ := json.Marshal(validSetting)
		req, _ := http.NewRequest("POST", "/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusCreated, rec.Code)

		// Verify response contains the created setting
		var response models.SettingResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, validSetting.Key, response.Key)
		assert.Equal(t, validSetting.Value, response.Value)
		assert.Equal(t, validSetting.Category, response.Category)

		// Verify mock expectations
		store.AssertExpectations(t)
	})

	// Test case: key already exists
	t.Run("Key Already Exists", func(t *testing.T) {
		// Mock store behavior
		existingSetting := &models.Setting{ID: 1, Key: "test_key"}
		store.On("GetByKey", mock.Anything, "test_key").Return(existingSetting, nil).Once()

		// Create request
		body, _ := json.Marshal(validSetting)
		req, _ := http.NewRequest("POST", "/settings", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Contains(t, rec.Body.String(), "conflict")

		// Verify mock expectations
		store.AssertExpectations(t)
	})
}

func TestSettingsResource_Get(t *testing.T) {
	// Setup
	store := new(MockSettingsStore)
	router := setupTestRouter(store)

	// Test case: successful retrieval
	t.Run("Success", func(t *testing.T) {
		// Mock store behavior
		setting := &models.Setting{ID: 1, Key: "test_key", Value: "test_value", Category: "test_category"}
		store.On("Get", mock.Anything, int64(1)).Return(setting, nil).Once()

		// Create request
		req, _ := http.NewRequest("GET", "/settings/1", nil)
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response contains the setting
		var response models.SettingResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, setting.ID, response.ID)
		assert.Equal(t, setting.Key, response.Key)
		assert.Equal(t, setting.Value, response.Value)

		// Verify mock expectations
		store.AssertExpectations(t)
	})

	// Test case: setting not found
	t.Run("Not Found", func(t *testing.T) {
		// Mock store behavior
		store.On("Get", mock.Anything, int64(999)).Return(nil, fmt.Errorf("not found")).Once()

		// Create request
		req, _ := http.NewRequest("GET", "/settings/999", nil)
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusNotFound, rec.Code)
		assert.Contains(t, rec.Body.String(), "not found")

		// Verify mock expectations
		store.AssertExpectations(t)
	})
}

func TestSettingsResource_List(t *testing.T) {
	// Setup
	store := new(MockSettingsStore)
	router := setupTestRouter(store)

	// Test case: successful list
	t.Run("Success", func(t *testing.T) {
		// Mock store behavior
		settings := []*models.Setting{
			{ID: 1, Key: "key1", Value: "value1", Category: "category1"},
			{ID: 2, Key: "key2", Value: "value2", Category: "category2"},
		}
		store.On("List", mock.Anything).Return(settings, nil).Once()

		// Create request
		req, _ := http.NewRequest("GET", "/settings", nil)
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response contains the settings
		var response []*models.SettingResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Len(t, response, 2)
		assert.Equal(t, settings[0].Key, response[0].Key)
		assert.Equal(t, settings[1].Key, response[1].Key)

		// Verify mock expectations
		store.AssertExpectations(t)
	})
}

func TestSettingsResource_Update(t *testing.T) {
	// Setup
	store := new(MockSettingsStore)
	router := setupTestRouter(store)

	// Test data
	updatedSetting := models.SettingRequest{
		Key:      "updated_key",
		Value:    "updated_value",
		Category: "updated_category",
	}

	// Test case: successful update
	t.Run("Success", func(t *testing.T) {
		// Mock store behavior
		existingSetting := &models.Setting{ID: 1, Key: "test_key", Value: "test_value", Category: "test_category"}
		store.On("Get", mock.Anything, int64(1)).Return(existingSetting, nil).Once()
		store.On("GetByKey", mock.Anything, "updated_key").Return(nil, fmt.Errorf("not found")).Once()
		store.On("Update", mock.Anything, int64(1), mock.AnythingOfType("*models.Setting")).Return(nil).Once()

		// Create request
		body, _ := json.Marshal(updatedSetting)
		req, _ := http.NewRequest("PUT", "/settings/1", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusOK, rec.Code)

		// Verify response contains the updated setting
		var response models.SettingResponse
		err := json.Unmarshal(rec.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Equal(t, updatedSetting.Key, response.Key)
		assert.Equal(t, updatedSetting.Value, response.Value)
		assert.Equal(t, updatedSetting.Category, response.Category)

		// Verify mock expectations
		store.AssertExpectations(t)
	})
}

func TestSettingsResource_Delete(t *testing.T) {
	// Setup
	store := new(MockSettingsStore)
	router := setupTestRouter(store)

	// Test case: successful deletion
	t.Run("Success", func(t *testing.T) {
		// Mock store behavior
		existingSetting := &models.Setting{ID: 1, Key: "test_key"}
		store.On("Get", mock.Anything, int64(1)).Return(existingSetting, nil).Once()
		store.On("Delete", mock.Anything, int64(1)).Return(nil).Once()

		// Create request
		req, _ := http.NewRequest("DELETE", "/settings/1", nil)
		rec := httptest.NewRecorder()

		// Execute request
		router.ServeHTTP(rec, req)

		// Assert
		assert.Equal(t, http.StatusNoContent, rec.Code)

		// Verify mock expectations
		store.AssertExpectations(t)
	})
}
