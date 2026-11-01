package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	payrollWageTypesVersion     = "1.15.721"
	payrollWageTypesDescription = "Eigene Lohnarten und Zuschläge für Nacht-, Sonntags- und Feiertagsarbeit im DATEV-Export (#4807)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     payrollWageTypesVersion,
		Description: payrollWageTypesDescription,
		DependsOn: []string{
			parentConferencesVersion, // the last migration before this one, whose erasure functions this extends
			staffAbsenceTypesVersion, // the absence types that gain a wage type
		},
	})
	Migrations.MustRegister(payrollWageTypesUp, payrollWageTypesDown)
}

// payrollWageTypesUpSQL stores the school's own wage types and surcharge
// rules (#4807).
//
//   - active.payroll_wage_types is one Lohnart of the school next to the seven
//     fixed categories: a name, the DATEV number (empty until the school
//     knows it; the DATEV preflight names it then) and what its line carries.
//     Archiving keeps the row, so a closed month that froze it still reads.
//   - active.payroll_surcharge_rules is one rule for night, Sunday or holiday
//     work. The window (minutes after midnight on the school's wall clock)
//     exists for night rules only. rate_hundredths is the percentage times
//     100 for percent rules and the amount per hour in cent for amount rules.
//   - active.staff_absence_types.wage_type_id books a school absence type
//     onto a wage type; NULL keeps the category it had.
//   - active.staff_month_balance_snapshots.payroll_rules freezes the wage
//     types, rules and mapping in force at the close, so a closed month keeps
//     them. NULL (every close before this migration) books with the current
//     ones.
//
// Both references to a wage type carry the tenant, so neither can point into
// another school.
const payrollWageTypesUpSQL = `
		CREATE TABLE IF NOT EXISTS active.payroll_wage_types (
			id          BIGSERIAL PRIMARY KEY,
			tenant_id   BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			name        TEXT NOT NULL,
			number      TEXT NOT NULL DEFAULT '',
			unit        TEXT NOT NULL,
			archived_at TIMESTAMPTZ,
			created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_payroll_wage_types_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT chk_payroll_wage_types_name CHECK (btrim(name) <> '' AND char_length(name) <= 100),
			CONSTRAINT chk_payroll_wage_types_number CHECK (number ~ '^([0-9]{1,4})?$'),
			CONSTRAINT chk_payroll_wage_types_unit CHECK (unit IN ('stunden', 'tage', 'betrag'))
		);
		CREATE UNIQUE INDEX IF NOT EXISTS uq_payroll_wage_types_active_name
			ON active.payroll_wage_types (tenant_id, lower(btrim(name)))
			WHERE archived_at IS NULL;

		COMMENT ON TABLE active.payroll_wage_types IS
			'Eigene Lohnart einer Schule für den DATEV-Export (#4807); archiviert statt gelöscht.';

		CREATE TABLE IF NOT EXISTS active.payroll_surcharge_rules (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			kind                TEXT NOT NULL,
			window_start_minute SMALLINT,
			window_end_minute   SMALLINT,
			rate_type           TEXT NOT NULL,
			rate_hundredths     INTEGER NOT NULL,
			wage_type_id        BIGINT NOT NULL,
			archived_at         TIMESTAMPTZ,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_payroll_surcharge_rules_wage_type
				FOREIGN KEY (tenant_id, wage_type_id)
				REFERENCES active.payroll_wage_types (tenant_id, id),
			CONSTRAINT chk_payroll_surcharge_rules_kind CHECK (kind IN ('nacht', 'sonntag', 'feiertag')),
			CONSTRAINT chk_payroll_surcharge_rules_rate_type CHECK (rate_type IN ('prozent', 'betrag')),
			CONSTRAINT chk_payroll_surcharge_rules_rate CHECK (rate_hundredths BETWEEN 1 AND 100000),
			CONSTRAINT chk_payroll_surcharge_rules_window CHECK (
				CASE WHEN kind = 'nacht' THEN
					window_start_minute IS NOT NULL AND window_end_minute IS NOT NULL
					AND window_start_minute BETWEEN 0 AND 1439
					AND window_end_minute BETWEEN 0 AND 1439
					AND window_start_minute <> window_end_minute
				ELSE window_start_minute IS NULL AND window_end_minute IS NULL END
			)
		);
		CREATE INDEX IF NOT EXISTS idx_payroll_surcharge_rules_wage_type
			ON active.payroll_surcharge_rules (tenant_id, wage_type_id);

		COMMENT ON TABLE active.payroll_surcharge_rules IS
			'Zuschlagsregel einer Schule für Nacht-, Sonntags- oder Feiertagsarbeit (#4807); archiviert statt gelöscht.';

		ALTER TABLE active.staff_absence_types
			ADD COLUMN IF NOT EXISTS wage_type_id BIGINT;
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS fk_staff_absence_types_wage_type;
		ALTER TABLE active.staff_absence_types
			ADD CONSTRAINT fk_staff_absence_types_wage_type
				FOREIGN KEY (tenant_id, wage_type_id)
				REFERENCES active.payroll_wage_types (tenant_id, id);
		CREATE INDEX IF NOT EXISTS idx_staff_absence_types_wage_type
			ON active.staff_absence_types (tenant_id, wage_type_id)
			WHERE wage_type_id IS NOT NULL;

		ALTER TABLE active.staff_month_balance_snapshots
			ADD COLUMN IF NOT EXISTS payroll_rules JSONB;
		ALTER TABLE active.staff_month_balance_snapshots
			DROP CONSTRAINT IF EXISTS chk_staff_month_balance_snapshots_payroll_rules;
		ALTER TABLE active.staff_month_balance_snapshots
			ADD CONSTRAINT chk_staff_month_balance_snapshots_payroll_rules
				CHECK (payroll_rules IS NULL OR jsonb_typeof(payroll_rules) = 'object');

		DROP TRIGGER IF EXISTS update_payroll_wage_types_updated_at ON active.payroll_wage_types;
		CREATE TRIGGER update_payroll_wage_types_updated_at
		BEFORE UPDATE ON active.payroll_wage_types
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_payroll_surcharge_rules_updated_at ON active.payroll_surcharge_rules;
		CREATE TRIGGER update_payroll_surcharge_rules_updated_at
		BEFORE UPDATE ON active.payroll_surcharge_rules
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE ON active.payroll_wage_types, active.payroll_surcharge_rules
			TO phoenix_tenant;
		GRANT ALL ON active.payroll_wage_types, active.payroll_surcharge_rules TO phoenix_admin;
		GRANT USAGE ON SEQUENCE active.payroll_wage_types_id_seq, active.payroll_surcharge_rules_id_seq
			TO phoenix_tenant;
		GRANT ALL ON SEQUENCE active.payroll_wage_types_id_seq, active.payroll_surcharge_rules_id_seq
			TO phoenix_admin;
`

