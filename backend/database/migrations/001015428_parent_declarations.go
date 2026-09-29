package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentDeclarationsVersion     = "1.15.428"
	parentDeclarationsDescription = "Add Erklärungen to parent announcements: frozen versions, an append-only submission record and the declarations.submit guardian permission (#3430)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentDeclarationsVersion,
		Description: parentDeclarationsDescription,
		DependsOn: []string{
			parentLetterDeliveryVersion, // users.parent_announcements.delivery_mode
			"1.15.399",                  // users.student_profiles is the child's owner row
			guardianOwnerCutoverVersion, // auth.guardian_student_access holds the permissions
			staffOwnerContractVersion,   // previous head
		},
	})

	Migrations.MustRegister(parentDeclarationsUp, parentDeclarationsDown)
}

// An Erklärung (#3430) is a third publication MODE of the existing
// announcement, like the Elternbrief: targeting, draft/publish, attachments,
// e-mail and reminders are the announcement machinery. What it adds is proof:
//
//   - parent_announcement_declaration_versions freezes the exact wording and
//     attachment digests at publication. A correction (unpublish, edit,
//     republish) creates the next version; earlier versions never change.
//   - parent_announcement_declaration_submissions records every declaration a
//     guardian gives, one row per action. Nothing is updated or deleted by the
//     application: a change of mind is a new row, a revocation is a new row.
//
// Both tables are insert-only for the request roles. Deleting the child
// (retention) or the school still removes the rows through the foreign keys,
// which PostgreSQL executes with the table owner's rights.
func parentDeclarationsUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// The declaration settings live on the announcement. They are NULL/false
	// for every other mode, and the shape constraint makes "a declaration
	// without its settings" impossible in the database, not only in the
	// service.
	if _, err := tx.ExecContext(ctx, `
		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS declaration_kind              TEXT,
			ADD COLUMN IF NOT EXISTS declaration_signers           TEXT,
			ADD COLUMN IF NOT EXISTS declaration_revocable         BOOLEAN NOT NULL DEFAULT false,
			ADD COLUMN IF NOT EXISTS declaration_requires_password BOOLEAN NOT NULL DEFAULT false;

		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_delivery_mode;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_delivery_mode
			CHECK (delivery_mode IN ('standard','letter','declaration'));

		-- A deadline used to mean "poll answer cut-off" only. A declaration
		-- has one too (its Frist), so the constraint admits that mode.
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_response_deadline;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_response_deadline
			CHECK (response_deadline IS NULL OR response_type <> 'none' OR delivery_mode = 'declaration');

		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_declaration_shape CHECK (
				(delivery_mode = 'declaration') = (declaration_kind IS NOT NULL AND declaration_signers IS NOT NULL)
				AND (declaration_kind IS NULL OR declaration_kind = 'consent')
				AND (declaration_signers IS NULL OR declaration_signers IN ('any','all'))
				AND (NOT declaration_revocable OR declaration_kind = 'consent')
				AND (NOT declaration_requires_password OR delivery_mode = 'declaration')
			),
			ADD CONSTRAINT chk_parent_announcements_declaration_not_poll CHECK (
				delivery_mode <> 'declaration' OR (response_type = 'none' AND NOT requires_acknowledgement AND email_audience = 'portal_only')
			);
	`); err != nil {
		return fmt.Errorf("error adding declaration columns to users.parent_announcements: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TABLE users.parent_announcement_declaration_versions (
			id               BIGSERIAL PRIMARY KEY,
			tenant_id        BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id  BIGINT NOT NULL,
			version_no       INT NOT NULL CHECK (version_no > 0),
			title            TEXT NOT NULL,
			body             TEXT NOT NULL,
			declaration_kind TEXT NOT NULL CHECK (declaration_kind = 'consent'),
			attachments      JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(attachments) = 'array'),
			content_hash     TEXT NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
			published_at     TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_parent_declaration_versions_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT uq_parent_declaration_versions_number UNIQUE (announcement_id, version_no),
			CONSTRAINT fk_parent_declaration_versions_announcement FOREIGN KEY (tenant_id, announcement_id)
				REFERENCES users.parent_announcements(tenant_id, id) ON DELETE CASCADE
		);
		CREATE INDEX idx_parent_declaration_versions_announcement
			ON users.parent_announcement_declaration_versions (tenant_id, announcement_id, version_no DESC);

		-- One row per declared action. The foreign keys to the announcement and
		-- the version deliberately have NO cascade: an announcement that
		-- carries declarations cannot be deleted, so the proof cannot vanish
		-- by deleting its context. The child and the school still cascade
		-- (retention and school deletion), and a deleted account keeps the
		-- row with the signer's name frozen in signer_name. account_id is not
		-- part of record_hash because this foreign key is set to NULL on deletion.
		CREATE TABLE users.parent_announcement_declaration_submissions (
			id                 BIGSERIAL PRIMARY KEY,
			tenant_id          BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id    BIGINT NOT NULL,
			version_id         BIGINT NOT NULL,
			student_id         BIGINT NOT NULL,
			account_id         BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			guardian_profile_id BIGINT,
			signer_name        TEXT NOT NULL,
			guardian_role      TEXT NOT NULL,
			action             TEXT NOT NULL CHECK (action IN ('agreed','declined','revoked')),
			method             TEXT NOT NULL DEFAULT 'simple_electronic' CHECK (method IN ('simple_electronic')),
			password_confirmed BOOLEAN NOT NULL DEFAULT false,
			content_hash       TEXT NOT NULL CHECK (content_hash ~ '^[0-9a-f]{64}$'),
			record_hash        TEXT NOT NULL CHECK (record_hash ~ '^[0-9a-f]{64}$'),
			submitted_at       TIMESTAMPTZ NOT NULL,
			created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_parent_declaration_submissions_announcement FOREIGN KEY (tenant_id, announcement_id)
				REFERENCES users.parent_announcements(tenant_id, id),
			CONSTRAINT fk_parent_declaration_submissions_version FOREIGN KEY (tenant_id, version_id)
				REFERENCES users.parent_announcement_declaration_versions(tenant_id, id),
			CONSTRAINT fk_parent_declaration_submissions_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE
		);
		CREATE INDEX idx_parent_declaration_submissions_announcement
			ON users.parent_announcement_declaration_submissions (tenant_id, announcement_id, student_id, submitted_at DESC, id DESC);
		CREATE INDEX idx_parent_declaration_submissions_version
			ON users.parent_announcement_declaration_submissions (tenant_id, version_id);
		CREATE INDEX idx_parent_declaration_submissions_student
			ON users.parent_announcement_declaration_submissions (tenant_id, student_id);
		CREATE INDEX idx_parent_declaration_submissions_account
			ON users.parent_announcement_declaration_submissions (account_id)
			WHERE account_id IS NOT NULL;

		-- The down migration removes the permission only from relationships this
		-- migration changed. A permission granted manually or by a later
		-- migration must survive its rollback.
		CREATE TABLE auth.parent_declaration_permission_grants (
			tenant_id       BIGINT NOT NULL,
			relationship_id BIGINT NOT NULL,
			PRIMARY KEY (tenant_id, relationship_id)
		);

		GRANT SELECT, INSERT ON users.parent_announcement_declaration_versions TO phoenix_tenant, phoenix_admin;
		GRANT SELECT, INSERT ON users.parent_announcement_declaration_submissions TO phoenix_tenant, phoenix_admin;
		REVOKE UPDATE, DELETE, TRUNCATE ON users.parent_announcement_declaration_versions FROM phoenix_tenant, phoenix_admin;
		REVOKE UPDATE, DELETE, TRUNCATE ON users.parent_announcement_declaration_submissions FROM phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE users.parent_announcement_declaration_versions_id_seq TO phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE users.parent_announcement_declaration_submissions_id_seq TO phoenix_tenant, phoenix_admin;
	`); err != nil {
		return fmt.Errorf("error creating declaration tables: %w", err)
	}

	if err := provisionTenantRLS(ctx, tx,
		"users.parent_announcement_declaration_versions",
		"users.parent_announcement_declaration_submissions",
	); err != nil {
		return err
	}

	// Full guardians (the roles that carry custody) may declare by default;
	// pickup-only people, emergency contacts and social workers never do. New
	// relationships receive it from the role presets in auth/authorize.
	if _, err := tx.ExecContext(ctx, `
		WITH granted AS (
			UPDATE auth.guardian_student_access AS access
			SET permissions = access.permissions || '{"parent_portal.declarations.submit": true}'::jsonb
			FROM users.student_guardian_relationships AS relationship
			WHERE relationship.tenant_id = access.tenant_id AND relationship.id = access.relationship_id
				AND relationship.guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
				AND NOT (access.permissions ? 'parent_portal.declarations.submit')
			RETURNING access.tenant_id, access.relationship_id
		)
		INSERT INTO auth.parent_declaration_permission_grants (tenant_id, relationship_id)
		SELECT tenant_id, relationship_id FROM granted;
	`); err != nil {
		return fmt.Errorf("error granting parent_portal.declarations.submit: %w", err)
	}

	return tx.Commit()
}

func parentDeclarationsDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.ExecContext(ctx, `
		UPDATE auth.guardian_student_access AS access
		SET permissions = access.permissions - 'parent_portal.declarations.submit'
		FROM auth.parent_declaration_permission_grants AS marker
		WHERE marker.tenant_id = access.tenant_id AND marker.relationship_id = access.relationship_id
			AND access.permissions @> '{"parent_portal.declarations.submit": true}'::jsonb;
		DROP TABLE IF EXISTS auth.parent_declaration_permission_grants;

		DROP TABLE IF EXISTS users.parent_announcement_declaration_submissions;
		DROP TABLE IF EXISTS users.parent_announcement_declaration_versions;

		DELETE FROM users.parent_announcements WHERE delivery_mode = 'declaration';

		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_not_poll,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_shape,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_response_deadline,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_delivery_mode;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_delivery_mode
				CHECK (delivery_mode IN ('standard','letter')),
			ADD CONSTRAINT chk_parent_announcements_response_deadline
				CHECK (response_deadline IS NULL OR response_type <> 'none');
		ALTER TABLE users.parent_announcements
			DROP COLUMN IF EXISTS declaration_requires_password,
			DROP COLUMN IF EXISTS declaration_revocable,
			DROP COLUMN IF EXISTS declaration_signers,
			DROP COLUMN IF EXISTS declaration_kind;
	`); err != nil {
		return fmt.Errorf("error dropping parent declarations: %w", err)
	}
	return tx.Commit()
}
