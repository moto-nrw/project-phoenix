package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const guardianInvitationAddressBindingVersion = "1.15.865"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationAddressBindingVersion,
		Description: "Elterneinladungen an die E-Mail-Adresse binden, an die der Link ging; offene Altlinks ohne belegte Empfängeradresse beenden (#6294)",
		DependsOn: []string{
			declarationAlwaysRevocableVersion, // the last migration before this one
		},
	})
	Migrations.MustRegister(guardianInvitationAddressBindingUp, guardianInvitationAddressBindingDown)
}

// guardianInvitationAddressBindingUpSQL binds a guardian invitation to the
// address its link was mailed to (#6294). The application writes the column
// with every new token and accepts a link only while the profile still has
// that address, so a link mailed to a mistyped address cannot create the
// account of the corrected one.
//
// Legacy rows do not record the recipient. Neither updated_at proves it:
// token hashing in 1.15.521 advances invitation timestamps, and now() records
// transaction start rather than commit order. Every redeemable legacy link
// without an address therefore ends; staff must issue a new invitation. A
// request awaiting approval stays open with no address: approval issues and
// mails a fresh link. Spent and expired links also keep no address.
//
// Only unbound links are ended, so a rerun preserves links issued by the
// application with a stored recipient.
const guardianInvitationAddressBindingUpSQL = `
ALTER TABLE auth.guardian_invitations ADD COLUMN IF NOT EXISTS email TEXT;
COMMENT ON COLUMN auth.guardian_invitations.email IS
	'Normalized address the link in token was issued for; the link redeems only while the guardian profile still has it (#6294).';

UPDATE auth.guardian_invitations AS invitation
	SET expires_at = statement_timestamp()
	WHERE invitation.email IS NULL
		AND invitation.accepted_at IS NULL
		AND invitation.expires_at > statement_timestamp()
		AND invitation.approval_status IN ('not_required', 'approved');
`

// guardianInvitationAddressBindingDownSQL ends what the binding had already
// ended before the column goes: without it a link follows the profile again,
// so an open link whose address the profile no longer has would come back
// to life for the new address. A second Down finds the column gone.
const guardianInvitationAddressBindingDownSQL = `
DO $down$
BEGIN
	IF EXISTS (SELECT 1 FROM information_schema.columns
		WHERE table_schema = 'auth' AND table_name = 'guardian_invitations' AND column_name = 'email') THEN
		UPDATE auth.guardian_invitations AS invitation
			SET expires_at = statement_timestamp()
			FROM users.guardian_profiles AS profile
			WHERE profile.id = invitation.guardian_profile_id
				AND invitation.accepted_at IS NULL
				AND invitation.expires_at > statement_timestamp()
				AND invitation.approval_status IN ('not_required', 'approved')
				AND invitation.email IS DISTINCT FROM LOWER(BTRIM(profile.email, E' \t\r\n'));
	END IF;
END
$down$;
ALTER TABLE auth.guardian_invitations DROP COLUMN IF EXISTS email;
`

func guardianInvitationAddressBindingUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, guardianInvitationAddressBindingUpSQL); err != nil {
		return fmt.Errorf("bind guardian invitations to their address: %w", err)
	}
	return nil
}

func guardianInvitationAddressBindingDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, guardianInvitationAddressBindingDownSQL); err != nil {
		return fmt.Errorf("drop the address of guardian invitations: %w", err)
	}
	return nil
}
