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

	// Use high tenant IDs to avoid sequence collisions: other tests use
	// auto-increment (BIGSERIAL) for orgs/schools, so low explicit IDs like
	// 2–20 can collide when the sequence catches up. IDs 9990+ are safe.
	tenantA := int64(9990)
	tenantB := int64(9991)
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

	// FindByTenant returns only the queried tenant's rows
	allA, err := repo.FindByTenant(ctxA, tenantA)
	require.NoError(t, err)
	for _, sv := range allA {
		assert.Equal(t, tenantA, sv.TenantID, "FindByTenant should only return rows for the requested tenant")
	}

	t.Cleanup(func() {
		_ = repo.Delete(ctxA, tenantA, "test.isolation")
		_ = repo.Delete(ctxB, tenantB, "test.isolation")
	})
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
