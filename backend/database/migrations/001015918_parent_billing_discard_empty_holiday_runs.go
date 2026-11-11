package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const parentBillingDiscardEmptyHolidayRunsVersion = "1.15.918"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentBillingDiscardEmptyHolidayRunsVersion,
		Description: "Elternabrechnung: leere offene Ferienläufe verwerfen (#6628), Demo-Löschung nimmt geleerte Läufe mit (#6627)",
		DependsOn: []string{
			parentBillingDiscardEmptyRunsVersion, // the newest migration on development and the discard of #6626
			holidayClosureNoticesVersion,         // the erasure this one extends
		},
	})
	Migrations.MustRegister(parentBillingDiscardEmptyHolidayRunsUp, parentBillingDiscardEmptyHolidayRunsDown)
}

// parentBillingDiscardEmptyHolidayRunsUpSQL lets the Abrechnung discard an
// open holiday run that has nothing to issue (#6628), like the monthly and
// notice runs of 1.15.914:
//
//   - status "discarded" with discarded_at: the run stays in the list and in
//     its trail, nothing is deleted. Only a calculated run becomes
//     discarded; a released run never changes.
//   - A discarded run never changes either: the trigger of a released run
//     now guards it, and no due date of it is added or changed.
//   - A discarded run leaves uq_runs_calculated_period, so the period is
//     free for a new run.
//   - The trail action "discard" records who discarded the run and when.
const parentBillingDiscardEmptyHolidayRunsUpSQL = `
ALTER TABLE parent_billing.runs ADD COLUMN discarded_at TIMESTAMPTZ;
ALTER TABLE parent_billing.runs DROP CONSTRAINT chk_runs_status;
ALTER TABLE parent_billing.runs ADD CONSTRAINT chk_runs_status
	CHECK (status IN ('calculated', 'released', 'discarded'));
ALTER TABLE parent_billing.runs ADD CONSTRAINT chk_runs_discard
	CHECK ((status = 'discarded') = (discarded_at IS NOT NULL));
COMMENT ON COLUMN parent_billing.runs.discarded_at IS
	'When the Abrechnung discarded the open run without content (#6628); the trail names who.';
DROP TRIGGER refuse_parent_billing_released_run_change ON parent_billing.runs;
CREATE TRIGGER refuse_parent_billing_released_run_change
	BEFORE UPDATE ON parent_billing.runs
	FOR EACH ROW WHEN (OLD.status IN ('released', 'discarded'))
	EXECUTE FUNCTION parent_billing.refuse_issued_change();
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters', 'export', 'cancel', 'credit', 'discard'));

CREATE OR REPLACE FUNCTION parent_billing.refuse_released_due_date() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND NEW.set_by_account_id IS NULL
		AND to_jsonb(NEW) - 'set_by_account_id' = to_jsonb(OLD) - 'set_by_account_id' THEN
		RETURN NEW;
	END IF;
	IF EXISTS (SELECT 1 FROM parent_billing.care_runs WHERE organization_id = NEW.organization_id AND id = NEW.care_run_id AND status IN ('released', 'discarded'))
		OR EXISTS (SELECT 1 FROM parent_billing.runs WHERE organization_id = NEW.organization_id AND id = NEW.holiday_run_id AND status IN ('released', 'discarded')) THEN
		RAISE EXCEPTION 'the due dates of a released or discarded run never change' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
`

// The down migration has no discarded holiday run: it goes with its trail,
// as a run of a newer state cannot stay. The due dates of a discarded
// monthly or notice run of 1.15.914 stay guarded by the application only,
// as before.
const parentBillingDiscardEmptyHolidayRunsDownSQL = `
DELETE FROM parent_billing.runs WHERE status = 'discarded';
DELETE FROM parent_billing.run_events WHERE action = 'discard';
ALTER TABLE parent_billing.run_events DROP CONSTRAINT chk_run_events_action;
ALTER TABLE parent_billing.run_events ADD CONSTRAINT chk_run_events_action
	CHECK (action IN ('create', 'read', 'correct', 'release', 'letters', 'export', 'cancel', 'credit'));
DROP TRIGGER refuse_parent_billing_released_run_change ON parent_billing.runs;
CREATE TRIGGER refuse_parent_billing_released_run_change
	BEFORE UPDATE ON parent_billing.runs
	FOR EACH ROW WHEN (OLD.status = 'released')
	EXECUTE FUNCTION parent_billing.refuse_issued_change();
ALTER TABLE parent_billing.runs DROP CONSTRAINT chk_runs_discard;
ALTER TABLE parent_billing.runs DROP CONSTRAINT chk_runs_status;
ALTER TABLE parent_billing.runs ADD CONSTRAINT chk_runs_status CHECK (status IN ('calculated', 'released'));
ALTER TABLE parent_billing.runs DROP COLUMN discarded_at;

CREATE OR REPLACE FUNCTION parent_billing.refuse_released_due_date() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'UPDATE' AND NEW.set_by_account_id IS NULL
		AND to_jsonb(NEW) - 'set_by_account_id' = to_jsonb(OLD) - 'set_by_account_id' THEN
		RETURN NEW;
	END IF;
	IF EXISTS (SELECT 1 FROM parent_billing.care_runs WHERE organization_id = NEW.organization_id AND id = NEW.care_run_id AND status = 'released')
		OR EXISTS (SELECT 1 FROM parent_billing.runs WHERE organization_id = NEW.organization_id AND id = NEW.holiday_run_id AND status = 'released') THEN
		RAISE EXCEPTION 'the due dates of a released run never change' USING ERRCODE = '55000';
	END IF;
	RETURN NEW;
END;
$$;
`

func parentBillingDiscardEmptyHolidayRunsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, parentBillingDiscardEmptyHolidayRunsUpSQL); err != nil {
		return fmt.Errorf("let empty open holiday runs be discarded: %w", err)
	}
	return installEmptyRunErasure(ctx, db)
}

func parentBillingDiscardEmptyHolidayRunsDown(ctx context.Context, db *bun.DB) error {
	if err := installHolidayClosureErasure(ctx, db); err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, parentBillingDiscardEmptyHolidayRunsDownSQL); err != nil {
		return fmt.Errorf("drop the discarded holiday runs: %w", err)
	}
	return nil
}
