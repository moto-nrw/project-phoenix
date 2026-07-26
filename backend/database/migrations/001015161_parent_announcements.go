package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentAnnouncementsVersion     = "1.15.161"
	parentAnnouncementsDescription = "Create parent announcement tables (announcements, targets, reads) for tenant-authored broadcast news to guardians"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentAnnouncementsVersion,
		Description: parentAnnouncementsDescription,
		DependsOn: []string{
			"1.3.5",                // users.students (student / class / group targeting)
			"1.0.1",                // auth.accounts (created_by, reads.account_id)
			parentMessagingVersion, // sit after the messaging tables in the users schema timeline
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentAnnouncementsDown(ctx, db)
		},
	)
}

func parentAnnouncementsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.161: Creating parent announcement tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// A parent announcement is a tenant-authored broadcast to guardians. It is a
	// DRAFT while published_at IS NULL, becomes visible once published_at is set
	// and stays visible until expires_at (NULL = never expires) or active=false.
	// requires_acknowledgement turns the parent-side "gelesen" into an explicit
	// "gelesen und bestätigt" the staff stats can count. send_email opts this
	// announcement into an additional e-mail to the targeted guardians on
	// publish (in-app is always shown; e-mail is per-announcement opt-in).
	// Targeting lives in the separate parent_announcement_targets rows
	// (OR-union); a row with no targets reaches nobody (the audience resolver
	// matches at least one target).
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_announcements (
			id                       BIGSERIAL PRIMARY KEY,
			tenant_id                BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			title                    TEXT NOT NULL,
			body                     TEXT NOT NULL,
			priority                 TEXT NOT NULL DEFAULT 'info',
			link_url                 TEXT,
			requires_acknowledgement BOOLEAN NOT NULL DEFAULT false,
			send_email               BOOLEAN NOT NULL DEFAULT false,
			published_at             TIMESTAMPTZ,
			expires_at               TIMESTAMPTZ,
			active                   BOOLEAN NOT NULL DEFAULT true,
			created_by               BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE RESTRICT,
			created_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at               TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_parent_announcements_title_not_blank CHECK (length(btrim(title)) > 0),
			CONSTRAINT chk_parent_announcements_body_not_blank CHECK (length(btrim(body)) > 0),
			CONSTRAINT chk_parent_announcements_priority CHECK (priority IN ('info','important')),
			CONSTRAINT chk_parent_announcements_expiry CHECK (expires_at IS NULL OR expires_at > created_at)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_announcements_manage
			ON users.parent_announcements (tenant_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_announcements_feed
			ON users.parent_announcements (tenant_id, published_at DESC)
			WHERE active AND published_at IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_announcements: %w", err)
	}

	// One row per audience selector on an announcement; an announcement's reach is
	// the OR-union of its targets. target_ref_id carries the entity id for
	// group/activity_group/student targets; target_ref_text carries the class
	// string ('1a') because users.students.school_class is a denormalized string,
	// not an FK. school_all and pending_enrollment carry neither. The CHECK keeps a
	// row's ref columns consistent with its type so the resolver can trust them.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_announcement_targets (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL REFERENCES users.parent_announcements(id) ON DELETE CASCADE,
			target_type     TEXT NOT NULL,
			target_ref_id   BIGINT,
			target_ref_text TEXT,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_parent_announcement_targets_type
				CHECK (target_type IN ('school_all','class','group','activity_group','student','pending_enrollment')),
			CONSTRAINT chk_parent_announcement_targets_ref CHECK (
				(target_type = 'class' AND target_ref_text IS NOT NULL AND target_ref_id IS NULL)
				OR (target_type IN ('group','activity_group','student') AND target_ref_id IS NOT NULL AND target_ref_text IS NULL)
				OR (target_type IN ('school_all','pending_enrollment') AND target_ref_id IS NULL AND target_ref_text IS NULL)
			)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_announcement_targets_announcement
			ON users.parent_announcement_targets (tenant_id, announcement_id);
		CREATE INDEX IF NOT EXISTS idx_parent_announcement_targets_lookup
			ON users.parent_announcement_targets (tenant_id, target_type, target_ref_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_announcement_targets: %w", err)
	}

	// Per-guardian-account read/ack state. A row exists once the guardian has
	// opened the announcement (read_at) and, if requires_acknowledgement,
	// acknowledged_at is stamped when they confirm. Read state is account-global
	// (one row per account+announcement), not per child: the feed is a
	// cross-child, cross-tenant list and "gelesen" is the guardian's, not a
	// child's. clock_timestamp() so the stamp is the real open/ack instant, not
	// the transaction start.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_announcement_reads (
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			announcement_id BIGINT NOT NULL REFERENCES users.parent_announcements(id) ON DELETE CASCADE,
			account_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			read_at         TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			acknowledged_at TIMESTAMPTZ,
			PRIMARY KEY (announcement_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_announcement_reads_account
			ON users.parent_announcement_reads (tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_announcement_reads: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_parent_announcements_updated_at ON users.parent_announcements;
		CREATE TRIGGER update_parent_announcements_updated_at
		BEFORE UPDATE ON users.parent_announcements
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger: %w", err)
	}

	// RLS — tenant isolation, identical shape to the messaging tables (CREATE
	// POLICY has no IF NOT EXISTS, so guard with a pg_policies existence check).
	for _, table := range []string{"parent_announcements", "parent_announcement_targets", "parent_announcement_reads"} {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE users.%[1]s ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.%[1]s FORCE ROW LEVEL SECURITY;

			DO $$
			BEGIN
				IF NOT EXISTS (
					SELECT 1 FROM pg_policies
					WHERE schemaname = 'users'
						AND tablename = '%[1]s'
						AND policyname = 'tenant_isolation_users_%[1]s'
				) THEN
					CREATE POLICY tenant_isolation_users_%[1]s ON users.%[1]s
						FOR ALL
						USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
						WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
				END IF;
			END $$;

			GRANT SELECT, INSERT, UPDATE, DELETE ON users.%[1]s TO phoenix_tenant;
		`, table))
		if err != nil {
			return fmt.Errorf("error enabling RLS on users.%s: %w", table, err)
		}
	}

	_, err = tx.ExecContext(ctx, `
		GRANT USAGE ON SEQUENCE users.parent_announcements_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_announcement_targets_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting sequence usage: %w", err)
	}

	// NOTE: the communications:announce permission is deliberately NOT registered
	// in auth.permissions yet. In v1 every /api/parent-announcements route is gated
	// on admin:* (there is no per-target audience scoping, so a non-admin holder
	// could otherwise broadcast school-wide and reach other authors' announcements
	// — #1669 "nur Admins"). Registering it now would let a school grant a
	// permission that resolves to nothing but 403s. It is re-introduced together
	// with the delegated/scoped announcer role that will actually honor it (the
	// reserved permissions.CommunicationsAnnounce constant documents the future).

	return tx.Commit()
}

func parentAnnouncementsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.161: Dropping parent announcement tables...")

	if _, err := db.NewRaw(`
		DROP TRIGGER IF EXISTS update_parent_announcements_updated_at ON users.parent_announcements;
		DROP TABLE IF EXISTS users.parent_announcement_reads CASCADE;
		DROP TABLE IF EXISTS users.parent_announcement_targets CASCADE;
		DROP TABLE IF EXISTS users.parent_announcements CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping parent announcement tables: %w", err)
	}
	return nil
}
