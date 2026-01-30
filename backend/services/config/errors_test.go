package config

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestConfigErrorVariables tests that error variables have correct messages
func TestConfigErrorVariables(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{"ErrSettingNotFound", ErrSettingNotFound, "setting not found"},
		{"ErrInvalidSettingData", ErrInvalidSettingData, "invalid setting data"},
		{"ErrDuplicateKey", ErrDuplicateKey, "duplicate setting key"},
		{"ErrValueParsingFailed", ErrValueParsingFailed, "failed to parse setting value"},
		{"ErrSystemSettingsLocked", ErrSystemSettingsLocked, "system settings are locked"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

// TestConfigErrorVariablesAreDistinct ensures each error is unique
func TestConfigErrorVariablesAreDistinct(t *testing.T) {
	errorVars := []error{
		ErrSettingNotFound,
		ErrInvalidSettingData,
		ErrDuplicateKey,
		ErrValueParsingFailed,
		ErrSystemSettingsLocked,
	}

	for i, err1 := range errorVars {
		for j, err2 := range errorVars {
			if i == j {
				assert.True(t, errors.Is(err1, err2))
			} else {
				assert.False(t, errors.Is(err1, err2))
			}
		}
	}
}

// TestConfigError tests the ConfigError wrapper type
func TestConfigError(t *testing.T) {
	t.Run("Error message format", func(t *testing.T) {
		underlyingErr := errors.New("connection timeout")
		configErr := &ConfigError{
			Op:  "GetSetting",
			Err: underlyingErr,
		}

		expected := "config service error in GetSetting: connection timeout"
		assert.Equal(t, expected, configErr.Error())
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlyingErr := ErrSettingNotFound
		configErr := &ConfigError{
			Op:  "GetSetting",
			Err: underlyingErr,
		}

		assert.Equal(t, underlyingErr, configErr.Unwrap())
	})

	t.Run("errors.Is works with wrapped ConfigError", func(t *testing.T) {
		configErr := &ConfigError{
			Op:  "GetSetting",
			Err: ErrSettingNotFound,
		}

		assert.True(t, errors.Is(configErr, ErrSettingNotFound))
	})
}

// TestSettingNotFoundError tests the SettingNotFoundError type
func TestSettingNotFoundError(t *testing.T) {
	t.Run("Error message includes key", func(t *testing.T) {
		err := &SettingNotFoundError{
			Key: "app.theme",
		}

		expected := "setting not found: app.theme"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("Unwrap returns base error", func(t *testing.T) {
		err := &SettingNotFoundError{
			Key: "app.theme",
		}

		assert.Equal(t, ErrSettingNotFound, err.Unwrap())
	})

	t.Run("errors.Is identifies as ErrSettingNotFound", func(t *testing.T) {
		err := &SettingNotFoundError{
			Key: "app.theme",
		}

		assert.True(t, errors.Is(err, ErrSettingNotFound))
	})
}

// TestDuplicateKeyError tests the DuplicateKeyError type
func TestDuplicateKeyError(t *testing.T) {
	t.Run("Error message includes key", func(t *testing.T) {
		err := &DuplicateKeyError{
			Key: "app.theme",
		}

		expected := "duplicate setting key: app.theme"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("Unwrap returns base error", func(t *testing.T) {
		err := &DuplicateKeyError{
			Key: "app.theme",
		}

		assert.Equal(t, ErrDuplicateKey, err.Unwrap())
	})

	t.Run("errors.Is identifies as ErrDuplicateKey", func(t *testing.T) {
		err := &DuplicateKeyError{
			Key: "app.theme",
		}

		assert.True(t, errors.Is(err, ErrDuplicateKey))
	})
}

// TestValueParsingError tests the ValueParsingError type
func TestValueParsingError(t *testing.T) {
	t.Run("Error message includes key, value, and type", func(t *testing.T) {
		err := &ValueParsingError{
			Key:   "app.maxUsers",
			Value: "abc",
			Type:  "integer",
		}

		expected := "failed to parse 'abc' as integer for setting: app.maxUsers"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("Unwrap returns base error", func(t *testing.T) {
		err := &ValueParsingError{
			Key:   "app.maxUsers",
			Value: "abc",
			Type:  "integer",
		}

		assert.Equal(t, ErrValueParsingFailed, err.Unwrap())
	})

	t.Run("errors.Is identifies as ErrValueParsingFailed", func(t *testing.T) {
		err := &ValueParsingError{
			Key:   "app.maxUsers",
			Value: "abc",
			Type:  "integer",
		}

		assert.True(t, errors.Is(err, ErrValueParsingFailed))
	})
}

// TestSystemSettingsLockedError tests the SystemSettingsLockedError type
func TestSystemSettingsLockedError(t *testing.T) {
	t.Run("Error message includes key", func(t *testing.T) {
		err := &SystemSettingsLockedError{
			Key: "system.version",
		}

		expected := "system setting is locked: system.version"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("Unwrap returns base error", func(t *testing.T) {
		err := &SystemSettingsLockedError{
			Key: "system.version",
		}

		assert.Equal(t, ErrSystemSettingsLocked, err.Unwrap())
	})

	t.Run("errors.Is identifies as ErrSystemSettingsLocked", func(t *testing.T) {
		err := &SystemSettingsLockedError{
			Key: "system.version",
		}

		assert.True(t, errors.Is(err, ErrSystemSettingsLocked))
	})
}

// TestBatchOperationError tests the BatchOperationError type
func TestBatchOperationError(t *testing.T) {
	t.Run("Error message includes count", func(t *testing.T) {
		err := &BatchOperationError{
			Errors: []error{
				errors.New("error 1"),
				errors.New("error 2"),
			},
		}

		expected := "batch operation failed with 2 errors"
		assert.Equal(t, expected, err.Error())
	})

	t.Run("AddError appends to error list", func(t *testing.T) {
		err := &BatchOperationError{}

		assert.False(t, err.HasErrors())

		err.AddError(errors.New("first error"))
		assert.True(t, err.HasErrors())
		assert.Len(t, err.Errors, 1)

		err.AddError(errors.New("second error"))
		assert.Len(t, err.Errors, 2)
	})

	t.Run("HasErrors returns false when empty", func(t *testing.T) {
		err := &BatchOperationError{}
		assert.False(t, err.HasErrors())
	})

	t.Run("HasErrors returns true when has errors", func(t *testing.T) {
		err := &BatchOperationError{
			Errors: []error{errors.New("test")},
		}
		assert.True(t, err.HasErrors())
	})

	t.Run("can add multiple errors", func(t *testing.T) {
		err := &BatchOperationError{}

		for i := 0; i < 5; i++ {
			err.AddError(errors.New("error"))
		}

		assert.Len(t, err.Errors, 5)
		assert.Equal(t, "batch operation failed with 5 errors", err.Error())
	})
}
