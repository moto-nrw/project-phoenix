package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingChange_TableName(t *testing.T) {
	sc := &SettingChange{}
	assert.Equal(t, "audit.setting_changes", sc.TableName())
}

func TestSettingChange_GetID(t *testing.T) {
	sc := &SettingChange{ID: 123}
	assert.Equal(t, int64(123), sc.GetID())
}

func TestSettingChange_GetCreatedAt(t *testing.T) {
	now := time.Now()
	sc := &SettingChange{CreatedAt: now}
	assert.Equal(t, now, sc.GetCreatedAt())
}

func TestSettingChange_GetUpdatedAt(t *testing.T) {
	now := time.Now()
	sc := &SettingChange{CreatedAt: now}
	assert.Equal(t, now, sc.GetUpdatedAt())
}

func TestSettingChange_Validate(t *testing.T) {
	tests := []struct {
		name        string
		change      SettingChange
		expectError string
	}{
		{
			name: "valid create change",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "system",
				ChangeType: "create",
			},
			expectError: "",
		},
		{
			name: "valid update change",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "og",
				ChangeType: "update",
			},
			expectError: "",
		},
		{
			name: "valid delete change",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "user",
				ChangeType: "delete",
			},
			expectError: "",
		},
		{
			name: "valid reset change",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "system",
				ChangeType: "reset",
			},
			expectError: "",
		},
		{
			name: "missing setting_key",
			change: SettingChange{
				ScopeType:  "system",
				ChangeType: "create",
			},
			expectError: "setting_key is required",
		},
		{
			name: "missing scope_type",
			change: SettingChange{
				SettingKey: "test.setting",
				ChangeType: "create",
			},
			expectError: "scope_type is required",
		},
		{
			name: "missing change_type",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "system",
			},
			expectError: "change_type is required",
		},
		{
			name: "invalid change_type",
			change: SettingChange{
				SettingKey: "test.setting",
				ScopeType:  "system",
				ChangeType: "invalid",
			},
			expectError: "invalid change_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.change.Validate()
			if tt.expectError == "" {
				assert.NoError(t, err)
				// Should set CreatedAt if not set
				assert.False(t, tt.change.CreatedAt.IsZero())
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectError)
			}
		})
	}
}

func TestSettingChange_Validate_PreservesCreatedAt(t *testing.T) {
	fixedTime := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sc := &SettingChange{
		SettingKey: "test.setting",
		ScopeType:  "system",
		ChangeType: "create",
		CreatedAt:  fixedTime,
	}

	err := sc.Validate()
	require.NoError(t, err)
	assert.Equal(t, fixedTime, sc.CreatedAt)
}

func TestSettingChange_GetOldValueTyped(t *testing.T) {
	tests := []struct {
		name     string
		oldValue []byte
		expected any
		hasError bool
	}{
		{
			name:     "nil old value",
			oldValue: nil,
			expected: nil,
			hasError: false,
		},
		{
			name:     "empty old value",
			oldValue: []byte{},
			expected: nil,
			hasError: false,
		},
		{
			name:     "bool value",
			oldValue: jsonMustMarshal(map[string]any{"value": true}),
			expected: true,
			hasError: false,
		},
		{
			name:     "string value",
			oldValue: jsonMustMarshal(map[string]any{"value": "hello"}),
			expected: "hello",
			hasError: false,
		},
		{
			name:     "number value",
			oldValue: jsonMustMarshal(map[string]any{"value": 42.0}),
			expected: 42.0,
			hasError: false,
		},
		{
			name:     "invalid json",
			oldValue: []byte("not json"),
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SettingChange{OldValue: tt.oldValue}
			result, err := sc.GetOldValueTyped()
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSettingChange_GetNewValueTyped(t *testing.T) {
	tests := []struct {
		name     string
		newValue []byte
		expected any
		hasError bool
	}{
		{
			name:     "nil new value",
			newValue: nil,
			expected: nil,
			hasError: false,
		},
		{
			name:     "empty new value",
			newValue: []byte{},
			expected: nil,
			hasError: false,
		},
		{
			name:     "bool value",
			newValue: jsonMustMarshal(map[string]any{"value": false}),
			expected: false,
			hasError: false,
		},
		{
			name:     "string value",
			newValue: jsonMustMarshal(map[string]any{"value": "world"}),
			expected: "world",
			hasError: false,
		},
		{
			name:     "number value",
			newValue: jsonMustMarshal(map[string]any{"value": 3.14}),
			expected: 3.14,
			hasError: false,
		},
		{
			name:     "invalid json",
			newValue: []byte("invalid"),
			expected: nil,
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &SettingChange{NewValue: tt.newValue}
			result, err := sc.GetNewValueTyped()
			if tt.hasError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestSettingChangeType_Constants(t *testing.T) {
	assert.Equal(t, SettingChangeType("create"), SettingChangeCreate)
	assert.Equal(t, SettingChangeType("update"), SettingChangeUpdate)
	assert.Equal(t, SettingChangeType("delete"), SettingChangeDelete)
	assert.Equal(t, SettingChangeType("reset"), SettingChangeReset)
}

// jsonMustMarshal is a helper for tests
func jsonMustMarshal(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
