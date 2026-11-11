package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.946",
		Description: "Persist the parents-portal language for accounts without a school profile",
		DependsOn:   []string{holidayRegistrationParentAccountVersion},
	})
	Migrations.MustRegister(accountPortalLocaleUp, accountPortalLocaleDown)
}

func accountPortalLocaleUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.accounts ADD COLUMN portal_locale TEXT;
 COMMENT ON COLUMN auth.accounts.portal_locale IS 'Explicit parents-portal language for an account without a guardian profile; NULL means never chosen.';`)
	if err != nil {
		return fmt.Errorf("add account portal locale: %w", err)
	}
	return nil
}

func accountPortalLocaleDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.accounts DROP COLUMN portal_locale`)
	if err != nil {
		return fmt.Errorf("drop account portal locale: %w", err)
	}
	return nil
}
