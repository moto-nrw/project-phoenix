package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/uptrace/bun"
)

const (
	schulhofRoomColorClearDefaultVersion     = "1.15.305"
	schulhofRoomColorClearDefaultDescription = "Back up + clear the auto-provisioned #7ED321 color on Schulhof rooms so the orange Schulhof default applies and admins start from an unset color (#2405)"
)

// schulhofAutoProvisionedHex is the color the Schulhof bootstrap stamped on
// the room before #2405 (constants.SchulhofColor). Frozen here as a literal
// rather than a reference: a migration must keep doing what it did on the day
// it shipped, even after the constant moves on.
const schulhofAutoProvisionedHex = "#7ED321"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schulhofRoomColorClearDefaultVersion,
		Description: schulhofRoomColorClearDefaultDescription,
		DependsOn: []string{
			roomsColorUniqueVersion,   // 1.15.45 — creates audit.room_color_migration_backup
			pwaStandaloneUsageVersion, // 1.15.304 — preserves ladder order
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return schulhofRoomColorClearDefaultUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return schulhofRoomColorClearDefaultDown(ctx, db)
		},
	)
}

// schulhofRoomColorClearDefaultUp clears #7ED321 from Schulhof rooms.
//
// Until #2405 the Schulhof room's color was not editable: the form hid the
// picker and UpdateRoom rejected changes. Every Schulhof row carrying
// #7ED321 therefore got it from the bootstrap (services/facilities and
// services/iot/checkin both stamped constants.SchulhofColor on create), never
// from a deliberate admin choice — the same provenance argument migration
// 1.15.45 made for the #4F46E5 form-bug default.
//
// Clearing it matters for two reasons:
//
//  1. The documented default is orange (LOCATION_COLORS.SCHOOLYARD). With a
//     stale green sitting in the column, "I never picked a color" renders
//     green, and no school can reach the default state through the UI —
//     the picker sets colors, it does not unset them.
//  2. The per-tenant unique color index means the Schulhof was silently
//     squatting on #7ED321 for every other room in the school.
//
// Scoped to the canonical Schulhof room name so a normal room that happens
// to carry the same hex — legitimately picked while the Schulhof was
// invisible to the uniqueness check on other tenants — is left alone.
//
// The original values go into audit.room_color_migration_backup, same table
// and same shape as 1.15.45 / 1.15.272.
func schulhofRoomColorClearDefaultUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.305: Backing up + clearing the auto-provisioned Schulhof room color...")

	// 1.15.45 creates this table, and DependsOn pins that ordering. Recreating
	// it defensively costs nothing and keeps this migration runnable against a
	// database where the audit schema was rebuilt by hand.
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS audit.room_color_migration_backup (
			id           BIGSERIAL PRIMARY KEY,
			room_id      BIGINT NOT NULL,
			tenant_id    BIGINT NOT NULL,
			name         TEXT NOT NULL,
			color        TEXT NOT NULL,
			migrated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
		);
		CREATE INDEX IF NOT EXISTS idx_room_color_migration_backup_tenant
			ON audit.room_color_migration_backup (tenant_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed ensuring audit.room_color_migration_backup: %w", err)
	}

	// Backup before the UPDATE, as one INSERT … SELECT so the snapshot and the
	// clear see the same set even under a concurrent writer. LOWER() on the
	// color because rooms written before the model started upper-casing on
	// write may still hold lower-case hexes; the name comparison stays
	// exact-case, mirroring constants.IsSchulhofRoomName.
	backupRes, err := db.NewRaw(`
		INSERT INTO audit.room_color_migration_backup
			(room_id, tenant_id, name, color)
		SELECT id, tenant_id, name, color
		FROM facilities.rooms
		WHERE name = ?
		  AND color IS NOT NULL
		  AND LOWER(color) = ?;
	`, constants.SchulhofRoomName, strings.ToLower(schulhofAutoProvisionedHex)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed populating audit.room_color_migration_backup: %w", err)
	}
	if backed, raErr := backupRes.RowsAffected(); raErr == nil && backed > 0 {
		fmt.Printf("Migration 1.15.305: backed up %d Schulhof room(s) into audit.room_color_migration_backup before clearing\n", backed)
	}

	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET color = NULL
		WHERE name = ?
		  AND color IS NOT NULL
		  AND LOWER(color) = ?;
	`, constants.SchulhofRoomName, strings.ToLower(schulhofAutoProvisionedHex)).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed clearing the auto-provisioned Schulhof room color: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.305: cleared the auto-provisioned color from %d Schulhof room(s) (audit.room_color_migration_backup retains the original values)\n", affected)
	}

	return nil
}

// schulhofRoomColorClearDefaultDown is deliberately a no-op on the data.
//
// Re-stamping #7ED321 would collide with any room that has since claimed the
// hex — the per-tenant unique index would fail the rollback mid-flight — and
// would also overwrite a color an admin picked in the meantime. Operators who
// want the old value back can take it from the backup table:
//
//	UPDATE facilities.rooms r
//	SET color = b.color
//	FROM audit.room_color_migration_backup b
//	WHERE r.id = b.room_id
//	  AND r.color IS NULL;
func schulhofRoomColorClearDefaultDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.305: the cleared Schulhof color is NOT auto-restored — see audit.room_color_migration_backup")
	return nil
}
