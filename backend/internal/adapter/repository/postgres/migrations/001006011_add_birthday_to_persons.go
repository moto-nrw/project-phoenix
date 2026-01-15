package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	addBirthdayToPersonsVersion     = "1.6.11"
	addBirthdayToPersonsDescription = "Add birthday column to users.persons"
)

func init() {
	MigrationRegistry[addBirthdayToPersonsVersion] = &Migration{
		Version:     addBirthdayToPersonsVersion,
		Description: addBirthdayToPersonsDescription,
		DependsOn: []string{
			UsersPersonsVersion,
		},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addBirthdayToPersonsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return addBirthdayToPersonsDown(ctx, db)
		},
	)
}

func addBirthdayToPersonsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE users.persons
		ADD COLUMN IF NOT EXISTS birthday DATE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding birthday column to users.persons: %w", err)
	}
	return nil
}

func addBirthdayToPersonsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE users.persons
		DROP COLUMN IF EXISTS birthday;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping birthday column from users.persons: %w", err)
	}
	return nil
}
