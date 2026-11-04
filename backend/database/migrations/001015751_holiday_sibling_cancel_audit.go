package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidaySiblingCancelAuditVersion = "1.15.751"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidaySiblingCancelAuditVersion,
		Description: "Ferienanmeldung: Verlaufseintrag, wenn eine Stornierung die Geschwistergruppe löst (#5662)",
		DependsOn: []string{
			attendanceCheckOutReasonVersion, // the last migration before this one
			holidaySiblingGroupsVersion,     // its audit actions this keeps
		},
	})
	Migrations.MustRegister(holidaySiblingCancelAuditUp, holidaySiblingCancelAuditDown)
}

// holidaySiblingCancelAuditUpSQL lets enrollment.holiday_registration_audit
// take the action sibling_cancel (#5662): a cancellation took a registration
// out of its sibling group, and the Elternabrechnung computed the rest of
// the group again. Every action of 1.15.746 stays; the down migration
// restores its list.
const holidaySiblingCancelAuditUpSQL = `
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number',
			'edit', 'proof_upload', 'sibling_merge', 'sibling_split', 'sibling_cancel'));
`

const holidaySiblingCancelAuditDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'sibling_cancel';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'debtor_number',
			'edit', 'proof_upload', 'sibling_merge', 'sibling_split'));
`

func holidaySiblingCancelAuditUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySiblingCancelAuditUpSQL); err != nil {
		return fmt.Errorf("add the audit action sibling_cancel: %w", err)
	}
	return nil
}

func holidaySiblingCancelAuditDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidaySiblingCancelAuditDownSQL); err != nil {
		return fmt.Errorf("drop the audit action sibling_cancel: %w", err)
	}
	return nil
}
