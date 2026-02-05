package config

// ValueType represents the data type of a setting value
type ValueType string

const (
	// ValueTypeString is a simple text value
	ValueTypeString ValueType = "string"
	// ValueTypeInt is an integer value
	ValueTypeInt ValueType = "int"
	// ValueTypeFloat is a floating-point value
	ValueTypeFloat ValueType = "float"
	// ValueTypeBool is a boolean value
	ValueTypeBool ValueType = "bool"
	// ValueTypeEnum is a value from a predefined list
	ValueTypeEnum ValueType = "enum"
	// ValueTypeTime is a time value (HH:MM:SS)
	ValueTypeTime ValueType = "time"
	// ValueTypeDuration is a duration value (e.g., "30m", "1h")
	ValueTypeDuration ValueType = "duration"
	// ValueTypeObjectRef is a reference to a database entity
	ValueTypeObjectRef ValueType = "object_ref"
	// ValueTypeJSON is arbitrary JSON data
	ValueTypeJSON ValueType = "json"
	// ValueTypeAction is an executable action (not a stored value)
	ValueTypeAction ValueType = "action"
)

// IsValid checks if the value type is valid
func (v ValueType) IsValid() bool {
	switch v {
	case ValueTypeString, ValueTypeInt, ValueTypeFloat, ValueTypeBool,
		ValueTypeEnum, ValueTypeTime, ValueTypeDuration, ValueTypeObjectRef,
		ValueTypeJSON, ValueTypeAction:
		return true
	}
	return false
}

// Scope represents the hierarchy level where a setting can be configured
type Scope string

const (
	// ScopeSystem is the global default scope (lowest priority)
	ScopeSystem Scope = "system"
	// ScopeUser is user-specific preferences (medium priority)
	ScopeUser Scope = "user"
	// ScopeDevice is device-specific configuration (highest priority)
	ScopeDevice Scope = "device"
)

// IsValid checks if the scope is valid
func (s Scope) IsValid() bool {
	switch s {
	case ScopeSystem, ScopeUser, ScopeDevice:
		return true
	}
	return false
}

// Priority returns the priority level of the scope (higher = more specific)
func (s Scope) Priority() int {
	switch s {
	case ScopeDevice:
		return 3
	case ScopeUser:
		return 2
	case ScopeSystem:
		return 1
	}
	return 0
}

// AllScopes returns all valid scopes in resolution order (highest priority first)
func AllScopes() []Scope {
	return []Scope{ScopeDevice, ScopeUser, ScopeSystem}
}

// AllValueTypes returns all valid value types
func AllValueTypes() []ValueType {
	return []ValueType{
		ValueTypeString,
		ValueTypeInt,
		ValueTypeFloat,
		ValueTypeBool,
		ValueTypeEnum,
		ValueTypeTime,
		ValueTypeDuration,
		ValueTypeObjectRef,
		ValueTypeJSON,
		ValueTypeAction,
	}
}

// ScopeContext provides the context for resolving effective setting values
type ScopeContext struct {
	// AccountID is the user's account ID (for user scope)
	AccountID *int64
	// DeviceID is the device ID (for device scope)
	DeviceID *int64
}

// HasUserContext returns true if user context is available
func (sc *ScopeContext) HasUserContext() bool {
	return sc != nil && sc.AccountID != nil
}

// HasDeviceContext returns true if device context is available
func (sc *ScopeContext) HasDeviceContext() bool {
	return sc != nil && sc.DeviceID != nil
}
