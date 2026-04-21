package operator

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func TestNewSettingsResource(t *testing.T) {
	res := NewSettingsResource(nil, nil)
	assert.NotNil(t, res)
	assert.Nil(t, res.settingsService)
	assert.Nil(t, res.db)
}

func TestRenderOperatorSettingsError_DefinitionNotFound(t *testing.T) {
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
	err := assert.AnError

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/test", nil)

	renderOperatorSettingsError(w, r, err)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestEnforcePresenceModeSwitchGuard_IgnoresOtherKeys(t *testing.T) {
	// Other settings short-circuit before the attendance query — so the tx
	// is never touched. A zero-value bun.Tx is safe here because we assert
	// that code path specifically.
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		bun.Tx{},
		configModel.KeyCheckoutSchulhofEnabled, // any key != presence_mode
		false,
	)
	assert.NoError(t, err, "non-presence_mode keys must pass through the guard unchecked")
}

func TestEnforcePresenceModeSwitchGuard_ForceBypass(t *testing.T) {
	// The ?force=true escape hatch skips the attendance check entirely so
	// operators can recover from stuck rows without re-running daily end.
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		bun.Tx{},
		configModel.KeyPresenceMode,
		true, // force
	)
	assert.NoError(t, err, "force=true must bypass the attendance check")
}
