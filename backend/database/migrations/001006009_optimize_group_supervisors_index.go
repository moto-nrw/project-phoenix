package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	OptimizeGroupSupervisorsIndexVersion     = "1.6.9"
	OptimizeGroupSupervisorsIndexDescription = "Add composite index on active.group_supervisors (group_id, end_date) for query optimization"
)

func init() {
	// Register migration with explicit version
	MigrationRegistry[OptimizeGroupSupervisorsIndexVersion] = &Migration{
		Version:     OptimizeGroupSupervisorsIndexVersion,
		Description: OptimizeGroupSupervisorsIndexDescription,
		DependsOn:   []string{"1.4.3"}, // Depends on active.group_supervisors table creation
	}

	// Migration 1.6.9: Add composite index for query optimization
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addGroupSupervisorsCompositeIndex(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropGroupSupervisorsCompositeIndex(ctx, db)
		},
	)
}

// addGroupSupervisorsCompositeIndex adds a composite index on (group_id, end_date)
func addGroupSupervisorsCompositeIndex(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.6.9: Adding composite index on active.group_supervisors (group_id, end_date)...")

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

	// Create composite index to optimize LEFT JOIN queries filtering by group_id and end_date
	// This index improves performance for queries like:
	// LEFT JOIN active.group_supervisors AS "sup" ON "sup"."group_id" = "group"."id"
	//   AND ("sup"."end_date" IS NULL OR "sup"."end_date" > CURRENT_DATE)
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_group_supervisors_group_end
		ON active.group_supervisors(group_id, end_date);
	`)
	if err != nil {
		return fmt.Errorf("error creating composite index on group_supervisors: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}

// dropGroupSupervisorsCompositeIndex drops the composite index
func dropGroupSupervisorsCompositeIndex(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.6.9: Dropping composite index on active.group_supervisors...")

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

	// Drop the composite index
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_group_supervisors_group_end;
	`)
	if err != nil {
		return fmt.Errorf("error dropping composite index on group_supervisors: %w", err)
	}

	// Commit the transaction
	return tx.Commit()
}
