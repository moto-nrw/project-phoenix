package config

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingAuditEntryHasHonestShape(t *testing.T) {
	t.Parallel()

	modelType := reflect.TypeFor[SettingAuditEntry]()
	for _, method := range []string{"GetID", "GetCreatedAt", "GetUpdatedAt"} {
		if _, ok := reflect.PointerTo(modelType).MethodByName(method); ok {
			t.Fatalf("SettingAuditEntry must not declare the generic %s contract", method)
		}
	}
	idField, ok := modelType.FieldByName("ID")
	if !ok || len(idField.Index) != 1 || idField.Tag.Get("bun") != "id,pk,autoincrement" {
		t.Fatal("SettingAuditEntry must keep its direct audit identity mapping")
	}
	changedAtField, ok := modelType.FieldByName("ChangedAt")
	if !ok || changedAtField.Tag.Get("bun") != "changed_at,notnull,default:current_timestamp" {
		t.Fatal("SettingAuditEntry must keep its changed_at mapping")
	}
}

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
