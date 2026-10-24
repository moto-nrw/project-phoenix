package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const dropEnrollmentCaptchaKeySettingsVersion = "1.15.470"

// dropEnrollmentCaptchaKeySettingsKeys are the retired per-school Turnstile
// keys. The server now reads TURNSTILE_SITE_KEY and TURNSTILE_SECRET_KEY:
// one widget serves every school.
var dropEnrollmentCaptchaKeySettingsKeys = []string{
	"enrollment.captcha_site_key",
	"enrollment.captcha_secret_key",
}

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dropEnrollmentCaptchaKeySettingsVersion,
		Description: "Delete config rows of the retired per-school Turnstile key settings",
		DependsOn: []string{
			configSettingValuesVersion, // config.setting_values + config.setting_audit
			"1.15.469",                 // latest migration at authoring time
		},
	})
	Migrations.MustRegister(dropEnrollmentCaptchaKeySettingsUp, dropEnrollmentCaptchaKeySettingsDown)
}

// dropEnrollmentCaptchaKeySettingsUp deletes the tenant overrides and their
// audit rows. The superuser connection bypasses RLS on both tables.
func dropEnrollmentCaptchaKeySettingsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx,
		`DELETE FROM config.setting_values WHERE setting_key IN (?)`,
		bun.List(dropEnrollmentCaptchaKeySettingsKeys),
	); err != nil {
		return fmt.Errorf("delete setting_values: %w", err)
	}
	if _, err := db.ExecContext(ctx,
		`DELETE FROM config.setting_audit WHERE setting_key IN (?)`,
		bun.List(dropEnrollmentCaptchaKeySettingsKeys),
	); err != nil {
		return fmt.Errorf("delete setting_audit: %w", err)
	}
	return nil
}

// dropEnrollmentCaptchaKeySettingsDown is a no-op: the registry definitions
// are gone, so there is nothing meaningful to restore.
func dropEnrollmentCaptchaKeySettingsDown(context.Context, *bun.DB) error {
	return nil
}
