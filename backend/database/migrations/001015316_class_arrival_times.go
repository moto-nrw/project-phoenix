package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	classArrivalTimesVersion     = "1.15.316"
	classArrivalTimesDescription = "education.class_arrival_times: Unterrichtsschluss per Klasse und Wochentag as the arrival-time baseline (#2414)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classArrivalTimesVersion,
		Description: classArrivalTimesDescription,
		DependsOn:   []string{classListEntriesVersion},
	})

	Migrations.MustRegister(classArrivalTimesUp, classArrivalTimesDown)
}

// classArrivalTimesUp creates education.class_arrival_times (#2414): one row
// per tenant and school class carrying the Unterrichtsschluss per weekday as
// {"mon":"11:45",...} — the same shape as enrollment.care_offerings.pickup_times.
//
// It is the baseline the arrival projection reads (ADR 0005). The production
// analysis behind that decision: 97.5% to 100% of the existing per-child rows
// equal the value of their (class, weekday) pair, while the booked care
// offering carries two to three different arrival times per weekday and
// therefore says nothing about the time.
//
// The class is the same free-text value as users.students.school_class,
// matched via LOWER(BTRIM(...)) like education.class_teachers and
// users.class_list_entries. There is no class entity to reference.
//
// schedule.student_arrival_schedules deliberately gets NO source column: a
// derived row is never stored, so a stored row IS the manual override. The
// pickup side needed that column only to keep its legacy materialized rows
// distinguishable.
func classArrivalTimesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.316: Class arrival times...")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.class_arrival_times (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			school_class  TEXT NOT NULL,
			arrival_times JSONB NOT NULL DEFAULT '{}'::jsonb,
			updated_by    BIGINT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_class_arrival_times_school_class CHECK (BTRIM(school_class) <> '')
		);

		COMMENT ON COLUMN education.class_arrival_times.arrival_times IS
			'Unterrichtsschluss per weekday ({"mon":"11:45",...}); projected onto schedule.student_arrival_schedules at read time (#2414, ADR 0005)';

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_class_arrival_times_class
			ON education.class_arrival_times (tenant_id, LOWER(BTRIM(school_class)));

		DROP TRIGGER IF EXISTS update_class_arrival_times_updated_at ON education.class_arrival_times;
		CREATE TRIGGER update_class_arrival_times_updated_at
		BEFORE UPDATE ON education.class_arrival_times
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.class_arrival_times TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.class_arrival_times_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating education.class_arrival_times: %w", err)
	}

	return provisionTenantRLS(ctx, db, "education.class_arrival_times")
}

func classArrivalTimesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.316: Dropping education.class_arrival_times...")

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS education.class_arrival_times;`)
	if err != nil {
		return fmt.Errorf("error dropping education.class_arrival_times: %w", err)
	}
	return nil
}
