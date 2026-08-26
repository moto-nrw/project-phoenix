package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	schoolInvoicesVersion     = "1.15.336"
	schoolInvoicesDescription = "Per-school invoice schedule for the contract overview (#1459 demo)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     schoolInvoicesVersion,
		Description: schoolInvoicesDescription,
		DependsOn:   []string{staffMessagingVersion},
	})

	Migrations.MustRegister(schoolInvoicesUp, schoolInvoicesDown)
}

// schoolInvoicesUp creates platform.school_invoices: the payment schedule the
// moto team maintains per school and the school reads back on /vertrag.
//
// Why a table and not settings: the schedule is a LIST (one row per billing
// period). The contract facts around it — tier, contingent, price, term — are
// scalars and live in the settings registry as vertrag.* (operator-only).
//
// Amounts are integer cents. Never a float: 19,90 € is not representable in
// binary floating point, and a rounding drift on money is a support ticket.
//
// due_date / paid_on are DATE columns because they are calendar days, not
// instants — the models bind them as timezone.Date (see
// .claude/rules/calendar-dates.md).
//
// Grants: phoenix_tenant gets full CRUD, not just SELECT, because the operator
// write path runs through tenant.WithTenantTx (SET LOCAL ROLE phoenix_tenant)
// exactly like the operator settings editor does. Method-level separation is
// enforced at the API layer — the tenant router exposes GET only — mirroring
// how config.setting_values is CRUD-granted while operator-only keys are
// guarded in api/config.
func schoolInvoicesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.336: Creating platform.school_invoices...")

	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS platform.school_invoices (
			id             BIGSERIAL PRIMARY KEY,
			tenant_id      BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			period_label   TEXT NOT NULL,
			invoice_number TEXT NOT NULL DEFAULT '',
			amount_cents   BIGINT NOT NULL CHECK (amount_cents >= 0),
			due_date       DATE NOT NULL,
			status         TEXT NOT NULL DEFAULT 'offen'
				CHECK (status IN ('offen', 'bezahlt', 'storniert')),
			paid_on        DATE,
			note           TEXT NOT NULL DEFAULT '',
			created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_school_invoice_paid_on CHECK (
				(status = 'bezahlt' AND paid_on IS NOT NULL)
				OR (status <> 'bezahlt' AND paid_on IS NULL)
			),
			CONSTRAINT chk_school_invoice_period_label CHECK (period_label <> '')
		);

		CREATE INDEX IF NOT EXISTS idx_school_invoices_tenant_due
			ON platform.school_invoices (tenant_id, due_date DESC, id DESC);

		CREATE UNIQUE INDEX IF NOT EXISTS uq_school_invoices_number
			ON platform.school_invoices (tenant_id, invoice_number)
			WHERE invoice_number <> '';

		COMMENT ON TABLE platform.school_invoices IS
			'Operator-maintained payment schedule per school; read-only for the tenant (#1459).';

		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.school_invoices TO phoenix_tenant;
		GRANT USAGE, SELECT ON SEQUENCE platform.school_invoices_id_seq TO phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.school_invoices TO phoenix_auth;
		GRANT USAGE, SELECT ON SEQUENCE platform.school_invoices_id_seq TO phoenix_auth;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating platform.school_invoices: %w", err)
	}

	return provisionTenantRLS(ctx, db, "platform.school_invoices")
}

func schoolInvoicesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back 1.15.336: Dropping platform.school_invoices...")

	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS platform.school_invoices;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping platform.school_invoices: %w", err)
	}

	return nil
}
