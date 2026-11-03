package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationEditVersion = "1.15.741"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationEditVersion,
		Description: "Ferienanmeldung: das Träger-Büro ändert die Angaben einer Anmeldung, der Verlauf hält es fest (#5629)",
		DependsOn: []string{
			holidayRegistrationDeclarationVersion, // the last migration before this one
			holidayHealthDataVersion,              // the audit actions this extends
		},
	})
	Migrations.MustRegister(holidayRegistrationEditUp, holidayRegistrationEditDown)
}

// holidayRegistrationEditUpSQL admits the action 'edit' in the audit trail
// of a holiday registration: the Träger-Büro changed the details the family
// entered (#5629). The entry names the changed fields, never their values.
const holidayRegistrationEditUpSQL = `
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health', 'edit'));
`

// holidayRegistrationEditDownSQL restores the actions of 1.15.725. The
// entries of a change of the details go with it; the details stay changed.
const holidayRegistrationEditDownSQL = `
DELETE FROM enrollment.holiday_registration_audit WHERE action = 'edit';
ALTER TABLE enrollment.holiday_registration_audit
	DROP CONSTRAINT IF EXISTS chk_holiday_registration_audit_action;
ALTER TABLE enrollment.holiday_registration_audit
	ADD CONSTRAINT chk_holiday_registration_audit_action CHECK (
		action IN ('list', 'read', 'export', 'confirm', 'cancel', 'rebook', 'freeze', 'reopen', 'proof', 'health'));
`

func holidayRegistrationEditUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationEditUpSQL); err != nil {
		return fmt.Errorf("admit the holiday registration audit action edit: %w", err)
	}
	return nil
}

func holidayRegistrationEditDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationEditDownSQL); err != nil {
		return fmt.Errorf("remove the holiday registration audit action edit: %w", err)
	}
	return nil
}
