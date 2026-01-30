package config

import (
	"errors"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorInvalidRequest(t *testing.T) {
	err := errors.New("invalid input")
	renderer := ErrorInvalidRequest(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "invalid input")
}

func TestErrorInternalServer(t *testing.T) {
	err := errors.New("internal failure")
	renderer := ErrorInternalServer(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "internal failure")
}

func TestErrorNotFound(t *testing.T) {
	err := errors.New("resource missing")
	renderer := ErrorNotFound(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "resource missing")
}

func TestErrorForbidden(t *testing.T) {
	err := errors.New("access denied")
	renderer := ErrorForbidden(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "access denied")
}

func TestErrorConflict(t *testing.T) {
	err := errors.New("duplicate entry")
	renderer := ErrorConflict(err)

	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "duplicate entry")
}

// Test mapSentinelError function via ErrorRenderer

func TestMapSentinelError_SettingNotFound(t *testing.T) {
	err := &configSvc.ConfigError{
		Op:  "GetSetting",
		Err: configSvc.ErrSettingNotFound,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

func TestMapSentinelError_InvalidSettingData(t *testing.T) {
	err := &configSvc.ConfigError{
		Op:  "ValidateSetting",
		Err: configSvc.ErrInvalidSettingData,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

func TestMapSentinelError_ValueParsingFailed(t *testing.T) {
	err := &configSvc.ConfigError{
		Op:  "ParseValue",
		Err: configSvc.ErrValueParsingFailed,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

func TestMapSentinelError_DuplicateKey(t *testing.T) {
	err := &configSvc.ConfigError{
		Op:  "CreateSetting",
		Err: configSvc.ErrDuplicateKey,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

func TestMapSentinelError_SystemSettingsLocked(t *testing.T) {
	err := &configSvc.ConfigError{
		Op:  "UpdateSystemSetting",
		Err: configSvc.ErrSystemSettingsLocked,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

// Test mapErrorType function via ErrorRenderer

func TestMapErrorType_SettingNotFoundError(t *testing.T) {
	err := &configSvc.ConfigError{
		Op: "GetSetting",
		Err: &configSvc.SettingNotFoundError{
			Key: "test.key",
		},
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "test.key")
}

func TestMapErrorType_DuplicateKeyError(t *testing.T) {
	err := &configSvc.ConfigError{
		Op: "CreateSetting",
		Err: &configSvc.DuplicateKeyError{
			Key: "duplicate.key",
		},
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "duplicate.key")
}

func TestMapErrorType_ValueParsingError(t *testing.T) {
	err := &configSvc.ConfigError{
		Op: "ParseValue",
		Err: &configSvc.ValueParsingError{
			Key:   "numeric.key",
			Value: "not_a_number",
			Type:  "int",
		},
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "numeric.key")
	assert.Contains(t, errResp.ErrorText, "not_a_number")
}

func TestMapErrorType_SystemSettingsLockedError(t *testing.T) {
	err := &configSvc.ConfigError{
		Op: "UpdateSystemSetting",
		Err: &configSvc.SystemSettingsLockedError{
			Key: "system.locked.key",
		},
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "system.locked.key")
}

func TestMapErrorType_BatchOperationError(t *testing.T) {
	batchErr := &configSvc.BatchOperationError{}
	batchErr.AddError(errors.New("error 1"))
	batchErr.AddError(errors.New("error 2"))

	err := &configSvc.ConfigError{
		Op:  "BatchUpdate",
		Err: batchErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "2 errors")
}

func TestMapErrorType_UnknownConfigError(t *testing.T) {
	// ConfigError with unknown underlying error type should fall to default case
	unknownErr := errors.New("unknown config error")
	err := &configSvc.ConfigError{
		Op:  "UnknownOperation",
		Err: unknownErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "unknown config error")
}

func TestErrorRenderer_NonConfigError(t *testing.T) {
	// Non-ConfigError should be treated as internal server error
	err := errors.New("some random error")

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
	assert.Contains(t, errResp.ErrorText, "some random error")
}

func TestErrorRenderer_ConfigErrorNilUnwrap(t *testing.T) {
	// ConfigError with nil Err (Unwrap returns nil) should fall through sentinel mapping
	// and hit default case in type mapping
	err := &configSvc.ConfigError{
		Op:  "SomeOperation",
		Err: nil,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	assert.Equal(t, http.StatusInternalServerError, errResp.HTTPStatusCode)
	assert.Equal(t, "error", errResp.Status)
}

// Test that sentinel errors take precedence over typed errors

func TestSentinelPrecedence_SettingNotFound(t *testing.T) {
	// SettingNotFoundError wraps ErrSettingNotFound
	// Both sentinel and type mapping should return 404
	typedErr := &configSvc.SettingNotFoundError{
		Key: "test.key",
	}

	err := &configSvc.ConfigError{
		Op:  "GetSetting",
		Err: typedErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	// Should be 404 from sentinel mapping (which runs first)
	assert.Equal(t, http.StatusNotFound, errResp.HTTPStatusCode)
}

func TestSentinelPrecedence_DuplicateKey(t *testing.T) {
	// DuplicateKeyError wraps ErrDuplicateKey
	// Both sentinel and type mapping should return 409
	typedErr := &configSvc.DuplicateKeyError{
		Key: "test.key",
	}

	err := &configSvc.ConfigError{
		Op:  "CreateSetting",
		Err: typedErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	// Should be 409 from sentinel mapping (which runs first)
	assert.Equal(t, http.StatusConflict, errResp.HTTPStatusCode)
}

func TestSentinelPrecedence_ValueParsing(t *testing.T) {
	// ValueParsingError wraps ErrValueParsingFailed
	// Both sentinel and type mapping should return 400
	typedErr := &configSvc.ValueParsingError{
		Key:   "test.key",
		Value: "invalid",
		Type:  "int",
	}

	err := &configSvc.ConfigError{
		Op:  "ParseValue",
		Err: typedErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	// Should be 400 from sentinel mapping (which runs first)
	assert.Equal(t, http.StatusBadRequest, errResp.HTTPStatusCode)
}

func TestSentinelPrecedence_SystemSettingsLocked(t *testing.T) {
	// SystemSettingsLockedError wraps ErrSystemSettingsLocked
	// Both sentinel and type mapping should return 403
	typedErr := &configSvc.SystemSettingsLockedError{
		Key: "test.key",
	}

	err := &configSvc.ConfigError{
		Op:  "UpdateSystemSetting",
		Err: typedErr,
	}

	renderer := ErrorRenderer(err)
	require.NotNil(t, renderer)

	errResp, ok := renderer.(*common.ErrResponse)
	require.True(t, ok, "Expected *common.ErrResponse")
	// Should be 403 from sentinel mapping (which runs first)
	assert.Equal(t, http.StatusForbidden, errResp.HTTPStatusCode)
}
