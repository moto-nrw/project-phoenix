package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	GroupSupervisorsVersion     = "1.4.3"
	GroupSupervisorsDescription = "Create active.group_supervisors table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[GroupSupervisorsVersion] = &Migration{
		Version:     GroupSupervisorsVersion,
		Description: GroupSupervisorsDescription,
		DependsOn:   []string{"1.4.1", "1.2.3"}, // Depends on active_groups and users_staff
	}

	// Migration 1.4.3: Create active.group_supervisors table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createGroupSupervisorsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropGroupSupervisorsTable(ctx, db)
		},
	)
}

// createGroupSupervisorsTable creates the active.group_supervisors table
func createGroupSupervisorsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.4.3: Creating active.group_supervisors table...")

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

	// Create the group_supervisors table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.group_supervisors (
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
		return fmt.Errorf("error creating active.group_supervisors table: %w", err)
	}

	// Create indexes to improve query performance
	_, err = tx.ExecContext(ctx, `
		-- Add indexes to speed up queries
		CREATE INDEX IF NOT EXISTS idx_supervision_staff_id ON active.group_supervisors(staff_id);
		CREATE INDEX IF NOT EXISTS idx_supervision_group_id ON active.group_supervisors(group_id);
		CREATE INDEX IF NOT EXISTS idx_supervision_role ON active.group_supervisors(role);
		CREATE INDEX IF NOT EXISTS idx_supervision_date_range ON active.group_supervisors(start_date, end_date);

		-- Index for finding active supervisions (where end_date is null)
		-- Note: We can't use CURRENT_DATE in index predicate as it's not IMMUTABLE
		CREATE INDEX IF NOT EXISTS idx_supervision_active ON active.group_supervisors(staff_id, group_id)
		WHERE (end_date IS NULL);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_supervisors table: %w", err)
	}

	// Create trigger for updating updated_at column
	_, err = tx.ExecContext(ctx, `
		-- Trigger for group_supervisors
		DROP TRIGGER IF EXISTS update_supervision_updated_at ON active.group_supervisors;
		CREATE TRIGGER update_supervision_updated_at
		BEFORE UPDATE ON active.group_supervisors
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for group_supervisors: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropGroupSupervisorsTable drops the active.group_supervisors table
func dropGroupSupervisorsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.4.3: Removing active.group_supervisors table...")

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
		DROP TRIGGER IF EXISTS update_supervision_updated_at ON active.group_supervisors;
	`)
	if err != nil {
		return fmt.Errorf("error dropping trigger for group_supervisors table: %w", err)
	}

	// Drop the table
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.group_supervisors CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping active.group_supervisors table: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
