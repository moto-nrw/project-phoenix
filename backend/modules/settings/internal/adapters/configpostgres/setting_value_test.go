package config

import (
	"encoding/json"
	"testing"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
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
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	err := testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.upsert_insert", `"hello"`))
	require.NoError(t, err)

	found, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByTenantAndKey, testpkg.Tenant(t), "test.upsert_insert")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"hello"`), found.Value)

	t.Cleanup(func() { _ = testpkg.TenantCommand2(t, ctx, db, repo.Delete, testpkg.Tenant(t), "test.upsert_insert") })
}

func TestSettingValueRepository_Upsert_Update(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.upsert_update", `"first"`)))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.upsert_update", `"second"`)))

	found, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByTenantAndKey, testpkg.Tenant(t), "test.upsert_update")
	require.NoError(t, err)
	require.NotNil(t, found)
	assert.Equal(t, json.RawMessage(`"second"`), found.Value)

	t.Cleanup(func() { _ = testpkg.TenantCommand2(t, ctx, db, repo.Delete, testpkg.Tenant(t), "test.upsert_update") })
}

func TestSettingValueRepository_FindByTenantAndKey_NotFound(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	found, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByTenantAndKey, testpkg.Tenant(t), "nonexistent.key")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_FindByTenantAndKeys(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.batch_one", `true`)))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.batch_two", `false`)))
	t.Cleanup(func() {
		_ = testpkg.TenantCommand2(t, ctx, db, repo.Delete, testpkg.Tenant(t), "test.batch_one")
		_ = testpkg.TenantCommand2(t, ctx, db, repo.Delete, testpkg.Tenant(t), "test.batch_two")
	})

	found, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByTenantAndKeys, testpkg.Tenant(t), []string{
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
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, repo.Upsert, newSV(t, "test.delete_me", `"bye"`)))

	err := testpkg.TenantCommand2(t, ctx, db, repo.Delete, testpkg.Tenant(t), "test.delete_me")
	require.NoError(t, err)

	found, err := testpkg.TenantQuery2(t, ctx, db, repo.FindByTenantAndKey, testpkg.Tenant(t), "test.delete_me")
	require.NoError(t, err)
	assert.Nil(t, found)
}

func TestSettingValueRepository_TenantIsolation(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))

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
	require.NoError(t, testpkg.TenantCommand1(t, ctxA, db, repo.Upsert, svA))

	svB := &config.SettingValue{
		SettingKey: "test.isolation",
		Value:      json.RawMessage(`"tenantB"`),
	}
	svB.TenantID = tenantB
	require.NoError(t, testpkg.TenantCommand1(t, ctxB, db, repo.Upsert, svB))

	// Each tenant's query scopes to its own data via tenant_id parameter
	foundA, err := testpkg.TenantQuery2(t, ctxA, db, repo.FindByTenantAndKey, tenantA, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, foundA)
	assert.Equal(t, json.RawMessage(`"tenantA"`), foundA.Value)

	foundB, err := testpkg.TenantQuery2(t, ctxB, db, repo.FindByTenantAndKey, tenantB, "test.isolation")
	require.NoError(t, err)
	require.NotNil(t, foundB)
	assert.Equal(t, json.RawMessage(`"tenantB"`), foundB.Value)

	// Verify nonexistent key returns nil
	notFound, err := testpkg.TenantQuery2(t, ctxA, db, repo.FindByTenantAndKey, tenantA, "test.isolation_nonexistent")
	require.NoError(t, err)
	assert.Nil(t, notFound)

	t.Cleanup(func() {
		_ = testpkg.TenantCommand2(t, ctxA, db, repo.Delete, tenantA, "test.isolation")
		_ = testpkg.TenantCommand2(t, ctxB, db, repo.Delete, tenantB, "test.isolation")
	})
}

func TestSettingValueRepository_Validate_RejectsEmpty(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := NewSettingValueRepository(testpkg.ConfigRuntime(db))
	ctx := testpkg.Ctx(t)

	// Missing key
	err := testpkg.TenantCommand1(t, ctx, db, repo.Upsert, &config.SettingValue{
		Value: json.RawMessage(`"val"`),
	})
	assert.Error(t, err)

	// Missing value
	sv := &config.SettingValue{SettingKey: "test.empty"}
	sv.TenantID = testpkg.Tenant(t)
	err = testpkg.TenantCommand1(t, ctx, db, repo.Upsert, sv)
	assert.Error(t, err)
}

// Ensure tenant import is used
func init() {
}
