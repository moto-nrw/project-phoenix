package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const parentBillingCareMonthsVersion = "1.15.789"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareMonthsVersion,
		Description: "Elternabrechnung: Monatsbeitrag aus dem Betreuungsvertrag mit Tarifen je Träger (#4684)",
		DependsOn: []string{
			careContractAmendmentsVersion,          // the versions in force the monthly run reads
			parentBillingHolidayRunsVersion,        // the number ranges and the refusal of a change
			demoVisitorErasureCareContractsVersion, // the demo erasure this one extends
		},
	})
	Migrations.MustRegister(parentBillingCareMonthsUp, parentBillingCareMonthsDown)
}

// parentBillingCareMonthsUpSQL adds the monthly contribution of the care
// contract (#4684, ADR 0119 addendum 2026-11-07):
//
//   - care_tariffs: the monthly price of one booked weekday of a module
//     (care offering) at a school, from the first day of a month on; it holds
//     until the next price of the same module and weekday. No price stands
//     in the code.
//   - care_runs: the monthly run of a Träger. A month has at most one run
//     that is not released; a later run of the month bills the contracts
//     the earlier ones left out.
//   - care_invoices: the issued invoice of one contract and month, with its
//     own copy of child, school and recipient. A contract has at most one
//     invoice per month, also with two releases at once.
//   - care_invoice_positions: one position per booked module, with the
//     booked weekdays and their prices; never one line per calendar day.
//   - care_run_events: the trail of every read and change of a run.
//
// Invoices, positions and released runs never change, for any role
// (parent_billing.refuse_issued_change, ADR 0119 point 7).
const parentBillingCareMonthsUpSQL = `
ALTER TABLE parent_billing.number_ranges DROP CONSTRAINT chk_number_ranges_service;
ALTER TABLE parent_billing.number_ranges ADD CONSTRAINT chk_number_ranges_service CHECK (service IN ('holiday', 'care'));
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_service;
ALTER TABLE parent_billing.number_range_counters ADD CONSTRAINT chk_number_range_counters_service CHECK (service IN ('holiday', 'care'));

CREATE TABLE parent_billing.care_tariffs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	school_id BIGINT NOT NULL,
	care_offering_id BIGINT NOT NULL,
	module_name TEXT NOT NULL,
	weekday VARCHAR(3) NOT NULL,
	valid_from DATE NOT NULL,
	monthly_cents BIGINT NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_care_tariffs_price UNIQUE (organization_id, school_id, care_offering_id, weekday, valid_from),
	CONSTRAINT chk_care_tariffs_weekday CHECK (weekday IN ('mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun')),
	CONSTRAINT chk_care_tariffs_month CHECK (EXTRACT(DAY FROM valid_from) = 1),
	CONSTRAINT chk_care_tariffs_amount CHECK (monthly_cents BETWEEN 0 AND 100000000),
	CONSTRAINT chk_care_tariffs_module CHECK (BTRIM(module_name) <> '')
);
CREATE INDEX idx_care_tariffs_school ON parent_billing.care_tariffs (organization_id, school_id, valid_from);
COMMENT ON TABLE parent_billing.care_tariffs IS
	'Monthly price of one booked weekday of a care module at a school, from the first day of a month until the next price (#4684).';

CREATE TABLE parent_billing.care_runs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	billing_month DATE NOT NULL,
	status VARCHAR(16) NOT NULL DEFAULT 'calculated',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	released_at TIMESTAMPTZ,
	issued_on DATE,
	document_count INT,
	total_cents BIGINT,
	month_rule VARCHAR(24),
	CONSTRAINT uq_care_runs_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_care_runs_month CHECK (EXTRACT(DAY FROM billing_month) = 1),
	CONSTRAINT chk_care_runs_status CHECK (status IN ('calculated', 'released')),
	CONSTRAINT chk_care_runs_rule CHECK (month_rule IS NULL OR month_rule IN ('version_on_first', 'pro_rata')),
	CONSTRAINT chk_care_runs_release CHECK ((status = 'released') = (
		released_at IS NOT NULL AND issued_on IS NOT NULL AND document_count >= 1 AND total_cents >= 0 AND month_rule IS NOT NULL))
);
CREATE UNIQUE INDEX uq_care_runs_calculated_month
	ON parent_billing.care_runs (organization_id, billing_month) WHERE status = 'calculated';
CREATE INDEX idx_care_runs_organization ON parent_billing.care_runs (organization_id, billing_month DESC, id DESC);
COMMENT ON TABLE parent_billing.care_runs IS
	'Monthly run of the care contributions of a Träger (#4684, ADR 0119); at most one calculated run per month.';

CREATE TABLE parent_billing.care_run_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	details JSONB NOT NULL DEFAULT '{}'::jsonb,
	CONSTRAINT fk_care_run_events_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_run_events_action CHECK (action IN ('create', 'read', 'release'))
);
CREATE INDEX idx_care_run_events_run ON parent_billing.care_run_events (run_id, occurred_at);
COMMENT ON TABLE parent_billing.care_run_events IS
	'Trail of a monthly run: who started, read and released it, with the contracts it read (#4684).';

CREATE TABLE parent_billing.care_invoices (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	number VARCHAR(64) NOT NULL,
	number_year SMALLINT NOT NULL,
	number_sequence BIGINT NOT NULL,
	issued_on DATE NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	billing_month DATE NOT NULL,
	care_contract_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	school_name TEXT NOT NULL,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	recipient_first_name TEXT NOT NULL,
	recipient_last_name TEXT NOT NULL,
	recipient_email TEXT NOT NULL,
	month_rule VARCHAR(24) NOT NULL,
	total_cents BIGINT NOT NULL,
	CONSTRAINT uq_care_invoices_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_care_invoices_number UNIQUE (organization_id, number),
	CONSTRAINT uq_care_invoices_sequence UNIQUE (organization_id, number_year, number_sequence),
	CONSTRAINT uq_care_invoices_contract_month UNIQUE (organization_id, care_contract_id, billing_month),
	CONSTRAINT fk_care_invoices_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_invoices_number CHECK (BTRIM(number) <> '' AND number_sequence >= 1),
	CONSTRAINT chk_care_invoices_month CHECK (EXTRACT(DAY FROM billing_month) = 1),
	CONSTRAINT chk_care_invoices_rule CHECK (month_rule IN ('version_on_first', 'pro_rata')),
	CONSTRAINT chk_care_invoices_total CHECK (total_cents >= 0)
);
CREATE INDEX idx_care_invoices_run ON parent_billing.care_invoices (run_id, number_sequence);
CREATE INDEX idx_care_invoices_school_month ON parent_billing.care_invoices (organization_id, school_id, billing_month);
COMMENT ON TABLE parent_billing.care_invoices IS
	'Issued invoice of the monthly care contribution of one contract and month (#4684); never changes.';

CREATE TABLE parent_billing.care_invoice_positions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	invoice_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	care_offering_id BIGINT NOT NULL,
	module_name TEXT NOT NULL,
	contract_version_number SMALLINT NOT NULL,
	service_from DATE NOT NULL,
	service_until DATE NOT NULL,
	weekdays JSONB NOT NULL,
	amount_cents BIGINT NOT NULL,
	net_cents BIGINT NOT NULL,
	tax_cents BIGINT NOT NULL DEFAULT 0,
	CONSTRAINT fk_care_invoice_positions_invoice FOREIGN KEY (organization_id, invoice_id)
		REFERENCES parent_billing.care_invoices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_invoice_positions_position UNIQUE (invoice_id, position),
	CONSTRAINT chk_care_invoice_positions_amounts CHECK (amount_cents >= 0 AND amount_cents = net_cents + tax_cents AND tax_cents = 0),
	CONSTRAINT chk_care_invoice_positions_days CHECK (service_until >= service_from),
	CONSTRAINT chk_care_invoice_positions_weekdays CHECK (jsonb_typeof(weekdays) = 'array' AND jsonb_array_length(weekdays) >= 1)
);
COMMENT ON TABLE parent_billing.care_invoice_positions IS
	'Position of a monthly care invoice: one booked module with its weekdays and their monthly prices (#4684); never changes.';

CREATE TRIGGER refuse_parent_billing_care_invoice_change
	BEFORE UPDATE ON parent_billing.care_invoices
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_care_invoice_position_change
	BEFORE UPDATE ON parent_billing.care_invoice_positions
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_released_care_run_change
	BEFORE UPDATE ON parent_billing.care_runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.care_tariffs ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_tariffs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_tariffs ON parent_billing.care_tariffs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_runs ON parent_billing.care_runs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_run_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_run_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_run_events ON parent_billing.care_run_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_invoices ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoices FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_invoices ON parent_billing.care_invoices FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_invoice_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_invoice_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_invoice_positions ON parent_billing.care_invoice_positions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_tariffs, parent_billing.care_runs, parent_billing.care_run_events,
	parent_billing.care_invoices, parent_billing.care_invoice_positions FROM PUBLIC;
GRANT SELECT, INSERT, DELETE ON parent_billing.care_tariffs TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE ON parent_billing.care_runs TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.care_run_events, parent_billing.care_invoices,
	parent_billing.care_invoice_positions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_tariffs_id_seq, parent_billing.care_runs_id_seq,
	parent_billing.care_run_events_id_seq, parent_billing.care_invoices_id_seq,
	parent_billing.care_invoice_positions_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingCareMonthsDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_invoice_positions, parent_billing.care_invoices,
	parent_billing.care_run_events, parent_billing.care_runs, parent_billing.care_tariffs;
DELETE FROM parent_billing.number_range_counters WHERE service = 'care';
DELETE FROM parent_billing.number_ranges WHERE service = 'care';
ALTER TABLE parent_billing.number_ranges DROP CONSTRAINT chk_number_ranges_service;
ALTER TABLE parent_billing.number_ranges ADD CONSTRAINT chk_number_ranges_service CHECK (service IN ('holiday'));
ALTER TABLE parent_billing.number_range_counters DROP CONSTRAINT chk_number_range_counters_service;
ALTER TABLE parent_billing.number_range_counters ADD CONSTRAINT chk_number_range_counters_service CHECK (service IN ('holiday'));
`

