package migrations

import (
	"context"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	ScheduledCheckoutsVersion     = "1.6.7"
	ScheduledCheckoutsDescription = "Create active.scheduled_checkouts table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[ScheduledCheckoutsVersion] = &Migration{
		Version:     ScheduledCheckoutsVersion,
		Description: ScheduledCheckoutsDescription,
		DependsOn:   []string{"1.6.5", "1.2.1", "1.2.2"}, // Depends on active.attendance, users.students, users.staff
	}

	// Migration 1.6.7: Create active.scheduled_checkouts table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createScheduledCheckoutsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackScheduledCheckoutsTable(ctx, db)
		},
	)
}

func createScheduledCheckoutsTable(ctx context.Context, db *bun.DB) error {
	log.Println("Creating active.scheduled_checkouts table...")

	// Create the scheduled_checkouts table in the active schema
	query := `
	CREATE TABLE IF NOT EXISTS active.scheduled_checkouts (
		id BIGSERIAL PRIMARY KEY,
		student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
		scheduled_by BIGINT NOT NULL REFERENCES users.staff(id),
		scheduled_for TIMESTAMPTZ NOT NULL,
		reason VARCHAR(255),
		status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'executed', 'cancelled')),
		executed_at TIMESTAMPTZ,
		cancelled_at TIMESTAMPTZ,
		cancelled_by BIGINT REFERENCES users.staff(id),
		created_at TIMESTAMPTZ DEFAULT NOW(),
		updated_at TIMESTAMPTZ DEFAULT NOW()
	);

	-- Create indexes for common queries
	CREATE INDEX idx_scheduled_checkouts_student_status ON active.scheduled_checkouts(student_id, status);
	CREATE INDEX idx_scheduled_checkouts_scheduled_for ON active.scheduled_checkouts(scheduled_for) WHERE status = 'pending';
	CREATE INDEX idx_scheduled_checkouts_created_by ON active.scheduled_checkouts(scheduled_by);

	-- Ensure only one pending checkout per student
	CREATE UNIQUE INDEX idx_scheduled_checkouts_unique_pending ON active.scheduled_checkouts(student_id, status) WHERE status = 'pending';

	-- Create updated_at trigger
	CREATE TRIGGER update_scheduled_checkouts_updated_at BEFORE UPDATE ON active.scheduled_checkouts
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

	-- Add comment for documentation
	COMMENT ON TABLE active.scheduled_checkouts IS 'Tracks scheduled future checkouts for students initiated by supervisors';
	COMMENT ON COLUMN active.scheduled_checkouts.scheduled_for IS 'The time when the student should be automatically checked out';
	COMMENT ON COLUMN active.scheduled_checkouts.reason IS 'Optional reason for the scheduled checkout (e.g., doctor appointment)';
	`

	_, err := db.ExecContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to create scheduled_checkouts table: %w", err)
	}

	// Grant permissions to app_user
	grantQuery := `
		GRANT ALL ON active.scheduled_checkouts TO app_user;
		GRANT USAGE ON SEQUENCE active.scheduled_checkouts_id_seq TO app_user;
	`
	if _, err := db.ExecContext(ctx, grantQuery); err != nil {
		return fmt.Errorf("failed to grant permissions on scheduled_checkouts: %w", err)
	}

	log.Println("Created active.scheduled_checkouts table successfully")
	return nil
}

func rollbackScheduledCheckoutsTable(ctx context.Context, db *bun.DB) error {
	log.Println("Rolling back active.scheduled_checkouts table...")

	if _, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS active.scheduled_checkouts CASCADE`); err != nil {
		return fmt.Errorf("failed to drop scheduled_checkouts table: %w", err)
	}

	log.Println("Rolled back active.scheduled_checkouts table successfully")
	return nil
}