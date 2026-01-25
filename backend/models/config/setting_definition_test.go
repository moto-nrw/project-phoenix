package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingType_Constants(t *testing.T) {
	assert.Equal(t, SettingType("bool"), SettingTypeBool)
	assert.Equal(t, SettingType("int"), SettingTypeInt)
	assert.Equal(t, SettingType("float"), SettingTypeFloat)
	assert.Equal(t, SettingType("string"), SettingTypeString)
	assert.Equal(t, SettingType("enum"), SettingTypeEnum)
	assert.Equal(t, SettingType("time"), SettingTypeTime)
	assert.Equal(t, SettingType("json"), SettingTypeJSON)
}

func TestScopeType_Constants(t *testing.T) {
	assert.Equal(t, ScopeType("system"), ScopeSystem)
	assert.Equal(t, ScopeType("school"), ScopeSchool)
	assert.Equal(t, ScopeType("og"), ScopeOG)
	assert.Equal(t, ScopeType("user"), ScopeUser)
	assert.Equal(t, ScopeType("device"), ScopeDevice)
}

func TestAllScopeTypes(t *testing.T) {
	scopes := AllScopeTypes()
	assert.Len(t, scopes, 5)
	assert.Contains(t, scopes, ScopeSystem)
	assert.Contains(t, scopes, ScopeSchool)
	assert.Contains(t, scopes, ScopeOG)
	assert.Contains(t, scopes, ScopeUser)
	assert.Contains(t, scopes, ScopeDevice)
}

