package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	templateExtensionsVersion     = "1.15.34"
	templateExtensionsDescription = "Add template extension columns to activities tables for timetable system"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     templateExtensionsVersion,
		Description: templateExtensionsDescription,
		DependsOn: []string{
			ActivitiesGroupsVersion,             // 1.3.2
			ActivitiesSchedulesVersion,          // 1.3.3
			ActivitySupervisorsVersion,          // 1.3.4
			ActivitiesStudentEnrollmentsVersion, // 1.3.8
			createCalendarPeriodsVersion,        // 1.15.33
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return templateExtensionsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return templateExtensionsDown(ctx, db)
		},
	)
}

func templateExtensionsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.34: Adding template extension columns to activities tables...")

	// 1. activities.groups — add type, education_group_id, is_template
	_, err := db.NewRaw(`
		ALTER TABLE activities.groups
			ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'activity',
			ADD COLUMN IF NOT EXISTS education_group_id BIGINT REFERENCES education.groups(id),
			ADD COLUMN IF NOT EXISTS is_template BOOLEAN NOT NULL DEFAULT true;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding columns to activities.groups: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE activities.groups
			ADD CONSTRAINT check_group_type CHECK (type IN ('activity', 'care', 'external'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding type check constraint to activities.groups: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_activities_groups_type
		ON activities.groups(type);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating index on activities.groups.type: %w", err)
	}

	// 2. activities.schedules — add week_pattern, calendar_period_id
	_, err = db.NewRaw(`
		ALTER TABLE activities.schedules
			ADD COLUMN IF NOT EXISTS week_pattern SMALLINT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding columns to activities.schedules: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE activities.schedules
			ADD CONSTRAINT check_week_pattern CHECK (week_pattern >= 0);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding week_pattern check constraint: %w", err)
	}

	// 3. activities.student_enrollments — rename enrollment_date to valid_from, add valid_until, calendar_period_id
	_, err = db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			RENAME COLUMN enrollment_date TO valid_from;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed renaming enrollment_date to valid_from: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			ADD COLUMN IF NOT EXISTS valid_until DATE,
			ADD COLUMN IF NOT EXISTS calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding columns to activities.student_enrollments: %w", err)
	}

	// Drop old unique index and create partial unique index
	_, err = db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_student_enrollments_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping old enrollment unique index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE UNIQUE INDEX idx_student_enrollments_active
		ON activities.student_enrollments (tenant_id, student_id, activity_group_id)
		WHERE valid_until IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating partial unique index on student_enrollments: %w", err)
	}

	// Rename the old enrollment_date index
	_, err = db.NewRaw(`
		ALTER INDEX IF EXISTS activities.idx_student_enrollments_date RENAME TO idx_student_enrollments_valid_from;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed renaming enrollment_date index: %w", err)
	}

	// 4. activities.supervisors — add valid_from, valid_until, calendar_period_id
	_, err = db.NewRaw(`
		ALTER TABLE activities.supervisors
			ADD COLUMN IF NOT EXISTS valid_from DATE NOT NULL DEFAULT CURRENT_DATE,
			ADD COLUMN IF NOT EXISTS valid_until DATE,
			ADD COLUMN IF NOT EXISTS calendar_period_id BIGINT REFERENCES schedule.calendar_periods(id);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding columns to activities.supervisors: %w", err)
	}

	// Drop old unique index and create partial unique index
	_, err = db.NewRaw(`
		DROP INDEX IF EXISTS activities.idx_supervisors_planned_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping old supervisor unique index: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE UNIQUE INDEX idx_supervisors_active
		ON activities.supervisors (tenant_id, staff_id, group_id)
		WHERE valid_until IS NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating partial unique index on supervisors: %w", err)
	}

	return nil
}

func templateExtensionsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.34: Removing template extension columns...")

	// Reverse supervisors changes
	_, _ = db.NewRaw(`DROP INDEX IF EXISTS activities.idx_supervisors_active;`).Exec(ctx)
	_, _ = db.NewRaw(`
		CREATE UNIQUE INDEX idx_supervisors_planned_tenant
		ON activities.supervisors (tenant_id, staff_id, group_id);
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.supervisors
			DROP COLUMN IF EXISTS calendar_period_id,
			DROP COLUMN IF EXISTS valid_until,
			DROP COLUMN IF EXISTS valid_from;
	`).Exec(ctx)

	// Reverse student_enrollments changes
	_, _ = db.NewRaw(`DROP INDEX IF EXISTS activities.idx_student_enrollments_active;`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER INDEX IF EXISTS activities.idx_student_enrollments_valid_from RENAME TO idx_student_enrollments_date;
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		CREATE UNIQUE INDEX idx_student_enrollments_tenant
		ON activities.student_enrollments (tenant_id, student_id, activity_group_id);
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			DROP COLUMN IF EXISTS calendar_period_id,
			DROP COLUMN IF EXISTS valid_until;
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.student_enrollments
			RENAME COLUMN valid_from TO enrollment_date;
	`).Exec(ctx)

	// Reverse schedules changes
	_, _ = db.NewRaw(`
		ALTER TABLE activities.schedules
			DROP CONSTRAINT IF EXISTS check_week_pattern;
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.schedules
			DROP COLUMN IF EXISTS calendar_period_id,
			DROP COLUMN IF EXISTS week_pattern;
	`).Exec(ctx)

	// Reverse groups changes
	_, _ = db.NewRaw(`DROP INDEX IF EXISTS activities.idx_activities_groups_type;`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.groups
			DROP CONSTRAINT IF EXISTS check_group_type;
	`).Exec(ctx)
	_, _ = db.NewRaw(`
		ALTER TABLE activities.groups
			DROP COLUMN IF EXISTS is_template,
			DROP COLUMN IF EXISTS education_group_id,
			DROP COLUMN IF EXISTS type;
	`).Exec(ctx)

	return nil
}
