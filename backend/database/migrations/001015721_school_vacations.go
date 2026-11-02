package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolVacationsVersion     = "1.15.721"
	schoolVacationsDescription = "Schulferien je Bundesland und bewusst geplante Ferienbetreuung je Schule (#5345)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolVacationsVersion,
		Description: schoolVacationsDescription,
		DependsOn: []string{
			parentConferencesVersion, // the last migration before this one, whose erasure functions this extends
		},
	})
	Migrations.MustRegister(schoolVacationsUp, schoolVacationsDown)
}

// schoolVacationsUpSQL stores the school vacations (#5345).
//
//   - calendar.school_vacations holds the school vacations of every German
//     federal state, as the daily worker sync reads them from the
//     OpenHolidays API. They hold for every school of a state, so the table
//     has no tenant: every school reads it, only the administrative role
//     (the worker) writes it.
//   - calendar.school_vacation_acknowledgements records that a school plans
//     care in one vacation window on purpose. The window is named by state,
//     first and last day, so a later sync that renews the vacation rows keeps
//     the decision.
const schoolVacationsUpSQL = `
		CREATE TABLE IF NOT EXISTS calendar.school_vacations (
			id          BIGSERIAL PRIMARY KEY,
			region_code TEXT NOT NULL,
			name        TEXT NOT NULL,
			start_date  DATE NOT NULL,
			end_date    DATE NOT NULL,
			synced_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_calendar_school_vacations_region CHECK (region_code ~ '^DE-[A-Z]{2}$'),
			CONSTRAINT chk_calendar_school_vacations_name CHECK (btrim(name) <> ''),
			CONSTRAINT chk_calendar_school_vacations_range CHECK (start_date <= end_date),
			CONSTRAINT uq_calendar_school_vacations_window UNIQUE (region_code, start_date, end_date)
		);
		CREATE INDEX IF NOT EXISTS idx_calendar_school_vacations_region_end
			ON calendar.school_vacations (region_code, end_date);
		COMMENT ON TABLE calendar.school_vacations IS
			'Schulferien je Bundesland aus der OpenHolidays API (#5345); der tägliche Abgleich des Workers ersetzt sie, ein fehlgeschlagener Abruf lässt sie stehen.';

		-- The worker writes the vacations in the administrative transaction;
		-- until now no administrative command needed the calendar schema.
		GRANT USAGE ON SCHEMA calendar TO phoenix_admin;
		REVOKE ALL ON calendar.school_vacations FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
		GRANT SELECT ON calendar.school_vacations TO phoenix_tenant, phoenix_admin;
		GRANT INSERT, DELETE ON calendar.school_vacations TO phoenix_admin;
		GRANT USAGE ON SEQUENCE calendar.school_vacations_id_seq TO phoenix_admin;

		CREATE TABLE IF NOT EXISTS calendar.school_vacation_acknowledgements (
			id                         BIGSERIAL PRIMARY KEY,
			tenant_id                  BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			region_code                TEXT NOT NULL,
			start_date                 DATE NOT NULL,
			end_date                   DATE NOT NULL,
			acknowledged_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			acknowledged_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_calendar_school_vacation_acknowledgements_range CHECK (start_date <= end_date),
			CONSTRAINT uq_calendar_school_vacation_acknowledgements_window
				UNIQUE (tenant_id, region_code, start_date, end_date)
		);
		COMMENT ON TABLE calendar.school_vacation_acknowledgements IS
			'Die Schule plant in diesem Ferienfenster bewusst Betreuung (#5345); der Hinweis auf Regeltermine in den Ferien entfällt dafür.';

		GRANT SELECT, INSERT ON calendar.school_vacation_acknowledgements TO phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE calendar.school_vacation_acknowledgements_id_seq TO phoenix_tenant, phoenix_admin;
`

const schoolVacationsDownSQL = `
		DROP TABLE IF EXISTS calendar.school_vacation_acknowledgements;
		DROP TABLE IF EXISTS calendar.school_vacations;
		REVOKE USAGE ON SCHEMA calendar FROM phoenix_admin;
`

func schoolVacationsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, schoolVacationsUpSQL); err != nil {
			return fmt.Errorf("create the school vacation tables: %w", err)
		}
		if err := provisionTenantRLS(ctx, tx, "calendar.school_vacation_acknowledgements"); err != nil {
			return err
		}
		// The school erasure deletes the acknowledgements with the school.
		for _, step := range []func(context.Context, bun.Tx) error{
			installSchoolVacationErasurePlan, installSchoolVacationErasurePreflight, installSchoolVacationErasureResidual,
			installSchoolVacationErasureDelete, installSchoolVacationErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("install the school vacation school erasure: %w", err)
			}
		}
		return nil
	})
}

// schoolVacationsDown restores the 1.15.720 erasure functions before it drops
// the tables, so the installed plan never names a missing table.
func schoolVacationsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		for _, step := range []func(context.Context, bun.Tx) error{
			installParentConferenceErasurePlan, installParentConferenceErasurePreflight, installParentConferenceErasureResidual,
			installParentConferenceErasureDelete, installParentConferenceErasurePreview,
		} {
			if err := step(ctx, tx); err != nil {
				return fmt.Errorf("restore the parent conference school erasure: %w", err)
			}
		}
		if _, err := tx.ExecContext(ctx, schoolVacationsDownSQL); err != nil {
			return fmt.Errorf("drop the school vacation tables: %w", err)
		}
		return nil
	})
}
