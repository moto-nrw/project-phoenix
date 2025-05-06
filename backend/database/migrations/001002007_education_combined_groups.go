package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	EducationCombinedGroupsVersion     = "1.2.7"
	EducationCombinedGroupsDescription = "Create education.combined_groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[EducationCombinedGroupsVersion] = &Migration{
		Version:     EducationCombinedGroupsVersion,
		Description: EducationCombinedGroupsDescription,
		DependsOn:   []string{"1.2.6"}, // Depends on education.groups
	}

	// Migration 1.2.7: Create education.combined_groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEducationCombinedGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropEducationCombinedGroupsTable(ctx, db)
		},
	)
}

// createEducationCombinedGroupsTable creates the education.combined_groups table
func createEducationCombinedGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.7: Creating education.combined_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the combined_groups table - special groupings
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			is_active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			valid_until TIMESTAMPTZ,
			access_policy TEXT NOT NULL DEFAULT 'all',
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_groups table: %w", err)
	}

	// Create indexes for combined_groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_groups_name ON education.combined_groups(name);
		CREATE INDEX IF NOT EXISTS idx_combined_groups_is_active ON education.combined_groups(is_active);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_groups table: %w", err)
	}

	// Create the combined_group_members table - which base groups form a combined group
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_group_members (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			group_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_combined_group_members_combined_group FOREIGN KEY (combined_group_id) 
				REFERENCES education.combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_members_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT uk_combined_group_members UNIQUE (combined_group_id, group_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_members table: %w", err)
	}

	// Create indexes for combined_group_members
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_members_combined_group_id ON education.combined_group_members(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_members_group_id ON education.combined_group_members(group_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_members table: %w", err)
	}

	// Create the combined_group_teacher table - teachers for combined groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.combined_group_teacher (
			id BIGSERIAL PRIMARY KEY,
			combined_group_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_combined_group_teacher_combined_group FOREIGN KEY (combined_group_id) 
				REFERENCES education.combined_groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_combined_group_teacher_teacher FOREIGN KEY (teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE,
			CONSTRAINT uk_combined_group_teacher UNIQUE (combined_group_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating combined_group_teacher table: %w", err)
	}

	// Create indexes for combined_group_teacher
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_combined_group_teacher_combined_group_id ON education.combined_group_teacher(combined_group_id);
		CREATE INDEX IF NOT EXISTS idx_combined_group_teacher_teacher_id ON education.combined_group_teacher(teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for combined_group_teacher table: %w", err)
	}

	// Create triggers for updating updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Trigger for combined_groups
		CREATE TRIGGER update_combined_groups_updated_at
		BEFORE UPDATE ON education.combined_groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropEducationCombinedGroupsTable drops the education.combined_groups and related tables
func dropEducationCombinedGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.7: Removing education.combined_groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		-- Drop triggers first
		DROP TRIGGER IF EXISTS update_combined_groups_updated_at ON education.combined_groups;
		
		-- Drop tables in order of dependencies
		DROP TABLE IF EXISTS education.combined_group_teacher;
		DROP TABLE IF EXISTS education.combined_group_members;
		DROP TABLE IF EXISTS education.combined_groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping education.combined_groups tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
