package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	ActivitiesTablesVersion     = "1.5.0"
	ActivitiesTablesDescription = "Activities schema tables for activity management"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitiesTablesVersion] = &Migration{
		Version:     ActivitiesTablesVersion,
		Description: ActivitiesTablesDescription,
		DependsOn:   []string{"1.4.0"}, // Depends on schedule tables
	}

	// Migration 1.5.0: Activities schema tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return activitiesTablesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return activitiesTablesDown(ctx, db)
		},
	)
}

// activitiesTablesUp creates the activities schema tables
func activitiesTablesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.5.0: Creating activities schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. Create the categories table - for categorizing activities
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.categories (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			description TEXT,
			color TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating categories table: %w", err)
	}

	// Create indexes for categories
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_categories_name ON activities.categories(name);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for categories table: %w", err)
	}

	// 2. Create the groups table - activity groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL,
			max_participants INT NOT NULL,
			is_open BOOLEAN NOT NULL DEFAULT FALSE,
			supervisor_id BIGINT NOT NULL,
			category_id BIGINT NOT NULL,
			dateframe_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_activity_groups_supervisor FOREIGN KEY (supervisor_id) 
				REFERENCES users.teachers(id) ON DELETE RESTRICT,
			CONSTRAINT fk_activity_groups_category FOREIGN KEY (category_id) 
				REFERENCES activities.categories(id) ON DELETE RESTRICT,
			CONSTRAINT fk_activity_groups_dateframe FOREIGN KEY (dateframe_id)
				REFERENCES schedule.dateframes(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating groups table: %w", err)
	}

	// Create indexes for groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_groups_name ON activities.groups(name);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_supervisor ON activities.groups(supervisor_id);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_category ON activities.groups(category_id);
		CREATE INDEX IF NOT EXISTS idx_activity_groups_open ON activities.groups(is_open);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for groups table: %w", err)
	}

	// 3. Create the schedules table - for when activities are scheduled
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.schedules (
			id BIGSERIAL PRIMARY KEY,
			weekday TEXT NOT NULL,
			timeframe_id BIGINT,
			activity_group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_activity_schedules_activity_group FOREIGN KEY (activity_group_id) 
				REFERENCES activities.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_activity_schedules_timeframe FOREIGN KEY (timeframe_id)
				REFERENCES schedule.timeframes(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating schedules table: %w", err)
	}

	// Create indexes for schedules
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_activity_schedules_weekday ON activities.schedules(weekday);
		CREATE INDEX IF NOT EXISTS idx_activity_schedules_activity_group_id ON activities.schedules(activity_group_id);
		CREATE INDEX IF NOT EXISTS idx_activity_schedules_timeframe_id ON activities.schedules(timeframe_id);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_activity_schedules_unique ON activities.schedules(weekday, timeframe_id, activity_group_id) WHERE timeframe_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for schedules table: %w", err)
	}

	// 4. Create the student_enrollments table - for student participation in activities
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.student_enrollments (
			id BIGSERIAL PRIMARY KEY,
			student_id BIGINT NOT NULL,
			activity_group_id BIGINT NOT NULL,
			enrollment_date DATE NOT NULL DEFAULT CURRENT_DATE,
			attendance_status TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_student_enrollments_student FOREIGN KEY (student_id) 
				REFERENCES users.students(id) ON DELETE CASCADE,
			CONSTRAINT fk_student_enrollments_activity_group FOREIGN KEY (activity_group_id) 
				REFERENCES activities.groups(id) ON DELETE CASCADE,
			CONSTRAINT uk_student_activity_enrollment UNIQUE (student_id, activity_group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating student_enrollments table: %w", err)
	}

	// Create indexes for student_enrollments
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_student_id ON activities.student_enrollments(student_id);
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_activity_group_id ON activities.student_enrollments(activity_group_id);
		CREATE INDEX IF NOT EXISTS idx_student_enrollments_date ON activities.student_enrollments(enrollment_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for student_enrollments table: %w", err)
	}

	// Create triggers for updating updated_at timestamps
	_, err = tx.ExecContext(ctx, `
		-- Trigger for categories
		DROP TRIGGER IF EXISTS update_activity_categories_updated_at ON activities.categories;
		CREATE TRIGGER update_activity_categories_updated_at
		BEFORE UPDATE ON activities.categories
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for groups
		DROP TRIGGER IF EXISTS update_activity_groups_updated_at ON activities.groups;
		CREATE TRIGGER update_activity_groups_updated_at
		BEFORE UPDATE ON activities.groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for schedules
		DROP TRIGGER IF EXISTS update_activity_schedules_updated_at ON activities.schedules;
		CREATE TRIGGER update_activity_schedules_updated_at
		BEFORE UPDATE ON activities.schedules
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for student_enrollments
		DROP TRIGGER IF EXISTS update_student_enrollments_updated_at ON activities.student_enrollments;
		CREATE TRIGGER update_student_enrollments_updated_at
		BEFORE UPDATE ON activities.student_enrollments
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// activitiesTablesDown removes the activities schema tables
func activitiesTablesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.5.0: Removing activities schema tables...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.student_enrollments;
		DROP TABLE IF EXISTS activities.schedules;
		DROP TABLE IF EXISTS activities.groups;
		DROP TABLE IF EXISTS activities.categories;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities schema tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
