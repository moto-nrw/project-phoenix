package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const holidayRegistrationDeclarationVersion = "1.15.737"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     holidayRegistrationDeclarationVersion,
		Description: "Ferienanmeldung: Sorgerecht und Name als Erklärung speichern (#5626)",
		DependsOn: []string{
			unusedAccountErasureWithoutWaitVersion, // the last migration before this one
			holidayHealthDataVersion,               // the consent version this extends
		},
	})
	Migrations.MustRegister(holidayRegistrationDeclarationUp, holidayRegistrationDeclarationDown)
}

// A holiday registration under the consent texts of version 3 (#5626) keeps
// when the parent confirmed the custody and the name the parent typed to give
// the declarations. Both are required from version 3 on; registrations
// before keep NULL and work as before. Both go with the registration: the
// retention after 92 days (ADR 0118) and the erasure of the Träger delete
// the row.
const holidayRegistrationDeclarationUpSQL = `
ALTER TABLE enrollment.holiday_registrations
	ADD COLUMN custody_confirmed_at TIMESTAMPTZ,
	ADD COLUMN declaration_name TEXT,
	ADD CONSTRAINT chk_holiday_registrations_declaration_name
		CHECK (declaration_name IS NULL OR char_length(declaration_name) BETWEEN 1 AND 100),
	ADD CONSTRAINT chk_holiday_registrations_declaration
		CHECK (consent_version IS NULL OR consent_version < 3
			OR (custody_confirmed_at IS NOT NULL AND declaration_name IS NOT NULL));
COMMENT ON COLUMN enrollment.holiday_registrations.custody_confirmed_at IS
	'When the parent confirmed sole custody or the consent of the other custodian (#5626); NULL before consent version 3.';
COMMENT ON COLUMN enrollment.holiday_registrations.declaration_name IS
	'Name the parent typed to give the declarations of the registration (#5626); no signature. NULL before consent version 3.';
`

const holidayRegistrationDeclarationDownSQL = `
ALTER TABLE enrollment.holiday_registrations
	DROP CONSTRAINT IF EXISTS chk_holiday_registrations_declaration,
	DROP CONSTRAINT IF EXISTS chk_holiday_registrations_declaration_name,
	DROP COLUMN IF EXISTS declaration_name,
	DROP COLUMN IF EXISTS custody_confirmed_at;
`

func holidayRegistrationDeclarationUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationDeclarationUpSQL); err != nil {
		return fmt.Errorf("add the declaration of the holiday registrations: %w", err)
	}
	return nil
}

// holidayRegistrationDeclarationDown drops the declaration. A registration
// under version 3 then keeps only its consent version.
func holidayRegistrationDeclarationDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, holidayRegistrationDeclarationDownSQL); err != nil {
		return fmt.Errorf("drop the declaration of the holiday registrations: %w", err)
	}
	return nil
}
