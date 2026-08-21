package migrations

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/uptrace/bun"
)

const (
	schulhofRoomColorClearDefaultVersion     = "1.15.305"
	schulhofRoomColorClearDefaultDescription = "Back up + clear the auto-provisioned #7ED321 color on Schulhof rooms so the orange Schulhof default applies and admins start from an unset color (#2405)"
)

// The bootstrap fingerprint: the three values both auto-provisioning paths
// (services/facilities/schulhof_service.go and services/iot/checkin's
// ensureSystemRoom) stamp on the Schulhof room they create, plus the color
// they stamped before #2405. Frozen here as literals rather than references
// to constants.Schulhof*: a migration must keep doing what it did on the day
// it shipped, even after the constants move on.
const (
	schulhofAutoProvisionedHex      = "#7ED321"  // constants.SchulhofColor
	schulhofAutoProvisionedCapacity = 300        // constants.SchulhofRoomCapacity
	schulhofAutoProvisionedCategory = "Schulhof" // constants.SchulhofCategoryName
)

// schulhofAutoProvisionedRooms selects the rooms this migration owns. Shared
// between the backup INSERT and the clearing UPDATE so the snapshot and the
// clear can never drift apart. Placeholder order: name, capacity, category,
// lower-cased hex — see schulhofAutoProvisionedArgs.
const schulhofAutoProvisionedRooms = `
		WHERE name = ?
		  AND is_system = TRUE
		  AND capacity = ?
		  AND category = ?
		  AND color IS NOT NULL
		  AND LOWER(color) = ?`

func schulhofAutoProvisionedArgs() []any {
	return []any{
		constants.SchulhofRoomName,
		schulhofAutoProvisionedCapacity,
		schulhofAutoProvisionedCategory,
		strings.ToLower(schulhofAutoProvisionedHex),
	}
}

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

// schulhofRoomColorClearDefaultUp clears #7ED321 from auto-provisioned
// Schulhof rooms.
//
// Until #2405 the Schulhof room's color was not editable: the form hid the
// picker and UpdateRoom rejected changes. A Schulhof row that the bootstrap
// created therefore got its #7ED321 from that bootstrap, never from a
// deliberate admin choice — the same provenance argument migration 1.15.45
// made for the #4F46E5 form-bug default.
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
// Why the predicate is not just name + is_system: is_system is NOT proof of
// provenance. Migration 1.15.168 backfilled the flag by name alone
// (`WHERE name IN ('Schulhof', 'WC', 'Toilette')`), so a room an admin
// created and named "Schulhof" back when that was still possible — and gave
// #7ED321 to by hand — carries the flag today just like a bootstrapped one.
// Clearing that row would throw away a deliberate choice.
//
// So the predicate matches the full shape the bootstrap writes: the canonical
// name, is_system, capacity 300 and category "Schulhof", all four set by the
// same CreateRoom call in both provisioning paths. A hand-made room is very
// unlikely to carry that exact combination.
//
// The remaining error is deliberately one-sided. A false negative (a
// bootstrapped room whose capacity or category an admin has since edited
// keeps its green) costs a school nothing it cannot undo — since #2405 the
// picker is right there, and green is a legal color. A false positive
// silently deletes an admin's pick. On doubt the row stays.
//
// The original values go into audit.room_color_migration_backup, same table
// and same shape as 1.15.45 / 1.15.272.
func schulhofRoomColorClearDefaultUp(ctx context.Context, db *bun.DB) error {
	slog.InfoContext(ctx, "clearing auto-provisioned yard room color",
		"migration_version", schulhofRoomColorClearDefaultVersion,
	)

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
		FROM facilities.rooms`+schulhofAutoProvisionedRooms+`;`,
		schulhofAutoProvisionedArgs()...).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed populating audit.room_color_migration_backup: %w", err)
	}
	if backed, raErr := backupRes.RowsAffected(); raErr == nil && backed > 0 {
		slog.InfoContext(ctx, "backed up auto-provisioned yard room colors",
			"migration_version", schulhofRoomColorClearDefaultVersion,
			"room_count", backed,
		)
	}

	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET color = NULL`+schulhofAutoProvisionedRooms+`;`,
		schulhofAutoProvisionedArgs()...).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed clearing the auto-provisioned Schulhof room color: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		slog.InfoContext(ctx, "cleared auto-provisioned yard room colors",
			"migration_version", schulhofRoomColorClearDefaultVersion,
			"room_count", affected,
		)
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
func schulhofRoomColorClearDefaultDown(ctx context.Context, _ *bun.DB) error {
	slog.InfoContext(ctx, "migration rollback leaves cleared yard room colors unchanged",
		"migration_version", schulhofRoomColorClearDefaultVersion,
	)
	return nil
}
