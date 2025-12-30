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

