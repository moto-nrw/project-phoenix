package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingValue_TableName(t *testing.T) {
	v := &SettingValue{}
	assert.Equal(t, "config.setting_values", v.TableName())
}

func TestSettingValue_Validate(t *testing.T) {
	validValue, _ := json.Marshal(map[string]any{"value": true})
	validScopeID := int64(1)
	zeroScopeID := int64(0)

	tests := []struct {
		name        string
		value       SettingValue
		expectError string
	}{
		{
			name: "valid system scope",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "system",
				ScopeID:      nil,
				Value:        validValue,
			},
			expectError: "",
		},
		{
			name: "valid og scope",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "og",
				ScopeID:      &validScopeID,
				Value:        validValue,
			},
			expectError: "",
		},
		{
			name: "valid user scope",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "user",
				ScopeID:      &validScopeID,
				Value:        validValue,
			},
			expectError: "",
		},
		{
			name: "missing definition_id",
			value: SettingValue{
				ScopeType: "system",
				ScopeID:   nil,
				Value:     validValue,
			},
			expectError: "definition_id is required",
		},
		{
			name: "zero definition_id",
			value: SettingValue{
				DefinitionID: 0,
				ScopeType:    "system",
				ScopeID:      nil,
				Value:        validValue,
			},
			expectError: "definition_id is required",
		},
		{
			name: "missing scope_type",
			value: SettingValue{
				DefinitionID: 1,
				ScopeID:      nil,
				Value:        validValue,
			},
			expectError: "scope_type is required",
		},
		{
			name: "invalid scope_type",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "invalid",
				ScopeID:      nil,
				Value:        validValue,
			},
			expectError: "invalid scope_type",
		},
		{
			name: "system scope with scope_id",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "system",
				ScopeID:      &validScopeID,
				Value:        validValue,
			},
			expectError: "system scope must not have scope_id",
		},
		{
			name: "non-system scope without scope_id",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "og",
				ScopeID:      nil,
				Value:        validValue,
			},
			expectError: "non-system scope requires scope_id",
		},
		{
			name: "non-system scope with zero scope_id",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "og",
				ScopeID:      &zeroScopeID,
				Value:        validValue,
			},
			expectError: "non-system scope requires scope_id",
		},
		{
			name: "missing value",
			value: SettingValue{
				DefinitionID: 1,
				ScopeType:    "system",
				ScopeID:      nil,
			},
			expectError: "value is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.value.Validate()
			if tt.expectError == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			}
		})
	}
}

func TestSettingValue_GetTypedValue(t *testing.T) {
	tests := []struct {
		name        string
		settingType SettingType
		rawValue    any
		expected    any
	}{
		{
			name:        "bool value",
			settingType: SettingTypeBool,
			rawValue:    true,
			expected:    true,
		},
		{
			name:        "int value",
			settingType: SettingTypeInt,
			rawValue:    float64(42),
			expected:    42,
		},
		{
			name:        "string value",
			settingType: SettingTypeString,
			rawValue:    "hello",
			expected:    "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valueBytes, _ := json.Marshal(map[string]any{"value": tt.rawValue})
			v := &SettingValue{Value: valueBytes}
			def := &SettingDefinition{Type: tt.settingType}

			result, err := v.GetTypedValue(def)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestScopeRef_NewSystemScope(t *testing.T) {
	scope := NewSystemScope()
	assert.Equal(t, ScopeSystem, scope.Type)
	assert.Nil(t, scope.ID)
}

func TestScopeRef_NewOGScope(t *testing.T) {
	ogID := int64(123)
	scope := NewOGScope(ogID)
	assert.Equal(t, ScopeOG, scope.Type)
	require.NotNil(t, scope.ID)
	assert.Equal(t, ogID, *scope.ID)
}

func TestScopeRef_NewUserScope(t *testing.T) {
	personID := int64(456)
	scope := NewUserScope(personID)
	assert.Equal(t, ScopeUser, scope.Type)
	require.NotNil(t, scope.ID)
	assert.Equal(t, personID, *scope.ID)
}

func TestScopeRef_String(t *testing.T) {
	tests := []struct {
		name     string
		scope    ScopeRef
		contains string
	}{
		{
			name:     "system scope without ID",
			scope:    NewSystemScope(),
			contains: "system",
		},
		{
			name:     "og scope with ID",
			scope:    NewOGScope(1),
			contains: "og",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.scope.String()
			assert.Contains(t, result, tt.contains)
		})
	}
}

func TestResolvedSetting_Structure(t *testing.T) {
	// Test that ResolvedSetting can be marshaled to JSON correctly
	rs := ResolvedSetting{
		Key:         "test.setting",
		Value:       true,
		Type:        SettingTypeBool,
		Category:    "test",
		Description: "Test setting",
		IsDefault:   true,
		IsActive:    true,
		CanModify:   true,
	}

	jsonBytes, err := json.Marshal(rs)
	require.NoError(t, err)

	var unmarshaled ResolvedSetting
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, rs.Key, unmarshaled.Key)
	assert.Equal(t, rs.Value, unmarshaled.Value)
	assert.Equal(t, rs.Type, unmarshaled.Type)
	assert.Equal(t, rs.Category, unmarshaled.Category)
	assert.Equal(t, rs.Description, unmarshaled.Description)
	assert.Equal(t, rs.IsDefault, unmarshaled.IsDefault)
	assert.Equal(t, rs.IsActive, unmarshaled.IsActive)
	assert.Equal(t, rs.CanModify, unmarshaled.CanModify)
}

func TestResolvedSetting_WithSource(t *testing.T) {
	source := &ScopeRef{Type: ScopeOG, ID: func() *int64 { v := int64(123); return &v }()}

	rs := ResolvedSetting{
		Key:       "test.setting",
		Value:     "value",
		Type:      SettingTypeString,
		Category:  "test",
		Source:    source,
		IsDefault: false,
		IsActive:  true,
		CanModify: true,
	}

	jsonBytes, err := json.Marshal(rs)
	require.NoError(t, err)

	// Verify source is included in JSON
	var data map[string]any
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)
	assert.Contains(t, data, "source")
}

func TestResolvedSetting_WithDependency(t *testing.T) {
	dep := &SettingDependency{
		Key:       "parent.setting",
		Condition: "equals",
		Value:     true,
	}

	rs := ResolvedSetting{
		Key:       "test.setting",
		Value:     "value",
		Type:      SettingTypeString,
		Category:  "test",
		DependsOn: dep,
		IsDefault: true,
		IsActive:  false, // Depends on parent
		CanModify: true,
	}

	jsonBytes, err := json.Marshal(rs)
	require.NoError(t, err)

	var data map[string]any
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)
	assert.Contains(t, data, "depends_on")
}

func TestSettingChangeInfo_Structure(t *testing.T) {
	byID := int64(1)
	info := SettingChangeInfo{
		By:        "admin@example.com",
		ByID:      &byID,
		FromValue: 10,
		ToValue:   20,
	}

	jsonBytes, err := json.Marshal(info)
	require.NoError(t, err)

	var unmarshaled SettingChangeInfo
	err = json.Unmarshal(jsonBytes, &unmarshaled)
	require.NoError(t, err)

	assert.Equal(t, info.By, unmarshaled.By)
	assert.Equal(t, *info.ByID, *unmarshaled.ByID)
	// Note: FromValue and ToValue lose their original types through JSON
}
