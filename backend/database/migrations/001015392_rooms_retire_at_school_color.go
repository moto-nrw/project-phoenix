package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	roomsRetireAtSchoolColorVersion     = "1.15.392"
	roomsRetireAtSchoolColorDescription = "Back up + clear room colors that became the Schule status badge (#217A78) into audit.room_color_migration_backup, so existing rooms carrying it stay editable. Rollback does NOT restore them — use the backup table."
)

// atSchoolStatusHexes are the hexes that became status badge colors with the
// pre-check-in "Schule" location (#3260). Before that change an admin could
// legally pick them for a room; afterwards facilities.IsReservedRoomColor
// rejects them.
//
// Frozen snapshot rather than a reference to reservedRoomColors: a migration
// must keep doing what it did on the day it shipped, even after the palette
// moves on.
var atSchoolStatusHexes = []string{
	"#217A78", // AT_SCHOOL (Schule)
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsRetireAtSchoolColorVersion,
		Description: roomsRetireAtSchoolColorDescription,
		DependsOn: []string{
			roomsRetireNewStatusColorsVersion, // 1.15.272 — backup table + prior status-color clear
			absenceAllowanceCarryoverVersion,  // 1.15.391 — preserves ladder order
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return roomsRetireAtSchoolColorUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return roomsRetireAtSchoolColorDown(ctx, db)
		},
	)
}

// roomsRetireAtSchoolColorUp nulls the color of every room carrying the
// Schule status hex. Without this those rooms are unsaveable: Room.Validate
// runs IsReservedRoomColor on UPDATE as well as INSERT, so renaming or
// changing capacity gets a 400 with no hint that the color is the problem.
func roomsRetireAtSchoolColorUp(ctx context.Context, db *bun.DB) error {
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

	backupRes, err := db.NewRaw(`
		INSERT INTO audit.room_color_migration_backup
			(room_id, tenant_id, name, color)
		SELECT id, tenant_id, name, color
		FROM facilities.rooms
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (?);
	`, bun.List(lowerHexes(atSchoolStatusHexes))).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed populating audit.room_color_migration_backup: %w", err)
	}
	if backed, raErr := backupRes.RowsAffected(); raErr == nil && backed > 0 {
		migrationLog().InfoContext(ctx, "room colors backed up before clearing",
			"rows", backed,
			"backup_table", "audit.room_color_migration_backup",
		)
	}

	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET color = NULL
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (?);
	`, bun.List(lowerHexes(atSchoolStatusHexes))).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed clearing the Schule status room color: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		migrationLog().InfoContext(ctx, "room colors cleared: Schule status badge color",
			"rows", affected,
			"backup_table", "audit.room_color_migration_backup",
		)
	}

	return nil
}

// roomsRetireAtSchoolColorDown is deliberately a no-op on the data. Restoring
// the colors would hand rooms back a hex that Room.Validate now rejects.
func roomsRetireAtSchoolColorDown(_ context.Context, _ *bun.DB) error {
	return nil
}
