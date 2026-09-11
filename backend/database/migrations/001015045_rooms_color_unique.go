package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	roomsColorUniqueVersion     = "1.15.45"
	roomsColorUniqueDescription = "Add partial UNIQUE(tenant_id, lower(color)) on facilities.rooms; back up + clear legacy bug-defaults (#4F46E5 from rooms.config.tsx, #FFFFFF from the 1.1.1 NOT NULL DEFAULT) into audit.room_color_migration_backup. Rollback drops the index but does NOT restore cleared colors — use the backup table for manual restore."
	roomsColorUniqueIndex       = "uniq_facilities_rooms_tenant_color"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsColorUniqueVersion,
		Description: roomsColorUniqueDescription,
		DependsOn: []string{
			FacilitiesRoomsOptionalFieldsVersion, // 1.1.5 — color column made nullable
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return roomsColorUniqueUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return roomsColorUniqueDown(ctx, db)
		},
	)
}

// roomsColorUniqueUp adds a partial unique index keyed by tenant + lowercase
// color so two rooms inside the same school cannot share a hex code.
//
// Pre-step nulls every row carrying one of two legacy bug-defaults:
//
//   - #4F46E5 — produced by a rooms.config.tsx transformBeforeSubmit bug that
//     stamped this hex onto every save regardless of admin input, so
//     production DBs almost certainly have many rooms with this exact hex
//     even though no Pflegekraft picked it. The previous LocationBadge
//     renderer ignored room.color and always rendered OTHER_ROOM blue, so
//     the stored value was invisible. With Issue #1324 the badge starts
//     honouring room.color, so the legacy default would suddenly paint as
//     a slightly purple-ish blue, mixed with rooms that fall through to
//     OTHER_ROOM blue — visually inconsistent for no reason.
//
//   - #FFFFFF — the original NOT NULL DEFAULT from migration 1.1.1
//     (see 001001001_facilities_rooms.go). Migration 1.1.5 dropped the
//     default + NOT NULL constraint but never nulled existing rows, so any
//     tenant with two rooms created before 1.1.5 has duplicate "#FFFFFF"
//     values today. The new partial UNIQUE(tenant_id, LOWER(color))
//     WHERE color IS NOT NULL would refuse to create against those rows
//     and break the deploy. Same provenance as #4F46E5: the value was
//     never user-chosen, only inherited from a column default.
//
// Both go into the same audit.room_color_migration_backup table; the
// `color` column on the backup row preserves the original value so a manual
// restore knows exactly what to put back per row. Genuine admin-picked
// colors (anything outside this set) are preserved untouched. Once cleared,
// the partial unique index can apply without contention.
func roomsColorUniqueUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.45: Backing up + clearing legacy bug-defaults (#4F46E5, #FFFFFF) then adding partial unique index...")

	// Backup table: persists rows we're about to NULL so a manual restore
	// is possible if our "no admin ever picked exactly #4F46E5" assumption
	// is ever wrong. Outlives deploy log rotation, ~1 row per affected
	// room. The CASCADE in the rollback drops it together with anything
	// else in the audit schema, but we only DROP the index there — keeping
	// the backup data untouched on rollback is the whole point.
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
		return fmt.Errorf("failed creating audit.room_color_migration_backup: %w", err)
	}

	// Insert backups BEFORE the UPDATE. Using a single INSERT … SELECT so
	// the snapshot is consistent — if a concurrent writer slipped in
	// between an enumerate and a clear, both would see the same set.
	// NOW() comes from the row default; we just project the columns we
	// need to be able to reverse-engineer the change. The backup row's
	// `color` column preserves the original hex per row so a manual
	// restore knows whether to put back #4F46E5 or #FFFFFF.
	backupRes, err := db.NewRaw(`
		INSERT INTO audit.room_color_migration_backup
			(room_id, tenant_id, name, color)
		SELECT id, tenant_id, name, color
		FROM facilities.rooms
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (LOWER('#4F46E5'), LOWER('#FFFFFF'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed populating audit.room_color_migration_backup: %w", err)
	}
	if backed, raErr := backupRes.RowsAffected(); raErr == nil && backed > 0 {
		fmt.Printf("Migration 1.15.45: backed up %d room(s) into audit.room_color_migration_backup before clearing\n", backed)
	}

	// NULL every room carrying either legacy bug-default. Case-insensitive
	// compare in case anything posted lower-case before the model started
	// uppercasing on write.
	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET color = NULL
		WHERE color IS NOT NULL
		  AND LOWER(color) IN (LOWER('#4F46E5'), LOWER('#FFFFFF'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed clearing legacy room color defaults before unique index: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.45: cleared %d room(s) carrying a legacy bug-default hex (#4F46E5 or #FFFFFF) (audit.room_color_migration_backup retains the original values)\n", affected)
	}

	createSQL := fmt.Sprintf(`
		CREATE UNIQUE INDEX IF NOT EXISTS %s
		ON facilities.rooms (tenant_id, LOWER(color))
		WHERE color IS NOT NULL;
	`, roomsColorUniqueIndex)
	if _, err := db.NewRaw(createSQL).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating %s: %w", roomsColorUniqueIndex, err)
	}

	return nil
}

// roomsColorUniqueDown drops the index. It does NOT restore cleared colours
// from audit.room_color_migration_backup — operators run the rollback when
// they want the index gone, but reverting the data side requires a conscious
// SQL statement (we can't tell the difference between "rollback the whole
// migration" and "drop just the index, keep the cleanup"). The backup table
// is left in place precisely so a human can do the restore deliberately:
//
//	UPDATE facilities.rooms r
//	SET color = b.color
//	FROM audit.room_color_migration_backup b
//	WHERE r.id = b.room_id
//	  AND r.color IS NULL;
//
// The audit table itself is left undropped on rollback — losing the only
// record of what was cleared on a panicky rollback is exactly the kind of
// foot-gun this whole table exists to prevent.
func roomsColorUniqueDown(ctx context.Context, db *bun.DB) error {
	fmt.Printf("Rolling back migration 1.15.45: dropping %s (cleared colours not auto-restored — see audit.room_color_migration_backup)...\n",
		roomsColorUniqueIndex)

	dropSQL := fmt.Sprintf(`DROP INDEX IF EXISTS facilities.%s;`, roomsColorUniqueIndex)
	if _, err := db.NewRaw(dropSQL).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping %s: %w", roomsColorUniqueIndex, err)
	}
	return nil
}
