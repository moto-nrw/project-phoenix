package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// These are sequential migrations
// Numbers are explicit to ensure order dependency is maintained

func init() {
	// First migration: Basic tables (core, accounts)
	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1: Creating basic tables...")

			// Create accounts table
			_, err := db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS accounts (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					last_login TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					email TEXT NOT NULL UNIQUE,
					username TEXT UNIQUE,
					name TEXT NOT NULL,
					active BOOLEAN NOT NULL DEFAULT TRUE,
					password_hash TEXT,
					roles TEXT[] NOT NULL DEFAULT '{user}'
				)
			`)
			if err != nil {
				return err
			}

			// Create tokens table for refresh tokens
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS tokens (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id INTEGER NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
					token TEXT NOT NULL UNIQUE,
					expiry TIMESTAMPTZ NOT NULL,
					mobile BOOLEAN NOT NULL DEFAULT FALSE,
					identifier TEXT
				)
			`)
			if err != nil {
				return err
			}

			// Create profiles table for user profiles
			_, err = db.ExecContext(ctx, `
				CREATE TABLE IF NOT EXISTS profiles (
					id SERIAL PRIMARY KEY,
					created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
					account_id INTEGER NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,
					avatar TEXT,
					bio TEXT,
					settings JSONB
				)
			`)

			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1: Dropping basic tables...")

			_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS profiles CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS tokens CASCADE`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `DROP TABLE IF EXISTS accounts CASCADE`)
			return err
		},
	)
}
