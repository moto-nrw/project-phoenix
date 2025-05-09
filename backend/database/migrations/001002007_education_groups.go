package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	EducationGroupsVersion     = "1.2.6"
	EducationGroupsDescription = "Create education.groups table"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[EducationGroupsVersion] = &Migration{
		Version:     EducationGroupsVersion,
		Description: EducationGroupsDescription,
		DependsOn:   []string{"1.1.1", "1.2.4"}, // Depends on facilities.rooms and users.teachers
	}

	// Migration 1.2.6: Create education.groups table
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createEducationGroupsTable(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropEducationGroupsTable(ctx, db)
		},
	)
}

// createEducationGroupsTable creates the education.groups table
func createEducationGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.2.6: Creating education.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Create the groups table - primary OGS groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.groups (
			id BIGSERIAL PRIMARY KEY,
			name TEXT NOT NULL UNIQUE,
			room_id BIGINT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_groups_room FOREIGN KEY (room_id) 
				REFERENCES facilities.rooms(id) ON DELETE SET NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating groups table: %w", err)
	}

	// Create indexes for groups
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_groups_name ON education.groups(name);
		CREATE INDEX IF NOT EXISTS idx_groups_room_id ON education.groups(room_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for groups table: %w", err)
	}

	// Create the group_teacher table - which specialists supervise which groups
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.group_teacher (
			id BIGSERIAL PRIMARY KEY,
			group_id BIGINT NOT NULL,
			teacher_id BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_group_teacher_group FOREIGN KEY (group_id) 
				REFERENCES education.groups(id) ON DELETE CASCADE,
			CONSTRAINT fk_group_teacher_teacher FOREIGN KEY (teacher_id) 
				REFERENCES users.teachers(id) ON DELETE CASCADE,
			CONSTRAINT uk_group_teacher UNIQUE (group_id, teacher_id)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating group_teacher table: %w", err)
	}

	// Create indexes for group_teacher
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_teacher_group_id ON education.group_teacher(group_id);
		CREATE INDEX IF NOT EXISTS idx_group_teacher_teacher_id ON education.group_teacher(teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for group_teacher table: %w", err)
	}

	// Create triggers for updating updated_at columns
	_, err = tx.ExecContext(ctx, `
		-- Trigger for groups
		CREATE TRIGGER update_groups_updated_at
		BEFORE UPDATE ON education.groups
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
		
		-- Trigger for group_teacher
		CREATE TRIGGER update_group_teacher_updated_at
		BEFORE UPDATE ON education.group_teacher
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropEducationGroupsTable drops the education.groups and related tables
func dropEducationGroupsTable(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.2.6: Removing education.groups table...")

	// Begin a transaction for atomicity
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Drop tables in reverse order of dependencies
	_, err = tx.ExecContext(ctx, `
		-- Drop triggers first
		DROP TRIGGER IF EXISTS update_groups_updated_at ON education.groups;
		DROP TRIGGER IF EXISTS update_group_teacher_updated_at ON education.group_teacher;
		
		-- Drop tables in order of dependencies
		DROP TABLE IF EXISTS education.group_teacher;
		DROP TABLE IF EXISTS education.groups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping education.groups tables: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
