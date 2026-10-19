package config

import (
	"encoding/json"
	"testing"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
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

	err := testpkg.TenantCommand1(t, ctx, db, repo.Create, entry)
	require.NoError(t, err)
	assert.Greater(t, entry.ID, int64(0), "should have an auto-generated ID")
}

func TestSettingAuditRepository_Create_ValidatesEntry(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingAuditRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	// Missing key
	err := testpkg.TenantCommand1(t, ctx, db, repo.Create, &config.SettingAuditEntry{
		TenantID: testpkg.Tenant(t),
		Action:   "set",
	})
	assert.Error(t, err)

	// Invalid action
	err = testpkg.TenantCommand1(t, ctx, db, repo.Create, &config.SettingAuditEntry{
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

	err := testpkg.TenantCommand1(t, ctx, db, repo.Create, nil)
	assert.Error(t, err)
}
