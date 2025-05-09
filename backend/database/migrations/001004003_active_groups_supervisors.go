package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	StaffGroupSupervisionVersion     = "1.4.3"
	StaffGroupSupervisionDescription = "Create active.staff_group_supervision table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[StaffGroupSupervisionVersion] = &Migration{
		Version:     StaffGroupSupervisionVersion,
		Description: StaffGroupSupervisionDescription,
		DependsOn:   []string{"1.4.1", "1.2.3"}, // Depends on active_groups and users_staff
	}

	// Migration 1.4.3: Create active.staff_group_supervision table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffGroupSupervisionUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffGroupSupervisionDown(ctx, db)
		},
	)
}

// staffGroupSupervisionUp creates the active.staff_group_supervision table
func staffGroupSupervisionUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.3: Creating active.staff_group_supervision table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the staff_group_supervision table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.staff_group_supervision (
			id BIGSERIAL PRIMARY KEY,
			staff_id BIGINT NOT NULL,             -- Reference to users.staff
			group_id BIGINT NOT NULL,             -- Reference to active.groups
			role VARCHAR(50) NOT NULL DEFAULT 'supervisor', -- Role in the group (supervisor, assistant, etc.)
			start_date DATE NOT NULL DEFAULT CURRENT_DATE,
			end_date DATE,                        -- Optional end date if supervision is temporary
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Composite unique constraint to prevent duplicates
			CONSTRAINT unique_staff_group_role UNIQUE (staff_id, group_id, role),

			-- Foreign key constraints
			CONSTRAINT fk_supervision_staff FOREIGN KEY (staff_id)
				REFERENCES users.staff(id) ON DELETE CASCADE,
			CONSTRAINT fk_supervision_group FOREIGN KEY (group_id)
				REFERENCES active.groups(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating active.staff_group_supervision table: %w", err)
	}

	// Create indexes to improve query performance
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_supervision_staff_id ON active.staff_group_supervision(staff_id);
		CREATE INDEX IF NOT EXISTS idx_supervision_group_id ON active.staff_group_supervision(group_id);
		CREATE INDEX IF NOT EXISTS idx_supervision_role ON active.staff_group_supervision(role);
		CREATE INDEX IF NOT EXISTS idx_supervision_date_range ON active.staff_group_supervision(start_date, end_date);

		-- Index for finding active supervisions (where end_date is null or >= current_date)
		CREATE INDEX IF NOT EXISTS idx_supervision_active ON active.staff_group_supervision(staff_id, group_id)
		WHERE (end_date IS NULL OR end_date >= CURRENT_DATE);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for staff_group_supervision table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for staff_group_supervision
		DROP TRIGGER IF EXISTS update_supervision_updated_at ON active.staff_group_supervision;
		CREATE TRIGGER update_supervision_updated_at
		BEFORE UPDATE ON active.staff_group_supervision
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for staff_group_supervision: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// staffGroupSupervisionDown drops the active.staff_group_supervision table
func staffGroupSupervisionDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.3: Removing active.staff_group_supervision table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop trigger first
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_supervision_updated_at ON active.staff_group_supervision;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for staff_group_supervision table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.staff_group_supervision CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.staff_group_supervision table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
