package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// traegerMemberDisplayNameVersion is the version of the Träger member name.
const traegerMemberDisplayNameVersion = "1.15.952"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerMemberDisplayNameVersion,
		Description: "Träger-Büro: Name je Mitgliedschaft für Kontenliste, Exportprotokoll und Rechnungsverlauf (#4693)",
		DependsOn: []string{
			demoVisitorErasureFamilyDebtorNumbersVersion, // the newest migration on development (1.15.951)
			traegerPortalVersion,                         // the Träger membership that carries the name
			traegerInvitationsVersion,                    // the account ledger that records a new name
		},
	})
	Migrations.MustRegister(traegerMemberDisplayNameUp, traegerMemberDisplayNameDown)
}

// traegerMemberDisplayNameUpSQL gives every Träger membership a name
// (#4693). A Träger-Büro account had only its address; the export protocol
// and the history of an invoice name who exported or acted, and colleagues
// without the right "Konten verwalten" may not read the addresses. The name
// belongs to the membership: one person may be named differently at two
// Träger, and the name goes with the membership when the account is erased
// or the Träger is deleted.
//
//   - platform.organization_accounts.display_name is the name. NULL until a
//     person sets it (memberships older than this migration); once set it is
//     trimmed and 1 to 100 characters long.
//   - audit.traeger_account_changes records a new name as 'name_changed' with
//     the name in target_name; the retention of the ledger deletes it with
//     the entry, like target_email.
//   - platform.traeger_member_names(organization, accounts) lets the Träger
//     transaction (tenant role, app.current_org_id) read the names of its own
//     Träger and nothing else: the role has no grant on the membership table.
const traegerMemberDisplayNameUpSQL = `
ALTER TABLE platform.organization_accounts ADD COLUMN display_name TEXT,
	ADD CONSTRAINT chk_organization_accounts_display_name CHECK (
		display_name IS NULL OR (display_name = btrim(display_name) AND char_length(display_name) BETWEEN 1 AND 100)
	);
COMMENT ON COLUMN platform.organization_accounts.display_name IS
	'Name of the person at this Träger (#4693), shown to colleagues of the same Träger in the account list, the export protocol and the invoice history; NULL until set.';

ALTER TABLE audit.traeger_account_changes ADD COLUMN target_name TEXT,
	DROP CONSTRAINT chk_traeger_account_changes_action,
	ADD CONSTRAINT chk_traeger_account_changes_action CHECK (action IN (
		'invitation_created', 'invitation_revoked', 'invitation_accepted',
		'roles_changed', 'member_removed', 'members_listed', 'name_changed'
	));
COMMENT ON COLUMN audit.traeger_account_changes.target_name IS
	'The name a name_changed or invitation_accepted entry set for the person (#4693).';

CREATE FUNCTION platform.traeger_member_names(p_organization_id BIGINT, p_account_ids BIGINT[])
RETURNS TABLE(account_id BIGINT, display_name TEXT)
LANGUAGE SQL STABLE SECURITY DEFINER
SET search_path = pg_catalog, platform
AS $function$
	SELECT membership.account_id, membership.display_name
	FROM platform.organization_accounts AS membership
	WHERE membership.organization_id = p_organization_id
		AND p_organization_id = NULLIF(current_setting('app.current_org_id', true), '')::bigint
		AND membership.account_id = ANY (p_account_ids)
		AND membership.display_name IS NOT NULL
$function$;
REVOKE ALL ON FUNCTION platform.traeger_member_names(BIGINT, BIGINT[]) FROM PUBLIC, phoenix_auth, phoenix_admin;
GRANT EXECUTE ON FUNCTION platform.traeger_member_names(BIGINT, BIGINT[]) TO phoenix_tenant;
`

// The rollback forgets the names: entries that record one go first, the
// action they use leaves the CHECK.
const traegerMemberDisplayNameDownSQL = `
DROP FUNCTION IF EXISTS platform.traeger_member_names(BIGINT, BIGINT[]);
DELETE FROM audit.traeger_account_changes WHERE action = 'name_changed';
ALTER TABLE audit.traeger_account_changes DROP CONSTRAINT chk_traeger_account_changes_action,
	ADD CONSTRAINT chk_traeger_account_changes_action CHECK (action IN (
		'invitation_created', 'invitation_revoked', 'invitation_accepted',
		'roles_changed', 'member_removed', 'members_listed'
	)),
	DROP COLUMN IF EXISTS target_name;
ALTER TABLE platform.organization_accounts DROP COLUMN IF EXISTS display_name;
`

func traegerMemberDisplayNameUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerMemberDisplayNameUpSQL); err != nil {
		return fmt.Errorf("add the name of a Träger membership: %w", err)
	}
	return nil
}

func traegerMemberDisplayNameDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerMemberDisplayNameDownSQL); err != nil {
		return fmt.Errorf("drop the name of a Träger membership: %w", err)
	}
	return nil
}
