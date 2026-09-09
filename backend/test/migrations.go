package test

import "github.com/uptrace/bun/migrate"

// NewMigrator lets migration tests exercise registered up/down
// functions without depending directly on the ORM implementation.
func NewMigrator(db *DB, registry *migrate.Migrations) *migrate.Migrator {
	return migrate.NewMigrator(db, registry, migrate.WithMarkAppliedOnSuccess(true))
}
