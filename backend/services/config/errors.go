package config

import (
	"errors"
	"fmt"

	svcerrors "github.com/moto-nrw/project-phoenix/services/errors"
)

// Common error types
var (
	ErrSettingNotFound      = errors.New("setting not found")
	ErrInvalidSettingData   = errors.New("invalid setting data")
	ErrDuplicateKey         = errors.New("duplicate setting key")
	ErrValueParsingFailed   = errors.New("failed to parse setting value")
	ErrSystemSettingsLocked = errors.New("system settings are locked")
	ErrInvalidID            = errors.New("invalid ID")
	ErrKeyEmpty             = errors.New("key cannot be empty")
	ErrCategoryEmpty        = errors.New("category cannot be empty")
	ErrKeyAndCategoryEmpty  = errors.New("key and category cannot be empty")
)

// ConfigError wraps config service errors with operation context
type ConfigError struct {
	Op  string // The operation that failed
	Err error  // The underlying error
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("config service error in %s: %v", e.Op, e.Err)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// SettingNotFoundError wraps a setting not found error
type SettingNotFoundError struct {
	Key string
}

func (e *SettingNotFoundError) Error() string {
	return fmt.Sprintf("setting not found: %s", e.Key)
}

func (e *SettingNotFoundError) Unwrap() error {
	return ErrSettingNotFound
}

// DuplicateKeyError wraps a duplicate key error
type DuplicateKeyError struct {
	Key string
}

func (e *DuplicateKeyError) Error() string {
	return fmt.Sprintf("duplicate setting key: %s", e.Key)
}

func (e *DuplicateKeyError) Unwrap() error {
	return ErrDuplicateKey
}

// ValueParsingError wraps a value parsing error
type ValueParsingError struct {
	Key   string
	Value string
	Type  string
}

func (e *ValueParsingError) Error() string {
	return fmt.Sprintf("failed to parse '%s' as %s for setting: %s", e.Value, e.Type, e.Key)
}

func (e *ValueParsingError) Unwrap() error {
	return ErrValueParsingFailed
}

// SystemSettingsLockedError wraps a system settings locked error
type SystemSettingsLockedError struct {
	Key string
}

func (e *SystemSettingsLockedError) Error() string {
	return fmt.Sprintf("system setting is locked: %s", e.Key)
}

func (e *SystemSettingsLockedError) Unwrap() error {
	return ErrSystemSettingsLocked
}

// BatchOperationError is re-exported from services/errors for backwards compatibility.
type BatchOperationError = svcerrors.BatchOperationError
