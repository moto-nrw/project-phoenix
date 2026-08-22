package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	backfillClassArrivalTimesVersion     = "1.15.317"
	backfillClassArrivalTimesDescription = "Lift the per-child arrival times into education.class_arrival_times and let the matching child rows inherit (#2414)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     backfillClassArrivalTimesVersion,
		Description: backfillClassArrivalTimesDescription,
		DependsOn:   []string{arrivalTimeOptionalVersion},
	})

	Migrations.MustRegister(backfillClassArrivalTimesUp, backfillClassArrivalTimesDown)
}

// backfillClassArrivalTimesUp turns the existing per-child times into the
// class timetable they always were (#2414, ADR 0005).
//
// Per tenant, class and weekday it takes the most common time among the
// active children and writes it to education.class_arrival_times. Every child
// row that already carries exactly that time then drops its own value and
// inherits — the projection reproduces it byte for byte, so nothing a school
// sees changes. Rows that deviate keep their time and stay visible as
// deviations, which is what they are.
//
// Measured against production at the time of writing: 925 child times become
// 12 class rows at one school, 501 become 17 at another, and across all
// tenants only 35 children deviate from their class.
//
// Deliberately NOT lifted: 00:00 rows (six of them, all data-entry noise) and
// classes with no children left. Ties are broken by the earlier time so the
// result is deterministic.
func backfillClassArrivalTimesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.317: Lifting per-child arrival times into class timetables...")

	res, err := db.ExecContext(ctx, `
		WITH ranked AS (
			SELECT
				arrival.tenant_id,
				MIN(BTRIM(student.school_class)) AS school_class,
				arrival.weekday,
				TO_CHAR(arrival.expected_arrival, 'HH24:MI') AS hhmm,
				COUNT(*) AS children,
				ROW_NUMBER() OVER (
					PARTITION BY arrival.tenant_id, LOWER(BTRIM(student.school_class)), arrival.weekday
					ORDER BY COUNT(*) DESC, TO_CHAR(arrival.expected_arrival, 'HH24:MI') ASC
				) AS rank
			FROM schedule.student_arrival_schedules AS arrival
			JOIN users.students AS student
				ON student.id = arrival.student_id
				AND student.tenant_id = arrival.tenant_id
			WHERE arrival.expected_arrival IS NOT NULL
				AND arrival.expected_arrival <> TIME '00:00'
				AND student.status <> 'alumnus'
				AND BTRIM(student.school_class) <> ''
			GROUP BY arrival.tenant_id, LOWER(BTRIM(student.school_class)), arrival.weekday, arrival.expected_arrival
		), per_class AS (
			SELECT
				tenant_id,
				MIN(school_class) AS school_class,
				JSONB_OBJECT_AGG(
					CASE weekday
						WHEN 1 THEN 'mon' WHEN 2 THEN 'tue' WHEN 3 THEN 'wed'
						WHEN 4 THEN 'thu' WHEN 5 THEN 'fri'
					END,
					hhmm
				) AS arrival_times
			FROM ranked
			WHERE rank = 1 AND weekday BETWEEN 1 AND 5
			GROUP BY tenant_id, LOWER(BTRIM(school_class))
		)
		INSERT INTO education.class_arrival_times (tenant_id, school_class, arrival_times)
		SELECT tenant_id, school_class, arrival_times FROM per_class
		ON CONFLICT (tenant_id, (LOWER(BTRIM(school_class)))) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("lift class arrival times: %w", err)
	}
	if rows, rowsErr := res.RowsAffected(); rowsErr == nil {
		fmt.Printf("  %d class timetables created\n", rows)
	}

	res, err = db.ExecContext(ctx, `
		UPDATE schedule.student_arrival_schedules AS arrival
		SET expected_arrival = NULL
		FROM users.students AS student
		JOIN education.class_arrival_times AS class_time
			ON class_time.tenant_id = student.tenant_id
			AND LOWER(BTRIM(class_time.school_class)) = LOWER(BTRIM(student.school_class))
		WHERE arrival.student_id = student.id
			AND arrival.tenant_id = student.tenant_id
			AND arrival.expected_arrival IS NOT NULL
			AND student.status <> 'alumnus'
			AND TO_CHAR(arrival.expected_arrival, 'HH24:MI') = class_time.arrival_times ->> (
				CASE arrival.weekday
					WHEN 1 THEN 'mon' WHEN 2 THEN 'tue' WHEN 3 THEN 'wed'
					WHEN 4 THEN 'thu' WHEN 5 THEN 'fri'
				END);
	`)
	if err != nil {
		return fmt.Errorf("let matching child rows inherit: %w", err)
	}
	if rows, rowsErr := res.RowsAffected(); rowsErr == nil {
		fmt.Printf("  %d child rows now inherit their class time\n", rows)
	}
	return nil
}

// backfillClassArrivalTimesDown resolves the inheriting rows back to their
// concrete time so no care day loses its arrival time. It deliberately keeps
// class timetables: a later school edit is indistinguishable from a backfilled
// row and must not be deleted by a partial rollback.
func backfillClassArrivalTimesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.317: writing class times back onto the child rows...")

	_, err := db.ExecContext(ctx, `
		UPDATE schedule.student_arrival_schedules AS arrival
		SET expected_arrival = (class_time.arrival_times ->> (
			CASE arrival.weekday
				WHEN 1 THEN 'mon' WHEN 2 THEN 'tue' WHEN 3 THEN 'wed'
				WHEN 4 THEN 'thu' WHEN 5 THEN 'fri'
			END))::time
		FROM users.students AS student
		JOIN education.class_arrival_times AS class_time
			ON class_time.tenant_id = student.tenant_id
			AND LOWER(BTRIM(class_time.school_class)) = LOWER(BTRIM(student.school_class))
		WHERE arrival.student_id = student.id
			AND arrival.tenant_id = student.tenant_id
			AND arrival.expected_arrival IS NULL
			AND class_time.arrival_times ->> (
				CASE arrival.weekday
					WHEN 1 THEN 'mon' WHEN 2 THEN 'tue' WHEN 3 THEN 'wed'
					WHEN 4 THEN 'thu' WHEN 5 THEN 'fri'
				END) IS NOT NULL;

	`)
	if err != nil {
		return fmt.Errorf("restore per-child arrival times: %w", err)
	}
	return nil
}
