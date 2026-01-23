package config

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
)

// =============================================================================
// MOCK IMPLEMENTATIONS
// =============================================================================

// mockConfigService implements configSvc.Service for testing error paths.
type mockConfigService struct {
	listSettingsFunc               func(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error)
	getSettingByIDFunc             func(ctx context.Context, id int64) (*config.Setting, error)
	getSettingByKeyFunc            func(ctx context.Context, key string) (*config.Setting, error)
	getSettingsByCategoryFunc      func(ctx context.Context, category string) ([]*config.Setting, error)
	createSettingFunc              func(ctx context.Context, setting *config.Setting) error
	updateSettingFunc              func(ctx context.Context, setting *config.Setting) error
	updateSettingValueFunc         func(ctx context.Context, key string, value string) error
	deleteSettingFunc              func(ctx context.Context, id int64) error
	importSettingsFunc             func(ctx context.Context, settings []*config.Setting) ([]error, error)
	initializeDefaultSettingsFunc  func(ctx context.Context) error
	requiresRestartFunc            func(ctx context.Context) (bool, error)
	requiresDatabaseResetFunc      func(ctx context.Context) (bool, error)
	getStringValueFunc             func(ctx context.Context, key string, defaultValue string) (string, error)
	getBoolValueFunc               func(ctx context.Context, key string, defaultValue bool) (bool, error)
	getIntValueFunc                func(ctx context.Context, key string, defaultValue int) (int, error)
	getFloatValueFunc              func(ctx context.Context, key string, defaultValue float64) (float64, error)
	getSettingByKeyAndCategoryFunc func(ctx context.Context, key string, category string) (*config.Setting, error)
	getTimeoutSettingsFunc         func(ctx context.Context) (*config.TimeoutSettings, error)
	updateTimeoutSettingsFunc      func(ctx context.Context, settings *config.TimeoutSettings) error
	getDeviceTimeoutSettingsFunc   func(ctx context.Context, deviceID int64) (*config.TimeoutSettings, error)
}

func (m *mockConfigService) WithTx(_ bun.Tx) interface{} { return m }

func (m *mockConfigService) ListSettings(ctx context.Context, filters map[string]interface{}) ([]*config.Setting, error) {
	if m.listSettingsFunc != nil {
		return m.listSettingsFunc(ctx, filters)
	}
	return []*config.Setting{}, nil
}

func (m *mockConfigService) GetSettingByID(ctx context.Context, id int64) (*config.Setting, error) {
	if m.getSettingByIDFunc != nil {
		return m.getSettingByIDFunc(ctx, id)
	}
	return nil, &configSvc.ConfigError{Op: "GetSettingByID", Err: configSvc.ErrSettingNotFound}
}

func (m *mockConfigService) GetSettingByKey(ctx context.Context, key string) (*config.Setting, error) {
	if m.getSettingByKeyFunc != nil {
		return m.getSettingByKeyFunc(ctx, key)
	}
	return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
}

func (m *mockConfigService) GetSettingsByCategory(ctx context.Context, category string) ([]*config.Setting, error) {
	if m.getSettingsByCategoryFunc != nil {
		return m.getSettingsByCategoryFunc(ctx, category)
	}
	return []*config.Setting{}, nil
}

func (m *mockConfigService) CreateSetting(ctx context.Context, setting *config.Setting) error {
	if m.createSettingFunc != nil {
		return m.createSettingFunc(ctx, setting)
	}
	setting.ID = 1
	return nil
}

func (m *mockConfigService) UpdateSetting(ctx context.Context, setting *config.Setting) error {
	if m.updateSettingFunc != nil {
		return m.updateSettingFunc(ctx, setting)
	}
	return nil
}

func (m *mockConfigService) UpdateSettingValue(ctx context.Context, key string, value string) error {
	if m.updateSettingValueFunc != nil {
		return m.updateSettingValueFunc(ctx, key, value)
	}
	return nil
}

func (m *mockConfigService) DeleteSetting(ctx context.Context, id int64) error {
	if m.deleteSettingFunc != nil {
		return m.deleteSettingFunc(ctx, id)
	}
	return nil
}

func (m *mockConfigService) ImportSettings(ctx context.Context, settings []*config.Setting) ([]error, error) {
	if m.importSettingsFunc != nil {
		return m.importSettingsFunc(ctx, settings)
	}
	return nil, nil
}

func (m *mockConfigService) InitializeDefaultSettings(ctx context.Context) error {
	if m.initializeDefaultSettingsFunc != nil {
		return m.initializeDefaultSettingsFunc(ctx)
	}
	return nil
}

func (m *mockConfigService) RequiresRestart(ctx context.Context) (bool, error) {
	if m.requiresRestartFunc != nil {
		return m.requiresRestartFunc(ctx)
	}
	return false, nil
}

func (m *mockConfigService) RequiresDatabaseReset(ctx context.Context) (bool, error) {
	if m.requiresDatabaseResetFunc != nil {
		return m.requiresDatabaseResetFunc(ctx)
	}
	return false, nil
}

