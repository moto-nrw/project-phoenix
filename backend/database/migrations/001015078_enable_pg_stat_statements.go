package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enablePgStatStatementsVersion     = "1.15.78"
	enablePgStatStatementsDescription = "Enable pg_stat_statements for production query performance analysis"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enablePgStatStatementsVersion,
		Description: enablePgStatStatementsDescription,
		DependsOn: []string{
			InfrastructureVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enablePgStatStatementsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return enablePgStatStatementsDown(ctx, db)
		},
	)
}

func enablePgStatStatementsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.78: Enabling pg_stat_statements extension...")

	_, err := db.NewRaw(`CREATE EXTENSION IF NOT EXISTS pg_stat_statements;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed enabling pg_stat_statements extension: %w", err)
	}

	return nil
}

func enablePgStatStatementsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.78: Disabling pg_stat_statements extension...")

	_, err := db.NewRaw(`DROP EXTENSION IF EXISTS pg_stat_statements;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed disabling pg_stat_statements extension: %w", err)
	}

	return nil
}
