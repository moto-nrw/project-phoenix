package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const guardianInvitationAddressBindingVersion = "1.15.832"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     guardianInvitationAddressBindingVersion,
		Description: "Elterneinladungen an die E-Mail-Adresse binden, an die der Link ging; offene Links nach einer Profiländerung beenden (#6294)",
		DependsOn: []string{
			accountCredentialGenerationBindingVersion, // the last migration before this one
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
// Before the column existed the link followed the profile. Which address a
// still open link went to is no longer known when the profile changed after
// the link was issued (guardian_profiles.updated_at after
// guardian_invitations.updated_at): those links end now, and a resend issues
// a new invitation goes to the address on file. That is at most the 168 hours
// a link lives. A request still waiting for a staff decision keeps going: its
// link was never mailed, and the approval mails a new one. Every other open
// link takes the address the profile has now, the one it was mailed to. A
// spent or expired link gets no address: nothing reads it any more.
//
// Addresses are compared like the application does (lower case, without
// surrounding blanks, tabs or line breaks). Both steps touch only rows
// without an address, so a second run changes nothing. The ending runs
// first: the backfill moves updated_at on.
const guardianInvitationAddressBindingUpSQL = `
ALTER TABLE auth.guardian_invitations ADD COLUMN IF NOT EXISTS email TEXT;
COMMENT ON COLUMN auth.guardian_invitations.email IS
	'Normalized address the link in token was issued for; the link redeems only while the guardian profile still has it (#6294).';

UPDATE auth.guardian_invitations AS invitation
	SET expires_at = statement_timestamp()
	FROM users.guardian_profiles AS profile
	WHERE profile.id = invitation.guardian_profile_id
		AND invitation.email IS NULL
		AND invitation.accepted_at IS NULL
		AND invitation.expires_at > statement_timestamp()
		AND invitation.approval_status IN ('not_required', 'approved')
		AND (profile.updated_at > invitation.updated_at OR NULLIF(BTRIM(profile.email, E' \t\r\n'), '') IS NULL);

UPDATE auth.guardian_invitations AS invitation
	SET email = LOWER(BTRIM(profile.email, E' \t\r\n'))
	FROM users.guardian_profiles AS profile
	WHERE profile.id = invitation.guardian_profile_id
		AND invitation.email IS NULL
		AND invitation.accepted_at IS NULL
		AND invitation.expires_at > statement_timestamp()
		AND NULLIF(BTRIM(profile.email, E' \t\r\n'), '') IS NOT NULL;
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
