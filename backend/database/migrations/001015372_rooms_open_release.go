package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	roomsOpenReleaseVersion     = "1.15.372"
	roomsOpenReleaseDescription = "Add is_open_room release flag to facilities.rooms and release existing canonical Schulhof rooms (#3064)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     roomsOpenReleaseVersion,
		Description: roomsOpenReleaseDescription,
		// is_system is what separates the auto-provisioned Schulhof from a
		// staff-created room that merely carries the same name, so the
		// backfill below cannot run before 1.15.168 introduced that column.
		DependsOn: []string{addIsSystemFlagsVersion},
	})

	Migrations.MustRegister(roomsOpenReleaseUp, roomsOpenReleaseDown)
}

func roomsOpenReleaseUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.372: Adding is_open_room release flag to facilities.rooms...")

	// An "offener Raum" is a place the OGS administration permanently releases
	// for use (#3062). The release belongs to the room, needs no daily opening,
	// and asserts neither that somebody supervises it nor that children may be
	// checked out to go home from there — those stay separate concerns.
	//
	// Default FALSE: ordinary rooms start unreleased and an administrator opts
	// them in. Only the Schulhof is released by the backfill below.
	if _, err := db.NewRaw(`
		ALTER TABLE facilities.rooms
		ADD COLUMN IF NOT EXISTS is_open_room BOOLEAN NOT NULL DEFAULT FALSE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed adding is_open_room column to facilities.rooms: %w", err)
	}

	// Existing schools already depend on the Schulhof being permanently
	// available: children pick it at a kiosk, staff reach it without claiming
	// supervision. Rolling out an unreleased Schulhof would take that away and
	// force every school into a new daily step, so the yard is released here.
	// This deliberately supersedes the "per Default aus" rule from #2183.
	//
	// The room name is spelled out rather than read from constants.SchulhofRoomName:
	// a migration records what the database looked like at this version and must
	// not change meaning when a constant is later renamed.
	//
	// is_system = TRUE is the second predicate on purpose. Room names are unique
	// per tenant, but a school that created its own "Schulhof" room before the
	// reservation guards landed would otherwise be released here without ever
	// having been auto-provisioned. Usage volume is explicitly NOT consulted:
	// retention windows and device-less sessions make a usage-based
	// classification unreliable (#3062).
	res, err := db.NewRaw(`
		UPDATE facilities.rooms
		SET is_open_room = TRUE
		WHERE name = 'Schulhof'
		  AND is_system = TRUE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed releasing canonical Schulhof rooms: %w", err)
	}
	if affected, affErr := res.RowsAffected(); affErr == nil {
		fmt.Printf("Migration 1.15.372: released %d canonical Schulhof room(s)\n", affected)
	}

	return nil
}

func roomsOpenReleaseDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.372...")

	if _, err := db.NewRaw(`
		ALTER TABLE facilities.rooms
		DROP COLUMN IF EXISTS is_open_room;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping is_open_room column from facilities.rooms: %w", err)
	}

	return nil
}