func (m *mockConfigService) GetStringValue(ctx context.Context, key string, defaultValue string) (string, error) {
	if m.getStringValueFunc != nil {
		return m.getStringValueFunc(ctx, key, defaultValue)
	}
	return defaultValue, nil
}

func (m *mockConfigService) GetBoolValue(ctx context.Context, key string, defaultValue bool) (bool, error) {
	if m.getBoolValueFunc != nil {
		return m.getBoolValueFunc(ctx, key, defaultValue)
	}
	return defaultValue, nil
}

func (m *mockConfigService) GetIntValue(ctx context.Context, key string, defaultValue int) (int, error) {
	if m.getIntValueFunc != nil {
		return m.getIntValueFunc(ctx, key, defaultValue)
	}
	return defaultValue, nil
}

func (m *mockConfigService) GetFloatValue(ctx context.Context, key string, defaultValue float64) (float64, error) {
	if m.getFloatValueFunc != nil {
		return m.getFloatValueFunc(ctx, key, defaultValue)
	}
	return defaultValue, nil
}

func (m *mockConfigService) GetSettingByKeyAndCategory(ctx context.Context, key string, category string) (*config.Setting, error) {
	if m.getSettingByKeyAndCategoryFunc != nil {
		return m.getSettingByKeyAndCategoryFunc(ctx, key, category)
	}
	return nil, &configSvc.ConfigError{Op: "GetSettingByKeyAndCategory", Err: &configSvc.SettingNotFoundError{Key: key}}
}

func (m *mockConfigService) GetTimeoutSettings(ctx context.Context) (*config.TimeoutSettings, error) {
	if m.getTimeoutSettingsFunc != nil {
		return m.getTimeoutSettingsFunc(ctx)
	}
	return &config.TimeoutSettings{
		GlobalTimeoutMinutes:    30,
		WarningThresholdMinutes: 5,
		CheckIntervalSeconds:    30,
	}, nil
}

func (m *mockConfigService) UpdateTimeoutSettings(ctx context.Context, settings *config.TimeoutSettings) error {
	if m.updateTimeoutSettingsFunc != nil {
		return m.updateTimeoutSettingsFunc(ctx, settings)
	}
	return nil
}

func (m *mockConfigService) GetDeviceTimeoutSettings(ctx context.Context, deviceID int64) (*config.TimeoutSettings, error) {
	if m.getDeviceTimeoutSettingsFunc != nil {
		return m.getDeviceTimeoutSettingsFunc(ctx, deviceID)
	}
	return m.GetTimeoutSettings(ctx)
}

// mockCleanupService implements active.CleanupService for testing.
type mockCleanupService struct {
	cleanupExpiredVisitsFunc     func(ctx context.Context) (*active.CleanupResult, error)
	cleanupVisitsForStudentFunc  func(ctx context.Context, studentID int64) (int64, error)
	getRetentionStatisticsFunc   func(ctx context.Context) (*active.RetentionStats, error)
	previewCleanupFunc           func(ctx context.Context) (*active.CleanupPreview, error)
	cleanupStaleAttendanceFunc   func(ctx context.Context) (*active.AttendanceCleanupResult, error)
	previewAttendanceCleanupFunc func(ctx context.Context) (*active.AttendanceCleanupPreview, error)
}

func (m *mockCleanupService) CleanupExpiredVisits(ctx context.Context) (*active.CleanupResult, error) {
	if m.cleanupExpiredVisitsFunc != nil {
		return m.cleanupExpiredVisitsFunc(ctx)
	}
	return &active.CleanupResult{
		Success:           true,
		StartedAt:         time.Now(),
		CompletedAt:       time.Now(),
		StudentsProcessed: 0,
		RecordsDeleted:    0,
		Errors:            []active.CleanupError{},
	}, nil
}

func (m *mockCleanupService) CleanupVisitsForStudent(ctx context.Context, studentID int64) (int64, error) {
	if m.cleanupVisitsForStudentFunc != nil {
		return m.cleanupVisitsForStudentFunc(ctx, studentID)
	}
	return 0, nil
}

func (m *mockCleanupService) GetRetentionStatistics(ctx context.Context) (*active.RetentionStats, error) {
	if m.getRetentionStatisticsFunc != nil {
		return m.getRetentionStatisticsFunc(ctx)
	}
	return &active.RetentionStats{
		TotalExpiredVisits:   0,
		StudentsAffected:     0,
		ExpiredVisitsByMonth: make(map[string]int64),
	}, nil
}

func (m *mockCleanupService) PreviewCleanup(ctx context.Context) (*active.CleanupPreview, error) {
	if m.previewCleanupFunc != nil {
		return m.previewCleanupFunc(ctx)
	}
	return &active.CleanupPreview{
		StudentVisitCounts: make(map[int64]int),
		TotalVisits:        0,
	}, nil
}

