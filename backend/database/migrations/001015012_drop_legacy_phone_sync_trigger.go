package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	dropLegacyPhoneSyncVersion     = "1.15.12"
	dropLegacyPhoneSyncDescription = "Drop legacy sync_phone_to_legacy trigger and guardian_profiles phone/mobile_phone columns"
)

func init() {
	MigrationRegistry[dropLegacyPhoneSyncVersion] = &Migration{
		Version:     dropLegacyPhoneSyncVersion,
		Description: dropLegacyPhoneSyncDescription,
		DependsOn:   []string{"1.15.11"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return dropLegacyPhoneSync(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackDropLegacyPhoneSync(ctx, db)
		},
	)
}

func dropLegacyPhoneSync(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.12: Dropping legacy phone sync trigger and columns...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Drop the trigger that syncs phone numbers back to legacy columns.
	// This trigger causes RLS permission errors under multi-tenancy because it
	// does a cross-table UPDATE (guardian_phone_numbers → guardian_profiles)
	// as SECURITY INVOKER under the phoenix_tenant role.
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS sync_phone_to_legacy ON users.guardian_phone_numbers;
		DROP FUNCTION IF EXISTS sync_guardian_phone_to_legacy();
	`)
	if err != nil {
		return fmt.Errorf("failed to drop sync trigger: %w", err)
	}
	fmt.Println("  Dropped trigger sync_phone_to_legacy and function sync_guardian_phone_to_legacy")

	// 2. Drop the legacy columns that nobody reads anymore.
	// Phone numbers are stored in users.guardian_phone_numbers since migration 1.7.6.
	// The Go model GuardianProfile does not have phone/mobile_phone fields.
	// No backend service, API handler, or frontend code reads these columns.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles DROP COLUMN IF EXISTS phone;
		ALTER TABLE users.guardian_profiles DROP COLUMN IF EXISTS mobile_phone;
	`)
	if err != nil {
		return fmt.Errorf("failed to drop legacy columns: %w", err)
	}
	fmt.Println("  Dropped legacy columns phone and mobile_phone from guardian_profiles")

	fmt.Println("Migration 1.15.12: Done")
	return tx.Commit()
}

func rollbackDropLegacyPhoneSync(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.12: Restoring legacy phone columns and sync trigger...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Re-add legacy columns
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles ADD COLUMN IF NOT EXISTS phone TEXT;
		ALTER TABLE users.guardian_profiles ADD COLUMN IF NOT EXISTS mobile_phone TEXT;
	`)
	if err != nil {
		return fmt.Errorf("failed to re-add legacy columns: %w", err)
	}

	// Re-create the sync function and trigger
	_, err = tx.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION sync_guardian_phone_to_legacy()
		RETURNS TRIGGER AS $$
		DECLARE
			primary_home TEXT;
			primary_mobile TEXT;
		BEGIN
			SELECT phone_number INTO primary_home
			FROM users.guardian_phone_numbers
			WHERE guardian_profile_id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id)
				AND phone_type = 'home'
			ORDER BY is_primary DESC, priority ASC
			LIMIT 1;

			SELECT phone_number INTO primary_mobile
			FROM users.guardian_phone_numbers
			WHERE guardian_profile_id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id)
				AND phone_type = 'mobile'
			ORDER BY is_primary DESC, priority ASC
			LIMIT 1;

			UPDATE users.guardian_profiles
			SET phone = primary_home,
				mobile_phone = primary_mobile
			WHERE id = COALESCE(NEW.guardian_profile_id, OLD.guardian_profile_id);

			RETURN COALESCE(NEW, OLD);
		END;
		$$ LANGUAGE plpgsql;

		CREATE TRIGGER sync_phone_to_legacy
		AFTER INSERT OR UPDATE OR DELETE ON users.guardian_phone_numbers
		FOR EACH ROW EXECUTE FUNCTION sync_guardian_phone_to_legacy();
	`)
	if err != nil {
		return fmt.Errorf("failed to re-create sync trigger: %w", err)
	}

	// Backfill legacy columns from guardian_phone_numbers for all existing guardians
	_, err = tx.ExecContext(ctx, `
		UPDATE users.guardian_profiles gp
		SET phone = (
			SELECT gpn.phone_number
			FROM users.guardian_phone_numbers gpn
			WHERE gpn.guardian_profile_id = gp.id AND gpn.phone_type = 'home'
			ORDER BY gpn.is_primary DESC, gpn.priority ASC
			LIMIT 1
		),
		mobile_phone = (
			SELECT gpn.phone_number
			FROM users.guardian_phone_numbers gpn
			WHERE gpn.guardian_profile_id = gp.id AND gpn.phone_type = 'mobile'
			ORDER BY gpn.is_primary DESC, gpn.priority ASC
			LIMIT 1
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to backfill legacy phone columns: %w", err)
	}

	return tx.Commit()
}
