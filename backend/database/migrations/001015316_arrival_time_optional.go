package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	arrivalTimeOptionalVersion     = "1.15.316"
	arrivalTimeOptionalDescription = "schedule.student_arrival_schedules.expected_arrival becomes optional: the row marks the care day, the class timetable supplies the time (#2414)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     arrivalTimeOptionalVersion,
		Description: arrivalTimeOptionalDescription,
		DependsOn:   []string{classArrivalTimesVersion},
	})

	Migrations.MustRegister(arrivalTimeOptionalUp, arrivalTimeOptionalDown)
}

// arrivalTimeOptionalUp splits the two things an arrival row carried at once
// (#2414, ADR 0005).
//
// Until now one row meant BOTH "this child is in care on that weekday" AND
// "it arrives at that time". Rolling the class timetable out as the time
// source therefore also rewrote the care days, which is wrong: a child booked
// Monday and Thursday must not become a Wednesday child because its class has
// a Wednesday timetable.
//
// After this migration the row means only "care day". A NULL expected_arrival
// says "take the time from education.class_arrival_times for my class and
// weekday"; a set value is a deliberate per-child deviation that wins over the
// class. There is never a time on a day without care — that combination is
// what left two deregistered children "expected" at OGS am Berg on 19.08.
func arrivalTimeOptionalUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.316: Optional arrival time (class timetable becomes the source)...")

	_, err := db.ExecContext(ctx, `
		ALTER TABLE schedule.student_arrival_schedules
			ALTER COLUMN expected_arrival DROP NOT NULL;

		COMMENT ON COLUMN schedule.student_arrival_schedules.expected_arrival IS
			'NULL = take the time from education.class_arrival_times for this child''s class and weekday; a set value is a per-child deviation that wins over the class (#2414, ADR 0005)';

		COMMENT ON TABLE schedule.student_arrival_schedules IS
			'One row per child and weekday the child is in care. The row is the care-day marker; the time is optional (#2414, ADR 0005)';
	`)
	if err != nil {
		return fmt.Errorf("make expected_arrival optional: %w", err)
	}
	return nil
}

func arrivalTimeOptionalDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.316: expected_arrival becomes mandatory again...")

	// A row that inherits its time from the class has no representation once
	// the column is mandatory again, so the rollback drops those rows. The
	// care days they carried are recoverable from the class timetable and the
	// booking links, which the rollback deliberately leaves untouched.
	_, err := db.ExecContext(ctx, `
		DELETE FROM schedule.student_arrival_schedules WHERE expected_arrival IS NULL;

		ALTER TABLE schedule.student_arrival_schedules
			ALTER COLUMN expected_arrival SET NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("restore mandatory expected_arrival: %w", err)
	}
	return nil
}
