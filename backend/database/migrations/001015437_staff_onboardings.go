package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffOnboardingsVersion     = "1.15.437"
	staffOnboardingsDescription = "Create config.staff_onboardings - first steps of care workers (#3748)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffOnboardingsVersion,
		Description: staffOnboardingsDescription,
		DependsOn: []string{
			schoolSetupsVersion,    // the school wizard this extends (ADR 0043)
			settingsCleanupVersion, // previous head
		},
	})

	Migrations.MustRegister(staffOnboardingsUp, staffOnboardingsDown)
}

// staffOnboardingsUp creates the progress behind the first steps of care
// workers (#3748, ADR 0043 addendum): one row per (school, account) with the
// steps the person finished or skipped, and whether the person hid the
// checklist. Unlike the school wizard, a step counts as done once the person
// finished its tour, so progress is stored rather than derived.
//
// A person WITHOUT a row has not started.
//
// PEOPLE WHO ALREADY WORK WITH MOTO ARE NOT ONBOARDED. The backfill gives every
// account that is active at a school at migration time a dismissed row, so only
// people who join afterwards (including invitations still pending now) see the
// checklist. It runs before RLS is forced, because the migration role has no
// tenant set and FORCE ROW LEVEL SECURITY would reject the insert.
//
// done_steps and skipped_steps are deliberately NOT constrained by a CHECK: the
// step catalogue lives in the application, and a key no step uses any more is
// inert.
//
// THE TABLE IS NOT A PERMISSION BOUNDARY. Every page a tour leads to enforces
// its own permissions.
func staffOnboardingsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil && rollbackErr != sql.ErrTxDone {
			logRollbackFailure(ctx, rollbackErr)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.staff_onboardings (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT      NOT NULL,
			account_id    BIGINT      NOT NULL,
			done_steps    TEXT[]      NOT NULL DEFAULT '{}',
			skipped_steps TEXT[]      NOT NULL DEFAULT '{}',
			dismissed_at  TIMESTAMPTZ,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_staff_onboardings_tenant_account
				UNIQUE (tenant_id, account_id),
			CONSTRAINT fk_staff_onboardings_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_onboardings_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		INSERT INTO config.staff_onboardings (tenant_id, account_id, dismissed_at)
		SELECT tenant_id, account_id, NOW()
		FROM auth.account_tenants
		WHERE status = 'active'
		ON CONFLICT (tenant_id, account_id) DO NOTHING;

		DROP TRIGGER IF EXISTS update_staff_onboardings_updated_at ON config.staff_onboardings;
		CREATE TRIGGER update_staff_onboardings_updated_at
		BEFORE UPDATE ON config.staff_onboardings
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE config.staff_onboardings ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.staff_onboardings FORCE  ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_config_staff_onboardings ON config.staff_onboardings;
		CREATE POLICY tenant_isolation_config_staff_onboardings
			ON config.staff_onboardings
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON config.staff_onboardings TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.staff_onboardings_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating staff onboarding table: %w", err)
	}

	return tx.Commit()
}

// staffOnboardingsDown drops the table. Without it every person counts as new
// again, so a later re-run of the up migration restores the backfill.
func staffOnboardingsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `DROP TABLE IF EXISTS config.staff_onboardings;`)
	if err != nil {
		return fmt.Errorf("error dropping staff onboarding table: %w", err)
	}
	return nil
}
