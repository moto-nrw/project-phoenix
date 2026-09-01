package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	roomsWCAliasUniqueVersion     = "1.15.48"
	roomsWCAliasUniqueDescription = "Add partial UNIQUE(tenant_id) WHERE name IN ('WC','Toilette') on facilities.rooms; archive losers when a tenant already has both aliases. Backup of every renamed row goes to audit.wc_alias_migration_backup. Rollback drops the index but does NOT restore archived names — see backup table for manual restore."
	roomsWCAliasUniqueIndex       = "uniq_facilities_rooms_tenant_wc_alias"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsWCAliasUniqueVersion,
		Description: roomsWCAliasUniqueDescription,
		DependsOn: []string{
			FacilitiesRoomsOptionalFieldsVersion, // 1.1.5 — facilities.rooms shape this index keys on
			ActiveGroupsVersion,                  // 1.4.1 — cleanup CTE reads active.groups for the tie-breaker
			roomsColorUniqueVersion,              // 1.15.45 — most recent facilities.rooms migration; preserves ladder order
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return roomsWCAliasUniqueUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return roomsWCAliasUniqueDown(ctx, db)
		},
	)
}

// roomsWCAliasUniqueUp closes the TOCTOU gap that the application-level
// guard in services/facilities.CreateRoom can't reach: two concurrent admin
// requests, one creating "WC" and one creating "Toilette", both passing the
// FindToiletRoom-then-Insert sequence and both succeeding. The fix is a
// partial unique index keyed by tenant where the name is one of the
// canonical toilet aliases (matching IsWCRoomName: exact case, "WC" or
// "Toilette" only).
//
// CREATE UNIQUE INDEX would fail outright on any tenant where the bug has
// already produced duplicates. Since this codebase has no way to inspect
// production data ahead of a deploy (operators don't run ad-hoc queries
// against staging/prod), the migration assumes duplicates exist and cleans
// them up in the same transaction the index is built in. Mirrors the
// pattern from 1.15.42 (attendance) and 1.15.47 (active visits) — cleanup
// + index in one tx so the framework rolls everything back together if
// the index build fails.
//
// Tie-breaker for which row wins per tenant (canonical = stays under one
// of the alias names; loser = renamed out of the alias namespace):
//
//  1. Most active.groups attached — the room currently being used as the
//     toilet special-room beats an unused duplicate.
//  2. Canonical name preference — `name = 'WC'` beats `name = 'Toilette'`,
//     matching the canonical-first order in
//     services/facilities.wcRoomAliasNames.
//  3. Oldest id — final deterministic tiebreaker.
//
// Losers are renamed to "<original> (archiviert v1.15.48 id=<id>)". The id
// suffix guarantees the archived name is unique within the tenant. The
// archived name no longer matches name IN ('WC','Toilette'), so the row
// falls out of the alias namespace and the partial unique index can build
// without contention.
//
// Renamed rooms are NOT deleted: they keep all FKs intact. A school admin
// can consult audit.wc_alias_migration_backup and rename the row back
// manually after deleting the canonical winner.
//
// Idempotency: re-running this migration after a partial failure is safe.
// The cleanup CTE only matches rows where name IN ('WC','Toilette'), and
// archived rows have names like "Toilette (archiviert v1.15.48 id=7)"
// which don't match the filter.
func roomsWCAliasUniqueUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.48: Archiving duplicate WC/Toilette aliases then adding partial unique index...")

	// Backup table: persists every row we're about to rename so a manual
	// restore is possible if the auto-picked winner turns out to be wrong
	// for some tenant. Outlives the deploy log; ~1 row per archived loser,
	// which in practice should be 0 or a handful of rows total. Same
	// retention philosophy as audit.room_color_migration_backup from
	// migration 1.15.45 — the table stays on rollback so a panicky operator
	// who runs the down migration doesn't lose the only record of what
	// changed.
	if _, err := db.NewRaw(`
		CREATE SCHEMA IF NOT EXISTS audit;

		CREATE TABLE IF NOT EXISTS audit.wc_alias_migration_backup (
			id              BIGSERIAL PRIMARY KEY,
			room_id         BIGINT NOT NULL,
			tenant_id       BIGINT NOT NULL,
			original_name   TEXT NOT NULL,
			archived_name   TEXT NOT NULL,
			migrated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_wc_alias_migration_backup_tenant
			ON audit.wc_alias_migration_backup (tenant_id);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating audit.wc_alias_migration_backup: %w", err)
	}

	// Cleanup: rank rooms within each tenant where LOWER(name) is a toilet
	// alias, pick a canonical winner per tenant via the tie-breaker order
	// documented above, archive the losers (rename + backup row) inside one
	// CTE so the snapshot is consistent.
	//
	// The RETURNING in the rename UPDATE feeds the backup INSERT so we
	// guarantee a 1:1 correspondence between renamed rows and audit rows —
	// no possibility of "renamed but not backed up" or "backed up but not
	// renamed". The whole migration is one tx so this is belt-and-braces,
	// but it costs nothing and survives a tx mode change in BUN later.
	//
	// PostgreSQL's CTE evaluation order: data-modifying CTEs run in
	// dependency order, and `archived` reads from the result of `losers`
	// (which reads from `ranked`) — so the chain is forced sequential.
	res, err := db.NewRaw(`
		WITH ranked AS (
			SELECT
				r.id,
				r.tenant_id,
				r.name,
				ROW_NUMBER() OVER (
					PARTITION BY r.tenant_id
					ORDER BY
						(SELECT COUNT(*) FROM active.groups ag WHERE ag.room_id = r.id) DESC,
						CASE r.name
							WHEN 'WC'       THEN 0
							WHEN 'Toilette' THEN 1
							ELSE 2
						END,
						r.id ASC
				) AS rn
			FROM facilities.rooms r
			WHERE r.name IN ('WC', 'Toilette')
		),
		losers AS (
			SELECT id, tenant_id, name FROM ranked WHERE rn > 1
		),
		archived AS (
			UPDATE facilities.rooms r
			SET name = l.name || ' (archiviert v1.15.48 id=' || l.id || ')',
			    updated_at = NOW()
			FROM losers l
			WHERE r.id = l.id
			RETURNING r.id AS room_id, r.tenant_id, l.name AS original_name, r.name AS archived_name
		)
		INSERT INTO audit.wc_alias_migration_backup
			(room_id, tenant_id, original_name, archived_name)
		SELECT room_id, tenant_id, original_name, archived_name
		FROM archived;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed archiving duplicate WC/Toilette aliases before unique index: %w", err)
	}
	if affected, raErr := res.RowsAffected(); raErr == nil && affected > 0 {
		fmt.Printf("Migration 1.15.48: archived %d duplicate WC/Toilette alias row(s) (originals preserved in audit.wc_alias_migration_backup)\n", affected)
	}

	// Build the partial unique index. After the cleanup above, every tenant
	// has at most one row matching the predicate, so the index can build
	// without contention. The repository layer maps 23505 against this
	// index name to ErrDuplicateRoom — see RoomWCAliasUniqueConstraintName
	// in models/facilities/room_colors.go.
	createSQL := fmt.Sprintf(`
		CREATE UNIQUE INDEX IF NOT EXISTS %s
		ON facilities.rooms (tenant_id)
		WHERE name IN ('WC', 'Toilette');
	`, roomsWCAliasUniqueIndex)
	if _, err := db.NewRaw(createSQL).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating %s: %w", roomsWCAliasUniqueIndex, err)
	}

	return nil
}

