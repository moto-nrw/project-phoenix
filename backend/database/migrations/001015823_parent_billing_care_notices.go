package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareNoticesVersion = "1.15.823"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareNoticesVersion,
		Description: "Elternabrechnung: Jahresbescheid mit Monatsraten und Änderungsbescheid (#6219)",
		DependsOn: []string{
			parentBillingCarePriceFormsVersion, // the newest parent-billing migration before this one
			parentBillingDocumentKindsVersion,  // the legal remedy of the fee notice
			mealBillingMonthsVersion,           // the erasure of 1.15.811 this one extends
		},
	})
	Migrations.MustRegister(parentBillingCareNoticesUp, parentBillingCareNoticesDown)
}

// parentBillingCareNoticesUpSQL adds the Jahresbescheid of the monthly care
// contribution (#6219, ADR 0175):
//
//   - care_notice_runs: a run that assesses the months of a calendar year
//     from its first month to December for its schools; at most one
//     calculated run per Träger and year. A released run never changes.
//   - care_notices: the issued fee notice of one contract with its lines
//     (module, reduction, cost bearer share with monthly and proportional
//     annual amount, "bisher / neu / Zu- bzw. Abgang"), the copy of the
//     recipient and of the legal remedy. A change notice names the notice
//     it amends; a notice is amended at most once and a contract has at most
//     one first notice per year. It never changes.
//   - care_notice_months: the installment of each month (due date, whether
//     it is due at once, before and now) and what the month bills as the
//     monthly computation billed it. The months of a notice nobody amended
//     count as billed for the monthly run. They never change.
//   - care_notice_run_events: the trail of a run (create, read, release,
//     letter) with the account and the contracts shown.
//
// Every table carries the Träger and is isolated by it (RLS); the request
// role reads and appends, it changes only the state of a calculated run.
const parentBillingCareNoticesUpSQL = `
CREATE TABLE parent_billing.care_notice_runs (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	billing_year SMALLINT NOT NULL,
	period_from DATE NOT NULL,
	period_until DATE NOT NULL,
	school_ids BIGINT[] NOT NULL,
	notice_date DATE,
	status VARCHAR(16) NOT NULL DEFAULT 'calculated',
	created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	released_at TIMESTAMPTZ,
	issued_on DATE,
	notice_count INT,
	total_cents BIGINT,
	CONSTRAINT uq_care_notice_runs_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_care_notice_runs_year CHECK (billing_year BETWEEN 2000 AND 2100),
	CONSTRAINT chk_care_notice_runs_period CHECK (EXTRACT(DAY FROM period_from) = 1
		AND EXTRACT(YEAR FROM period_from) = billing_year
		AND period_until = make_date(billing_year, 12, 31)),
	CONSTRAINT chk_care_notice_runs_schools CHECK (cardinality(school_ids) >= 1),
	CONSTRAINT chk_care_notice_runs_status CHECK (status IN ('calculated', 'released')),
	CONSTRAINT chk_care_notice_runs_release CHECK ((status = 'released') = (
		released_at IS NOT NULL AND issued_on IS NOT NULL AND notice_count >= 0 AND total_cents >= 0))
);
CREATE UNIQUE INDEX uq_care_notice_runs_calculated_year
	ON parent_billing.care_notice_runs (organization_id, billing_year) WHERE status = 'calculated';
CREATE INDEX idx_care_notice_runs_organization
	ON parent_billing.care_notice_runs (organization_id, billing_year DESC, id DESC);
COMMENT ON TABLE parent_billing.care_notice_runs IS
	'Run of the Jahresbescheide of a Träger over the months of a year (#6219, ADR 0175); at most one calculated run per year; a released run never changes.';

CREATE TABLE parent_billing.care_notices (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	number VARCHAR(64) NOT NULL,
	number_year SMALLINT NOT NULL,
	number_sequence BIGINT NOT NULL,
	issued_on DATE NOT NULL,
	issued_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	billing_year SMALLINT NOT NULL,
	kind VARCHAR(16) NOT NULL,
	amends_notice_id BIGINT,
	care_contract_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	school_name TEXT NOT NULL,
	child_first_name TEXT NOT NULL,
	child_last_name TEXT NOT NULL,
	student_id BIGINT,
	recipient_email TEXT NOT NULL,
	family_email TEXT NOT NULL,
	recipient JSONB NOT NULL,
	payment_method VARCHAR(24) NOT NULL,
	period_from DATE NOT NULL,
	period_until DATE NOT NULL,
	lines JSONB NOT NULL,
	previous_cents BIGINT NOT NULL,
	total_cents BIGINT NOT NULL,
	delta_cents BIGINT NOT NULL,
	legal_remedy TEXT NOT NULL,
	CONSTRAINT uq_care_notices_organization UNIQUE (organization_id, id),
	CONSTRAINT uq_care_notices_number UNIQUE (organization_id, number),
	CONSTRAINT uq_care_notices_sequence UNIQUE (organization_id, number_year, number_sequence),
	CONSTRAINT fk_care_notices_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_notice_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_care_notices_amends FOREIGN KEY (organization_id, amends_notice_id)
		REFERENCES parent_billing.care_notices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_notices_number CHECK (BTRIM(number) <> '' AND number_sequence >= 1),
	CONSTRAINT chk_care_notices_kind CHECK ((kind = 'initial' AND amends_notice_id IS NULL)
		OR (kind = 'change' AND amends_notice_id IS NOT NULL)),
	CONSTRAINT chk_care_notices_period CHECK (EXTRACT(DAY FROM period_from) = 1 AND period_until >= period_from
		AND EXTRACT(YEAR FROM period_from) = billing_year AND EXTRACT(YEAR FROM period_until) = billing_year),
	CONSTRAINT chk_care_notices_amounts CHECK (previous_cents >= 0 AND total_cents >= 0
		AND delta_cents = total_cents - previous_cents),
	CONSTRAINT chk_care_notices_shape CHECK (jsonb_typeof(lines) = 'array' AND jsonb_typeof(recipient) = 'object'),
	CONSTRAINT chk_care_notices_legal_remedy CHECK (BTRIM(legal_remedy) <> '' AND char_length(legal_remedy) <= 5000)
);
CREATE UNIQUE INDEX uq_care_notices_amends ON parent_billing.care_notices (organization_id, amends_notice_id)
	WHERE amends_notice_id IS NOT NULL;
CREATE UNIQUE INDEX uq_care_notices_first ON parent_billing.care_notices (organization_id, care_contract_id, billing_year)
	WHERE kind = 'initial';
CREATE INDEX idx_care_notices_run ON parent_billing.care_notices (run_id, number_sequence);
CREATE INDEX idx_care_notices_school ON parent_billing.care_notices (organization_id, school_id);
COMMENT ON TABLE parent_billing.care_notices IS
	'Issued Jahresbescheid of one care contract over months of a year with bisher/neu/Zu-/Abgang (#6219, ADR 0175); a change notice amends one notice; never changes.';

CREATE TABLE parent_billing.care_notice_months (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	notice_id BIGINT NOT NULL,
	care_contract_id BIGINT NOT NULL,
	billing_month DATE NOT NULL,
	due_on DATE NOT NULL,
	immediately_due BOOLEAN NOT NULL,
	previous_cents BIGINT NOT NULL,
	amount_cents BIGINT NOT NULL,
	snapshot JSONB NOT NULL,
	CONSTRAINT fk_care_notice_months_notice FOREIGN KEY (organization_id, notice_id)
		REFERENCES parent_billing.care_notices (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_care_notice_months_month UNIQUE (notice_id, billing_month),
	CONSTRAINT chk_care_notice_months_month CHECK (EXTRACT(DAY FROM billing_month) = 1 AND due_on >= billing_month),
	CONSTRAINT chk_care_notice_months_amounts CHECK (previous_cents >= 0 AND amount_cents >= 0),
	CONSTRAINT chk_care_notice_months_snapshot CHECK (jsonb_typeof(snapshot) = 'object')
);
CREATE INDEX idx_care_notice_months_month
	ON parent_billing.care_notice_months (organization_id, billing_month, care_contract_id);
COMMENT ON TABLE parent_billing.care_notice_months IS
	'Installment of one month of a Jahresbescheid with what the month bills (#6219, ADR 0175); a month of a notice nobody amended is billed for the monthly run; never changes.';

CREATE TABLE parent_billing.care_notice_run_events (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	action VARCHAR(16) NOT NULL,
	actor_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	occurred_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	details JSONB NOT NULL DEFAULT '{}'::jsonb,
	CONSTRAINT fk_care_notice_run_events_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_notice_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_notice_run_events_action CHECK (action IN ('create', 'read', 'release', 'letter'))
);
CREATE INDEX idx_care_notice_run_events_run ON parent_billing.care_notice_run_events (run_id, occurred_at);
COMMENT ON TABLE parent_billing.care_notice_run_events IS
	'Trail of a run of Jahresbescheide (#6219): every read with the contracts shown and every change with the account.';

CREATE TRIGGER refuse_parent_billing_care_notice_change
	BEFORE UPDATE ON parent_billing.care_notices
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_care_notice_month_change
	BEFORE UPDATE ON parent_billing.care_notice_months
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_issued_change();
CREATE TRIGGER refuse_parent_billing_released_care_notice_run_change
	BEFORE UPDATE ON parent_billing.care_notice_runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();

ALTER TABLE parent_billing.care_notice_runs ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notice_runs FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notice_runs ON parent_billing.care_notice_runs FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_notices ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notices FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notices ON parent_billing.care_notices FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_notice_months ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notice_months FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notice_months ON parent_billing.care_notice_months FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE parent_billing.care_notice_run_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_notice_run_events FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_notice_run_events ON parent_billing.care_notice_run_events FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_notice_runs, parent_billing.care_notices, parent_billing.care_notice_months,
	parent_billing.care_notice_run_events FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.care_notice_runs TO phoenix_tenant;
GRANT SELECT, INSERT ON parent_billing.care_notices, parent_billing.care_notice_months,
	parent_billing.care_notice_run_events TO phoenix_tenant;
GRANT USAGE ON SEQUENCE parent_billing.care_notice_runs_id_seq, parent_billing.care_notices_id_seq,
	parent_billing.care_notice_months_id_seq, parent_billing.care_notice_run_events_id_seq TO phoenix_tenant;
GRANT ALL ON parent_billing.care_notice_runs, parent_billing.care_notices, parent_billing.care_notice_months,
	parent_billing.care_notice_run_events TO phoenix_admin;
GRANT ALL ON SEQUENCE parent_billing.care_notice_runs_id_seq, parent_billing.care_notices_id_seq,
	parent_billing.care_notice_months_id_seq, parent_billing.care_notice_run_events_id_seq TO phoenix_admin;
`

const parentBillingCareNoticesDownSQL = `
DROP TABLE IF EXISTS parent_billing.care_notice_run_events, parent_billing.care_notice_months,
	parent_billing.care_notices, parent_billing.care_notice_runs;
`

func parentBillingCareNoticesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareNoticesUpSQL); err != nil {
		return fmt.Errorf("create the fee notices of parent billing: %w", err)
	}
	return installCareNoticeErasure(ctx, db)
}

func parentBillingCareNoticesDown(ctx context.Context, db *bun.DB) error {
	if err := installMealBillingErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, parentBillingCareNoticesDownSQL); err != nil {
		return fmt.Errorf("drop the fee notices of parent billing: %w", err)
	}
	return nil
}
