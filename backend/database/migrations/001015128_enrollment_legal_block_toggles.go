package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	enrollmentLegalBlockTogglesVersion     = "1.15.128"
	enrollmentLegalBlockTogglesDescription = "Add explicit settings toggles for enrollment legal blocks"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     enrollmentLegalBlockTogglesVersion,
		Description: enrollmentLegalBlockTogglesDescription,
		DependsOn:   []string{formSchemaLegalBlocksActivateTextVersion, configSettingValuesVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enrollmentLegalBlockTogglesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return enrollmentLegalBlockTogglesDown(ctx, db)
		},
	)
}

func enrollmentLegalBlockTogglesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.128: Backfilling enrollment legal block toggles...")

	if _, err := db.NewRaw(`
		INSERT INTO config.setting_values (tenant_id, setting_key, value)
		SELECT tenant_id, target_key, 'true'::jsonb
		FROM (
			SELECT tenant_id, 'enrollment.legal_dsgvo_enabled' AS target_key, value
			FROM config.setting_values
			WHERE setting_key = 'enrollment.legal_dsgvo_text'

			UNION ALL

			SELECT tenant_id, 'enrollment.legal_photo_enabled' AS target_key, value
			FROM config.setting_values
			WHERE setting_key = 'enrollment.legal_photo_text'

			UNION ALL

			SELECT tenant_id, 'enrollment.legal_email_contact_enabled' AS target_key, value
			FROM config.setting_values
			WHERE setting_key = 'enrollment.legal_email_contact_text'
		) AS source
		WHERE jsonb_typeof(value) = 'string'
			AND btrim(COALESCE(value #>> '{}', '')) <> ''
		ON CONFLICT (tenant_id, setting_key) DO NOTHING;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed backfilling enrollment legal block toggles: %w", err)
	}

	return nil
}

func enrollmentLegalBlockTogglesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.128: Removing enrollment legal block toggles...")

	if _, err := db.NewRaw(`
		DELETE FROM config.setting_values
		WHERE setting_key IN (
			'enrollment.legal_dsgvo_enabled',
			'enrollment.legal_photo_enabled',
			'enrollment.legal_email_contact_enabled'
		);

		DELETE FROM config.setting_audit
		WHERE setting_key IN (
			'enrollment.legal_dsgvo_enabled',
			'enrollment.legal_photo_enabled',
			'enrollment.legal_email_contact_enabled'
		);
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed removing enrollment legal block toggles: %w", err)
	}

	return nil
}
