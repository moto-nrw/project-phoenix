package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ActivitiesSchedulesVersion     = "1.3.3"
	ActivitiesSchedulesDescription = "Create activities.schedules table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ActivitiesSchedulesVersion] = &Migration{
		Version:     ActivitiesSchedulesVersion,
		Description: ActivitiesSchedulesDescription,
		DependsOn:   []string{"1.3.2", "1.1.2"}, // Depends on activities.groups and schedule.timeframes
	}

	// Migration 1.3.3: Create activities.schedules table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createActivitiesSchedulesTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropActivitiesSchedulesTable(ctx, db)
		},
	)
}

// createActivitiesSchedulesTable creates the activities.schedules table
func createActivitiesSchedulesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.3.3: Creating activities.schedules table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the schedules table - for when activities are scheduled
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS activities.schedules (
			id BIGSERIAL PRIMARY KEY,
			weekday INTEGER NOT NULL CHECK (weekday >= 1 AND weekday <= 7),
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

	// Add comment for weekday column documentation
	_, err = tx.ExecContext(ctx, `
		COMMENT ON COLUMN activities.schedules.weekday IS 'ISO 8601 weekday (1=Monday, 7=Sunday)';
	`)
	if err != nil {
		return fmt.Errorf("error adding column comment: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for schedules
		DROP TRIGGER IF EXISTS update_activity_schedules_updated_at ON activities.schedules;
		CREATE TRIGGER update_activity_schedules_updated_at
		BEFORE UPDATE ON activities.schedules
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating trigger for schedules table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropActivitiesSchedulesTable drops the activities.schedules table
func dropActivitiesSchedulesTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.3.3: Removing activities.schedules table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_activity_schedules_updated_at ON activities.schedules;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for schedules table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS activities.schedules;
	`)
	if err != nil {
		return fmt.Errorf("error dropping activities.schedules table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
