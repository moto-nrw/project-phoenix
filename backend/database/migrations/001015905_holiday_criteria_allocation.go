package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayCriteriaAllocationVersion = "1.15.905"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayCriteriaAllocationVersion,
		Description: "Ferien: Platzvergabe nach Kriterien mit Entscheidung durch das Büro (#4700)",
		DependsOn: []string{
			"1.15.904",                 // the newest migration on development
			holidayCareWishVersion,     // the audit action 'wish' this one keeps
			holidayWeekCapacityVersion, // the placement this one extends
		},
	})
	Migrations.MustRegister(holidayCriteriaAllocationUp, holidayCriteriaAllocationDown)
}

// holidayCriteriaAllocationUpSQL lets a Träger allocate the places of a
// holiday period by criteria, decided by the Träger-Büro (#4700, decision
// of 09.11. model A):
//
//   - holiday_periods.allocation_mode: "arrival" gives the places in the
//     order the registrations come in (#4699); "criteria" gives none when the
//     parents send the form. The application changes it only while the
//     period has no registration.
//   - holiday_registration_bookings.placement "pending": a booking that waits
//     for the decision of the Büro. It takes no place, is not billed and does
//     not reach the host school; while a week has one, no booking moves up
//     into it automatically.
//   - holiday_allocation_criteria: the criteria of a Träger with their
//     priority (position, 1 first). Removing one archives it, so answers and
//     decisions keep naming it. They hold no personal data.
//   - holiday_registration_criteria: the parents' yes or no to a criterion,
//     for the family or one child, and the Büro's check of it. Only the
//     Träger-Büro reads them; they go with their registration (retention
//     and demo erasure delete the registrations they cascade from).
//   - holiday_allocation_decisions: every decision of the Büro with the
//     proposal, the rank and the criteria met at that moment, and the
//     reason of a decision that deviates from the proposal. They go with
//     their registration.
//   - holiday_registration_audit: 'allocate' and 'criteria_check' of a
//     registration, 'allocation' for a read of the allocation list of a
//     period.
const holidayCriteriaAllocationUpSQL = `
ALTER TABLE enrollment.holiday_periods ADD COLUMN IF NOT EXISTS allocation_mode TEXT NOT NULL DEFAULT 'arrival';
ALTER TABLE enrollment.holiday_periods DROP CONSTRAINT IF EXISTS chk_holiday_periods_allocation_mode;
ALTER TABLE enrollment.holiday_periods ADD CONSTRAINT chk_holiday_periods_allocation_mode
	CHECK (allocation_mode IN ('arrival', 'criteria'));
COMMENT ON COLUMN enrollment.holiday_periods.allocation_mode IS
	'arrival: places in the order the registrations come in (#4699); criteria: the Träger-Büro decides on each place, sorted by the criteria of the Träger (#4700).';

ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_placement;
ALTER TABLE enrollment.holiday_registration_bookings ADD CONSTRAINT chk_holiday_registration_bookings_placement
	CHECK (placement IN ('placed', 'waitlisted', 'pending'));
COMMENT ON COLUMN enrollment.holiday_registration_bookings.placement IS
	'placed: the child has a place in the week; waitlisted: it waits for one (#4699); pending: the Träger-Büro has not decided yet (#4700). Only placed bookings reach the host school and the contribution.';

CREATE TABLE enrollment.holiday_allocation_criteria (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	name TEXT NOT NULL,
	question TEXT NOT NULL,
	scope TEXT NOT NULL,
	position SMALLINT NOT NULL,
	active BOOLEAN NOT NULL DEFAULT true,
	archived_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	CONSTRAINT uq_holiday_allocation_criteria_organization UNIQUE (organization_id, id),
	CONSTRAINT chk_holiday_allocation_criteria_texts CHECK (
		BTRIM(name) <> '' AND char_length(name) <= 60 AND BTRIM(question) <> '' AND char_length(question) <= 300),
	CONSTRAINT chk_holiday_allocation_criteria_scope CHECK (scope IN ('family', 'child')),
	CONSTRAINT chk_holiday_allocation_criteria_position CHECK (position BETWEEN 1 AND 100)
);
CREATE INDEX idx_holiday_allocation_criteria_organization
	ON enrollment.holiday_allocation_criteria (organization_id, position) WHERE archived_at IS NULL;
COMMENT ON TABLE enrollment.holiday_allocation_criteria IS
	'Criteria of a Träger for the allocation of holiday places, position 1 first (#4700). Removing one archives it.';

CREATE TABLE enrollment.holiday_registration_criteria (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT,
	criterion_id BIGINT NOT NULL,
	stated BOOLEAN,
	checked BOOLEAN,
	checked_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	checked_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT fk_holiday_registration_criteria_registration
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_registration_criteria_child
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_child_id)
		REFERENCES enrollment.holiday_registration_children (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_registration_criteria_criterion
		FOREIGN KEY (organization_id, criterion_id)
		REFERENCES enrollment.holiday_allocation_criteria (organization_id, id) ON DELETE CASCADE,
	CONSTRAINT uq_holiday_registration_criteria_answer
		UNIQUE NULLS NOT DISTINCT (holiday_registration_id, criterion_id, holiday_registration_child_id),
	CONSTRAINT chk_holiday_registration_criteria_value CHECK (stated IS NOT NULL OR checked IS NOT NULL),
	CONSTRAINT chk_holiday_registration_criteria_check CHECK ((checked IS NULL) = (checked_at IS NULL))
);
CREATE INDEX idx_holiday_registration_criteria_period
	ON enrollment.holiday_registration_criteria (organization_id, holiday_period_id);
CREATE INDEX idx_holiday_registration_criteria_child
	ON enrollment.holiday_registration_criteria (holiday_registration_child_id) WHERE holiday_registration_child_id IS NOT NULL;
COMMENT ON TABLE enrollment.holiday_registration_criteria IS
	'Parents'' yes or no to a criterion of the Träger, for the family or one child, and the check of the Träger-Büro (#4700). Only the Büro reads it; it goes with its registration.';

CREATE TABLE enrollment.holiday_allocation_decisions (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL,
	holiday_period_id BIGINT NOT NULL,
	holiday_registration_id BIGINT NOT NULL,
	holiday_registration_child_id BIGINT NOT NULL,
	holiday_period_week_id BIGINT NOT NULL,
	week_starts_on DATE NOT NULL,
	week_ends_on DATE NOT NULL,
	placement TEXT NOT NULL,
	proposal TEXT NOT NULL,
	rank INTEGER NOT NULL,
	criteria_met TEXT[] NOT NULL DEFAULT '{}',
	reason TEXT NOT NULL DEFAULT '',
	decided_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	decided_at TIMESTAMPTZ NOT NULL DEFAULT statement_timestamp(),
	CONSTRAINT fk_holiday_allocation_decisions_registration
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_id)
		REFERENCES enrollment.holiday_registrations (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT fk_holiday_allocation_decisions_child
		FOREIGN KEY (organization_id, holiday_period_id, holiday_registration_child_id)
		REFERENCES enrollment.holiday_registration_children (organization_id, holiday_period_id, id) ON DELETE CASCADE,
	CONSTRAINT chk_holiday_allocation_decisions_placement CHECK (
		placement IN ('placed', 'waitlisted') AND proposal IN ('placed', 'waitlisted') AND rank >= 1),
	CONSTRAINT chk_holiday_allocation_decisions_reason CHECK (
		char_length(reason) <= 500 AND (placement = proposal OR BTRIM(reason) <> ''))
);
CREATE INDEX idx_holiday_allocation_decisions_period
	ON enrollment.holiday_allocation_decisions (organization_id, holiday_period_id, decided_at DESC);
CREATE INDEX idx_holiday_allocation_decisions_registration
	ON enrollment.holiday_allocation_decisions (holiday_registration_id);
COMMENT ON TABLE enrollment.holiday_allocation_decisions IS
	'Decision of the Träger-Büro on a holiday place with the proposal, rank and criteria met at that moment, and the reason of a deviation (#4700). It goes with its registration.';

ALTER TABLE enrollment.holiday_allocation_criteria ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_allocation_criteria FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_allocation_criteria ON enrollment.holiday_allocation_criteria FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_registration_criteria ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_registration_criteria FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_registration_criteria ON enrollment.holiday_registration_criteria FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);
ALTER TABLE enrollment.holiday_allocation_decisions ENABLE ROW LEVEL SECURITY;
ALTER TABLE enrollment.holiday_allocation_decisions FORCE ROW LEVEL SECURITY;
CREATE POLICY traeger_isolation_enrollment_holiday_allocation_decisions ON enrollment.holiday_allocation_decisions FOR ALL
	USING (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint)
	WITH CHECK (organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint);

REVOKE ALL ON enrollment.holiday_allocation_criteria, enrollment.holiday_registration_criteria,
	enrollment.holiday_allocation_decisions FROM PUBLIC, phoenix_tenant;
GRANT SELECT, INSERT, UPDATE ON enrollment.holiday_allocation_criteria TO phoenix_tenant;
GRANT SELECT, INSERT, UPDATE, DELETE ON enrollment.holiday_registration_criteria TO phoenix_tenant;
GRANT SELECT, INSERT ON enrollment.holiday_allocation_decisions TO phoenix_tenant;
GRANT USAGE ON SEQUENCE enrollment.holiday_allocation_criteria_id_seq,
	enrollment.holiday_registration_criteria_id_seq,
	enrollment.holiday_allocation_decisions_id_seq TO phoenix_tenant;

ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action,
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_subject;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number', 'edit',
		'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel',
		'promote', 'capacity', 'places', 'wish', 'allocate', 'allocation', 'criteria_check')),
	ADD CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export', 'capacity', 'places', 'allocation')) = (holiday_registration_id IS NULL));
`

