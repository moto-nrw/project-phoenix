package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	fixDeviceAuthPermissionsVersion     = "1.15.14"
	fixDeviceAuthPermissionsDescription = "Grant phoenix_auth access to iot.devices for device API key authentication"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     fixDeviceAuthPermissionsVersion,
		Description: fixDeviceAuthPermissionsDescription,
		DependsOn:   []string{"1.15.13"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.14: Granting phoenix_auth access to iot.devices...")

			// Device authentication (auth/device/device_auth.go) runs BEFORE any
			// tenant transaction middleware — it queries iot.devices by API key to
			// identify the device (and thus the tenant). phoenix_auth is NOINHERIT,
			// so it needs explicit grants on the iot schema.

			// Grant schema-level USAGE so phoenix_auth can reference iot.devices at all.
			_, err := db.ExecContext(ctx, `GRANT USAGE ON SCHEMA iot TO phoenix_auth;`)
			if err != nil {
				return fmt.Errorf("grant USAGE on schema iot: %w", err)
			}

			// Grant SELECT (FindByAPIKey) on the whole table, but restrict UPDATE to
			// only the last_seen column (UpdateDeviceLastSeenAt). Principle of least
			// privilege: phoenix_auth must not be able to mutate api_key, tenant_id, etc.
			_, err = db.ExecContext(ctx, `
				GRANT SELECT ON iot.devices TO phoenix_auth;
				GRANT UPDATE (last_seen) ON iot.devices TO phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("grant SELECT + UPDATE(last_seen) on iot.devices: %w", err)
			}

			// RLS policy: device auth is a cross-tenant operation (we don't know the
			// tenant until AFTER we look up the device by API key). Allow phoenix_auth
			// to read any device row. This is safe because phoenix_auth is the server's
			// own connection role, never exposed to end-users.
			_, err = db.ExecContext(ctx, `
				CREATE POLICY device_auth_select ON iot.devices
					FOR SELECT TO phoenix_auth USING (true);
			`)
			if err != nil {
				return fmt.Errorf("create RLS policy device_auth_select: %w", err)
			}

			// Allow phoenix_auth to update last_seen on any device (also cross-tenant,
			// runs from the debounced background goroutine with no tenant context).
			_, err = db.ExecContext(ctx, `
				CREATE POLICY device_auth_update ON iot.devices
					FOR UPDATE TO phoenix_auth USING (true);
			`)
			if err != nil {
				return fmt.Errorf("create RLS policy device_auth_update: %w", err)
			}

			fmt.Println("Migration 1.15.14: Done")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.14: Revoking phoenix_auth IoT grants...")

			_, err := db.ExecContext(ctx, `
				DROP POLICY IF EXISTS device_auth_update ON iot.devices;
				DROP POLICY IF EXISTS device_auth_select ON iot.devices;
				REVOKE UPDATE (last_seen) ON iot.devices FROM phoenix_auth;
				REVOKE SELECT ON iot.devices FROM phoenix_auth;
				REVOKE USAGE ON SCHEMA iot FROM phoenix_auth;
			`)
			if err != nil {
				return fmt.Errorf("error revoking phoenix_auth IoT grants: %w", err)
			}

			return nil
		},
	)
}
