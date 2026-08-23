package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

func TestNewSettingsResource(t *testing.T) {
	t.Parallel()

	res := NewSettingsResource(nil, nil, nil, nil, nil)
	assert.NotNil(t, res)
	assert.Nil(t, res.settingsService)
	assert.Nil(t, res.db)
	assert.Nil(t, res.schoolService)
	assert.NotNil(t, res.operatorSettings)
}

func TestRenderOperatorSettingsError_DefinitionNotFound(t *testing.T) {
	t.Parallel()

	err := &configSvc.SettingsError{
		Op:  "resolve",
		Err: &configSvc.DefinitionNotFoundError{Key: "bad.key"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRenderOperatorSettingsError_InvalidValue(t *testing.T) {
	t.Parallel()

	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: &configSvc.InvalidValueError{Key: "test", Reason: "too small"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRenderOperatorSettingsError_PermissionDenied(t *testing.T) {
	t.Parallel()

	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: &configSvc.PermissionDeniedError{Key: "admin.setting", RequiredPermission: "config:manage"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRenderOperatorSettingsError_GenericSettingsError(t *testing.T) {
	t.Parallel()

	err := &configSvc.SettingsError{
		Op:  "set_value",
		Err: assert.AnError,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestRenderOperatorSettingsError_NonSettingsError(t *testing.T) {
	t.Parallel()

	err := assert.AnError

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
