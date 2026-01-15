package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

// Migration represents a database migration with metadata
type Migration struct {
	Version     string   // Semantic version of the migration
	Description string   // Human-readable description
	DependsOn   []string // Versions this migration depends on
	Up          func(ctx context.Context, db *bun.DB) error
	Down        func(ctx context.Context, db *bun.DB) error
}

// Note: Migration logging helpers are defined in main.go:
// - LogMigration(version, msg) - logs info messages with migration version
// - LogMigrationError(version, msg, err) - logs errors with version context
// - logRollbackError(err) - logs rollback errors (fire-and-forget)
