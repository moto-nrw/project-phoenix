package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	GradeTransitionsVersion     = "1.7.6"
	GradeTransitionsDescription = "Create grade transitions tables for school year class changes"
)

func init() {
	MigrationRegistry[GradeTransitionsVersion] = &Migration{
		Version:     GradeTransitionsVersion,
		Description: GradeTransitionsDescription,
		DependsOn:   []string{"1.0.1", "1.3.5"}, // Depends on auth.accounts and users.students
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createGradeTransitionsTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropGradeTransitionsTables(ctx, db)
		},
	)
}

// createGradeTransitionsTables creates the grade transitions tables for bulk class changes
func createGradeTransitionsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.7.6: Creating grade transitions tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Create the main grade_transitions table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.grade_transitions (
			id BIGSERIAL PRIMARY KEY,
			academic_year VARCHAR(9) NOT NULL,                    -- e.g., "2025-2026"
			status VARCHAR(20) NOT NULL DEFAULT 'draft',          -- draft, applied, reverted
			applied_at TIMESTAMPTZ,
			applied_by BIGINT REFERENCES auth.accounts(id),
			reverted_at TIMESTAMPTZ,
			reverted_by BIGINT REFERENCES auth.accounts(id),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			created_by BIGINT NOT NULL REFERENCES auth.accounts(id),
			notes TEXT,
			metadata JSONB DEFAULT '{}',

			-- Ensure valid status values
			CONSTRAINT chk_grade_transition_status CHECK (status IN ('draft', 'applied', 'reverted'))
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating education.grade_transitions table: %w", err)
	}

	// Create indexes for grade_transitions
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_grade_transitions_academic_year
			ON education.grade_transitions(academic_year);
		CREATE INDEX IF NOT EXISTS idx_grade_transitions_status
			ON education.grade_transitions(status);
		CREATE INDEX IF NOT EXISTS idx_grade_transitions_created_at
			ON education.grade_transitions(created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for grade_transitions: %w", err)
	}

	// Create the grade_transition_mappings table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.grade_transition_mappings (
			id BIGSERIAL PRIMARY KEY,
			transition_id BIGINT NOT NULL REFERENCES education.grade_transitions(id) ON DELETE CASCADE,
			from_class VARCHAR(50) NOT NULL,
			to_class VARCHAR(50),                                 -- NULL = graduates (delete)
			UNIQUE(transition_id, from_class)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating education.grade_transition_mappings table: %w", err)
	}

	// Create index for mappings
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_grade_transition_mappings_transition
			ON education.grade_transition_mappings(transition_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for grade_transition_mappings: %w", err)
	}

	// Create the grade_transition_history table for audit trail
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.grade_transition_history (
			id BIGSERIAL PRIMARY KEY,
			transition_id BIGINT NOT NULL REFERENCES education.grade_transitions(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL,                           -- Keep even if student deleted
			person_name VARCHAR(255) NOT NULL,                    -- Snapshot for audit trail
			from_class VARCHAR(50) NOT NULL,
			to_class VARCHAR(50),                                 -- NULL = graduated/deleted
			action VARCHAR(20) NOT NULL,                          -- 'promoted', 'graduated', 'unchanged'
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Ensure valid action values
			CONSTRAINT chk_grade_transition_action CHECK (action IN ('promoted', 'graduated', 'unchanged'))
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating education.grade_transition_history table: %w", err)
	}

	// Create indexes for history
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_grade_transition_history_transition
			ON education.grade_transition_history(transition_id);
		CREATE INDEX IF NOT EXISTS idx_grade_transition_history_student
			ON education.grade_transition_history(student_id);
		CREATE INDEX IF NOT EXISTS idx_grade_transition_history_action
			ON education.grade_transition_history(action);
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for grade_transition_history: %w", err)
	}

	// Create trigger for updated_at on grade_transitions
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_grade_transitions_updated_at ON education.grade_transitions;
	`)
	if err != nil {
		return fmt.Errorf("error dropping existing trigger: %w", err)
	}

	return tx.Commit()
}

// dropGradeTransitionsTables drops all grade transitions tables
func dropGradeTransitionsTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.7.6: Removing grade transitions tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop tables in reverse order (due to foreign key constraints)
	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.grade_transition_history CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping grade_transition_history table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.grade_transition_mappings CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping grade_transition_mappings table: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.grade_transitions CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping grade_transitions table: %w", err)
	}

	return tx.Commit()
}