// holidayCriteriaAllocationDownSQL puts every pending booking on the waiting
// list, where the order of arrival takes over, and drops the criteria, the
// answers and the decisions.
const holidayCriteriaAllocationDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action IN ('allocate', 'allocation', 'criteria_check');
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action,
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_subject;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number', 'edit',
		'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel',
		'promote', 'capacity', 'places', 'wish')),
	ADD CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export', 'capacity', 'places')) = (holiday_registration_id IS NULL));
DROP TABLE IF EXISTS enrollment.holiday_allocation_decisions;
DROP TABLE IF EXISTS enrollment.holiday_registration_criteria;
DROP TABLE IF EXISTS enrollment.holiday_allocation_criteria;
UPDATE enrollment.holiday_registration_bookings SET placement = 'waitlisted' WHERE placement = 'pending';
ALTER TABLE enrollment.holiday_registration_bookings DROP CONSTRAINT IF EXISTS chk_holiday_registration_bookings_placement;
ALTER TABLE enrollment.holiday_registration_bookings ADD CONSTRAINT chk_holiday_registration_bookings_placement
	CHECK (placement IN ('placed', 'waitlisted'));
COMMENT ON COLUMN enrollment.holiday_registration_bookings.placement IS
	'placed: the child has a place in the week; waitlisted: it waits for one (#4699). Only placed bookings reach the host school and the contribution.';
ALTER TABLE enrollment.holiday_periods DROP CONSTRAINT IF EXISTS chk_holiday_periods_allocation_mode;
ALTER TABLE enrollment.holiday_periods DROP COLUMN IF EXISTS allocation_mode;
`

func holidayCriteriaAllocationUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayCriteriaAllocationUpSQL); err != nil {
		return fmt.Errorf("add holiday allocation by criteria: %w", err)
	}
	return nil
}

func holidayCriteriaAllocationDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayCriteriaAllocationDownSQL); err != nil {
		return fmt.Errorf("drop holiday allocation by criteria: %w", err)
	}
	return nil
}
