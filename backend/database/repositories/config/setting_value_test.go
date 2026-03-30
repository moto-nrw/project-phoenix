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

func newSV(key string, value string) *config.SettingValue {
	sv := &config.SettingValue{
		SettingKey: key,
		Value:      json.RawMessage(value),
	}
	sv.TenantID = 1
	return sv
}

func TestSettingValueRepository_Upsert_Insert(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	err := repo.Upsert(ctx, newSV("test.upsert_insert", `"hello"`))
	require.NoError(t, err)

	found, err := repo.FindByTenantAndKey(ctx, 1, "test.upsert_insert")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"hello"`), found.Value)

	t.Cleanup(func() { _ = repo.Delete(ctx, 1, "test.upsert_insert") })
}

func TestSettingValueRepository_Upsert_Update(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	require.NoError(t, repo.Upsert(ctx, newSV("test.upsert_update", `"first"`)))
	require.NoError(t, repo.Upsert(ctx, newSV("test.upsert_update", `"second"`)))

	found, err := repo.FindByTenantAndKey(ctx, 1, "test.upsert_update")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"second"`), found.Value)

	t.Cleanup(func() { _ = repo.Delete(ctx, 1, "test.upsert_update") })
}

func TestSettingValueRepository_FindByTenantAndKey_NotFound(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	found, err := repo.FindByTenantAndKey(ctx, 1, "nonexistent.key")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_FindByTenant(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	// Clean up first to avoid interference from other tests
	_ = repo.Delete(ctx, 1, "test.find_a")
	_ = repo.Delete(ctx, 1, "test.find_b")

	require.NoError(t, repo.Upsert(ctx, newSV("test.find_a", `"a"`)))
	require.NoError(t, repo.Upsert(ctx, newSV("test.find_b", `"b"`)))

	results, err := repo.FindByTenant(ctx, 1)
	require.NoError(t, err)

	found := 0
	for _, r := range results {
		if r.SettingKey == "test.find_a" || r.SettingKey == "test.find_b" {
			found++
		}
	}
	assert.Equal(t, 2, found)

	t.Cleanup(func() {
		_ = repo.Delete(ctx, 1, "test.find_a")
		_ = repo.Delete(ctx, 1, "test.find_b")
	})
}

func TestSettingValueRepository_Delete(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx := testpkg.TenantContext(1)

	require.NoError(t, repo.Upsert(ctx, newSV("test.delete_me", `"bye"`)))

	err := repo.Delete(ctx, 1, "test.delete_me")
	require.NoError(t, err)

	found, err := repo.FindByTenantAndKey(ctx, 1, "test.delete_me")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_TenantIsolation(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	repo := configRepo.NewSettingValueRepository(db)
	ctx1 := testpkg.TenantContext(1)

	require.NoError(t, repo.Upsert(ctx1, newSV("test.isolation", `"tenant1"`)))

	// Tenant 1 can see it
	found, err := repo.FindByTenantAndKey(ctx1, 1, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, found)

	// Query for tenant 2 should NOT find tenant 1's value
	found2, err := repo.FindByTenantAndKey(ctx1, 2, "test.isolation")
	require.NoError(t, err)
	assert.Nil(t, found2)

	t.Cleanup(func() { _ = repo.Delete(ctx1, 1, "test.isolation") })
}

func TestSettingValueRepository_Validate_RejectsEmpty(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
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

// Ensure tenant import is used
func init() {
	_ = tenant.WithTenantID
}
