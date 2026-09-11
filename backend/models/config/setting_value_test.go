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
