package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	roomCategoriesVersion     = "1.15.472"
	roomCategoriesDescription = "Create facilities.room_categories - one badge color per room category, backfilled per school; drop the per-school uniqueness of room colors (#4352)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomCategoriesVersion,
		Description: roomCategoriesDescription,
		DependsOn: []string{
			roomsColorUniqueVersion, // the index this migration drops
			"1.15.471",              // previous head
		},
	})

	Migrations.MustRegister(roomCategoriesUp, roomCategoriesDown)
}

// roomCategoriesUp gives every school a room category catalog (#4352): one
// row per category with the color the presence badges show for the rooms of
// that category. Rooms keep naming their category by text and are matched
// case-insensitively; a room's own color still wins over the category color.
//
// The backfill gives every school the four categories the room form offered
// as fixed options (facilities.DefaultRoomCategories, keep the colors in
// sync), plus every other category its ordinary rooms already use. Those get
// a color from a small muted palette in name order. System rooms (Schulhof,
// WC) are skipped: their categories are activity infrastructure, not part of
// the catalog. It runs before RLS is forced, because the migration role has
// no tenant set.
//
// Room colors stop being unique per school: rooms of one category share the
// category color, so an own color may repeat as well.
func roomCategoriesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logRollbackFailure(ctx, rollbackErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS facilities.room_categories (
			id         BIGSERIAL PRIMARY KEY,
			tenant_id  BIGINT      NOT NULL,
			name       TEXT        NOT NULL,
			color      TEXT        NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_room_categories_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT chk_room_categories_name
				CHECK (BTRIM(name) <> '' AND char_length(name) <= 60),
			CONSTRAINT chk_room_categories_color
				CHECK (color ~ '^#[0-9A-F]{6}$')
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_facilities_room_categories_tenant_name
			ON facilities.room_categories (tenant_id, LOWER(name));

		INSERT INTO facilities.room_categories (tenant_id, name, color)
		SELECT school.id, defaults.name, defaults.color
		FROM platform.schools AS school
		CROSS JOIN (VALUES
			('Gruppenraum', '#283590'),
			('Normaler Raum', '#374750'),
			('Themenraum', '#6A1B90'),
			('Sport', '#827710')
		) AS defaults(name, color)
		ON CONFLICT (tenant_id, LOWER(name)) DO NOTHING;

		INSERT INTO facilities.room_categories (tenant_id, name, color)
		SELECT used.tenant_id, used.name,
			(ARRAY['#8D3B1E', '#6D4C41', '#455A64', '#7B5E2A'])[
				1 + ((ROW_NUMBER() OVER (PARTITION BY used.tenant_id ORDER BY LOWER(used.name)) - 1) % 4)::int
			]
		FROM (
			SELECT DISTINCT ON (room.tenant_id, LOWER(BTRIM(room.category)))
				room.tenant_id, BTRIM(room.category) AS name
			FROM facilities.rooms AS room
			WHERE NOT room.is_system
			  AND room.category IS NOT NULL
			  AND BTRIM(room.category) <> ''
			  AND char_length(BTRIM(room.category)) <= 60
			-- COLLATE "C": the spelling kept for a category must not depend on
			-- the database collation; capitalised spellings sort first.
			ORDER BY room.tenant_id, LOWER(BTRIM(room.category)), BTRIM(room.category) COLLATE "C"
		) AS used
		WHERE NOT EXISTS (
			SELECT 1 FROM facilities.room_categories AS existing
			WHERE existing.tenant_id = used.tenant_id AND LOWER(existing.name) = LOWER(used.name)
		)
		ON CONFLICT (tenant_id, LOWER(name)) DO NOTHING;

		DROP TRIGGER IF EXISTS update_room_categories_updated_at ON facilities.room_categories;
		CREATE TRIGGER update_room_categories_updated_at
		BEFORE UPDATE ON facilities.room_categories
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP INDEX IF EXISTS facilities.uniq_facilities_rooms_tenant_color;
	`)
	if err != nil {
		return fmt.Errorf("error creating room category catalog: %w", err)
	}
	if err := provisionTenantRLS(ctx, tx, "facilities.room_categories"); err != nil {
		return err
	}

	return tx.Commit()
}

// roomCategoriesDown drops the catalog and restores the per-school uniqueness
// of room colors. The index fails to build while two rooms of a school share
// a color; clear the duplicates first in that case.
func roomCategoriesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS facilities.room_categories;
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_facilities_rooms_tenant_color
			ON facilities.rooms (tenant_id, LOWER(color))
			WHERE color IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error dropping room category catalog: %w", err)
	}
	return nil
}
