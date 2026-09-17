package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schulhofReleaseBlockPlanningVersion     = "1.15.390"
	schulhofReleaseBlockPlanningDescription = "Revoke the open-room release of Schulhof rooms that host planned blocks (#3280, ADR 0019)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schulhofReleaseBlockPlanningVersion,
		Description: schulhofReleaseBlockPlanningDescription,
		// 1.15.372 released every canonical Schulhof; 1.15.36 created the
		// timetable instances the predicate reads.
		DependsOn: []string{roomsOpenReleaseVersion, createActivityInstancesVersion},
	})

	Migrations.MustRegister(schulhofReleaseBlockPlanningUp, schulhofReleaseBlockPlanningDown)
}

// schulhofReleaseBlockPlanningUp corrects the blanket release of 1.15.372.
// Schools that plan their blocks in the Schulhof lost the block list on the
// supervision page once the yard became an open room (#3276). The release is
// revoked where the canonical Schulhof hosts upcoming, non-spontaneous,
// non-cancelled timetable instances of a regular activity: that describes
// "the school plans blocks in the yard". Kiosk schools, whose yard only hosts
// the Schulhof Freispiel system activity, keep the release.
//
// Only is_open_room changes. Running sessions, open visits and the room row
// itself stay as they are; the Schulhof remains selectable in staff lists
// without a release (ADR 0019, point 6).
func schulhofReleaseBlockPlanningUp(ctx context.Context, db *bun.DB) error {
	// The room name is spelled out rather than read from a constant, like in
	// 1.15.372: a migration records the database at this version. The Berlin
	// calendar day decides "upcoming", independent of the server timezone.
	res, err := db.NewRaw(`
		UPDATE facilities.rooms AS room
		SET is_open_room = FALSE, updated_at = now()
		WHERE room.name = 'Schulhof'
		  AND room.is_system = TRUE
		  AND room.is_open_room = TRUE
		  AND EXISTS (
			SELECT 1
			FROM schedule.activity_instances AS instance
			LEFT JOIN activities.groups AS activity
			  ON activity.id = instance.activity_group_id
			 AND activity.tenant_id = instance.tenant_id
			WHERE instance.tenant_id = room.tenant_id
			  AND instance.room_id = room.id
			  AND instance.date >= (now() AT TIME ZONE 'Europe/Berlin')::date
			  AND instance.is_spontaneous = FALSE
			  AND instance.status <> 'cancelled'
			  AND COALESCE(activity.is_system, FALSE) = FALSE
		  );
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed revoking the release of block-planning Schulhof rooms: %w", err)
	}
	if affected, affErr := res.RowsAffected(); affErr == nil {
		migrationLog().InfoContext(ctx, "Schulhof open-room release revoked",
			"rows", affected,
		)
	}

	return nil
}

// schulhofReleaseBlockPlanningDown is deliberately a no-op. The migration
// does not record which rooms it changed, and re-releasing every Schulhof
// would override decisions administrators made after the correction.
func schulhofReleaseBlockPlanningDown(_ context.Context, _ *bun.DB) error {
	return nil
}