// careMonthsDemoErasureAnchor is the statement of 1.15.786 that locks the
// contracts of the retired demo schools; the monthly invoices go right after
// it, while the contracts they name still exist.
const careMonthsDemoErasureAnchor = `
					PERFORM 1 FROM care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
					ORDER BY contract.id
					FOR UPDATE;
`

// careMonthsDemoErasureSQL also deletes what the monthly contribution keeps
// of a retired demo school (#4684). An invoice copies child, recipient and
// mail address and outlives the contract, so nothing removes it with the
// school: the invoices of the school go, their positions by cascade. A read
// in the trail of a run names the contracts it showed; such an event goes
// with the contracts of the school. The run and its other events name no
// one and stay. The numbers of a demo school's invoices leave a gap in the
// number range of its Träger; ADR 0119 accepts that for demo schools as for
// holiday invoices.
const careMonthsDemoErasureSQL = careMonthsDemoErasureAnchor + `					DELETE FROM parent_billing.care_invoices
					WHERE school_id = ANY (hidden_schools);
					DELETE FROM parent_billing.care_run_events AS event
					USING care_contract.contracts AS contract
					WHERE contract.school_id = ANY (hidden_schools)
						AND event.organization_id = contract.organization_id
						AND event.details->'contract_ids' @> to_jsonb(contract.id);
`

