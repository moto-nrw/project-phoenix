package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerWithdrawalContactVersion = "1.15.815"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerWithdrawalContactVersion,
		Description: "Träger-Portal: eigene Telefonnummer und E-Mail-Adresse für Widerrufe in den Stammdaten (#6200)",
		DependsOn: []string{
			traegerDocumentTemplatesVersion, // the audit subjects this one extends
		},
	})
	Migrations.MustRegister(traegerWithdrawalContactUp, traegerWithdrawalContactDown)
}

// traegerWithdrawalContactUpSQL gives the Träger its own phone number and
// e-mail address for withdrawals (#6200). The withdrawal notice of a care
// contract names the addressee with name, address, phone and e-mail
// (Gestaltungshinweis 2 zu Anlage 1 zu Art. 246a EGBGB). Empty columns take
// the contact person's phone and e-mail, so a Träger without its own entry
// prints what it printed before. The log admits the subject 'withdrawal'.
const traegerWithdrawalContactUpSQL = `
ALTER TABLE platform.traeger_profiles
	ADD COLUMN withdrawal_phone TEXT NOT NULL DEFAULT '',
	ADD COLUMN withdrawal_email TEXT NOT NULL DEFAULT '',
	ADD CONSTRAINT chk_traeger_profiles_withdrawal CHECK (
		char_length(withdrawal_phone) <= 40 AND char_length(withdrawal_email) <= 254
	);
COMMENT ON COLUMN platform.traeger_profiles.withdrawal_phone IS
	'Phone number the withdrawal notice names (#6200); empty takes contact_phone.';
COMMENT ON COLUMN platform.traeger_profiles.withdrawal_email IS
	'E-mail address the withdrawal notice names (#6200); empty takes contact_email.';

ALTER TABLE platform.traeger_master_data_audit DROP CONSTRAINT chk_traeger_master_data_audit_subject;
ALTER TABLE platform.traeger_master_data_audit ADD CONSTRAINT chk_traeger_master_data_audit_subject
	CHECK (subject IN ('master_data', 'contact', 'logo', 'bank_account', 'template', 'withdrawal'));
`

const traegerWithdrawalContactDownSQL = `
DELETE FROM platform.traeger_master_data_audit WHERE subject = 'withdrawal';
ALTER TABLE platform.traeger_master_data_audit DROP CONSTRAINT chk_traeger_master_data_audit_subject;
ALTER TABLE platform.traeger_master_data_audit ADD CONSTRAINT chk_traeger_master_data_audit_subject
	CHECK (subject IN ('master_data', 'contact', 'logo', 'bank_account', 'template'));
ALTER TABLE platform.traeger_profiles
	DROP CONSTRAINT IF EXISTS chk_traeger_profiles_withdrawal,
	DROP COLUMN IF EXISTS withdrawal_email,
	DROP COLUMN IF EXISTS withdrawal_phone;
`

func traegerWithdrawalContactUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerWithdrawalContactUpSQL); err != nil {
		return fmt.Errorf("add Träger withdrawal contact: %w", err)
	}
	return nil
}

func traegerWithdrawalContactDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerWithdrawalContactDownSQL); err != nil {
		return fmt.Errorf("drop Träger withdrawal contact: %w", err)
	}
	return nil
}