const payrollWageTypesDownSQL = `
		ALTER TABLE active.staff_month_balance_snapshots
			DROP CONSTRAINT IF EXISTS chk_staff_month_balance_snapshots_payroll_rules;
		ALTER TABLE active.staff_month_balance_snapshots
			DROP COLUMN IF EXISTS payroll_rules;
		DROP INDEX IF EXISTS active.idx_staff_absence_types_wage_type;
		ALTER TABLE active.staff_absence_types
			DROP CONSTRAINT IF EXISTS fk_staff_absence_types_wage_type;
		ALTER TABLE active.staff_absence_types
			DROP COLUMN IF EXISTS wage_type_id;
		DROP TABLE IF EXISTS active.payroll_surcharge_rules;
		DROP TABLE IF EXISTS active.payroll_wage_types;
`

func payrollWageTypesUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, payrollWageTypesUpSQL); err != nil {
			return fmt.Errorf("create the payroll wage type tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx,
			"active.payroll_wage_types",
			"active.payroll_surcharge_rules",
		); err != nil {
			return err
		}
		// The school erasure deletes both tables with the school; the FK
		// fingerprint covers the two new tenant references.
		for _, step := range []func(context.Context, bun.Tx) error{
			installPayrollWageTypeErasurePlan, installPayrollWageTypeErasurePreflight, installPayrollWageTypeErasureResidual,
			installPayrollWageTypeErasureDelete, installPayrollWageTypeErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the payroll wage type school erasure: %w", err)
			}
		}
		return nil
	})
}

// payrollWageTypesDown reinstalls the 1.15.720 erasure before it drops the
// tables, so the installed plan never names a missing table.
func payrollWageTypesDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installParentConferenceErasurePlan, installParentConferenceErasurePreflight, installParentConferenceErasureResidual,
			installParentConferenceErasureDelete, installParentConferenceErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the school erasure of 1.15.720: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, payrollWageTypesDownSQL); err != nil {
			return fmt.Errorf("drop the payroll wage type tables: %w", err)
		}
		return nil
	})
}
