package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingHolidayRunsVersion = "1.15.668"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingHolidayRunsVersion,
		Description: "Elternabrechnung: Abrechnungslauf für einen Ferienzeitraum mit Nummernkreis und ausgestellten Rechnungen (#4668)",
		DependsOn: []string{
			parentBillingHolidayContributionsVersion, // the frozen contributions a run bills
		},
	})
	Migrations.MustRegister(parentBillingHolidayRunsUp, parentBillingHolidayRunsDown)
}

// parentBillingHolidayRunsUpSQL adds the billing run of the Elternabrechnung
// (parent-billing, ADR 0088, #4668, ADR 0119):
//
//   - parent_billing.number_ranges: the pattern of the invoice numbers of a
//     Träger per service, such as FE-{JJJJ}-{NNNN}.
//   - parent_billing.number_range_counters: the last number issued per
//     Träger, service and year (0 for a pattern without a year). The release
//     of a run raises it in its own transaction, so the numbers have no gap.
//   - parent_billing.runs: a run for one holiday period. It is calculated
//     until the Abrechnung releases it; a released run never changes.
//   - parent_billing.run_corrections: a changed amount of one child in a
//     calculated run, with its reason. Corrections are only added.
//   - parent_billing.run_events: the trail of a run: every read of personal
//     data and every change, with the account.
//   - parent_billing.documents and parent_billing.document_positions: the
//     invoices a release issues, with their own copy of the recipient, the
//     children and every position. An issued invoice never changes.
//
// Every table carries organization_id and the Träger policy of ADR 0105.
// The request role may never update or delete an invoice, a released run, a
// correction or an event; a trigger refuses any update of an invoice or a
// released run for every role. Invoices and released runs have no foreign
// key on the holiday registration, so they stay when the registration goes
// after 92 days; they go with the Träger.
const parentBillingHolidayRunsUpSQL = `
CREATE TABLE parent_billing.number_ranges (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	pattern VARCHAR(40) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_number_ranges_service UNIQUE (organization_id, service),
	CONSTRAINT chk_number_ranges_service CHECK (service IN ('holiday')),
	CONSTRAINT chk_number_ranges_pattern CHECK (pattern ~ '\{N{1,8}\}')
);
COMMENT ON TABLE parent_billing.number_ranges IS
	'Pattern of the invoice numbers of a Träger per service (#4668, ADR 0119); read and written in the Träger transaction.';

CREATE TABLE parent_billing.number_range_counters (
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	year SMALLINT NOT NULL,
	last_number BIGINT NOT NULL,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (organization_id, service, year),
	CONSTRAINT chk_number_range_counters_service CHECK (service IN ('holiday')),
	CONSTRAINT chk_number_range_counters_year CHECK (year = 0 OR year BETWEEN 2000 AND 2999),
	CONSTRAINT chk_number_range_counters_last CHECK (last_number >= 1)
);
COMMENT ON TABLE parent_billing.number_range_counters IS
	'Last invoice number issued per Träger, service and year (#4668, ADR 0119); raised in the transaction of a release, so the numbers have no gap.';

CREATE TABLE parent_billing.runs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	service VARCHAR(16) NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	period_name TEXT NOT NULL,
	service_from DATE NOT NULL,
	service_until DATE NOT NULL,
	status VARCHAR(16) NOT NULL DEFAULT 'calculated',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	released_at TIMESTAMPTZ,
	issued_on DATE,
	document_count INT,
	total_cents BIGINT,
	CONSTRAINT uq_runs_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_runs_service CHECK (service IN ('holiday')),
	CONSTRAINT chk_runs_status CHECK (status IN ('calculated', 'released')),
	CONSTRAINT chk_runs_period CHECK (BTRIM(period_name) <> '' AND service_until >= service_from),
	CONSTRAINT chk_runs_release CHECK ((status = 'released') = (
		released_at IS NOT NULL AND issued_on IS NOT NULL AND document_count >= 1 AND total_cents >= 0))
);
CREATE UNIQUE INDEX uq_runs_calculated_period
	ON parent_billing.runs (organization_id, holiday_period_id) WHERE status = 'calculated';
CREATE INDEX idx_runs_organization ON parent_billing.runs (organization_id, created_at DESC);
COMMENT ON TABLE parent_billing.runs IS
	'Billing run of a Träger for a holiday period (#4668, ADR 0119): calculated until released; a released run never changes.';

CREATE TABLE parent_billing.run_corrections (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	amount_cents BIGINT NOT NULL,
	total_before_cents BIGINT NOT NULL,
	total_after_cents BIGINT NOT NULL,
	reason TEXT NOT NULL,
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_run_corrections_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_run_corrections_child
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_child_id)
		REFERENCES enrollment.holiday_registration_children (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_run_corrections_amount CHECK (
		amount_cents <> 0 AND total_after_cents >= 0 AND total_after_cents = total_before_cents + amount_cents),
	CONSTRAINT chk_run_corrections_reason CHECK (BTRIM(reason) <> '' AND char_length(reason) <= 500)
);
CREATE INDEX idx_run_corrections_run ON parent_billing.run_corrections (run_id, holiday_registration_child_id, id);
COMMENT ON TABLE parent_billing.run_corrections IS
	'Changed amount of one child in a calculated billing run with its reason (#4668, ADR 0119); only added, the history of a correction.';

CREATE TABLE parent_billing.run_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	details JSONB NOT NULL DEFAULT '{}'::jsonb,
	CONSTRAINT fk_run_events_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_run_events_action CHECK (action IN ('create', 'read', 'correct', 'release'))
);
CREATE INDEX idx_run_events_run ON parent_billing.run_events (run_id, occurred_at);
COMMENT ON TABLE parent_billing.run_events IS
	'Append-only trail of every read of personal data and every change of a billing run (#4668, ADR 0119); goes with its run.';

CREATE TABLE parent_billing.documents (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	service VARCHAR(16) NOT NULL,
	number VARCHAR(64) NOT NULL,
	number_year SMALLINT NOT NULL,
	number_sequence BIGINT NOT NULL,
	issued_on DATE NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	recipient_first_name TEXT NOT NULL,
	recipient_last_name TEXT NOT NULL,
	recipient_email TEXT NOT NULL,
	service_from DATE NOT NULL,
	service_until DATE NOT NULL,
	total_cents BIGINT NOT NULL,
	CONSTRAINT uq_documents_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_documents_number UNIQUE (organization_id, number),
	CONSTRAINT uq_documents_sequence UNIQUE (organization_id, service, number_year, number_sequence),
	CONSTRAINT uq_documents_registration UNIQUE (organization_id, service, holiday_registration_id),
	CONSTRAINT fk_documents_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_documents_service CHECK (service IN ('holiday')),
	CONSTRAINT chk_documents_number CHECK (BTRIM(number) <> '' AND number_sequence >= 1),
	CONSTRAINT chk_documents_total CHECK (total_cents >= 0),
	CONSTRAINT chk_documents_period CHECK (service_until >= service_from)
);
CREATE INDEX idx_documents_run ON parent_billing.documents (run_id, number_sequence);
COMMENT ON TABLE parent_billing.documents IS
	'Invoice a release issues (#4668, ADR 0119) with its number and its own copy of the recipient; never changes, stays when the registration goes and goes with the Träger.';

CREATE TABLE parent_billing.document_positions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	document_id BIGINT NOT NULL,
	position SMALLINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	kind VARCHAR(32) NOT NULL,
	amount_cents BIGINT NOT NULL,
	net_cents BIGINT NOT NULL,
	tax_cents BIGINT NOT NULL DEFAULT 0,
	week_starts_on DATE,
	week_ends_on DATE,
	time_model_name TEXT NOT NULL DEFAULT '',
	child_rank SMALLINT,
	discount_basis_points INT,
	discount_cents BIGINT,
	note TEXT NOT NULL DEFAULT '',
	CONSTRAINT fk_document_positions_document FOREIGN KEY (organization_id, document_id)
		REFERENCES parent_billing.documents (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_document_positions_position UNIQUE (document_id, position),
	CONSTRAINT chk_document_positions_kind CHECK (kind IN ('base', 'sibling', 'benefits_exemption', 'correction')),
	CONSTRAINT chk_document_positions_amounts CHECK (amount_cents = net_cents + tax_cents AND tax_cents = 0),
	CONSTRAINT chk_document_positions_correction CHECK (kind <> 'correction' OR (amount_cents <> 0 AND BTRIM(note) <> ''))
);
COMMENT ON TABLE parent_billing.document_positions IS
	'Position of an issued invoice (#4668, ADR 0119): the frozen contribution lines and the corrections of the run; never changes.';

CREATE TRIGGER update_parent_billing_number_ranges_updated_at
	BEFORE UPDATE ON parent_billing.number_ranges
	FOR EACH ROW EXECUTE FUNCTION update_modified_column();

-- An issued invoice and a released run never change, for any role.
CREATE FUNCTION parent_billing.refuse_issued_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	RAISE EXCEPTION 'an issued invoice of the Elternabrechnung never changes (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER refuse_parent_billing_document_change
	BEFORE UPDATE ON parent_billing.documents
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_document_position_change
	BEFORE UPDATE ON parent_billing.document_positions
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_released_run_change
	BEFORE UPDATE ON parent_billing.runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_issued_change();

ALTER TABLE parent_billing.number_ranges ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.number_ranges FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_number_ranges ON parent_billing.number_ranges FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.number_range_counters ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.number_range_counters FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_number_range_counters ON parent_billing.number_range_counters FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.runs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_runs ON parent_billing.runs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.run_corrections ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.run_corrections FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_run_corrections ON parent_billing.run_corrections FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.run_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.run_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_run_events ON parent_billing.run_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.documents FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_documents ON parent_billing.documents FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.document_positions ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.document_positions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_document_positions ON parent_billing.document_positions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.number_ranges, parent_billing.number_range_counters, parent_billing.runs,
	parent_billing.run_corrections, parent_billing.run_events, parent_billing.documents,
	parent_billing.document_positions FROM PUBLIC;
REVOKE ALL ON FUNCTION parent_billing.refuse_issued_change() FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.number_ranges, parent_billing.number_range_counters,
	parent_billing.runs TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.run_corrections, parent_billing.run_events, parent_billing.documents,
	parent_billing.document_positions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.number_ranges_id_seq, parent_billing.runs_id_seq,
	parent_billing.run_corrections_id_seq, parent_billing.run_events_id_seq, parent_billing.documents_id_seq,
	parent_billing.document_positions_id_seq TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
GRANT ALL ON ALL SEQUENCES IN SCHEMA parent_billing TO phoenix_admin;
`

const parentBillingHolidayRunsDownSQL = `
DROP TABLE IF EXISTS parent_billing.document_positions, parent_billing.documents, parent_billing.run_events,
	parent_billing.run_corrections, parent_billing.runs, parent_billing.number_range_counters,
	parent_billing.number_ranges;
DROP FUNCTION IF EXISTS parent_billing.refuse_issued_change();
`

func parentBillingHolidayRunsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingHolidayRunsUpSQL); err != nil {
		return fmt.Errorf("create parent billing holiday runs: %w", err)
	}
	return nil
}

func parentBillingHolidayRunsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingHolidayRunsDownSQL); err != nil {
		return fmt.Errorf("drop parent billing holiday runs: %w", err)
	}
	return nil
}
