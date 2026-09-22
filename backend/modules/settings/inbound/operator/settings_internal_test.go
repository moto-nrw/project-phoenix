package operator

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/stretchr/testify/assert"
)

func TestNewSettingsResource(t *testing.T) {
	t.Parallel()

	res := NewSettingsResource(SettingsConfig{})
	assert.NotNil(t, res)
	assert.Nil(t, res.settings)
	assert.Nil(t, res.schoolService)
}

func TestRenderOperatorSettingsError_DefinitionNotFound(t *testing.T) {
	t.Parallel()

	err := &settings.SettingsError{
		Op:  "resolve",
		Err: &settings.DefinitionNotFoundError{Key: "bad.key"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestRenderOperatorSettingsError_InvalidValue(t *testing.T) {
	t.Parallel()

	err := &settings.SettingsError{
		Op:  "set_value",
		Err: &settings.InvalidValueError{Key: "test", Reason: "too small"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestRenderOperatorSettingsError_PermissionDenied(t *testing.T) {
	t.Parallel()

	err := &settings.SettingsError{
		Op:  "set_value",
		Err: &settings.PermissionDeniedError{Key: "admin.setting", RequiredPermission: "config:manage"},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestRenderOperatorSettingsError_GenericSettingsError(t *testing.T) {
	t.Parallel()

	err := &settings.SettingsError{
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
