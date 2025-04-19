package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [up migration] add tauri_devices table...")

		// Create the tauri_devices table for tracking registered Tauri desktop apps
		_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tauri_devices (
			id BIGSERIAL PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			last_sync_at TIMESTAMP,
			last_ip VARCHAR(45),
			status VARCHAR(50) NOT NULL DEFAULT 'active',
			api_key VARCHAR(255) NOT NULL UNIQUE,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL
		);
		`)

		if err != nil {
			return err
		}

		// Also create a table for device sync history
		fmt.Print(" [up migration] add tauri_device_syncs table...")
		_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS tauri_device_syncs (
			id BIGSERIAL PRIMARY KEY,
			device_id VARCHAR(255) NOT NULL,
			sync_at TIMESTAMP NOT NULL,
			ip_address VARCHAR(45),
			tags_count INT NOT NULL DEFAULT 0,
			app_version VARCHAR(50),
			created_at TIMESTAMP NOT NULL,
			FOREIGN KEY (device_id) REFERENCES tauri_devices(device_id) ON DELETE CASCADE
		);
		`)

		return err
	}, func(ctx context.Context, db *bun.DB) error {
		fmt.Print(" [down migration] drop tauri_device_syncs table...")
		_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS tauri_device_syncs;`)
		if err != nil {
			return err
		}

		fmt.Print(" [down migration] drop tauri_devices table...")
		_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS tauri_devices;`)

		return err
	})
}
