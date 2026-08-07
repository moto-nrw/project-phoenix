package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	roomsRetireNewStatusColorsVersion     = "1.15.271"
	roomsRetireNewStatusColorsDescription = "Back up + clear room colors that the unified visual system newly reserved as status badge colors (#78716C, #DC2626, #0891B2, #365D83) into audit.room_color_migration_backup, so existing rooms carrying them stay editable. Rollback does NOT restore them — use the backup table."
)

// newlyReservedStatusHexes are the hexes that became status badge colors with
// the visual-system unification. Before that change an admin could legally
// pick any of them for a room; afterwards facilities.IsReservedRoomColor
// rejects them.
//
// Keep this list in sync with the entries added to reservedRoomColors in
// models/facilities/room_colors.go by the same change. It is deliberately a
// frozen snapshot rather than a reference to that map: a migration must keep
// doing what it did on the day it shipped, even after the palette moves on.
var newlyReservedStatusHexes = []string{
	"#78716C", // UNKNOWN (Unbekannt)
	"#DC2626", // SICK / DANGER (Krank / Gefahr)
	"#0891B2", // CLASS_TRIP (Klassenfahrt)
	"#365D83", // NOT_ARRIVAL (Kommt heute nicht)
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsRetireNewStatusColorsVersion,
		Description: roomsRetireNewStatusColorsDescription,
		DependsOn: []string{
			roomsColorUniqueVersion,              // 1.15.45 — creates audit.room_color_migration_backup
			guardianInvitationRoleUpgradeVersion, // 1.15.270 — preserves ladder order
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return roomsRetireNewStatusColorsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return roomsRetireNewStatusColorsDown(ctx, db)
		},
	)
}

// roomsRetireNewStatusColorsUp nulls the color of every room carrying a hex
// that only became reserved with the visual-system unification.
//
// Without this the rooms are not merely mis-tinted, they are unsaveable:
// Room.Validate runs IsReservedRoomColor on UPDATE as well as INSERT (see
// services/facilities.UpdateRoom), so an admin renaming such a room or
// changing its capacity gets a 400 ErrReservedColor with no hint that the
// color is the problem. The room becomes uneditable until someone guesses to
// change its color first.
//
// Same shape and same backup table as migration 1.15.45, which faced the
// mirror-image situation. Difference in provenance is worth stating: 1.15.45
// cleared values no human ever picked (a form bug and a column default),
// whereas these four may well have been chosen deliberately back when they
// were free. That is why the backup table matters more here, not less — a
// tenant that notices a room went colorless can have its original hex put
// back from audit.room_color_migration_backup and simply pick a fresh color.
func roomsRetireNewStatusColorsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.271: Backing up + clearing room colors that became status badge colors...")

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
	// clear see the same set even under a concurrent writer. LOWER() on both
	// sides because rooms written before the model started upper-casing on
	// write may still hold lower-case hexes.
	backupRes, err := db.NewRaw(`
		INSERT INTO audit.room_color_migration_backup
			(room_id, tenant_id, name, color)
		SELECT id, tenant_id, name, color
		FROM facilities.rooms
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (?);
	`, bun.In(lowerHexes(newlyReservedStatusHexes))).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed populating audit.room_color_migration_backup: %w", err)
	}
	if backed, raErr := backupRes.RowsAffected(); raErr == nil && backed > 0 {
		fmt.Printf("Migration 1.15.271: backed up %d room(s) into audit.room_color_migration_backup before clearing\n", backed)
	}

	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET color = NULL
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (?);
	`, bun.In(lowerHexes(newlyReservedStatusHexes))).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed clearing newly reserved room colors: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.271: cleared %d room(s) carrying a newly reserved status color (audit.room_color_migration_backup retains the original values)\n", affected)
	}

	return nil
}

// roomsRetireNewStatusColorsDown is deliberately a no-op on the data.
//
// Restoring the colors would hand rooms back a hex that Room.Validate now
// rejects, so the very next save of such a room would fail — a rollback that
// reintroduces the bug it fixed. Operators who genuinely want the old values
// back can take them from the backup table:
//
//	UPDATE facilities.rooms r
//	SET color = b.color
//	FROM audit.room_color_migration_backup b
//	WHERE r.id = b.room_id
//	  AND r.color IS NULL;
//
// and should pair that with reverting the reservedRoomColors change, or the
// restored rooms are unsaveable again.
func roomsRetireNewStatusColorsDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.271: cleared colors are NOT auto-restored — see audit.room_color_migration_backup")
	return nil
}

// lowerHexes returns hexes lower-cased, matching the LOWER(color) comparison
// the queries above use.
func lowerHexes(hexes []string) []string {
	out := make([]string, 0, len(hexes))
	for _, hex := range hexes {
		out = append(out, strings.ToLower(hex))
	}
	return out
}