func (m *mockCleanupService) CleanupStaleAttendance(ctx context.Context) (*active.AttendanceCleanupResult, error) {
	if m.cleanupStaleAttendanceFunc != nil {
		return m.cleanupStaleAttendanceFunc(ctx)
	}
	return &active.AttendanceCleanupResult{
		Success:     true,
		StartedAt:   time.Now(),
		CompletedAt: time.Now(),
	}, nil
}

func (m *mockCleanupService) PreviewAttendanceCleanup(ctx context.Context) (*active.AttendanceCleanupPreview, error) {
	if m.previewAttendanceCleanupFunc != nil {
		return m.previewAttendanceCleanupFunc(ctx)
	}
	return &active.AttendanceCleanupPreview{
		StudentRecords: make(map[int64]int),
		RecordsByDate:  make(map[string]int),
	}, nil
}

// =============================================================================
// TEST HELPER
// =============================================================================

func executeTestRequest(router chi.Router, method, target string, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	} else {
		req = httptest.NewRequest(method, target, nil)
	}
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}

// =============================================================================
// LIST SETTINGS TESTS
// =============================================================================

func TestListSettings_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{
		listSettingsFunc: func(_ context.Context, _ map[string]interface{}) ([]*config.Setting, error) {
			return nil, errors.New("database connection failed")
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config", resource.listSettings)

	rr := executeTestRequest(router, "GET", "/config", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// GET SETTING BY KEY TESTS
// =============================================================================

func TestGetSettingByKey_EmptyKey(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/key/{key}", resource.getSettingByKey)

	// Chi router will not match empty key, so we need to test with a valid URL
	// The empty key check happens when chi.URLParam returns ""
	// This is tested by checking with a path that doesn't set the key param
	req := httptest.NewRequest("GET", "/config/key/", nil)
	req.Header.Set("Content-Type", "application/json")

	// Use chi context to simulate empty key
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	resource.getSettingByKey(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetSettingByKey_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        1,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      key,
				Value:    "test_value",
				Category: "test",
			}, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/key/{key}", resource.getSettingByKey)

	rr := executeTestRequest(router, "GET", "/config/key/test_key", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "test_value")
}

func TestGetSettingByKey_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, _ string) (*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: "test_key"}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/key/{key}", resource.getSettingByKey)

	rr := executeTestRequest(router, "GET", "/config/key/test_key", "")

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// =============================================================================
// GET SETTINGS BY CATEGORY TESTS
// =============================================================================

func TestGetSettingsByCategory_EmptyCategory(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)

	req := httptest.NewRequest("GET", "/config/category/", nil)
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("category", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	resource.getSettingsByCategory(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestGetSettingsByCategory_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingsByCategoryFunc: func(_ context.Context, _ string) ([]*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingsByCategory", Err: errors.New("database error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/category/{category}", resource.getSettingsByCategory)

	rr := executeTestRequest(router, "GET", "/config/category/test", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// CREATE SETTING TESTS
// =============================================================================

func TestCreateSetting_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{
		createSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return &configSvc.ConfigError{Op: "CreateSetting", Err: &configSvc.DuplicateKeyError{Key: "test_key"}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config", resource.createSetting)

	body := `{"key":"test_key","value":"test_value","category":"test"}`
	rr := executeTestRequest(router, "POST", "/config", body)

	assert.Equal(t, http.StatusConflict, rr.Code)
}

func TestCreateSetting_InvalidJSON(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config", resource.createSetting)

	body := `{"key":}` // Invalid JSON
	rr := executeTestRequest(router, "POST", "/config", body)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// =============================================================================
// UPDATE SETTING VALUE TESTS
// =============================================================================

func TestUpdateSettingValue_EmptyKey(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)

	req := httptest.NewRequest("PATCH", "/config/key/", strings.NewReader(`{"value":"new_value"}`))
	req.Header.Set("Content-Type", "application/json")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("key", "")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	rr := httptest.NewRecorder()
	resource.updateSettingValue(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateSettingValue_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		updateSettingValueFunc: func(_ context.Context, _ string, _ string) error {
			return nil
		},
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        1,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      key,
				Value:    "new_value",
				Category: "test",
			}, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Patch("/config/key/{key}", resource.updateSettingValue)

	body := `{"value":"new_value"}`
	rr := executeTestRequest(router, "PATCH", "/config/key/test_key", body)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "new_value")
}

func TestUpdateSettingValue_UpdateError(t *testing.T) {
	mockSvc := &mockConfigService{
		updateSettingValueFunc: func(_ context.Context, _ string, _ string) error {
			return &configSvc.ConfigError{Op: "UpdateSettingValue", Err: &configSvc.SettingNotFoundError{Key: "test_key"}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Patch("/config/key/{key}", resource.updateSettingValue)

	body := `{"value":"new_value"}`
	rr := executeTestRequest(router, "PATCH", "/config/key/test_key", body)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestUpdateSettingValue_GetAfterUpdateError(t *testing.T) {
	mockSvc := &mockConfigService{
		updateSettingValueFunc: func(_ context.Context, _ string, _ string) error {
			return nil
		},
		getSettingByKeyFunc: func(_ context.Context, _ string) (*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: errors.New("unexpected error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Patch("/config/key/{key}", resource.updateSettingValue)

	body := `{"value":"new_value"}`
	rr := executeTestRequest(router, "PATCH", "/config/key/test_key", body)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// UPDATE SETTING TESTS
// =============================================================================

func TestUpdateSetting_ValidationError(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/{id}", resource.updateSetting)

	body := `{"key":"","value":"","category":""}` // Missing required fields
	rr := executeTestRequest(router, "PUT", "/config/1", body)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestUpdateSetting_UpdateServiceError(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByIDFunc: func(_ context.Context, _ int64) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        1,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      "test_key",
				Value:    "old_value",
				Category: "test",
			}, nil
		},
		updateSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return &configSvc.ConfigError{Op: "UpdateSetting", Err: &configSvc.SystemSettingsLockedError{Key: "test_key"}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/{id}", resource.updateSetting)

	body := `{"key":"test_key","value":"new_value","category":"test"}`
	rr := executeTestRequest(router, "PUT", "/config/1", body)

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// =============================================================================
// IMPORT SETTINGS TESTS
// =============================================================================

func TestImportSettings_PartialFailure(t *testing.T) {
	mockSvc := &mockConfigService{
		importSettingsFunc: func(_ context.Context, _ []*config.Setting) ([]error, error) {
			return []error{errors.New("failed to import setting 1")}, errors.New("import failed with errors")
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/import", resource.importSettings)

	body := `{"settings":[{"key":"key1","value":"val1","category":"test"},{"key":"key2","value":"val2","category":"test"}]}`
	rr := executeTestRequest(router, "POST", "/config/import", body)

	assert.Equal(t, http.StatusPartialContent, rr.Code)
	assert.Contains(t, rr.Body.String(), "errors")
}

func TestImportSettings_TotalFailure(t *testing.T) {
	mockSvc := &mockConfigService{
		importSettingsFunc: func(_ context.Context, _ []*config.Setting) ([]error, error) {
			return nil, &configSvc.ConfigError{Op: "ImportSettings", Err: errors.New("database error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/import", resource.importSettings)

	body := `{"settings":[{"key":"key1","value":"val1","category":"test"}]}`
	rr := executeTestRequest(router, "POST", "/config/import", body)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// SYSTEM STATUS TESTS
// =============================================================================

func TestGetSystemStatus_RequiresRestartError(t *testing.T) {
	mockSvc := &mockConfigService{
		requiresRestartFunc: func(_ context.Context) (bool, error) {
			return false, errors.New("database error")
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/system-status", resource.getSystemStatus)

	rr := executeTestRequest(router, "GET", "/config/system-status", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetSystemStatus_RequiresDatabaseResetError(t *testing.T) {
	mockSvc := &mockConfigService{
		requiresRestartFunc: func(_ context.Context) (bool, error) {
			return true, nil
		},
		requiresDatabaseResetFunc: func(_ context.Context) (bool, error) {
			return false, errors.New("database error")
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/system-status", resource.getSystemStatus)

	rr := executeTestRequest(router, "GET", "/config/system-status", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// RETENTION SETTINGS TESTS
// =============================================================================

func TestGetRetentionSettings_WithLastCleanupRun(t *testing.T) {
	lastCleanupTime := time.Now().Add(-1 * time.Hour)
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			switch key {
			case "default_visit_retention_days":
				return &config.Setting{Model: base.Model{ID: 1}, Key: key, Value: "15"}, nil
			case "last_retention_cleanup":
				return &config.Setting{Model: base.Model{ID: 2}, Key: key, Value: lastCleanupTime.Format(time.RFC3339)}, nil
			default:
				return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
			}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/retention", resource.getRetentionSettings)

	rr := executeTestRequest(router, "GET", "/config/retention", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "last_cleanup_run")
}

func TestGetRetentionSettings_InvalidRetentionDays(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			if key == "default_visit_retention_days" {
				return &config.Setting{Model: base.Model{ID: 1}, Key: key, Value: "100"}, nil // Out of range
			}
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/retention", resource.getRetentionSettings)

	rr := executeTestRequest(router, "GET", "/config/retention", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	// Should default to 30 when value is out of range
	assert.Contains(t, rr.Body.String(), `"visit_retention_days":30`)
}

func TestGetRetentionSettings_InvalidLastCleanupTimestamp(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			switch key {
			case "default_visit_retention_days":
				return &config.Setting{Model: base.Model{ID: 1}, Key: key, Value: "15"}, nil
			case "last_retention_cleanup":
				return &config.Setting{Model: base.Model{ID: 2}, Key: key, Value: "invalid-timestamp"}, nil
			default:
				return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
			}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/retention", resource.getRetentionSettings)

	rr := executeTestRequest(router, "GET", "/config/retention", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateRetentionSettings_CreateNewSetting(t *testing.T) {
	now := time.Now()
	createdSetting := &config.Setting{
		Model: base.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Key:      "default_visit_retention_days",
		Value:    "15",
		Category: "privacy",
	}

	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			// First call returns not found, second call returns the created setting
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
		createSettingFunc: func(_ context.Context, setting *config.Setting) error {
			setting.ID = createdSetting.ID
			setting.CreatedAt = createdSetting.CreatedAt
			setting.UpdatedAt = createdSetting.UpdatedAt
			return nil
		},
	}

	// Use a counter to track calls
	callCount := 0
	mockSvc.getSettingByKeyFunc = func(_ context.Context, key string) (*config.Setting, error) {
		callCount++
		if callCount == 1 {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		}
		return createdSetting, nil
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/retention", resource.updateRetentionSettings)

	body := `{"visit_retention_days":15}`
	rr := executeTestRequest(router, "PUT", "/config/retention", body)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestUpdateRetentionSettings_CreateError(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
		createSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return &configSvc.ConfigError{Op: "CreateSetting", Err: errors.New("database error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/retention", resource.updateRetentionSettings)

	body := `{"visit_retention_days":15}`
	rr := executeTestRequest(router, "PUT", "/config/retention", body)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestUpdateRetentionSettings_UpdateError(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        1,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      key,
				Value:    "30",
				Category: "privacy",
			}, nil
		},
		updateSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return &configSvc.ConfigError{Op: "UpdateSetting", Err: errors.New("database error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/retention", resource.updateRetentionSettings)

	body := `{"visit_retention_days":15}`
	rr := executeTestRequest(router, "PUT", "/config/retention", body)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// TRIGGER RETENTION CLEANUP TESTS
// =============================================================================

func TestTriggerRetentionCleanup_NilCleanupService(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil) // nil CleanupService
	router := chi.NewRouter()
	router.Post("/config/retention/cleanup", resource.triggerRetentionCleanup)

	rr := executeTestRequest(router, "POST", "/config/retention/cleanup", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "cleanup service not available")
}

func TestTriggerRetentionCleanup_CleanupError(t *testing.T) {
	mockSvc := &mockConfigService{}
	mockCleanup := &mockCleanupService{
		cleanupExpiredVisitsFunc: func(_ context.Context) (*active.CleanupResult, error) {
			return nil, errors.New("cleanup failed")
		},
	}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Post("/config/retention/cleanup", resource.triggerRetentionCleanup)

	rr := executeTestRequest(router, "POST", "/config/retention/cleanup", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestTriggerRetentionCleanup_WithErrors(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
		createSettingFunc: func(_ context.Context, setting *config.Setting) error {
			setting.ID = 1
			return nil
		},
	}
	mockCleanup := &mockCleanupService{
		cleanupExpiredVisitsFunc: func(_ context.Context) (*active.CleanupResult, error) {
			now := time.Now()
			return &active.CleanupResult{
				Success:           false,
				StartedAt:         now.Add(-10 * time.Second),
				CompletedAt:       now,
				StudentsProcessed: 5,
				RecordsDeleted:    10,
				Errors: []active.CleanupError{
					{StudentID: 1, Error: "error 1", Timestamp: now},
					{StudentID: 2, Error: "error 2", Timestamp: now},
					{StudentID: 3, Error: "error 3", Timestamp: now},
					{StudentID: 4, Error: "error 4", Timestamp: now},
					{StudentID: 5, Error: "error 5", Timestamp: now},
					{StudentID: 6, Error: "error 6", Timestamp: now},
				},
			}, nil
		},
	}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Post("/config/retention/cleanup", resource.triggerRetentionCleanup)

	rr := executeTestRequest(router, "POST", "/config/retention/cleanup", "")

	assert.Equal(t, http.StatusPartialContent, rr.Code)
	assert.Contains(t, rr.Body.String(), "error_count")
	assert.Contains(t, rr.Body.String(), "error_summary")
}

func TestTriggerRetentionCleanup_UpdateExistingTimestamp(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			if key == "last_retention_cleanup" {
				return &config.Setting{
					Model: base.Model{
						ID:        1,
						CreatedAt: now.Add(-24 * time.Hour),
						UpdatedAt: now.Add(-1 * time.Hour),
					},
					Key:      key,
					Value:    now.Add(-1 * time.Hour).Format(time.RFC3339),
					Category: "privacy",
				}, nil
			}
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
		updateSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return nil
		},
	}
	mockCleanup := &mockCleanupService{}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Post("/config/retention/cleanup", resource.triggerRetentionCleanup)

	rr := executeTestRequest(router, "POST", "/config/retention/cleanup", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestTriggerRetentionCleanup_WithFewerThanMaxErrors(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
		createSettingFunc: func(_ context.Context, setting *config.Setting) error {
			setting.ID = 1
			return nil
		},
	}
	now := time.Now()
	mockCleanup := &mockCleanupService{
		cleanupExpiredVisitsFunc: func(_ context.Context) (*active.CleanupResult, error) {
			return &active.CleanupResult{
				Success:           false,
				StartedAt:         now.Add(-10 * time.Second),
				CompletedAt:       now,
				StudentsProcessed: 2,
				RecordsDeleted:    3,
				Errors: []active.CleanupError{
					{StudentID: 1, Error: "error 1", Timestamp: now},
					{StudentID: 2, Error: "error 2", Timestamp: now},
				},
			}, nil
		},
	}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Post("/config/retention/cleanup", resource.triggerRetentionCleanup)

	rr := executeTestRequest(router, "POST", "/config/retention/cleanup", "")

	assert.Equal(t, http.StatusPartialContent, rr.Code)
}

// =============================================================================
// GET RETENTION STATS TESTS
// =============================================================================

func TestGetRetentionStats_NilCleanupService(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil) // nil CleanupService
	router := chi.NewRouter()
	router.Get("/config/retention/stats", resource.getRetentionStats)

	rr := executeTestRequest(router, "GET", "/config/retention/stats", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "cleanup service not available")
}

func TestGetRetentionStats_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{}
	mockCleanup := &mockCleanupService{
		getRetentionStatisticsFunc: func(_ context.Context) (*active.RetentionStats, error) {
			return nil, errors.New("database error")
		},
	}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Get("/config/retention/stats", resource.getRetentionStats)

	rr := executeTestRequest(router, "GET", "/config/retention/stats", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

func TestGetRetentionStats_WithOldestVisit(t *testing.T) {
	oldestVisit := time.Now().Add(-30 * 24 * time.Hour)
	mockSvc := &mockConfigService{}
	mockCleanup := &mockCleanupService{
		getRetentionStatisticsFunc: func(_ context.Context) (*active.RetentionStats, error) {
			return &active.RetentionStats{
				TotalExpiredVisits:   100,
				StudentsAffected:     10,
				OldestExpiredVisit:   &oldestVisit,
				ExpiredVisitsByMonth: map[string]int64{"2024-01": 50, "2024-02": 50},
			}, nil
		},
	}

	resource := NewResource(mockSvc, mockCleanup)
	router := chi.NewRouter()
	router.Get("/config/retention/stats", resource.getRetentionStats)

	rr := executeTestRequest(router, "GET", "/config/retention/stats", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "oldest_expired_visit")
	assert.Contains(t, rr.Body.String(), "expired_visits_by_month")
}

// =============================================================================
// INITIALIZE DEFAULTS TESTS
// =============================================================================

func TestInitializeDefaults_Success(t *testing.T) {
	mockSvc := &mockConfigService{
		initializeDefaultSettingsFunc: func(_ context.Context) error {
			return nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/initialize-defaults", resource.initializeDefaults)

	rr := executeTestRequest(router, "POST", "/config/initialize-defaults", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestInitializeDefaults_Error(t *testing.T) {
	mockSvc := &mockConfigService{
		initializeDefaultSettingsFunc: func(_ context.Context) error {
			return &configSvc.ConfigError{Op: "InitializeDefaultSettings", Err: errors.New("database error")}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/initialize-defaults", resource.initializeDefaults)

	rr := executeTestRequest(router, "POST", "/config/initialize-defaults", "")

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// =============================================================================
// REQUEST BINDING TESTS
// =============================================================================

func TestSettingRequest_Bind_AllFieldsMissing(t *testing.T) {
	req := &SettingRequest{}
	err := req.Bind(nil)
	require.Error(t, err)
}

func TestSettingValueRequest_Bind_MissingValue(t *testing.T) {
	req := &SettingValueRequest{Value: ""}
	err := req.Bind(nil)
	require.Error(t, err)
}

func TestImportSettingsRequest_Bind_EmptySettings(t *testing.T) {
	req := &ImportSettingsRequest{Settings: []SettingRequest{}}
	err := req.Bind(nil)
	require.Error(t, err)
}

func TestImportSettingsRequest_Bind_InvalidSetting(t *testing.T) {
	req := &ImportSettingsRequest{
		Settings: []SettingRequest{
			{Key: "key1", Value: "", Category: ""}, // Missing required fields
		},
	}
	err := req.Bind(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid setting at index 0")
}

func TestRetentionSettingsRequest_Bind_Valid(t *testing.T) {
	req := &RetentionSettingsRequest{VisitRetentionDays: 15}
	err := req.Bind(nil)
	require.NoError(t, err)
}

func TestRetentionSettingsRequest_Bind_Zero(t *testing.T) {
	req := &RetentionSettingsRequest{VisitRetentionDays: 0}
	err := req.Bind(nil)
	require.Error(t, err)
}

func TestRetentionSettingsRequest_Bind_TooHigh(t *testing.T) {
	req := &RetentionSettingsRequest{VisitRetentionDays: 32}
	err := req.Bind(nil)
	require.Error(t, err)
}

// =============================================================================
// RESPONSE HELPER TESTS
// =============================================================================

func TestNewSettingResponse(t *testing.T) {
	now := time.Now()
	setting := &config.Setting{
		Model: base.Model{
			ID:        1,
			CreatedAt: now,
			UpdatedAt: now,
		},
		Key:             "test_key",
		Value:           "test_value",
		Category:        "system",
		Description:     "Test description",
		RequiresRestart: true,
		RequiresDBReset: false,
	}

	response := newSettingResponse(setting)

	assert.Equal(t, setting.ID, response.ID)
	assert.Equal(t, setting.Key, response.Key)
	assert.Equal(t, setting.Value, response.Value)
	assert.Equal(t, setting.Category, response.Category)
	assert.Equal(t, setting.Description, response.Description)
	assert.Equal(t, setting.RequiresRestart, response.RequiresRestart)
	assert.Equal(t, setting.RequiresDBReset, response.RequiresDBReset)
	assert.True(t, response.IsSystemSetting) // "system" category makes it a system setting
}

// =============================================================================
// ROUTER TESTS
// =============================================================================

func TestRouter_ReturnsValidRouter(t *testing.T) {
	mockSvc := &mockConfigService{}
	resource := NewResource(mockSvc, nil)

	router := resource.Router()

	require.NotNil(t, router)
}

// =============================================================================
// ADDITIONAL EDGE CASE TESTS
// =============================================================================

func TestGetSetting_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByIDFunc: func(_ context.Context, id int64) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        id,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      "test_key",
				Value:    "test_value",
				Category: "test",
			}, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/{id}", resource.getSetting)

	rr := executeTestRequest(router, "GET", "/config/1", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "test_value")
}

func TestDeleteSetting_Success(t *testing.T) {
	mockSvc := &mockConfigService{
		deleteSettingFunc: func(_ context.Context, _ int64) error {
			return nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Delete("/config/{id}", resource.deleteSetting)

	rr := executeTestRequest(router, "DELETE", "/config/1", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestDeleteSetting_ServiceError(t *testing.T) {
	mockSvc := &mockConfigService{
		deleteSettingFunc: func(_ context.Context, _ int64) error {
			return &configSvc.ConfigError{Op: "DeleteSetting", Err: &configSvc.SystemSettingsLockedError{Key: "test_key"}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Delete("/config/{id}", resource.deleteSetting)

	rr := executeTestRequest(router, "DELETE", "/config/1", "")

	assert.Equal(t, http.StatusForbidden, rr.Code)
}

func TestUpdateSetting_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingByIDFunc: func(_ context.Context, id int64) (*config.Setting, error) {
			return &config.Setting{
				Model: base.Model{
					ID:        id,
					CreatedAt: now,
					UpdatedAt: now,
				},
				Key:      "test_key",
				Value:    "old_value",
				Category: "test",
			}, nil
		},
		updateSettingFunc: func(_ context.Context, _ *config.Setting) error {
			return nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/{id}", resource.updateSetting)

	body := `{"key":"test_key","value":"new_value","category":"test"}`
	rr := executeTestRequest(router, "PUT", "/config/1", body)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestCreateSetting_Success(t *testing.T) {
	mockSvc := &mockConfigService{
		createSettingFunc: func(_ context.Context, setting *config.Setting) error {
			setting.ID = 1
			setting.CreatedAt = time.Now()
			setting.UpdatedAt = time.Now()
			return nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config", resource.createSetting)

	body := `{"key":"new_key","value":"new_value","category":"test"}`
	rr := executeTestRequest(router, "POST", "/config", body)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestGetSettingsByCategory_Success(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		getSettingsByCategoryFunc: func(_ context.Context, category string) ([]*config.Setting, error) {
			return []*config.Setting{
				{Model: base.Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Key: "key1", Value: "val1", Category: category},
				{Model: base.Model{ID: 2, CreatedAt: now, UpdatedAt: now}, Key: "key2", Value: "val2", Category: category},
			}, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/category/{category}", resource.getSettingsByCategory)

	rr := executeTestRequest(router, "GET", "/config/category/test", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestImportSettings_Success(t *testing.T) {
	mockSvc := &mockConfigService{
		importSettingsFunc: func(_ context.Context, _ []*config.Setting) ([]error, error) {
			return nil, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/import", resource.importSettings)

	body := `{"settings":[{"key":"key1","value":"val1","category":"test"}]}`
	rr := executeTestRequest(router, "POST", "/config/import", body)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"count":1`)
}

func TestGetSystemStatus_Success(t *testing.T) {
	mockSvc := &mockConfigService{
		requiresRestartFunc: func(_ context.Context) (bool, error) {
			return true, nil
		},
		requiresDatabaseResetFunc: func(_ context.Context) (bool, error) {
			return false, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/system-status", resource.getSystemStatus)

	rr := executeTestRequest(router, "GET", "/config/system-status", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"requires_restart":true`)
	assert.Contains(t, rr.Body.String(), `"requires_db_reset":false`)
}

func TestGetRetentionSettings_NegativeRetentionDays(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			if key == "default_visit_retention_days" {
				return &config.Setting{Model: base.Model{ID: 1}, Key: key, Value: "-5"}, nil // Negative value
			}
			return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/retention", resource.getRetentionSettings)

	rr := executeTestRequest(router, "GET", "/config/retention", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	// Should default to 30 when value is negative
	assert.Contains(t, rr.Body.String(), `"visit_retention_days":30`)
}

func TestUpdateRetentionSettings_ExistingSettingUpdate(t *testing.T) {
	now := time.Now()
	existingSetting := &config.Setting{
		Model: base.Model{
			ID:        1,
			CreatedAt: now.Add(-24 * time.Hour),
			UpdatedAt: now.Add(-1 * time.Hour),
		},
		Key:      "default_visit_retention_days",
		Value:    "30",
		Category: "privacy",
	}

	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, _ string) (*config.Setting, error) {
			return existingSetting, nil
		},
		updateSettingFunc: func(_ context.Context, setting *config.Setting) error {
			existingSetting.Value = setting.Value
			existingSetting.UpdatedAt = time.Now()
			return nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Put("/config/retention", resource.updateRetentionSettings)

	body := `{"visit_retention_days":15}`
	rr := executeTestRequest(router, "PUT", "/config/retention", body)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestGetRetentionSettings_EmptyLastCleanupValue(t *testing.T) {
	mockSvc := &mockConfigService{
		getSettingByKeyFunc: func(_ context.Context, key string) (*config.Setting, error) {
			switch key {
			case "default_visit_retention_days":
				return &config.Setting{Model: base.Model{ID: 1}, Key: key, Value: "15"}, nil
			case "last_retention_cleanup":
				return &config.Setting{Model: base.Model{ID: 2}, Key: key, Value: ""}, nil // Empty value
			default:
				return nil, &configSvc.ConfigError{Op: "GetSettingByKey", Err: &configSvc.SettingNotFoundError{Key: key}}
			}
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config/retention", resource.getRetentionSettings)

	rr := executeTestRequest(router, "GET", "/config/retention", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestListSettings_WithBothFilters(t *testing.T) {
	now := time.Now()
	mockSvc := &mockConfigService{
		listSettingsFunc: func(_ context.Context, filters map[string]interface{}) ([]*config.Setting, error) {
			// Verify both filters are passed
			_, hasCategory := filters["category"]
			_, hasSearch := filters["search"]
			if hasCategory && hasSearch {
				return []*config.Setting{
					{Model: base.Model{ID: 1, CreatedAt: now, UpdatedAt: now}, Key: "app_name", Value: "Test App", Category: "system"},
				}, nil
			}
			return []*config.Setting{}, nil
		},
	}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Get("/config", resource.listSettings)

	rr := executeTestRequest(router, "GET", "/config?category=system&search=app", "")

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestImportSettings_ValidationErrorAtSpecificIndex(t *testing.T) {
	mockSvc := &mockConfigService{}

	resource := NewResource(mockSvc, nil)
	router := chi.NewRouter()
	router.Post("/config/import", resource.importSettings)

	// First setting is valid, second is invalid
	body := `{"settings":[{"key":"key1","value":"val1","category":"test"},{"key":"key2","value":"","category":""}]}`
	rr := executeTestRequest(router, "POST", "/config/import", body)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "invalid setting at index 1")
}

func TestNewResource(t *testing.T) {
	mockSvc := &mockConfigService{}
	mockCleanup := &mockCleanupService{}

	resource := NewResource(mockSvc, mockCleanup)

	require.NotNil(t, resource)
	assert.Equal(t, mockSvc, resource.ConfigService)
	assert.Equal(t, mockCleanup, resource.CleanupService)
}

// Test that handler getter methods work correctly
func TestHandlerGetters(t *testing.T) {
	mockSvc := &mockConfigService{}
	resource := NewResource(mockSvc, nil)

	tests := []struct {
		name   string
		getter func() http.HandlerFunc
	}{
		{"ListSettingsHandler", resource.ListSettingsHandler},
		{"GetSettingHandler", resource.GetSettingHandler},
		{"GetSettingByKeyHandler", resource.GetSettingByKeyHandler},
		{"GetSettingsByCategoryHandler", resource.GetSettingsByCategoryHandler},
		{"GetSystemStatusHandler", resource.GetSystemStatusHandler},
		{"GetDefaultSettingsHandler", resource.GetDefaultSettingsHandler},
		{"CreateSettingHandler", resource.CreateSettingHandler},
		{"UpdateSettingHandler", resource.UpdateSettingHandler},
		{"UpdateSettingValueHandler", resource.UpdateSettingValueHandler},
		{"DeleteSettingHandler", resource.DeleteSettingHandler},
		{"ImportSettingsHandler", resource.ImportSettingsHandler},
		{"InitializeDefaultsHandler", resource.InitializeDefaultsHandler},
		{"GetRetentionSettingsHandler", resource.GetRetentionSettingsHandler},
		{"UpdateRetentionSettingsHandler", resource.UpdateRetentionSettingsHandler},
		{"TriggerRetentionCleanupHandler", resource.TriggerRetentionCleanupHandler},
		{"GetRetentionStatsHandler", resource.GetRetentionStatsHandler},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := tt.getter()
			require.NotNil(t, handler, fmt.Sprintf("%s should return a non-nil handler", tt.name))
		})
	}
}