func TestIsValidScopeType(t *testing.T) {
	tests := []struct {
		name     string
		scope    string
		expected bool
	}{
		{"system is valid", "system", true},
		{"school is valid", "school", true},
		{"og is valid", "og", true},
		{"user is valid", "user", true},
		{"device is valid", "device", true},
		{"invalid scope", "invalid", false},
		{"empty string", "", false},
		{"uppercase not valid", "SYSTEM", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidScopeType(tt.scope)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSettingDefinition_TableName(t *testing.T) {
	d := &SettingDefinition{}
	assert.Equal(t, "config.setting_definitions", d.TableName())
}

func TestSettingDefinition_Validate(t *testing.T) {
	validDefault, _ := json.Marshal(map[string]any{"value": true})

	tests := []struct {
		name        string
		definition  SettingDefinition
		expectError string
	}{
		{
			name: "valid bool definition",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "",
		},
		{
			name: "missing key",
			definition: SettingDefinition{
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "key is required",
		},
		{
			name: "missing type",
			definition: SettingDefinition{
				Key:              "test.setting",
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "type is required",
		},
		{
			name: "invalid type",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingType("invalid"),
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "invalid setting type",
		},
		{
			name: "missing default value",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "default_value is required",
		},
		{
			name: "missing category",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "category is required",
		},
		{
			name: "missing allowed scopes",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				Category:         "test",
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "allowed_scopes is required",
		},
		{
			name: "invalid scope in allowed scopes",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"invalid"},
				ScopePermissions: map[string]string{"invalid": "config:write"},
			},
			expectError: "invalid scope type",
		},
		{
			name: "missing scope permissions",
			definition: SettingDefinition{
				Key:           "test.setting",
				Type:          SettingTypeBool,
				DefaultValue:  validDefault,
				Category:      "test",
				AllowedScopes: []string{"system"},
			},
			expectError: "scope_permissions is required",
		},
		{
			name: "missing permission for allowed scope",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeBool,
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system", "og"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "missing permission for scope: og",
		},
		{
			name: "enum without options",
			definition: SettingDefinition{
				Key:              "test.setting",
				Type:             SettingTypeEnum,
				DefaultValue:     validDefault,
				Category:         "test",
				AllowedScopes:    []string{"system"},
				ScopePermissions: map[string]string{"system": "config:write"},
			},
			expectError: "enum type requires validation.options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.definition.Validate()
			if tt.expectError == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			}
		})
	}
}

func TestSettingDefinition_Validate_Normalizes(t *testing.T) {
	validDefault, _ := json.Marshal(map[string]any{"value": true})

	d := SettingDefinition{
		Key:              "TEST.SETTING",
		Type:             SettingTypeBool,
		DefaultValue:     validDefault,
		Category:         "TEST",
		AllowedScopes:    []string{"system"},
		ScopePermissions: map[string]string{"system": "config:write"},
	}

	err := d.Validate()
	require.NoError(t, err)
	assert.Equal(t, "test.setting", d.Key, "key should be lowercased")
	assert.Equal(t, "test", d.Category, "category should be lowercased")
}

func TestSettingDefinition_IsScopeAllowed(t *testing.T) {
	d := &SettingDefinition{
		AllowedScopes: []string{"system", "og"},
	}

	assert.True(t, d.IsScopeAllowed(ScopeSystem))
	assert.True(t, d.IsScopeAllowed(ScopeOG))
	assert.False(t, d.IsScopeAllowed(ScopeUser))
	assert.False(t, d.IsScopeAllowed(ScopeDevice))
}

func TestSettingDefinition_GetPermissionForScope(t *testing.T) {
	d := &SettingDefinition{
		ScopePermissions: map[string]string{
			"system": "config:admin",
			"og":     "config:write",
		},
	}

	assert.Equal(t, "config:admin", d.GetPermissionForScope(ScopeSystem))
	assert.Equal(t, "config:write", d.GetPermissionForScope(ScopeOG))
	assert.Equal(t, "", d.GetPermissionForScope(ScopeUser))
}

func TestSettingDefinition_GetDefaultValueTyped(t *testing.T) {
	tests := []struct {
		name         string
		settingType  SettingType
		defaultValue any
		expected     any
		expectError  bool
	}{
		{
			name:         "bool true",
			settingType:  SettingTypeBool,
			defaultValue: true,
			expected:     true,
		},
		{
			name:         "bool false",
			settingType:  SettingTypeBool,
			defaultValue: false,
			expected:     false,
		},
		{
			name:         "int value",
			settingType:  SettingTypeInt,
			defaultValue: float64(42), // JSON numbers are float64
			expected:     42,
		},
		{
			name:         "float value",
			settingType:  SettingTypeFloat,
			defaultValue: 3.14,
			expected:     3.14,
		},
		{
			name:         "string value",
			settingType:  SettingTypeString,
			defaultValue: "hello",
			expected:     "hello",
		},
		{
			name:         "enum value",
			settingType:  SettingTypeEnum,
			defaultValue: "option1",
			expected:     "option1",
		},
		{
			name:         "time value",
			settingType:  SettingTypeTime,
			defaultValue: "14:30",
			expected:     "14:30",
		},
		{
			name:         "json value",
			settingType:  SettingTypeJSON,
			defaultValue: map[string]any{"key": "value"},
			expected:     map[string]any{"key": "value"},
		},
		{
			name:         "type mismatch - expected bool got string",
			settingType:  SettingTypeBool,
			defaultValue: "not a bool",
			expectError:  true,
		},
		{
			name:         "type mismatch - expected int got string",
			settingType:  SettingTypeInt,
			defaultValue: "not an int",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defaultBytes, _ := json.Marshal(map[string]any{"value": tt.defaultValue})
			d := &SettingDefinition{
				Type:         tt.settingType,
				DefaultValue: defaultBytes,
			}

			result, err := d.GetDefaultValueTyped()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSettingDefinition_ValidateValue(t *testing.T) {
	min := float64(1)
	max := float64(100)

	tests := []struct {
		name        string
		definition  SettingDefinition
		value       any
		expectError string
	}{
		{
			name:        "valid bool true",
			definition:  SettingDefinition{Type: SettingTypeBool},
			value:       true,
			expectError: "",
		},
		{
			name:        "valid bool false",
			definition:  SettingDefinition{Type: SettingTypeBool},
			value:       false,
			expectError: "",
		},
		{
			name:        "invalid bool - string",
			definition:  SettingDefinition{Type: SettingTypeBool},
			value:       "true",
			expectError: "value must be a boolean",
		},
		{
			name:        "valid int",
			definition:  SettingDefinition{Type: SettingTypeInt},
			value:       42,
			expectError: "",
		},
		{
			name:        "valid int from int64",
			definition:  SettingDefinition{Type: SettingTypeInt},
			value:       int64(42),
			expectError: "",
		},
		{
			name:        "valid int from float64",
			definition:  SettingDefinition{Type: SettingTypeInt},
			value:       float64(42),
			expectError: "",
		},
		{
			name:        "invalid int - string",
			definition:  SettingDefinition{Type: SettingTypeInt},
			value:       "42",
			expectError: "value must be an integer",
		},
		{
			name: "int below min",
			definition: SettingDefinition{
				Type:       SettingTypeInt,
				Validation: &Validation{Min: &min},
			},
			value:       0,
			expectError: "value must be at least 1",
		},
		{
			name: "int above max",
			definition: SettingDefinition{
				Type:       SettingTypeInt,
				Validation: &Validation{Max: &max},
			},
			value:       150,
			expectError: "value must be at most 100",
		},
		{
			name:        "valid float",
			definition:  SettingDefinition{Type: SettingTypeFloat},
			value:       3.14,
			expectError: "",
		},
		{
			name:        "valid float from float32",
			definition:  SettingDefinition{Type: SettingTypeFloat},
			value:       float32(3.14),
			expectError: "",
		},
		{
			name:        "valid float from int",
			definition:  SettingDefinition{Type: SettingTypeFloat},
			value:       42,
			expectError: "",
		},
		{
			name:        "invalid float - string",
			definition:  SettingDefinition{Type: SettingTypeFloat},
			value:       "3.14",
			expectError: "value must be a number",
		},
		{
			name: "float below min",
			definition: SettingDefinition{
				Type:       SettingTypeFloat,
				Validation: &Validation{Min: &min},
			},
			value:       0.5,
			expectError: "value must be at least",
		},
		{
			name:        "valid string",
			definition:  SettingDefinition{Type: SettingTypeString},
			value:       "hello",
			expectError: "",
		},
		{
			name:        "invalid string - int",
			definition:  SettingDefinition{Type: SettingTypeString},
			value:       42,
			expectError: "value must be a string",
		},
		{
			name: "valid enum",
			definition: SettingDefinition{
				Type:       SettingTypeEnum,
				Validation: &Validation{Options: []string{"opt1", "opt2"}},
			},
			value:       "opt1",
			expectError: "",
		},
		{
			name: "invalid enum - not in options",
			definition: SettingDefinition{
				Type:       SettingTypeEnum,
				Validation: &Validation{Options: []string{"opt1", "opt2"}},
			},
			value:       "opt3",
			expectError: "value must be one of: opt1, opt2",
		},
		{
			name:        "valid time format",
			definition:  SettingDefinition{Type: SettingTypeTime},
			value:       "14:30",
			expectError: "",
		},
		{
			name:        "invalid time - not a string",
			definition:  SettingDefinition{Type: SettingTypeTime},
			value:       1430,
			expectError: "value must be a time string",
		},
		{
			name:        "invalid time format",
			definition:  SettingDefinition{Type: SettingTypeTime},
			value:       "14-30",
			expectError: "value must be in HH:MM format",
		},
		{
			name:        "invalid time - wrong length",
			definition:  SettingDefinition{Type: SettingTypeTime},
			value:       "1:30",
			expectError: "value must be in HH:MM format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.definition.ValidateValue(tt.value)
			if tt.expectError == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			}
		})
	}
}

func TestMarshalValue(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		expected any // JSON unmarshals numbers as float64
	}{
		{"bool", true, true},
		{"int", 42, float64(42)}, // JSON numbers become float64
		{"float", 3.14, 3.14},
		{"string", "hello", "hello"},
		{"nil", nil, nil},
		{"map", map[string]any{"key": "value"}, map[string]any{"key": "value"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := MarshalValue(tt.value)
			require.NoError(t, err)
			assert.NotEmpty(t, result)

			// Verify it can be unmarshaled back
			var wrapper struct {
				Value any `json:"value"`
			}
			err = json.Unmarshal(result, &wrapper)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, wrapper.Value)
		})
	}
}

func TestSettingDefinition_ValidEnumWithOptions(t *testing.T) {
	enumDefault, _ := json.Marshal(map[string]any{"value": "option1"})

	d := SettingDefinition{
		Key:              "test.enum",
		Type:             SettingTypeEnum,
		DefaultValue:     enumDefault,
		Category:         "test",
		AllowedScopes:    []string{"system"},
		ScopePermissions: map[string]string{"system": "config:write"},
		Validation:       &Validation{Options: []string{"option1", "option2"}},
	}

	err := d.Validate()
	assert.NoError(t, err)
}
