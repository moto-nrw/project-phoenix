package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	activePathPartialIndexesVersion     = "1.12.8"
	activePathPartialIndexesDescription = "Add partial indexes for active visits and active groups hot paths"
)

func init() {
	MigrationRegistry[activePathPartialIndexesVersion] = &Migration{
		Version:     activePathPartialIndexesVersion,
		Description: activePathPartialIndexesDescription,
		DependsOn:   []string{"1.12.7"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addActivePathPartialIndexes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackActivePathPartialIndexes(ctx, db)
		},
	)
}

func addActivePathPartialIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.12.8: Adding hot-path partial indexes concurrently...")

	statements := []string{
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_active_visits_student_active_only
			ON active.visits(student_id)
			WHERE exit_time IS NULL`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_active_visits_group_active_only
			ON active.visits(active_group_id)
			WHERE exit_time IS NULL`,
		`CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_active_groups_room_active_only
			ON active.groups(room_id)
			WHERE end_time IS NULL`,
	}

	if err := execConcurrentIndexStatements(ctx, db, statements); err != nil {
		return fmt.Errorf("error creating hot-path partial indexes: %w", err)
	}

	fmt.Println("Migration 1.12.8 completed: hot-path partial indexes created")
	return nil
}

func rollbackActivePathPartialIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.12.8: Dropping hot-path partial indexes concurrently...")

	statements := []string{
		`DROP INDEX CONCURRENTLY IF EXISTS active.idx_active_visits_student_active_only`,
		`DROP INDEX CONCURRENTLY IF EXISTS active.idx_active_visits_group_active_only`,
		`DROP INDEX CONCURRENTLY IF EXISTS active.idx_active_groups_room_active_only`,
	}

	if err := execConcurrentIndexStatements(ctx, db, statements); err != nil {
		return fmt.Errorf("error dropping hot-path partial indexes: %w", err)
	}

	fmt.Println("Rollback 1.12.8 completed: hot-path partial indexes dropped")
	return nil
}

func execConcurrentIndexStatements(ctx context.Context, db *bun.DB, statements []string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("failed to acquire dedicated connection: %w", err)
	}
	defer func() { _ = conn.Close() }()

	for _, stmt := range statements {
		if _, err := conn.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
