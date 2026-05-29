package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentsPerChildConsentsVersion     = "1.15.75"
	studentsPerChildConsentsDescription = "Add per-student consent timestamps (AGB, Datenschutz, E-Mail-Kontakt) alongside the existing photo_consent_given_at, so staff can see consent state by looking at a single student row instead of joining back to the parent's enrollment.requests submission."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentsPerChildConsentsVersion,
		Description: studentsPerChildConsentsDescription,
		DependsOn: []string{
			// Original users.students migration that introduced
			// photo_consent_given_at — the new columns mirror its
			// shape so admin queries can scan all four consents in
			// one read.
			"1.3.5",
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.75: Adding per-student consent columns to users.students...")
			if _, err := db.NewRaw(`
				ALTER TABLE users.students
				ADD COLUMN IF NOT EXISTS agb_accepted_at TIMESTAMPTZ,
				ADD COLUMN IF NOT EXISTS data_processing_accepted_at TIMESTAMPTZ,
				ADD COLUMN IF NOT EXISTS email_contact_accepted_at TIMESTAMPTZ;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed adding consent columns: %w", err)
			}
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.75...")
			if _, err := db.NewRaw(`
				ALTER TABLE users.students
				DROP COLUMN IF EXISTS agb_accepted_at,
				DROP COLUMN IF EXISTS data_processing_accepted_at,
				DROP COLUMN IF EXISTS email_contact_accepted_at;
			`).Exec(ctx); err != nil {
				return fmt.Errorf("failed dropping consent columns: %w", err)
			}
			return nil
		},
	)
}
