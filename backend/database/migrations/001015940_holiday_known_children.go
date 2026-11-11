package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayKnownChildrenVersion = "1.15.940"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayKnownChildrenVersion,
		Description: "Ferien: Hinweis auf ein an einer Schule des Trägers bekanntes Kind protokollieren (#6683)",
		DependsOn: []string{
			messageAttachmentsVersion,        // the newest migration on development
			holidayCriteriaAllocationVersion, // the audit CHECKs this one extends
		},
	})
	Migrations.MustRegister(holidayKnownChildrenUp, holidayKnownChildrenDown)
}

// holidayKnownChildrenUpSQL admits 'known_children' to the audit trail of
// the holiday registrations (#6683, ADR 0214): the Träger-Büro compared the
// registered children of a period with the children of its schools. The
// entry belongs to the period, like the list; its details name the
// registration asked for and the IDs of the children found.
const holidayKnownChildrenUpSQL = `
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action,
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_subject;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number', 'edit',
		'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel',
		'promote', 'capacity', 'places', 'wish', 'allocate', 'allocation', 'criteria_check', 'known_children')),
	ADD CONSTRAINT chk_holiday_registration_audit_subject CHECK (
		(action IN ('list', 'export', 'capacity', 'places', 'allocation', 'known_children')) = (holiday_registration_id IS NULL));
`

// holidayKnownChildrenDownSQL deletes the entries of the comparison and
// writes back the CHECKs of 1.15.907.
const holidayKnownChildrenDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'known_children';
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

func holidayKnownChildrenUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayKnownChildrenUpSQL); err != nil {
		return fmt.Errorf("admit the known children to the holiday registration audit: %w", err)
	}
	return nil
}

func holidayKnownChildrenDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayKnownChildrenDownSQL); err != nil {
		return fmt.Errorf("remove the known children from the holiday registration audit: %w", err)
	}
	return nil
}
