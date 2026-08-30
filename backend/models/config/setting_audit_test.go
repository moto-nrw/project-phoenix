package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingAuditEntry_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   SettingAuditEntry
		wantErr string
	}{
		{
			name:    "valid set action",
			entry:   SettingAuditEntry{SettingKey: "test.key", Action: "set"},
			wantErr: "",
		},
		{
			name:    "valid reset action",
			entry:   SettingAuditEntry{SettingKey: "test.key", Action: "reset"},
			wantErr: "",
		},
		{
			name:    "valid delete action",
			entry:   SettingAuditEntry{SettingKey: "test.key", Action: "delete"},
			wantErr: "",
		},
		{
			name:    "missing key",
			entry:   SettingAuditEntry{Action: "set"},
			wantErr: "setting_key is required",
		},
		{
			name:    "invalid action",
			entry:   SettingAuditEntry{SettingKey: "test.key", Action: "update"},
			wantErr: "action must be set, reset, or delete",
		},
		{
			name:    "empty action",
			entry:   SettingAuditEntry{SettingKey: "test.key"},
			wantErr: "action must be set, reset, or delete",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.entry.Validate()
			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			}
		})
	}
}

func TestSettingAuditEntry_GetID(t *testing.T) {
	t.Parallel()

	entry := &SettingAuditEntry{ID: 99}
	assert.Equal(t, int64(99), entry.GetID())
}

func TestSettingAuditEntry_GetCreatedAt_ReturnsChangedAt(t *testing.T) {
	t.Parallel()

	entry := &SettingAuditEntry{}
	assert.Equal(t, entry.ChangedAt, entry.GetCreatedAt())
}

func TestSettingAuditEntry_GetUpdatedAt_ReturnsChangedAt(t *testing.T) {
	t.Parallel()

	entry := &SettingAuditEntry{}
	assert.Equal(t, entry.ChangedAt, entry.GetUpdatedAt())
}

func TestNewAuditEntry(t *testing.T) {
	t.Parallel()

	changedByValue := int64(42)
	changedBy := &changedByValue
	oldValue := json.RawMessage(`"old"`)
	newValue := json.RawMessage(`"new"`)

	entry := NewAuditEntry(5, "test.key", "set", oldValue, newValue, changedBy)

	assert.Equal(t, int64(5), entry.TenantID)
	assert.Equal(t, "test.key", entry.SettingKey)
	assert.Equal(t, "set", entry.Action)
	assert.Equal(t, json.RawMessage(`"old"`), entry.OldValue)
	assert.Equal(t, json.RawMessage(`"new"`), entry.NewValue)
	assert.Equal(t, changedBy, entry.ChangedBy)
	assert.False(t, entry.ChangedAt.IsZero())
}

func TestNewAuditEntry_NilValues(t *testing.T) {
	t.Parallel()

	entry := NewAuditEntry(1, "test.reset", "reset", nil, nil, nil)

	assert.Equal(t, "reset", entry.Action)
	assert.Nil(t, entry.OldValue)
	assert.Nil(t, entry.NewValue)
	assert.Nil(t, entry.ChangedBy)
}
