package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	accountAnalyticsConsentVersion     = "1.15.527"
	accountAnalyticsConsentDescription = "Add the account's usage analytics consent to auth.accounts (#4901)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     accountAnalyticsConsentVersion,
		Description: accountAnalyticsConsentDescription,
		DependsOn:   []string{schulverwaltungRoleVersion},
	})
	Migrations.MustRegister(accountAnalyticsConsentUp, accountAnalyticsConsentDown)
}

// accountAnalyticsConsentUp stores whether the person behind an account
// allowed the usage analytics in the browser (§ 25 TDDDG, #4901). NULL means
// the account has not decided yet, which collects nothing, like a refusal.
// The decision belongs to the account, not to a school: accounts are
// platform-wide, and one person answers once for every portal and school.
// decided_at records when the answer was given, as evidence of the consent.
func accountAnalyticsConsentUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			ADD COLUMN IF NOT EXISTS analytics_consent BOOLEAN,
			ADD COLUMN IF NOT EXISTS analytics_consent_decided_at TIMESTAMPTZ;
	`); err != nil {
		return fmt.Errorf("add auth.accounts analytics consent: %w", err)
	}
	return nil
}

func accountAnalyticsConsentDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE auth.accounts
			DROP COLUMN IF EXISTS analytics_consent_decided_at,
			DROP COLUMN IF EXISTS analytics_consent;
	`); err != nil {
		return fmt.Errorf("drop auth.accounts analytics consent: %w", err)
	}
	return nil
}
