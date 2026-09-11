package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	classArrivalExceptionsVersion     = "1.15.364"
	classArrivalExceptionsDescription = "education.class_arrival_exceptions: a different arrival time for a whole class on one date (#2962)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classArrivalExceptionsVersion,
		Description: classArrivalExceptionsDescription,
		DependsOn:   []string{classArrivalTimesVersion},
	})

	Migrations.MustRegister(classArrivalExceptionsUp, classArrivalExceptionsDown)
}

// classArrivalExceptionsUp creates education.class_arrival_exceptions (#2962):
// one row per tenant, school class and date carrying the arrival time that
// replaces the class's weekly Unterrichtsschluss on that date (Unterricht
// fällt aus, early release). The arrival projection (ADR 0005) reads it next
// to education.class_arrival_times, so nothing is written per child.
//
// The class is matched via LOWER(BTRIM(...)) like education.class_arrival_times.
func classArrivalExceptionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.364: Class arrival exceptions...")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.class_arrival_exceptions (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			school_class  TEXT NOT NULL,
			date          DATE NOT NULL,
			arrival_time  TIME WITHOUT TIME ZONE NOT NULL,
			reason        TEXT,
			created_by    BIGINT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_class_arrival_exceptions_school_class CHECK (BTRIM(school_class) <> '')
		);

		COMMENT ON TABLE education.class_arrival_exceptions IS
			'Arrival time of a whole class on one date, replacing class_arrival_times for that date in the arrival projection (#2962, ADR 0005)';

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_class_arrival_exceptions_class_date
			ON education.class_arrival_exceptions (tenant_id, LOWER(BTRIM(school_class)), date);

		CREATE INDEX IF NOT EXISTS idx_class_arrival_exceptions_tenant_date
			ON education.class_arrival_exceptions (tenant_id, date);

		DROP TRIGGER IF EXISTS update_class_arrival_exceptions_updated_at ON education.class_arrival_exceptions;
		CREATE TRIGGER update_class_arrival_exceptions_updated_at
		BEFORE UPDATE ON education.class_arrival_exceptions
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.class_arrival_exceptions TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.class_arrival_exceptions_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating education.class_arrival_exceptions: %w", err)
	}

	return provisionTenantRLS(ctx, db, "education.class_arrival_exceptions")
}

func classArrivalExceptionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.364: Dropping education.class_arrival_exceptions...")

	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS education.class_arrival_exceptions;`)
	if err != nil {
		return fmt.Errorf("error dropping education.class_arrival_exceptions: %w", err)
	}
	return nil
}
