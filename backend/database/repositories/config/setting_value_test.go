package config_test

import (
	"encoding/json"
	"testing"

	configRepo "github.com/moto-nrw/project-phoenix/database/repositories/config"
	"github.com/moto-nrw/project-phoenix/models/config"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newSV(tb testing.TB, key string, value string) *config.SettingValue {
	sv := &config.SettingValue{
		SettingKey: key,
		Value:      json.RawMessage(value),
	}
	sv.TenantID = testpkg.Tenant(tb)
	return sv
}

func TestSettingValueRepository_Upsert_Insert(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	err := repo.Upsert(ctx, newSV(t, "test.upsert_insert", `"hello"`))
	require.NoError(t, err)

	found, err := repo.FindByTenantAndKey(ctx, testpkg.Tenant(t), "test.upsert_insert")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"hello"`), found.Value)

	t.Cleanup(func() { _ = repo.Delete(ctx, testpkg.Tenant(t), "test.upsert_insert") })
}

func TestSettingValueRepository_Upsert_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, repo.Upsert(ctx, newSV(t, "test.upsert_update", `"first"`)))
	require.NoError(t, repo.Upsert(ctx, newSV(t, "test.upsert_update", `"second"`)))

	found, err := repo.FindByTenantAndKey(ctx, testpkg.Tenant(t), "test.upsert_update")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"second"`), found.Value)

	t.Cleanup(func() { _ = repo.Delete(ctx, testpkg.Tenant(t), "test.upsert_update") })
}

func TestSettingValueRepository_FindByTenantAndKey_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	found, err := repo.FindByTenantAndKey(ctx, testpkg.Tenant(t), "nonexistent.key")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_FindByTenantAndKeys(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, repo.Upsert(ctx, newSV(t, "test.batch_one", `true`)))
	require.NoError(t, repo.Upsert(ctx, newSV(t, "test.batch_two", `false`)))
	t.Cleanup(func() {
		_ = repo.Delete(ctx, testpkg.Tenant(t), "test.batch_one")
		_ = repo.Delete(ctx, testpkg.Tenant(t), "test.batch_two")
	})

	found, err := repo.FindByTenantAndKeys(ctx, testpkg.Tenant(t), []string{
		"test.batch_one",
		"test.batch_two",
		"test.not_stored",
	})
	require.NoError(t, err)
	require.Len(t, found, 2)

	byKey := make(map[string]json.RawMessage, len(found))
	for _, value := range found {
		byKey[value.SettingKey] = value.Value
	}
	assert.Equal(t, json.RawMessage(`true`), byKey["test.batch_one"])
	assert.Equal(t, json.RawMessage(`false`), byKey["test.batch_two"])
}

func TestSettingValueRepository_Delete(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, repo.Upsert(ctx, newSV(t, "test.delete_me", `"bye"`)))

	err := repo.Delete(ctx, testpkg.Tenant(t), "test.delete_me")
	require.NoError(t, err)

	found, err := repo.FindByTenantAndKey(ctx, testpkg.Tenant(t), "test.delete_me")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))

	// Use high tenant IDs to avoid sequence collisions: other tests use
	// auto-increment (BIGSERIAL) for orgs/schools, so low explicit IDs like
	// 2–20 can collide when the sequence catches up. IDs 9990+ are safe.
	tenantA := testpkg.UniqueTestTenantID(t)
	tenantB := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantA)
	testpkg.EnsureTestTenant(t, db, tenantB)

	ctxA := testpkg.TenantContext(tenantA)
	ctxB := testpkg.TenantContext(tenantB)

	// Insert same key for both tenants with different values
	svA := &config.SettingValue{
		SettingKey: "test.isolation",
		Value:      json.RawMessage(`"tenantA"`),
	}
	svA.TenantID = tenantA
	require.NoError(t, repo.Upsert(ctxA, svA))

	svB := &config.SettingValue{
		SettingKey: "test.isolation",
		Value:      json.RawMessage(`"tenantB"`),
	}
	svB.TenantID = tenantB
	require.NoError(t, repo.Upsert(ctxB, svB))

	// Each tenant's query scopes to its own data via tenant_id parameter
	foundA, err := repo.FindByTenantAndKey(ctxA, tenantA, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, foundA)
	assert.Equal(t, json.RawMessage(`"tenantA"`), foundA.Value)

	foundB, err := repo.FindByTenantAndKey(ctxB, tenantB, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, foundB)
	assert.Equal(t, json.RawMessage(`"tenantB"`), foundB.Value)

	// Verify nonexistent key returns nil
	notFound, err := repo.FindByTenantAndKey(ctxA, tenantA, "test.isolation_nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)

	t.Cleanup(func() {
		_ = repo.Delete(ctxA, tenantA, "test.isolation")
		_ = repo.Delete(ctxB, tenantB, "test.isolation")
	})
}

func TestSettingValueRepository_Validate_RejectsEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := configRepo.NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	// Missing key
	err := repo.Upsert(ctx, &config.SettingValue{
		Value: json.RawMessage(`"val"`),
	})
	assert.Error(t, err)

	// Missing value
	sv := &config.SettingValue{SettingKey: "test.empty"}
	sv.TenantID = testpkg.Tenant(t)
	err = repo.Upsert(ctx, sv)
	assert.Error(t, err)
}

// Ensure tenant import is used
func init() {
}
