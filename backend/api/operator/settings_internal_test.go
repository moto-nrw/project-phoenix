package operator

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
)

func TestNewSettingsResource(t *testing.T) {
	res := NewSettingsResource(nil, nil, nil, nil, nil)
	assert.NotNil(t, res)
	assert.Nil(t, res.settingsService)
	assert.Nil(t, res.db)
	assert.Nil(t, res.broadcaster)
	assert.Nil(t, res.schoolService)
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
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		nil,
		configModel.KeyCheckoutSchulhofEnabled,
		false,
	)
	assert.NoError(t, err)
}

func TestEnforcePresenceModeSwitchGuard_ForceBypass(t *testing.T) {
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		nil,
		configModel.KeyPresenceMode,
		true,
	)
	assert.NoError(t, err)
}

func TestErrPresenceModeSwitchBlocked_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("set value: %w", ErrPresenceModeSwitchBlocked)
	assert.True(t, errors.Is(wrapped, ErrPresenceModeSwitchBlocked))

	other := errors.New(errPresenceModeSwitchBlockedMsg)
	assert.False(t, errors.Is(other, ErrPresenceModeSwitchBlocked))
}

func TestEnforcePresenceModeSwitchGuard_NonPresenceForceIsNoop(t *testing.T) {
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		nil,
		configModel.KeyCheckoutSchulhofEnabled,
		true,
	)
	assert.NoError(t, err)
}
