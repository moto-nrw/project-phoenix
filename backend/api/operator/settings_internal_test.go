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
	"github.com/uptrace/bun"
)

func TestNewSettingsResource(t *testing.T) {
	res := NewSettingsResource(nil, nil, nil)
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

// TestErrPresenceModeSwitchBlocked_IsSentinel locks in the contract the
// SetSchoolSettingValue handler relies on: the guard returns a comparable
// sentinel (not a freshly-allocated string) so wrapping does not break the
// 409 detection. errors.Is must match through fmt.Errorf("...: %w", ...) too.
func TestErrPresenceModeSwitchBlocked_IsSentinel(t *testing.T) {
	wrapped := fmt.Errorf("set value: %w", ErrPresenceModeSwitchBlocked)
	assert.True(t, errors.Is(wrapped, ErrPresenceModeSwitchBlocked),
		"wrapped errors must still be detectable via errors.Is")

	other := errors.New(errPresenceModeSwitchBlockedMsg)
	assert.False(t, errors.Is(other, ErrPresenceModeSwitchBlocked),
		"a fresh error with the same message must not match — sentinel identity matters")
}

func TestEnforcePresenceModeSwitchGuard_NonPresenceForceIsNoop(t *testing.T) {
	// force=true is presence-mode-specific in the handler, but the guard
	// itself short-circuits on key-mismatch first. Confirm both branches
	// (force on, key not presence_mode) leave the tx untouched.
	err := enforcePresenceModeSwitchGuard(
		context.Background(),
		bun.Tx{},
		configModel.KeyCheckoutSchulhofEnabled,
		true,
	)
	assert.NoError(t, err)
}