// roomsWCAliasUniqueDown drops the partial unique index. It does NOT undo
// the archive renames or restore the originals from
// audit.wc_alias_migration_backup. Same reasoning as 1.15.45: the down
// migration runs when an operator wants the index gone, but the data
// cleanup was an independent decision — reverting it on every rollback
// would risk re-introducing the duplicate state the index was added to
// prevent. The backup table is left in place precisely so a human can do
// the restore deliberately:
//
//	UPDATE facilities.rooms r
//	SET name = b.original_name
//	FROM audit.wc_alias_migration_backup b
//	WHERE r.id = b.room_id
//	  AND r.name = b.archived_name;
//
// Note: that manual restore would re-create the duplicate, so it should
// only be run after first deciding which row should keep the alias and
// renaming the other one out of the namespace by hand.
func roomsWCAliasUniqueDown(ctx context.Context, db *bun.DB) error {
	fmt.Printf("Rolling back migration 1.15.48: dropping %s (archived names not auto-restored — see audit.wc_alias_migration_backup)...\n",
		roomsWCAliasUniqueIndex)

	dropSQL := fmt.Sprintf(`DROP INDEX IF EXISTS facilities.%s;`, roomsWCAliasUniqueIndex)
	if _, err := db.NewRaw(dropSQL).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping %s: %w", roomsWCAliasUniqueIndex, err)
	}
	return nil
}
