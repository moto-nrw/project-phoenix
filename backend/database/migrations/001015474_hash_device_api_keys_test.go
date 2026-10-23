package migrations

import (
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestDeviceKeyMigrationPreservesLegacyCredentialsWithoutPlaintext(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	// The private clone starts at today's schema. Recreate only the old
	// credential columns; no other test can observe this schema change.
	_, err := db.ExecContext(ctx, `ALTER TABLE iot.devices
		DROP COLUMN api_key_hash, DROP COLUMN masked_api_key, ADD COLUMN api_key VARCHAR(255) UNIQUE`)
	require.NoError(t, err)
	tenants := []int64{testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID}
	for i, tenantID := range tenants {
		// Public SHA-256 test vector: not a real device credential. Whitespace,
		// short values and nulls model the unrestricted pre-migration schema.
		var id int64
		require.NoError(t, db.NewRaw(`INSERT INTO iot.devices (tenant_id, device_id, device_type, api_key)
			VALUES (?, ?, 'terminal', ?) RETURNING id`, tenantID, "migration-reader", []any{"abc", nil}[i]).Scan(ctx, &id))
	}
	require.NoError(t, hashDeviceAPIKeysUp(ctx, db))
	var digest, mask string
	require.NoError(t, db.NewRaw(`SELECT api_key_hash, masked_api_key FROM iot.devices
		WHERE tenant_id = ? AND device_id = 'migration-reader'`, tenants[0]).Scan(ctx, &digest, &mask))
	require.True(t, digest == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", "legacy credential keeps its exact SHA-256 fingerprint")
	require.Equal(t, "...", mask, "short legacy keys must not be revealed")
	var missing bool
	require.NoError(t, db.NewRaw(`SELECT api_key_hash IS NULL AND masked_api_key = '' FROM iot.devices
		WHERE tenant_id = ? AND device_id = 'migration-reader'`, tenants[1]).Scan(ctx, &missing))
	require.True(t, missing, "keyless devices stay keyless across tenants")
	var plaintextColumns int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'iot' AND table_name = 'devices' AND column_name = 'api_key'`).Scan(ctx, &plaintextColumns))
	require.Zero(t, plaintextColumns)
	require.Error(t, hashDeviceAPIKeysDown(ctx, db))
}
