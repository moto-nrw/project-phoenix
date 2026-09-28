package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.416",
		Description: "Store the monthly billing key-date counts of every school (#2791)",
		DependsOn:   []string{"1.13.1", "1.14.1"},
	})
	Migrations.MustRegister(billingKeyDateCountsUp, billingKeyDateCountsDown)
}

// billingKeyDateCountsUp creates the operator billing report's two tables.
//
// platform.billing_settings holds the one key day that applies to every
// school. platform.billing_key_date_counts holds the figures captured on each
// month's key date. Past figures cannot be recomputed from the live tables:
// resuming care rewrites the enrollment dates, a hard delete removes the
// student row, and devices keep no status history. So a row is written once
// and never changed: the administrative role loses UPDATE and DELETE on it.
func billingKeyDateCountsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE TABLE platform.billing_settings (
			id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
			key_day SMALLINT NOT NULL DEFAULT 15 CHECK (key_day BETWEEN 1 AND 28),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_by_operator_id BIGINT
		);
		COMMENT ON COLUMN platform.billing_settings.key_day IS
			'Day of the month on which the billing counts are captured. Capped at 28 so every month has it.';
		INSERT INTO platform.billing_settings (id) VALUES (1);

		CREATE TABLE platform.billing_key_date_counts (
			id BIGSERIAL PRIMARY KEY,
			-- Historical counts outlive the live school row. The captured name
			-- and this retained school ID are the invoice reference, so no foreign
			-- key may cascade a hard deletion into this evidence.
			school_id BIGINT NOT NULL,
			period DATE NOT NULL,
			key_date DATE NOT NULL,
			school_name TEXT NOT NULL,
			organization_name TEXT NOT NULL,
			active_students INTEGER NOT NULL CHECK (active_students >= 0),
			active_terminals INTEGER NOT NULL CHECK (active_terminals >= 0),
			recorded_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT uq_billing_key_date_counts_school_period UNIQUE (school_id, period),
			CONSTRAINT chk_billing_key_date_counts_period CHECK (EXTRACT(DAY FROM period) = 1),
			CONSTRAINT chk_billing_key_date_counts_key_date
				CHECK (date_trunc('month', key_date)::date = period)
		);
		COMMENT ON TABLE platform.billing_key_date_counts IS
			'Operator billing report (#2791): active students and active terminals per school, captured once per month on the key date. Rows are never updated.';
		CREATE INDEX idx_billing_key_date_counts_period ON platform.billing_key_date_counts (period);

		REVOKE ALL ON platform.billing_settings, platform.billing_key_date_counts
			FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
		GRANT SELECT, UPDATE ON platform.billing_settings TO phoenix_admin;
		GRANT SELECT, INSERT ON platform.billing_key_date_counts TO phoenix_admin;
		GRANT USAGE ON SEQUENCE platform.billing_key_date_counts_id_seq TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create billing key-date counts: %w", err)
	}
	return nil
}

func billingKeyDateCountsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP TABLE IF EXISTS platform.billing_key_date_counts;
		DROP TABLE IF EXISTS platform.billing_settings;
	`).Exec(ctx)
	return err
}
