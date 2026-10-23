package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const hashDeviceAPIKeysVersion = "1.15.474"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     hashDeviceAPIKeysVersion,
		Description: "Store device credentials as SHA-256 digests (#3841)",
		DependsOn:   []string{relatedAccountsManagePermissionVersion},
	})
	Migrations.MustRegister(hashDeviceAPIKeysUp, hashDeviceAPIKeysDown)
}

func hashDeviceAPIKeysUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `
			ALTER TABLE iot.devices
				ADD COLUMN api_key_hash TEXT CHECK (api_key_hash ~ '^[0-9a-f]{64}$'),
				ADD COLUMN masked_api_key TEXT NOT NULL DEFAULT '';
			UPDATE iot.devices SET
				api_key_hash = encode(sha256(convert_to(api_key, 'UTF8')), 'hex'),
				masked_api_key = CASE WHEN char_length(api_key) > 8 THEN '...' || right(api_key, 4) ELSE '...' END
				WHERE api_key IS NOT NULL;
			ALTER TABLE iot.devices
				ADD CONSTRAINT devices_api_key_hash_key UNIQUE (api_key_hash),
				DROP COLUMN api_key;
		`)
		if err != nil {
			return fmt.Errorf("hash device credentials: %w", err)
		}
		return nil
	})
}

// Hashes cannot be reversed. Roll back application defects by rolling forward;
// never recreate plaintext credentials from a pre-migration backup.
func hashDeviceAPIKeysDown(_ context.Context, _ *bun.DB) error {
	return errors.New("device credential hashing is irreversible; roll forward")
}