// careMonthsDemoErasureUpSQL is platform.erase_closed_demo_visitors of
// 1.15.786 with the deletions of careMonthsDemoErasureSQL.
func careMonthsDemoErasureUpSQL() (string, error) {
	if strings.Count(demoVisitorErasureCareContractsUpSQL, careMonthsDemoErasureAnchor) != 1 {
		return "", fmt.Errorf("the demo erasure of %s lost its contract lock", demoVisitorErasureCareContractsVersion)
	}
	return strings.Replace(demoVisitorErasureCareContractsUpSQL, careMonthsDemoErasureAnchor, careMonthsDemoErasureSQL, 1), nil
}

func parentBillingCareMonthsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareMonthsUpSQL); err != nil {
		return fmt.Errorf("create parent billing care months: %w", err)
	}
	erasure, err := careMonthsDemoErasureUpSQL()
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, erasure); err != nil {
		return fmt.Errorf("erase the monthly invoices of retired demo schools: %w", err)
	}
	return nil
}

// parentBillingCareMonthsDown reinstalls the erasure of 1.15.786 before it
// drops the tables that erasure no longer knows.
func parentBillingCareMonthsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureCareContractsUpSQL); err != nil {
		return fmt.Errorf("restore the demo visitor erasure of %s: %w", demoVisitorErasureCareContractsVersion, err)
	}
	if _, err := db.ExecContext(ctx, parentBillingCareMonthsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing care months: %w", err)
	}
	return nil
}
