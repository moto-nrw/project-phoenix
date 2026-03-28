package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	compositePKIndexesVersion     = "1.14.4"
	compositePKIndexesDescription = "Create UNIQUE(tenant_id, id) indexes on 18 target tables for future composite FK support"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     compositePKIndexesVersion,
		Description: compositePKIndexesDescription,
		DependsOn:   []string{"1.14.2"}, // tenant_id columns exist
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createCompositePKIndexes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackCompositePKIndexes(ctx, db)
		},
	)
}

// compositePKTables lists the 18 tables that need UNIQUE(tenant_id, id) indexes.
// These are target tables referenced by foreign keys from other tenant-scoped tables.
// The composite unique index enables composite FKs (tenant_id, referenced_id) in Phase 4.
var compositePKTables = []struct {
	schema string
	table  string
}{
	{"users", "persons"},
	{"users", "staff"},
	{"users", "students"},
	{"users", "teachers"},
	{"users", "guardian_profiles"},
	{"users", "rfid_cards"},
	{"education", "groups"},
	{"education", "grade_transitions"},
	{"facilities", "rooms"},
	{"activities", "categories"},
	{"activities", "groups"},
	{"active", "groups"},
	{"active", "combined_groups"},
	{"active", "work_sessions"},
	{"iot", "devices"},
	{"schedule", "timeframes"},
	{"suggestions", "posts"},
	{"auth", "accounts_parents"},
}

func createCompositePKIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.4: Creating UNIQUE(tenant_id, id) indexes on target tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, t := range compositePKTables {
		indexName := fmt.Sprintf("idx_%s_tenant_pk", t.table)
		fullTable := fmt.Sprintf("%s.%s", t.schema, t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			CREATE UNIQUE INDEX IF NOT EXISTS %s ON %s(tenant_id, id);
		`, indexName, fullTable))
		if err != nil {
			return fmt.Errorf("error creating composite PK index on %s: %w", fullTable, err)
		}
	}

	fmt.Println("Migration 1.14.4: Successfully created 18 composite PK indexes")
	return tx.Commit()
}

func rollbackCompositePKIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.4: Dropping composite PK indexes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, t := range compositePKTables {
		indexName := fmt.Sprintf("idx_%s_tenant_pk", t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DROP INDEX IF EXISTS %s.%s;
		`, t.schema, indexName))
		if err != nil {
			return fmt.Errorf("error dropping composite PK index on %s.%s: %w", t.schema, t.table, err)
		}
	}

	fmt.Println("Migration 1.14.4: Successfully dropped composite PK indexes")
	return tx.Commit()
}
