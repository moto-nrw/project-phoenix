package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDiscardEmptyRunsVersion = "1.15.914"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDiscardEmptyRunsVersion,
		Description: "Elternabrechnung: leere offene Monats- und Jahresläufe verwerfen (#6626)",
		DependsOn: []string{
			holidayCriteriaAllocationVersion,      // the newest migration on development
			parentBillingCareReviewVersion,        // the trail actions and run schools of the monthly run
			parentBillingCareNoticeLettersVersion, // the trail actions of the notice run
		},
	})
	Migrations.MustRegister(parentBillingDiscardEmptyRunsUp, parentBillingDiscardEmptyRunsDown)
}

// parentBillingDiscardEmptyRunsUpSQL lets the Abrechnung discard an open
// monthly or notice run that has nothing to issue (#6626):
//
//   - status "discarded" with discarded_at: the run stays in the list and in
//     its trail, nothing is deleted. Only a calculated run becomes discarded;
//     a released run never changes (refuse_released_care_run_change).
//   - A discarded run never changes either: the same trigger now guards it.
//   - The schools of a discarded monthly run close (care_run_schools.open),
//     and a discarded notice run leaves uq_care_notice_runs_calculated_year,
//     so the month or year is free for a new run.
//   - The trail action "discard" records who discarded the run and when.
const parentBillingDiscardEmptyRunsUpSQL = `
ALTER TABLE parent_billing.care_runs ADD COLUMN discarded_at TIMESTAMPTZ;
ALTER TABLE parent_billing.care_runs DROP CONSTRAINT chk_care_runs_status;
ALTER TABLE parent_billing.care_runs ADD CONSTRAINT chk_care_runs_status
	CHECK (status IN ('calculated', 'released', 'discarded'));
ALTER TABLE parent_billing.care_runs ADD CONSTRAINT chk_care_runs_discard
	CHECK ((status = 'discarded') = (discarded_at IS NOT NULL));
COMMENT ON COLUMN parent_billing.care_runs.discarded_at IS
	'When the Abrechnung discarded the open run without content (#6626); the trail names who.';
DROP TRIGGER refuse_parent_billing_released_care_run_change ON parent_billing.care_runs;
CREATE TRIGGER refuse_parent_billing_released_care_run_change
	BEFORE UPDATE ON parent_billing.care_runs
	FOR EACH ROW WHEN (OLD.status IN ('released', 'discarded'))
	EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letters', 'mail', 'cancel', 'exclude', 'include', 'export', 'invoice_date', 'discard'));

ALTER TABLE parent_billing.care_notice_runs ADD COLUMN discarded_at TIMESTAMPTZ;
ALTER TABLE parent_billing.care_notice_runs DROP CONSTRAINT chk_care_notice_runs_status;
ALTER TABLE parent_billing.care_notice_runs ADD CONSTRAINT chk_care_notice_runs_status
	CHECK (status IN ('calculated', 'released', 'discarded'));
ALTER TABLE parent_billing.care_notice_runs ADD CONSTRAINT chk_care_notice_runs_discard
	CHECK ((status = 'discarded') = (discarded_at IS NOT NULL));
COMMENT ON COLUMN parent_billing.care_notice_runs.discarded_at IS
	'When the Abrechnung discarded the open run without content (#6626); the trail names who.';
DROP TRIGGER refuse_parent_billing_released_care_notice_run_change ON parent_billing.care_notice_runs;
CREATE TRIGGER refuse_parent_billing_released_care_notice_run_change
	BEFORE UPDATE ON parent_billing.care_notice_runs
	FOR EACH ROW WHEN (OLD.status IN ('released', 'discarded'))
	EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
ALTER TABLE parent_billing.care_notice_run_events DROP CONSTRAINT chk_care_notice_run_events_action;
ALTER TABLE parent_billing.care_notice_run_events ADD CONSTRAINT chk_care_notice_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letter', 'letters', 'mail', 'discard'));
`

// The down migration has no discarded state: a discarded run goes with its
// trail, as a run of a newer state cannot stay.
const parentBillingDiscardEmptyRunsDownSQL = `
DELETE FROM parent_billing.care_runs WHERE status = 'discarded';
DELETE FROM parent_billing.care_run_events WHERE action = 'discard';
ALTER TABLE parent_billing.care_run_events DROP CONSTRAINT chk_care_run_events_action;
ALTER TABLE parent_billing.care_run_events ADD CONSTRAINT chk_care_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letters', 'mail', 'cancel', 'exclude', 'include', 'export', 'invoice_date'));
DROP TRIGGER refuse_parent_billing_released_care_run_change ON parent_billing.care_runs;
CREATE TRIGGER refuse_parent_billing_released_care_run_change
	BEFORE UPDATE ON parent_billing.care_runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
ALTER TABLE parent_billing.care_runs DROP CONSTRAINT chk_care_runs_discard;
ALTER TABLE parent_billing.care_runs DROP CONSTRAINT chk_care_runs_status;
ALTER TABLE parent_billing.care_runs ADD CONSTRAINT chk_care_runs_status CHECK (status IN ('calculated', 'released'));
ALTER TABLE parent_billing.care_runs DROP COLUMN discarded_at;

DELETE FROM parent_billing.care_notice_runs WHERE status = 'discarded';
DELETE FROM parent_billing.care_notice_run_events WHERE action = 'discard';
ALTER TABLE parent_billing.care_notice_run_events DROP CONSTRAINT chk_care_notice_run_events_action;
ALTER TABLE parent_billing.care_notice_run_events ADD CONSTRAINT chk_care_notice_run_events_action
	CHECK (action IN ('create', 'read', 'release', 'letter', 'letters', 'mail'));
DROP TRIGGER refuse_parent_billing_released_care_notice_run_change ON parent_billing.care_notice_runs;
CREATE TRIGGER refuse_parent_billing_released_care_notice_run_change
	BEFORE UPDATE ON parent_billing.care_notice_runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_released_care_run_change();
ALTER TABLE parent_billing.care_notice_runs DROP CONSTRAINT chk_care_notice_runs_discard;
ALTER TABLE parent_billing.care_notice_runs DROP CONSTRAINT chk_care_notice_runs_status;
ALTER TABLE parent_billing.care_notice_runs ADD CONSTRAINT chk_care_notice_runs_status CHECK (status IN ('calculated', 'released'));
ALTER TABLE parent_billing.care_notice_runs DROP COLUMN discarded_at;
`

func parentBillingDiscardEmptyRunsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDiscardEmptyRunsUpSQL); err != nil {
		return fmt.Errorf("let empty open runs be discarded: %w", err)
	}
	return nil
}

func parentBillingDiscardEmptyRunsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDiscardEmptyRunsDownSQL); err != nil {
		return fmt.Errorf("drop the discarded runs: %w", err)
	}
	return nil
}
