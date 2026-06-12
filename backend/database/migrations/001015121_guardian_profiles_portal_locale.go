package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	guardianProfilesPortalLocaleVersion     = "1.15.121"
	guardianProfilesPortalLocaleDescription = "Add nullable portal_locale to users.guardian_profiles. NULL = the parent has never picked a language in the parents portal, which lets the portal honour an anonymous (cookie/Accept-Language) choice on first login instead of snapping back to German. Deliberately separate from language_preference, which records the guardian's contact/spoken language for the school (fed by import) and is written 'de' on creation everywhere."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianProfilesPortalLocaleVersion,
		Description: guardianProfilesPortalLocaleDescription,
		DependsOn:   []string{UsersGuardianProfilesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addGuardianProfilesPortalLocale(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return dropGuardianProfilesPortalLocale(ctx, db)
		},
	)
}

func addGuardianProfilesPortalLocale(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.121: Adding portal_locale column to users.guardian_profiles...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Nullable, NO default on purpose: NULL is the meaningful "never chosen
	// in the portal" state. Existing rows stay NULL, so the first portal
	// login adopts the anonymous locale rather than forcing German. INSERT/
	// SELECT/UPDATE on the table already cover all columns for the tenant
	// roles, so no GRANT is needed.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles
			ADD COLUMN IF NOT EXISTS portal_locale TEXT;

		COMMENT ON COLUMN users.guardian_profiles.portal_locale IS
			'Parents-portal UI language explicitly chosen by the guardian (NULL = never chosen). Distinct from language_preference, which is the contact/spoken language for the school.';
	`)
	if err != nil {
		return fmt.Errorf("error adding users.guardian_profiles.portal_locale: %w", err)
	}
	fmt.Println("  ✓ users.guardian_profiles.portal_locale — column added")

	return tx.Commit()
}

func dropGuardianProfilesPortalLocale(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.121: Dropping users.guardian_profiles.portal_locale...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `ALTER TABLE users.guardian_profiles DROP COLUMN IF EXISTS portal_locale;`)
	if err != nil {
		return fmt.Errorf("error dropping users.guardian_profiles.portal_locale: %w", err)
	}

	return tx.Commit()
}
