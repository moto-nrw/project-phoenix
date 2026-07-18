package config

import (
	"errors"
	"fmt"
)

// Sentinel errors for the settings service.
var (
	ErrDefinitionNotFound = errors.New("setting definition not found in registry")
	ErrInvalidValue       = errors.New("value does not pass validation")
	ErrPermissionDenied   = errors.New("insufficient permissions for this setting")
	// ErrOperatorAdminOnly is returned by CheckOperatorWritable when an
	// operator attempts to write or reveal an AccessAdminOnly setting.
	ErrOperatorAdminOnly = errors.New("setting is admin-only and cannot be modified by operators")
)

// SettingsError wraps settings service errors with operation context.
type SettingsError struct {
	Op  string
	Err error
}

func (e *SettingsError) Error() string {
	return fmt.Sprintf("settings service error in %s: %v", e.Op, e.Err)
}

func (e *SettingsError) Unwrap() error {
	return e.Err
}

// DefinitionNotFoundError indicates a setting key is not registered.
type DefinitionNotFoundError struct {
	Key string
}

func (e *DefinitionNotFoundError) Error() string {
	return fmt.Sprintf("setting definition not found: %s", e.Key)
}

func (e *DefinitionNotFoundError) Unwrap() error {
	return ErrDefinitionNotFound
}

// InvalidValueError indicates a value doesn't meet validation rules.
type InvalidValueError struct {
	Key    string
	Reason string
}

func (e *InvalidValueError) Error() string {
	return fmt.Sprintf("invalid value for setting %s: %s", e.Key, e.Reason)
}

func (e *InvalidValueError) Unwrap() error {
	return ErrInvalidValue
}

// PermissionDeniedError indicates the user lacks the required write permission.
type PermissionDeniedError struct {
	Key                string
	RequiredPermission string
}

func (e *PermissionDeniedError) Error() string {
	return fmt.Sprintf("permission denied for setting %s: requires %s", e.Key, e.RequiredPermission)
}

func (e *PermissionDeniedError) Unwrap() error {
	return ErrPermissionDenied
}
