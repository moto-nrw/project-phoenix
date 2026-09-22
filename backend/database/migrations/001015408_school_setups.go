package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolSetupsVersion     = "1.15.408"
	schoolSetupsDescription = "Create config.school_setups and config.school_setup_dismissals - onboarding wizard state (#2832)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolSetupsVersion,
		Description: schoolSetupsDescription,
		DependsOn:   []string{"1.15.406", "1.15.407"},
	})

	Migrations.MustRegister(schoolSetupsUp, schoolSetupsDown)
}

// schoolSetupsUp creates the state behind the onboarding wizard (#2832,
// ADR 0035). Progress itself is not stored: the school-setup-view projection
// derives it from rooms, invitations, groups, students and guardian
// invitations on every read.
//
// config.school_setups holds one row per school: the answers from the first
// step that are not settings (parent app), the steps the school skipped and
// when setup was completed. A school WITHOUT a row is new and has not started.
//
// config.school_setup_dismissals holds one row per (school, account): the
// person hid the wizard. Hiding is personal; progress belongs to the school.
//
// EXISTING SCHOOLS ARE NOT ONBOARDED. The backfill gives every school that
// exists at migration time a completed row, so only schools created afterwards
// see the wizard. It runs before RLS is forced, because the migration role has
// no tenant set and FORCE ROW LEVEL SECURITY would reject the insert.
//
// skipped_steps is deliberately NOT constrained by a CHECK: the step catalogue
// lives in the application, and a key no step uses any more is inert.
//
// NEITHER TABLE IS A PERMISSION BOUNDARY. Every page a step leads to enforces
// its own permissions.
func schoolSetupsUp(ctx context.Context, db *bun.DB) error {
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
		CREATE TABLE IF NOT EXISTS config.school_setups (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT      NOT NULL,
			parent_app_used     BOOLEAN,
			skipped_steps       TEXT[]      NOT NULL DEFAULT '{}',
			basics_confirmed_at TIMESTAMPTZ,
			completed_at        TIMESTAMPTZ,
			updated_by          BIGINT,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_school_setups_tenant
				UNIQUE (tenant_id),
			CONSTRAINT fk_school_setups_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_school_setups_updated_by
				FOREIGN KEY (updated_by) REFERENCES auth.accounts(id) ON DELETE SET NULL
		);

		CREATE TABLE IF NOT EXISTS config.school_setup_dismissals (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT      NOT NULL,
			account_id   BIGINT      NOT NULL,
			dismissed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_school_setup_dismissals_tenant_account
				UNIQUE (tenant_id, account_id),
			CONSTRAINT fk_school_setup_dismissals_tenant
				FOREIGN KEY (tenant_id) REFERENCES platform.schools(id) ON DELETE CASCADE,
			CONSTRAINT fk_school_setup_dismissals_account
				FOREIGN KEY (account_id) REFERENCES auth.accounts(id) ON DELETE CASCADE
		);

		INSERT INTO config.school_setups (tenant_id, basics_confirmed_at, completed_at)
		SELECT id, NOW(), NOW() FROM platform.schools
		ON CONFLICT (tenant_id) DO NOTHING;

		DROP TRIGGER IF EXISTS update_school_setups_updated_at ON config.school_setups;
		CREATE TRIGGER update_school_setups_updated_at
		BEFORE UPDATE ON config.school_setups
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		ALTER TABLE config.school_setups ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.school_setups FORCE  ROW LEVEL SECURITY;
		ALTER TABLE config.school_setup_dismissals ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.school_setup_dismissals FORCE  ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_config_school_setups ON config.school_setups;
		CREATE POLICY tenant_isolation_config_school_setups
			ON config.school_setups
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		DROP POLICY IF EXISTS tenant_isolation_config_school_setup_dismissals ON config.school_setup_dismissals;
		CREATE POLICY tenant_isolation_config_school_setup_dismissals
			ON config.school_setup_dismissals
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON config.school_setups TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.school_setups_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON config.school_setup_dismissals TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.school_setup_dismissals_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating school setup tables: %w", err)
	}

	return tx.Commit()
}

// schoolSetupsDown drops both tables. Without them every school counts as new
// again, so a later re-run of the up migration restores the backfill.
func schoolSetupsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS config.school_setup_dismissals;
		DROP TABLE IF EXISTS config.school_setups;
	`)
	if err != nil {
		return fmt.Errorf("error dropping school setup tables: %w", err)
	}
	return nil
}
