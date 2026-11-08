package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingCareReviewVersion = "1.15.814"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingCareReviewVersion,
		Description: "Elternabrechnung: Prüfphase im Monatslauf mit Standorten, Rechnungsdatum, Ausschluss und Prüfliste (#5916)",
		DependsOn: []string{
			parentBillingCareLettersVersion, // the trail actions of the letters this one keeps
		},
	})
	Migrations.MustRegister(parentBillingCareReviewUp, parentBillingCareReviewDown)
}

// parentBillingCareReviewUpSQL gives the monthly run the review before the
// release (#5916, ADR 0119 addendum 2026-11-08, decision D3/E1 A):
//
//   - care_run_schools: the schools a run bills. A school is in at most one
//     open run per month (uq_care_run_schools_open), also when two runs start
//     at once; runs of disjoint schools may be checked side by side. This
//     replaces "one calculated run per month" (uq_care_runs_calculated_month).
//     A run of 1.15.789 billed every school of its Träger; it keeps them.
//   - care_runs.invoice_date: the invoice date of the run; the release puts
//     it on every invoice (issued_on), NULL takes the day of the release.
//   - care_run_events: the exclusion and the return of an invoice before the
//     release ("exclude", "include"), with reason, account and time, and the
//     download of the review list ("export"); the trail is append only.
//   - chk_care_runs_release: a run whose invoices are all excluded closes
//     without an invoice and without a number, so its schools are free for
//     the next run of the month. Zero invoices mean zero money.
const parentBillingCareReviewUpSQL = `
ALTER TABLE parent_billing.care_runs ADD COLUMN invoice_date DATE;
COMMENT ON COLUMN parent_billing.care_runs.invoice_date IS
	'Invoice date of the run (#5916); the release puts it on every invoice, NULL takes the day of the release.';

CREATE TABLE parent_billing.care_run_schools (
	organization_id BIGINT NOT NULL,
	run_id BIGINT NOT NULL,
	school_id BIGINT NOT NULL,
	billing_month DATE NOT NULL,
	open BOOLEAN NOT NULL DEFAULT true,
	CONSTRAINT pk_care_run_schools PRIMARY KEY (organization_id, run_id, school_id),
	CONSTRAINT fk_care_run_schools_run FOREIGN KEY (organization_id, run_id)
		REFERENCES parent_billing.care_runs (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_care_run_schools_month CHECK (EXTRACT(DAY FROM billing_month) = 1)
);
CREATE UNIQUE INDEX uq_care_run_schools_open
	ON parent_billing.care_run_schools (organization_id, billing_month, school_id) WHERE open;
COMMENT ON TABLE parent_billing.care_run_schools IS
	'Schools a monthly run bills (#5916); a school is in at most one open run per month.';

-- A school of a released run never changes: the release closes it once.
CREATE FUNCTION parent_billing.refuse_closed_care_run_school_change() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF OLD.open AND NOT NEW.open AND to_jsonb(NEW) - 'open' = to_jsonb(OLD) - 'open' THEN
		RETURN NEW;
	END IF;
	RAISE EXCEPTION 'the schools of a monthly run only close with its release (%)', TG_TABLE_NAME
		USING ERRCODE = '55000';
END;
$$;
REVOKE ALL ON FUNCTION parent_billing.refuse_closed_care_run_school_change() FROM PUBLIC;
CREATE TRIGGER refuse_parent_billing_care_run_school_change
	BEFORE UPDATE ON parent_billing.care_run_schools
	FOR EACH ROW EXECUTE FUNCTION parent_billing.refuse_closed_care_run_school_change();

INSERT INTO parent_billing.care_run_schools (organization_id, run_id, school_id, billing_month, open)
SELECT run.organization_id, run.id, school.id, run.billing_month, run.status = 'calculated'
FROM parent_billing.care_runs AS run
JOIN platform.schools AS school ON school.organization_id = run.organization_id AND school.deleted_at IS NULL;

DROP INDEX parent_billing.uq_care_runs_calculated_month;

ALTER TABLE parent_billing.care_runs DROP CONSTRAINT chk_care_runs_release;
ALTER TABLE parent_billing.care_runs ADD CONSTRAINT chk_care_runs_release CHECK (
	(status = 'released') = (released_at IS NOT NULL AND issued_on IS NOT NULL
		AND document_count >= 0 AND total_cents >= 0 AND month_rule IS NOT NULL
		AND (document_count > 0 OR total_cents = 0)));

ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letters', 'mail', 'cancel', 'exclude', 'include', 'export', 'invoice_date'));

ALTER TABLE parent_billing.care_run_schools ENABLE ROW LEVEL SECURITY;
ALTER TABLE parent_billing.care_run_schools FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_parent_billing_care_run_schools ON parent_billing.care_run_schools FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON parent_billing.care_run_schools FROM PUBLIC;
GRANT SELECT, INSERT, UPDATE ON parent_billing.care_run_schools TO phoenix_tenant;
GRANT ALL ON ALL TABLES IN SCHEMA parent_billing TO phoenix_admin;
`

// The down migration gives the month back its single calculated run. Two
// open runs of one month (disjoint schools) cannot keep that index: the
// later ones go with their trail.
const parentBillingCareReviewDownSQL = `
DELETE FROM parent_billing.care_runs AS run
WHERE run.status = 'calculated' AND EXISTS (
	SELECT 1 FROM parent_billing.care_runs AS earlier
	WHERE earlier.organization_id = run.organization_id AND earlier.billing_month = run.billing_month
		AND earlier.status = 'calculated' AND earlier.id < run.id);
CREATE UNIQUE INDEX uq_care_runs_calculated_month
	ON parent_billing.care_runs (organization_id, billing_month) WHERE status = 'calculated';
DELETE FROM parent_billing.care_runs WHERE status = 'released' AND document_count = 0;
ALTER TABLE parent_billing.care_runs DROP CONSTRAINT chk_care_runs_release;
ALTER TABLE parent_billing.care_runs ADD CONSTRAINT chk_care_runs_release CHECK ((status = 'released') = (
	released_at IS NOT NULL AND issued_on IS NOT NULL AND document_count >= 1 AND total_cents >= 0 AND month_rule IS NOT NULL));
DELETE FROM parent_billing.care_run_events WHERE action IN ('exclude', 'include', 'export', 'invoice_date');
ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letters', 'mail', 'cancel'));
DROP TABLE IF EXISTS parent_billing.care_run_schools;
DROP FUNCTION IF EXISTS parent_billing.refuse_closed_care_run_school_change();
ALTER TABLE parent_billing.care_runs DROP COLUMN invoice_date;
`

func parentBillingCareReviewUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareReviewUpSQL); err != nil {
		return fmt.Errorf("add the review of the monthly run: %w", err)
	}
	return nil
}

func parentBillingCareReviewDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingCareReviewDownSQL); err != nil {
		return fmt.Errorf("drop the review of the monthly run: %w", err)
	}
	return nil
}
