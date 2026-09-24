package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	staffTargetOverridesVersion     = "1.15.419"
	staffTargetOverridesDescription = "Create config.staff_target_overrides - per-staff Sonderarbeitszeit ranges with one daily target (#3259)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     staffTargetOverridesVersion,
		Description: staffTargetOverridesDescription,
		DependsOn:   []string{"1.15.418"}, // preserves ladder order
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return staffTargetOverridesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return staffTargetOverridesDown(ctx, db)
		},
	)
}

// staffTargetOverridesUp creates the store for Sonderarbeitszeiten (#3259): a
// staff member works a different daily target for a date range, typically
// holiday care inside a closure period. One row is one inclusive range with
// one daily target that applies Monday to Friday; statutory holidays keep a
// target of zero. Overlapping ranges of the same staff member are rejected by
// the Workforce application under the staff balance lock, like the schedule
// versions, so no btree_gist EXCLUDE constraint is needed here.
func staffTargetOverridesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS config.staff_target_overrides (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id      BIGINT NOT NULL,
			start_date    DATE NOT NULL,
			end_date      DATE NOT NULL,
			daily_minutes INTEGER NOT NULL,
			created_by    BIGINT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT staff_target_overrides_range_ok CHECK (start_date <= end_date),
			CONSTRAINT staff_target_overrides_minutes_ok CHECK (daily_minutes BETWEEN 0 AND 720),
			CONSTRAINT fk_staff_target_overrides_staff
				FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_staff_target_overrides_created_by
				FOREIGN KEY (tenant_id, created_by)
				REFERENCES users.staff_school_memberships(tenant_id, id) ON DELETE SET NULL (created_by)
		);

		CREATE INDEX IF NOT EXISTS idx_staff_target_overrides_staff_start
			ON config.staff_target_overrides (staff_id, start_date);
		CREATE INDEX IF NOT EXISTS idx_staff_target_overrides_tenant
			ON config.staff_target_overrides (tenant_id);

		DROP TRIGGER IF EXISTS update_staff_target_overrides_updated_at ON config.staff_target_overrides;
		CREATE TRIGGER update_staff_target_overrides_updated_at
		BEFORE UPDATE ON config.staff_target_overrides
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE config.staff_target_overrides ENABLE ROW LEVEL SECURITY;
		ALTER TABLE config.staff_target_overrides FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_config_staff_target_overrides ON config.staff_target_overrides;
		CREATE POLICY tenant_isolation_config_staff_target_overrides ON config.staff_target_overrides
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON config.staff_target_overrides TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE config.staff_target_overrides_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating config.staff_target_overrides: %w", err)
	}

	return tx.Commit()
}

func staffTargetOverridesDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_staff_target_overrides_updated_at ON config.staff_target_overrides;
		DROP TABLE IF EXISTS config.staff_target_overrides CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping config.staff_target_overrides: %w", err)
	}
	return tx.Commit()
}
