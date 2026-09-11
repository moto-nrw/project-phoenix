package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	customLeaveAllowancesVersion     = "1.15.356"
	customLeaveAllowancesDescription = "Add yearly allowances for school-defined staff absence types (#2874)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version: customLeaveAllowancesVersion, Description: customLeaveAllowancesDescription,
		DependsOn: []string{additionalSupervisionAuditVersion},
	})
	Migrations.MustRegister(customLeaveAllowancesUp, customLeaveAllowancesDown)
}

func customLeaveAllowancesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE active.staff_absence_types
			ADD COLUMN IF NOT EXISTS allowance_enabled BOOLEAN NOT NULL DEFAULT FALSE,
			ADD COLUMN IF NOT EXISTS overrun_policy VARCHAR(10) NOT NULL DEFAULT 'warn';
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_overrun_policy;
		ALTER TABLE active.staff_absence_types
			ADD CONSTRAINT chk_sat_overrun_policy CHECK (overrun_policy IN ('warn', 'block'));

		CREATE TABLE IF NOT EXISTS active.staff_absence_type_allowances (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			absence_type_id BIGINT NOT NULL,
			year INTEGER NOT NULL,
			entitled_days NUMERIC(5,1) NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_staff_absence_type_allowance UNIQUE (tenant_id, staff_id, absence_type_id, year),
			CONSTRAINT fk_sat_allowance_staff FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_sat_allowance_type FOREIGN KEY (tenant_id, absence_type_id)
				REFERENCES active.staff_absence_types(tenant_id, id) ON DELETE RESTRICT,
			CONSTRAINT chk_sat_allowance_year CHECK (year BETWEEN 2000 AND 2100),
			CONSTRAINT chk_sat_allowance_days CHECK (
				entitled_days BETWEEN 0 AND 366 AND entitled_days * 2 = trunc(entitled_days * 2)
			)
		);

		CREATE TABLE IF NOT EXISTS active.staff_absence_type_allowance_changes (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id BIGINT NOT NULL,
			absence_type_id BIGINT NOT NULL,
			year INTEGER NOT NULL,
			old_entitled_days NUMERIC(5,1),
			new_entitled_days NUMERIC(5,1) NOT NULL,
			reason TEXT NOT NULL,
			changed_by BIGINT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_sat_allowance_change_staff FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_sat_allowance_change_actor FOREIGN KEY (tenant_id, changed_by)
				REFERENCES users.staff(tenant_id, id) ON DELETE RESTRICT,
			CONSTRAINT fk_sat_allowance_change_type FOREIGN KEY (tenant_id, absence_type_id)
				REFERENCES active.staff_absence_types(tenant_id, id) ON DELETE RESTRICT,
			CONSTRAINT chk_sat_allowance_change_reason CHECK (length(btrim(reason)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_sat_allowance_changes_staff
			ON active.staff_absence_type_allowance_changes (tenant_id, staff_id, created_at DESC);

		GRANT SELECT, INSERT, UPDATE, DELETE ON active.staff_absence_type_allowances TO phoenix_tenant;
		GRANT SELECT, INSERT ON active.staff_absence_type_allowance_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_absence_type_allowances_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE active.staff_absence_type_allowance_changes_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create custom leave allowances: %w", err)
	}
	if err := provisionTenantRLS(ctx, db, "active.staff_absence_type_allowances"); err != nil {
		return err
	}
	return provisionTenantRLS(ctx, db, "active.staff_absence_type_allowance_changes")
}

func customLeaveAllowancesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS active.staff_absence_type_allowance_changes;
		DROP TABLE IF EXISTS active.staff_absence_type_allowances;
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS chk_sat_overrun_policy,
			DROP COLUMN IF EXISTS overrun_policy,
			DROP COLUMN IF EXISTS allowance_enabled;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("drop custom leave allowances: %w", err)
	}
	return nil
}
