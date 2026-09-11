package config

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingAuditRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	entry := &config.SettingAuditEntry{
		TenantID:   testpkg.Tenant(t),
		SettingKey: "test.audit_create",
		Action:     "set",
		OldValue:   nil,
		NewValue:   json.RawMessage(`"new_value"`),
		ChangedAt:  time.Now(),
	}

	err := repo.Create(ctx, entry)
	require.NoError(t, err)
	assert.Greater(t, entry.ID, int64(0), "should have an auto-generated ID")
}

func TestSettingAuditRepository_Create_ValidatesEntry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	// Missing key
	err := repo.Create(ctx, &config.SettingAuditEntry{
		TenantID: testpkg.Tenant(t),
		Action:   "set",
	})
	assert.Error(t, err)

	// Invalid action
	err = repo.Create(ctx, &config.SettingAuditEntry{
		TenantID:   testpkg.Tenant(t),
		SettingKey: "test.key",
		Action:     "invalid",
	})
	assert.Error(t, err)
}

func TestSettingAuditRepository_CreateNil(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	err := repo.Create(ctx, nil)
	assert.Error(t, err)
}
