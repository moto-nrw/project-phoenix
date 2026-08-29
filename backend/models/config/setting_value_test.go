package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingValue_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		sv      SettingValue
		wantErr string
	}{
		{
			name:    "valid",
			sv:      SettingValue{SettingKey: "test.key", Value: json.RawMessage(`"hello"`)},
			wantErr: "",
		},
		{
			name:    "missing key",
			sv:      SettingValue{Value: json.RawMessage(`"hello"`)},
			wantErr: "setting_key is required",
		},
		{
			name:    "missing value",
			sv:      SettingValue{SettingKey: "test.key"},
			wantErr: "value is required",
		},
		{
			name:    "empty value",
			sv:      SettingValue{SettingKey: "test.key", Value: json.RawMessage{}},
			wantErr: "value is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.sv.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSettingValue_GetID(t *testing.T) {
	t.Parallel()

	sv := &SettingValue{}
	sv.ID = 42
	assert.Equal(t, int64(42), sv.GetID())
}

func TestSettingValue_GetCreatedAt(t *testing.T) {
	t.Parallel()

	sv := &SettingValue{}
	assert.True(t, sv.GetCreatedAt().IsZero())
}

func TestSettingValue_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	sv := &SettingValue{}
	assert.True(t, sv.GetUpdatedAt().IsZero())
}
