package config

import (
	"testing"
)

func TestSetting_Validate(t *testing.T) {
	tests := []struct {
		name     string
		setting  Setting
		wantErr  bool
		expected Setting
	}{
		{
			name: "Valid setting",
			setting: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "system",
			},
			wantErr: false,
			expected: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "system",
			},
		},
		{
			name: "Empty key",
			setting: Setting{
				Key:      "",
				Value:    "Project Phoenix",
				Category: "system",
			},
			wantErr: true,
		},
		{
			name: "Empty value",
			setting: Setting{
				Key:      "app_name",
				Value:    "",
				Category: "system",
			},
			wantErr: true,
		},
		{
			name: "Empty category",
			setting: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "",
			},
			wantErr: true,
		},
		{
			name: "Key normalization",
			setting: Setting{
				Key:      "APP NAME",
				Value:    "Project Phoenix",
				Category: "system",
			},
			wantErr: false,
			expected: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "system",
			},
		},
		{
			name: "Category normalization",
			setting: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "SYSTEM",
			},
			wantErr: false,
			expected: Setting{
				Key:      "app_name",
				Value:    "Project Phoenix",
				Category: "system",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.setting.Validate()

			// Check error condition
			if (err != nil) != tt.wantErr {
				t.Errorf("Setting.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}

			// Check normalization if no error
			if !tt.wantErr {
				if tt.setting.Key != tt.expected.Key {
					t.Errorf("Key normalization failed, got %s, want %s", tt.setting.Key, tt.expected.Key)
				}
				if tt.setting.Category != tt.expected.Category {
					t.Errorf("Category normalization failed, got %s, want %s", tt.setting.Category, tt.expected.Category)
				}
			}
		})
	}
}

func TestSetting_IsSystemSetting(t *testing.T) {
	tests := []struct {
		name     string
		category string
		expected bool
	}{
		{
			name:     "System category",
			category: "system",
			expected: true,
		},
		{
			name:     "Other category",
			category: "ui",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{
				Key:      "test_key",
				Value:    "test_value",
				Category: tt.category,
			}

			if got := setting.IsSystemSetting(); got != tt.expected {
				t.Errorf("IsSystemSetting() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestSetting_GetBoolValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "True lowercase",
			value:    "true",
			expected: true,
		},
		{
			name:     "True mixedcase",
			value:    "True",
			expected: true,
		},
		{
			name:     "False lowercase",
			value:    "false",
			expected: false,
		},
		{
			name:     "Other value",
			value:    "yes",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setting := Setting{
				Key:      "feature_enabled",
				Value:    tt.value,
				Category: "features",
			}

			if got := setting.GetBoolValue(); got != tt.expected {
				t.Errorf("GetBoolValue() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestSetting_GetFullKey(t *testing.T) {
	setting := Setting{
		Key:      "timeout",
		Value:    "30",
		Category: "network",
	}

	expected := "network.timeout"
	if got := setting.GetFullKey(); got != expected {
		t.Errorf("GetFullKey() = %v, expected %v", got, expected)
	}
}

func TestSetting_Clone(t *testing.T) {
	original := Setting{
		Key:             "theme",
		Value:           "dark",
		Category:        "ui",
		Description:     "UI theme setting",
		RequiresRestart: true,
		RequiresDBReset: false,
	}

	clone := original.Clone()

	// Verify all fields were cloned correctly
	if clone.Key != original.Key {
		t.Errorf("Clone() Key = %v, expected %v", clone.Key, original.Key)
	}
	if clone.Value != original.Value {
		t.Errorf("Clone() Value = %v, expected %v", clone.Value, original.Value)
	}
	if clone.Category != original.Category {
		t.Errorf("Clone() Category = %v, expected %v", clone.Category, original.Category)
	}
	if clone.Description != original.Description {
		t.Errorf("Clone() Description = %v, expected %v", clone.Description, original.Description)
	}
	if clone.RequiresRestart != original.RequiresRestart {
		t.Errorf("Clone() RequiresRestart = %v, expected %v", clone.RequiresRestart, original.RequiresRestart)
	}
	if clone.RequiresDBReset != original.RequiresDBReset {
		t.Errorf("Clone() RequiresDBReset = %v, expected %v", clone.RequiresDBReset, original.RequiresDBReset)
	}

	// Verify it's a deep copy by changing the original
	original.Value = "light"
	if clone.Value == original.Value {
		t.Error("Clone() did not create a deep copy")
	}
}
