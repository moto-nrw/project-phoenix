package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerInvitationsVersion = "1.15.541"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerInvitationsVersion,
		Description: "Träger-Büro: Einladungen und Protokoll der Kontenverwaltung (#4650)",
		DependsOn: []string{
			accountAnalyticsConsentVersion, // the last migration before this one
			traegerPortalVersion,           // the Träger membership and roles the invitation grants
		},
	})
	Migrations.MustRegister(traegerInvitationsUp, traegerInvitationsDown)
}

// traegerInvitationsUpSQL adds the invitation into a Träger-Büro and its
// account ledger (ADR 0084 section 4, #4650):
//
//   - auth.traeger_invitations is the one-time link a person with the right
//     "Konten verwalten" mails to a new colleague (Identity & Access). The
//     token column holds the hex SHA-256 of the link token only, like every
//     stored token (#4902). An invitation carries the Träger roles the
//     acceptance grants. At most one invitation per Träger and address is
//     open; a new one spends the old one.
//   - audit.traeger_account_changes records who invited, revoked, accepted,
//     changed roles, removed a person or read the list of the Träger-Büro
//     (Audit Platform). The runtime role appends; only the hourly retention
//     deletes, with the cutoff of the operator ledger.
//
// Like the membership and role tables neither has a tenant_id: the Träger is
// no tenant and gets no RLS policy family (ADR 0084). Only the administrative
// role reaches them, and both go with the Träger.
const traegerInvitationsUpSQL = `
CREATE TABLE auth.traeger_invitations (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	email TEXT NOT NULL,
	token TEXT NOT NULL,
	roles VARCHAR(32)[] NOT NULL,
	expires_at TIMESTAMPTZ NOT NULL,
	used_at TIMESTAMPTZ,
	revoked_at TIMESTAMPTZ,
	created_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT uq_traeger_invitations_token UNIQUE (token),
	CONSTRAINT chk_traeger_invitations_email CHECK (email = LOWER(email) AND email <> ''),
	CONSTRAINT chk_traeger_invitations_roles CHECK (
		cardinality(roles) > 0
		AND roles <@ ARRAY['administration', 'billing', 'personnel', 'read_only']::VARCHAR(32)[]
	)
);
CREATE UNIQUE INDEX uq_traeger_invitations_open ON auth.traeger_invitations (organization_id, email)
	WHERE used_at IS NULL AND revoked_at IS NULL;
COMMENT ON TABLE auth.traeger_invitations IS
	'Invitation into a Träger-Büro (ADR 0084, #4650): the acceptance activates the Träger membership with the roles named here.';

CREATE TABLE audit.traeger_account_changes (
	id BIGSERIAL PRIMARY KEY,
	organization_id BIGINT NOT NULL REFERENCES platform.organizations(id) ON DELETE CASCADE,
	actor_account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
	action VARCHAR(32) NOT NULL,
	target_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
	target_email TEXT,
	roles_before VARCHAR(32)[] NOT NULL DEFAULT '{}',
	roles_after VARCHAR(32)[] NOT NULL DEFAULT '{}',
	created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
	CONSTRAINT chk_traeger_account_changes_action CHECK (action IN (
		'invitation_created', 'invitation_revoked', 'invitation_accepted',
		'roles_changed', 'member_removed', 'members_listed'
	))
);
CREATE INDEX idx_traeger_account_changes_organization ON audit.traeger_account_changes (organization_id, created_at);
COMMENT ON TABLE audit.traeger_account_changes IS
	'Ledger of the Träger-Büro account management (#4650): invitations, role changes, removals and reads of the member list.';

REVOKE ALL ON auth.traeger_invitations, audit.traeger_account_changes
	FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
GRANT SELECT, INSERT, UPDATE, DELETE ON auth.traeger_invitations TO phoenix_admin;
GRANT SELECT, INSERT, DELETE ON audit.traeger_account_changes TO phoenix_admin;
GRANT USAGE ON SEQUENCE auth.traeger_invitations_id_seq, audit.traeger_account_changes_id_seq TO phoenix_admin;
`

const traegerInvitationsDownSQL = `
DROP TABLE IF EXISTS audit.traeger_account_changes;
DROP TABLE IF EXISTS auth.traeger_invitations;
`

func traegerInvitationsUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerInvitationsUpSQL); err != nil {
		return fmt.Errorf("create Träger invitations: %w", err)
	}
	return nil
}

func traegerInvitationsDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerInvitationsDownSQL); err != nil {
		return fmt.Errorf("drop Träger invitations: %w", err)
	}
	return nil
}
