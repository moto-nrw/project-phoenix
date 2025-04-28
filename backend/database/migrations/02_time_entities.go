package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	// Second migration: Time-related tables
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 2: Creating time-related tables...")

			// Create Timespan table
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS timespans (
					id SERIAL PRIMARY KEY,
					starttime TIME NOT NULL,
					endtime TIME,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)
			if err != nil {
				return err
			}

			// Create Datespan table
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS datespans (
					id SERIAL PRIMARY KEY,
					startdate TIMESTAMPTZ NOT NULL,
					enddate TIMESTAMPTZ NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
				)
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 2: Dropping time-related tables...")

			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS datespans CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS timespans CASCADE`)
			return err
		},
	)
}
