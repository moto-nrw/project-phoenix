package config_test

import (
	"encoding/json"
	"testing"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSettingValueRepository_Upsert_Insert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	sv := &config.SettingValue{
		SettingKey: "test.upsert_insert",
		Value:      json.RawMessage(`"hello"`),
	}
	sv.TenantID = 1

	err := repo.Upsert(ctx, sv)
	require.NoError(t, err)

	// Verify it was inserted
	found, err := repo.FindByTenantAndKey(ctx, 1, "test.upsert_insert")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"hello"`), found.Value)

	// Cleanup
	err = repo.Delete(ctx, 1, "test.upsert_insert")
	require.NoError(t, err)
}

func TestSettingValueRepository_Upsert_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	// Insert
	sv := &config.SettingValue{
		SettingKey: "test.upsert_update",
		Value:      json.RawMessage(`"first"`),
	}
	sv.TenantID = 1
	require.NoError(t, repo.Upsert(ctx, sv))

	// Update
	sv2 := &config.SettingValue{
		SettingKey: "test.upsert_update",
		Value:      json.RawMessage(`"second"`),
	}
	sv2.TenantID = 1
	require.NoError(t, repo.Upsert(ctx, sv2))

	// Verify updated
	found, err := repo.FindByTenantAndKey(ctx, 1, "test.upsert_update")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"second"`), found.Value)

	// Cleanup
	_ = repo.Delete(ctx, 1, "test.upsert_update")
}

func TestSettingValueRepository_FindByTenantAndKey_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	found, err := repo.FindByTenantAndKey(ctx, 1, "nonexistent.key")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_FindByTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	// Insert two values
	require.NoError(t, repo.Upsert(ctx, &config.SettingValue{
		SettingKey: "test.find_tenant_a",
		Value:      json.RawMessage(`"a"`),
	}))
	require.NoError(t, repo.Upsert(ctx, &config.SettingValue{
		SettingKey: "test.find_tenant_b",
		Value:      json.RawMessage(`"b"`),
	}))

	// The Upsert doesn't set TenantID from context automatically for the struct,
	// but the SQL uses tenant_id from the struct. Let me set it:
	sv1 := &config.SettingValue{SettingKey: "test.find_tenant_a", Value: json.RawMessage(`"a"`)}
	sv1.TenantID = 1
	sv2 := &config.SettingValue{SettingKey: "test.find_tenant_b", Value: json.RawMessage(`"b"`)}
	sv2.TenantID = 1
	_ = repo.Upsert(ctx, sv1)
	_ = repo.Upsert(ctx, sv2)

	results, err := repo.FindByTenant(ctx, 1)
	require.NoError(t, err)

	// Should have at least our 2 values
	found := 0
	for _, r := range results {
		if r.SettingKey == "test.find_tenant_a" || r.SettingKey == "test.find_tenant_b" {
			found++
		}
	}
	assert.GreaterOrEqual(t, found, 2)

	// Cleanup
	_ = repo.Delete(ctx, 1, "test.find_tenant_a")
	_ = repo.Delete(ctx, 1, "test.find_tenant_b")
}

func TestSettingValueRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	sv := &config.SettingValue{
		SettingKey: "test.delete_me",
		Value:      json.RawMessage(`"bye"`),
	}
	sv.TenantID = 1
	require.NoError(t, repo.Upsert(ctx, sv))

	// Delete
	err := repo.Delete(ctx, 1, "test.delete_me")
	require.NoError(t, err)

	// Verify gone
	found, err := repo.FindByTenantAndKey(ctx, 1, "test.delete_me")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)

	// Insert for tenant 1
	ctx1 := testpkg.TenantContext(1)
	sv := &config.SettingValue{
		SettingKey: "test.isolation",
		Value:      json.RawMessage(`"tenant1"`),
	}
	sv.TenantID = 1
	require.NoError(t, repo.Upsert(ctx1, sv))

	// Tenant 1 can see it
	found, err := repo.FindByTenantAndKey(ctx1, 1, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, found)

	// Tenant 2 should NOT see it (RLS enforcement depends on tenant context)
	found2, err := repo.FindByTenantAndKey(ctx1, 2, "test.isolation")
	require.NoError(t, err)
	assert.Nil(t, found2, "tenant 2 should not see tenant 1's value")

	// Cleanup
	_ = repo.Delete(ctx1, 1, "test.isolation")
}

func TestSettingValueRepository_Validate_RejectsEmpty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer db.Close()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	// Missing key
	err := repo.Upsert(ctx, &config.SettingValue{
		Value: json.RawMessage(`"val"`),
	})
	assert.Error(t, err)

	// Missing value
	sv := &config.SettingValue{SettingKey: "test.empty"}
	sv.TenantID = 1
	err = repo.Upsert(ctx, sv)
	assert.Error(t, err)
}

// TenantContext helper — creates a context with tenant ID set.
// Uses the tenant package to match production code.
func init() {
	// Ensure testpkg.TenantContext is available
	_ = tenant.WithTenantID
}
