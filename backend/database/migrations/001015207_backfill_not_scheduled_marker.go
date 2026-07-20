package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillNotScheduledMarkerVersion     = "1.15.207"
	backfillNotScheduledMarkerDescription = "Stamp the not_scheduled marker on legacy 'expected' rows of completed timetable instances the care plan does not book (#1747)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillNotScheduledMarkerVersion,
		Description: backfillNotScheduledMarkerDescription,
		DependsOn:   []string{instanceStudentsNotScheduledVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return backfillNotScheduledMarkerUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return backfillNotScheduledMarkerDown(ctx, db)
		},
	)
}

// backfillNotScheduledMarkerUp finishes the repair 1.15.205 started.
//
// 1.15.205 wrote the missing absences, but only for children the care plan
// booked that day; it deliberately left the rest untouched because the
// not_scheduled column did not exist yet (1.15.206 adds it). Those leftovers
// are the other half of the same population: a group- or year-wide assignment
// (#1838) put the child on a block occurring on a weekday they were never
// booked for, and the old force-completion path had no verdict to record.
//
// Stamping them is what the current completion path would have done — it
// spares exactly these children the absent stamp and marks them instead, so
// the planner counts them under "nicht eingeplant" and the attendance history
// keeps claiming nothing. Without the stamp they stay indistinguishable from
// genuinely expected children on a finished day.
//
// The population is defined by legacyCareDayBookedPredicate (1.15.205), simply
// negated, and carries the same "untouched since completion" guard: a row
// somebody reset to 'expected' by hand after the instance finished is a
// deliberate decision and stays as it is.
func backfillNotScheduledMarkerUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.207: Stamping not_scheduled on legacy unbooked attendance rows (#1747)...")

	res, err := db.NewRaw(`
		UPDATE schedule.instance_students AS student
		SET not_scheduled = TRUE
		FROM schedule.activity_instances AS instance
		WHERE instance.id = student.instance_id
			AND instance.tenant_id = student.tenant_id
			AND instance.status = 'completed'
			AND student.status = 'expected'
			AND student.not_scheduled = FALSE
			AND instance.completed_at IS NOT NULL
			AND student.updated_at <= instance.completed_at
			AND NOT ` + legacyCareDayBookedPredicate + `;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed stamping not_scheduled on legacy unbooked attendance: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil {
		fmt.Printf("Migration 1.15.207: marked %d attendance rows as not scheduled\n", affected)
	}

	return nil
}

// backfillNotScheduledMarkerDown is a no-op: the stamp is the same marker the
// regular completion path writes, and nothing records which rows this
// migration touched. Clearing them all would erase runtime-written markers
// too.
func backfillNotScheduledMarkerDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.207: no-op for legacy not_scheduled marker backfill")
	return nil
}
