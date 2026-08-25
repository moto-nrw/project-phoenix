package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentLetterDeliveryVersion     = "1.15.331"
	parentLetterDeliveryDescription = "Add Elternbrief delivery mode to parent announcements and a generic per-recipient e-mail delivery record"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentLetterDeliveryVersion,
		Description: parentLetterDeliveryDescription,
		DependsOn: []string{
			parentAnnouncementsVersion,   // users.parent_announcements
			createEmailOutboxVersion,     // platform.email_outbox (delivery rows point at it)
			UsersGuardianProfilesVersion, // users.guardian_profiles
			"1.0.1",                      // auth.accounts
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentLetterDeliveryUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentLetterDeliveryDown(ctx, db)
		},
	)
}

func parentLetterDeliveryUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.331: Adding Elternbrief delivery mode and per-recipient delivery records...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// An Elternbrief (#2384) is not a second message system — it is a publication
	// MODE of the existing announcement. delivery_mode = 'standard' is every
	// pre-existing row and keeps today's behaviour (e-mail and acknowledgement are
	// independent opt-ins, the mail carries title + portal link only).
	//
	// email_audience decides WHO receives the mail, independently of who sees the
	// announcement in the portal:
	//   portal_only  - guardians with parent_portal.access (today's audience)
	//   all_contacts - additionally every guardian of a reached child that has an
	//                  e-mail address but no portal access
	// The default is deliberately the NARROW one: a letter that leaks its body to
	// someone deliberately excluded from the portal is the failure this column
	// must never cause by omission.
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS delivery_mode  TEXT NOT NULL DEFAULT 'standard',
			ADD COLUMN IF NOT EXISTS email_audience TEXT NOT NULL DEFAULT 'portal_only';
	`)
	if err != nil {
		return fmt.Errorf("error adding letter columns to users.parent_announcements: %w", err)
	}

	// Constraints are added separately (ADD CONSTRAINT has no IF NOT EXISTS) so a
	// re-run on a partially applied schema does not fail.
	//
	// chk_parent_announcements_letter_channels is the load-bearing one: it is what
	// makes "e-mail and acknowledgement cannot be deselected" a property of the
	// database rather than of a code path. No handler bug, no data script and no
	// later refactor can produce half a letter.
	_, err = tx.ExecContext(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_delivery_mode'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_delivery_mode
					CHECK (delivery_mode IN ('standard','letter'));
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_email_audience'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_email_audience
					CHECK (email_audience IN ('portal_only','all_contacts'));
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_letter_channels'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_letter_channels
					CHECK (delivery_mode <> 'letter' OR (send_email AND requires_acknowledgement));
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_letter_not_poll'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_letter_not_poll
					CHECK (delivery_mode <> 'letter' OR response_type = 'none');
			END IF;

			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'chk_parent_announcements_wide_audience_needs_email'
			) THEN
				ALTER TABLE users.parent_announcements
					ADD CONSTRAINT chk_parent_announcements_wide_audience_needs_email
					CHECK (email_audience = 'portal_only' OR send_email);
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error adding letter constraints to users.parent_announcements: %w", err)
	}

	// One row per addressed person per announcement — the staff-facing recipient
	// matrix (#2384) and, later, the target of provider delivery events (#1937).
	//
	// It lives in `platform` and is named generically ON PURPOSE. The matrix is a
	// per-recipient delivery record, which is exactly what the e-mail migration
	// needs for every other mail kind too; building it announcement-specific now
	// would mean replacing it later instead of filling it in.
	//
	// Columns worth explaining:
	//   outbox_id           NULL when nothing was queued (no address, or excluded
	//                       from the e-mail audience) - the row still exists so the
	//                       school can SEE that person and fix the data.
	//   guardian_profile_id the stable identity of the recipient. account_id is
	//                       nullable because `all_contacts` reaches people without
	//                       a portal account at all.
	//   recipient_email     a SNAPSHOT: what we actually sent to, not what the
	//                       profile says today. Correcting a typo later must not
	//                       rewrite the history of a failed delivery.
	//   reachability        why no mail went out, if none did. Independent of the
	//                       e-mail status: with email_audience='all_contacts' a
	//                       person can legitimately be 'no_portal' AND have a
	//                       delivered mail.
	//   provider_message_id stays empty until #1999 lands an API-based provider.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS platform.email_delivery (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			related_entity_type TEXT   NOT NULL,
			related_entity_id   BIGINT NOT NULL,
			outbox_id           BIGINT REFERENCES platform.email_outbox(id) ON DELETE SET NULL,
			guardian_profile_id BIGINT REFERENCES users.guardian_profiles(id) ON DELETE CASCADE,
			account_id          BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			recipient_email     TEXT,
			reachability        TEXT NOT NULL DEFAULT 'ok',
			provider_message_id TEXT,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_email_delivery_reachability
				CHECK (reachability IN ('ok','no_email','no_portal','excluded')),
			CONSTRAINT chk_email_delivery_email_present
				CHECK (reachability <> 'ok' OR length(btrim(coalesce(recipient_email,''))) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_email_delivery_related
			ON platform.email_delivery (tenant_id, related_entity_type, related_entity_id);
		CREATE INDEX IF NOT EXISTS idx_email_delivery_outbox
			ON platform.email_delivery (outbox_id)
			WHERE outbox_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.email_delivery: %w", err)
	}

	// One delivery row per (entity, recipient). Makes a re-publish or a concurrent
	// publish idempotent at the table level instead of relying on the caller.
	// guardian_profile_id is NOT NULL in practice for announcements; the partial
	// index keeps the guarantee without forbidding future non-guardian recipients.
	_, err = tx.ExecContext(ctx, `
		CREATE UNIQUE INDEX IF NOT EXISTS uq_email_delivery_entity_guardian
			ON platform.email_delivery (related_entity_type, related_entity_id, guardian_profile_id)
			WHERE guardian_profile_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error creating platform.email_delivery unique index: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE platform.email_delivery ENABLE ROW LEVEL SECURITY;
		ALTER TABLE platform.email_delivery FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'platform'
					AND tablename = 'email_delivery'
					AND policyname = 'tenant_isolation_platform_email_delivery'
			) THEN
				CREATE POLICY tenant_isolation_platform_email_delivery ON platform.email_delivery
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;
	`)
	if err != nil {
		return fmt.Errorf("error enabling RLS on platform.email_delivery: %w", err)
	}

	// Request handlers write these rows inside the publish tenant tx and read them
	// for the matrix. The outbox worker runs as phoenix_admin, which already has
	// ALL on platform.* via the 1.14.1 default privileges.
	_, err = tx.ExecContext(ctx, `
		GRANT SELECT, INSERT, UPDATE, DELETE ON platform.email_delivery TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE platform.email_delivery_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting permissions on platform.email_delivery: %w", err)
	}

	return tx.Commit()
}

func parentLetterDeliveryDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.331: Dropping Elternbrief delivery mode and delivery records...")

	if _, err := db.NewRaw(`
		DROP TABLE IF EXISTS platform.email_delivery CASCADE;
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_wide_audience_needs_email,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_letter_not_poll,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_letter_channels,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_email_audience,
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_delivery_mode,
			DROP COLUMN IF EXISTS email_audience,
			DROP COLUMN IF EXISTS delivery_mode;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping Elternbrief delivery objects: %w", err)
	}
	return nil
}
