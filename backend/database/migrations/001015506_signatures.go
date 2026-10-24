package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const signaturesVersion = "1.15.506"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     signaturesVersion,
		Description: "Unterschrift: einfache elektronische Signatur mit gezeichneter Unterschrift als gemeinsamer Baustein, zuerst für Einverständnisse (#4785)",
		DependsOn: []string{
			"1.15.499",                // the last migration before this one (password login protection)
			morningAttendanceVersion,  // the latest school erasure functions this migration extends
			parentDeclarationsVersion, // the declaration columns and submissions this migration extends
			schoolErasureVersion,      // the erasure plan this migration extends
			demoVisitorErasureVersion, // the demo visitor erasure this migration extends
		},
	})
	Migrations.MustRegister(signaturesUp, signaturesDown)
}

// signaturesUpSQL adds the shared signature store (#4785).
//
//   - audit.signatures holds one signature per row: the PNG, its SHA-256, the
//     SHA-256 of exactly what was signed, the signer (account nullable: a
//     public enrollment has none, and account deletion clears it while the
//     frozen name stays), the time and a record hash. The request roles may
//     only read and insert. Every row belongs to a child or an enrollment
//     request and goes with it; the school deletion removes the rest.
//   - An Einverständnis can require a signature for agreeing
//     (declaration_requires_signature). A signed submission names it in
//     signature_id and uses the method simple_electronic_signature. A
//     signature only disappears with its child or school, which also removes
//     the submission in the same statement, so the school erasure may delete
//     the signatures with the other audit rows.
//   - The demo visitor erasure (1.15.456) deletes the visitor's signatures
//     with their declarations: both carry the visitor's name.
const signaturesUpSQL = `
		-- The enrollment subject is bound to its school like the child, so a
		-- signature can never name another school's request.
		CREATE UNIQUE INDEX IF NOT EXISTS uq_enrollment_requests_tenant_id
			ON enrollment.requests (tenant_id, id);

		CREATE TABLE IF NOT EXISTS audit.signatures (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			purpose TEXT NOT NULL CHECK (purpose IN ('declaration','enrollment','pickup_permission')),
			student_id BIGINT,
			enrollment_request_id BIGINT,
			signer_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			signer_name TEXT NOT NULL CHECK (btrim(signer_name) <> ''),
			method TEXT NOT NULL CHECK (method IN ('drawn','typed')),
			typed_name TEXT,
			image_png BYTEA NOT NULL CHECK (octet_length(image_png) BETWEEN 1 AND 204800),
			image_sha256 TEXT NOT NULL CHECK (image_sha256 ~ '^[0-9a-f]{64}$'),
			document_sha256 TEXT NOT NULL CHECK (document_sha256 ~ '^[0-9a-f]{64}$'),
			record_hash TEXT NOT NULL CHECK (record_hash ~ '^[0-9a-f]{64}$'),
			signed_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_signatures_tenant_id UNIQUE (tenant_id, id),
			CONSTRAINT fk_signatures_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT fk_signatures_enrollment_request FOREIGN KEY (tenant_id, enrollment_request_id)
				REFERENCES enrollment.requests(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_signatures_subject CHECK ((student_id IS NOT NULL) <> (enrollment_request_id IS NOT NULL)),
			CONSTRAINT chk_signatures_typed_name CHECK ((method = 'typed') = (typed_name IS NOT NULL))
		);
		CREATE INDEX IF NOT EXISTS idx_signatures_student
			ON audit.signatures (tenant_id, student_id) WHERE student_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_signatures_enrollment_request
			ON audit.signatures (tenant_id, enrollment_request_id) WHERE enrollment_request_id IS NOT NULL;
		CREATE INDEX IF NOT EXISTS idx_signatures_signer_account
			ON audit.signatures (signer_account_id) WHERE signer_account_id IS NOT NULL;

		GRANT SELECT, INSERT ON audit.signatures TO phoenix_tenant, phoenix_admin;
		REVOKE UPDATE, DELETE, TRUNCATE ON audit.signatures FROM phoenix_tenant, phoenix_admin;
		GRANT USAGE ON SEQUENCE audit.signatures_id_seq TO phoenix_tenant, phoenix_admin;

		ALTER TABLE users.parent_announcements
			ADD COLUMN IF NOT EXISTS declaration_requires_signature BOOLEAN NOT NULL DEFAULT false;
		ALTER TABLE users.parent_announcements
			ADD CONSTRAINT chk_parent_announcements_declaration_signature
			CHECK (NOT declaration_requires_signature OR delivery_mode = 'declaration');

		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD COLUMN IF NOT EXISTS signature_id BIGINT;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT fk_parent_declaration_submissions_signature FOREIGN KEY (tenant_id, signature_id)
				REFERENCES audit.signatures(tenant_id, id) ON DELETE CASCADE;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS parent_announcement_declaration_submissions_method_check;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT chk_parent_declaration_submissions_method CHECK (
				method IN ('simple_electronic','simple_electronic_signature')
				AND (method = 'simple_electronic_signature') = (signature_id IS NOT NULL)
			);
		CREATE INDEX IF NOT EXISTS idx_parent_declaration_submissions_signature
			ON users.parent_announcement_declaration_submissions (tenant_id, signature_id)
			WHERE signature_id IS NOT NULL;
`

const signaturesDownSQL = `
		DROP INDEX IF EXISTS users.idx_parent_declaration_submissions_signature;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS chk_parent_declaration_submissions_method;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP CONSTRAINT IF EXISTS fk_parent_declaration_submissions_signature;
		ALTER TABLE users.parent_announcement_declaration_submissions
			DROP COLUMN IF EXISTS signature_id;
		ALTER TABLE users.parent_announcement_declaration_submissions
			ADD CONSTRAINT parent_announcement_declaration_submissions_method_check
			CHECK (method IN ('simple_electronic'));
		ALTER TABLE users.parent_announcements
			DROP CONSTRAINT IF EXISTS chk_parent_announcements_declaration_signature;
		ALTER TABLE users.parent_announcements
			DROP COLUMN IF EXISTS declaration_requires_signature;
		DROP TABLE IF EXISTS audit.signatures;
		DROP INDEX IF EXISTS enrollment.uq_enrollment_requests_tenant_id;
`

// signaturesDemoVisitorErasureSQL is platform.erase_closed_demo_visitors of
// 1.15.456 with one statement more: the visitor's signatures go right after
// the declarations they belong to, because both carry the visitor's name.
// The rest of the function is unchanged.
const signaturesDemoVisitorErasureSQL = `
		CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			hidden_schools BIGINT[];
			legacy_school_ids BIGINT[];
			recovered_visitor_account_ids BIGINT[];
			recovered_visitor_parent_account_ids BIGINT[];
			erased_account_ids BIGINT[];
			visitor RECORD;
			target_name TEXT;
			erased INTEGER := 0;
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0));
			FOR visitor IN
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				ORDER BY state.name
			LOOP
				target_name := visitor.name;
				PERFORM pg_advisory_xact_lock(hashtextextended('demo school seed:' || target_name, 3461));
				PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || target_name, 0));
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				INTO visitor
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE state.name = target_name
					AND (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				FOR UPDATE OF state;
				IF NOT FOUND THEN
					CONTINUE;
				END IF;
				-- Keep the runtime path in addition to the migration backfill. It
				-- covers a row restored from a pre-migration backup before it is
				-- erased, including a final failed school's original subdomain.
				SELECT COALESCE(array_agg(school.id ORDER BY school.id), '{}'::BIGINT[])
				INTO legacy_school_ids
				FROM platform.schools AS school
				WHERE school.deleted_at IS NOT NULL
					AND (school.id = visitor.tenant_id
						OR school.subdomain = visitor.name
						OR school.subdomain = left('x' || school.id::TEXT || '-' || visitor.name, 63));
				visitor.visitor_school_ids := array_remove(visitor.visitor_school_ids || legacy_school_ids, NULL);
				SELECT COALESCE(array_agg(school.id), '{}') INTO hidden_schools
				FROM platform.schools AS school
				WHERE school.id = ANY (visitor.visitor_school_ids)
					AND school.deleted_at IS NOT NULL;
				-- SaveVisitor writes the account ids immediately after each account
				-- is created. If that database write itself failed, recover only the
				-- two deterministic seed identities in the affected schools. The
				-- random order suffix and attempt number make these account addresses
				-- independent of a visitor's display name.
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_account_ids
				FROM users.persons AS person
				JOIN auth.accounts AS account ON account.id = person.account_id
				WHERE person.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'julia.klein@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_parent_account_ids
				FROM users.guardian_profiles AS guardian
				JOIN auth.accounts AS account ON account.id = guardian.account_id
				WHERE guardian.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'sabine.schneider@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				visitor.visitor_account_ids := array_remove(visitor.visitor_account_ids || recovered_visitor_account_ids, NULL);
				visitor.visitor_parent_account_ids := array_remove(visitor.visitor_parent_account_ids || recovered_visitor_parent_account_ids, NULL);
				IF cardinality(visitor.visitor_account_ids) > 0 OR cardinality(visitor.visitor_parent_account_ids) > 0 THEN
					UPDATE users.persons SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_account_ids);
					UPDATE users.guardian_profiles SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE enrollment.requests SET guardian_first_name = 'Demo', guardian_last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND guardian_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE audit.guardian_changes SET actor_name_snapshot = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND actor_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE users.parent_messages SET sender_name = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND sender_account_id = ANY (visitor.visitor_parent_account_ids);
					DELETE FROM users.parent_announcement_declaration_submissions
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					DELETE FROM audit.signatures
					WHERE tenant_id = ANY (hidden_schools)
						AND signer_account_id = ANY (visitor.visitor_parent_account_ids);
					WITH visitor_outbox AS (
						SELECT outbox.id
						FROM platform.email_outbox AS outbox
						WHERE outbox.tenant_id = ANY (hidden_schools)
							AND (
								EXISTS (
									SELECT 1 FROM platform.email_delivery AS delivery
									WHERE delivery.outbox_id = outbox.id
										AND delivery.account_id = ANY (visitor.visitor_parent_account_ids))
								OR LOWER(outbox.recipient->>'address') IN (
									SELECT LOWER(account.email) FROM auth.accounts AS account
									WHERE account.id = ANY (visitor.visitor_parent_account_ids))
							)
					), deleted_deliveries AS (
						DELETE FROM platform.email_delivery AS delivery
						WHERE delivery.tenant_id = ANY (hidden_schools)
							AND (delivery.account_id = ANY (visitor.visitor_parent_account_ids)
								OR delivery.outbox_id IN (SELECT id FROM visitor_outbox))
					)
					DELETE FROM platform.email_outbox AS outbox
					WHERE outbox.id IN (SELECT id FROM visitor_outbox);
					-- Keep a visitor account that belongs to another school, but never
					-- leave its sessions or grants in a retired demo school behind.
					DELETE FROM auth.tokens
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_roles
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_permissions
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.guardian_student_access
					WHERE account_id = ANY (visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_tenants
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.mfa_overrides
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					SELECT COALESCE(array_agg(account.id), '{}') INTO erased_account_ids
					FROM auth.accounts AS account
					WHERE account.id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND NOT EXISTS (
							SELECT 1 FROM auth.account_tenants AS membership
							WHERE membership.account_id = account.id
								AND membership.tenant_id <> ALL (visitor.visitor_school_ids)
						)
						AND NOT EXISTS (
							SELECT 1 FROM users.persons AS person
							WHERE person.account_id = account.id AND person.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.guardian_profiles AS guardian
							WHERE guardian.account_id = account.id AND guardian.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM enrollment.requests AS request
							WHERE request.guardian_account_id = account.id AND request.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_messages AS message
							WHERE message.sender_account_id = account.id AND message.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_threads AS thread
							WHERE thread.guardian_account_id = account.id AND thread.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_reads AS read
							WHERE read.account_id = account.id AND read.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM audit.guardian_changes AS change
							WHERE change.actor_account_id = account.id AND change.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_announcement_declaration_submissions AS submission
							WHERE submission.account_id = account.id AND submission.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM platform.email_delivery AS delivery
							WHERE delivery.account_id = account.id AND delivery.tenant_id <> ALL (visitor.visitor_school_ids)
						);
					IF cardinality(erased_account_ids) > 0 THEN
						DELETE FROM auth.tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_roles WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_permissions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_tenants WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_email_challenges WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_trusted_devices WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_overrides WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_sessions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.demo_accesses
						WHERE account_id = ANY (erased_account_ids) OR parent_account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_rate_limits
						WHERE email IN (SELECT email FROM auth.accounts WHERE id = ANY (erased_account_ids));
						UPDATE auth.accounts SET
							email = 'demo-besuch-' || id || '@example.invalid',
							username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
							last_login = NULL, avatar = NULL, calendar_feed_token = NULL
						WHERE id = ANY (erased_account_ids);
					END IF;
				END IF;
				DELETE FROM platform.demo_school_states WHERE name = visitor.name;
				erased := erased + 1;
			END LOOP;
			RETURN erased;
		END;
		$$;
`

// signaturesDemoVisitorErasureDownSQL is the function exactly as 1.15.456
// installed it, for the down migration.
const signaturesDemoVisitorErasureDownSQL = `
		CREATE OR REPLACE FUNCTION platform.erase_closed_demo_visitors()
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog
		AS $$
		DECLARE
			hidden_schools BIGINT[];
			legacy_school_ids BIGINT[];
			recovered_visitor_account_ids BIGINT[];
			recovered_visitor_parent_account_ids BIGINT[];
			erased_account_ids BIGINT[];
			visitor RECORD;
			target_name TEXT;
			erased INTEGER := 0;
		BEGIN
			PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:visitor-erasure', 0));
			FOR visitor IN
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				ORDER BY state.name
			LOOP
				target_name := visitor.name;
				PERFORM pg_advisory_xact_lock(hashtextextended('demo school seed:' || target_name, 3461));
				PERFORM pg_advisory_xact_lock(hashtextextended('platform.demo_school_states:' || target_name, 0));
				SELECT state.name, state.tenant_id, state.attempts,
					array_remove(state.visitor_account_ids || ARRAY[state.visitor_account_id], NULL) AS visitor_account_ids,
					array_remove(state.visitor_parent_account_ids || ARRAY[state.visitor_parent_account_id], NULL) AS visitor_parent_account_ids,
					array_remove(state.visitor_school_ids || ARRAY[state.tenant_id], NULL) AS visitor_school_ids
				INTO visitor
				FROM platform.demo_school_states AS state
				LEFT JOIN platform.schools AS school ON school.id = state.tenant_id
				WHERE state.name = target_name
					AND (state.status = 'failed' OR (state.status = 'ready' AND school.deleted_at IS NOT NULL))
					AND NOT EXISTS (SELECT 1 FROM auth.demo_accesses AS access WHERE access.school_slug = state.name)
				FOR UPDATE OF state;
				IF NOT FOUND THEN
					CONTINUE;
				END IF;
				-- Keep the runtime path in addition to the migration backfill. It
				-- covers a row restored from a pre-migration backup before it is
				-- erased, including a final failed school's original subdomain.
				SELECT COALESCE(array_agg(school.id ORDER BY school.id), '{}'::BIGINT[])
				INTO legacy_school_ids
				FROM platform.schools AS school
				WHERE school.deleted_at IS NOT NULL
					AND (school.id = visitor.tenant_id
						OR school.subdomain = visitor.name
						OR school.subdomain = left('x' || school.id::TEXT || '-' || visitor.name, 63));
				visitor.visitor_school_ids := array_remove(visitor.visitor_school_ids || legacy_school_ids, NULL);
				SELECT COALESCE(array_agg(school.id), '{}') INTO hidden_schools
				FROM platform.schools AS school
				WHERE school.id = ANY (visitor.visitor_school_ids)
					AND school.deleted_at IS NOT NULL;
				-- SaveVisitor writes the account ids immediately after each account
				-- is created. If that database write itself failed, recover only the
				-- two deterministic seed identities in the affected schools. The
				-- random order suffix and attempt number make these account addresses
				-- independent of a visitor's display name.
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_account_ids
				FROM users.persons AS person
				JOIN auth.accounts AS account ON account.id = person.account_id
				WHERE person.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'julia.klein@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				SELECT COALESCE(array_agg(account.id), '{}'::BIGINT[])
				INTO recovered_visitor_parent_account_ids
				FROM users.guardian_profiles AS guardian
				JOIN auth.accounts AS account ON account.id = guardian.account_id
				WHERE guardian.tenant_id = ANY (hidden_schools)
					AND account.email = ANY (ARRAY(
						SELECT 'sabine.schneider@demo-' || regexp_replace(visitor.name, '^.*-', '') ||
							CASE WHEN attempt.number > 1 THEN '-' || attempt.number::TEXT ELSE '' END || '.moto-ogs.de'
						FROM generate_series(1, GREATEST(visitor.attempts, 1)) AS attempt(number)
					));
				visitor.visitor_account_ids := array_remove(visitor.visitor_account_ids || recovered_visitor_account_ids, NULL);
				visitor.visitor_parent_account_ids := array_remove(visitor.visitor_parent_account_ids || recovered_visitor_parent_account_ids, NULL);
				IF cardinality(visitor.visitor_account_ids) > 0 OR cardinality(visitor.visitor_parent_account_ids) > 0 THEN
					UPDATE users.persons SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_account_ids);
					UPDATE users.guardian_profiles SET first_name = 'Demo', last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE enrollment.requests SET guardian_first_name = 'Demo', guardian_last_name = 'Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND guardian_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE audit.guardian_changes SET actor_name_snapshot = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND actor_account_id = ANY (visitor.visitor_parent_account_ids);
					UPDATE users.parent_messages SET sender_name = 'Demo Besuch'
					WHERE tenant_id = ANY (hidden_schools)
						AND sender_account_id = ANY (visitor.visitor_parent_account_ids);
					DELETE FROM users.parent_announcement_declaration_submissions
					WHERE tenant_id = ANY (hidden_schools)
						AND account_id = ANY (visitor.visitor_parent_account_ids);
					WITH visitor_outbox AS (
						SELECT outbox.id
						FROM platform.email_outbox AS outbox
						WHERE outbox.tenant_id = ANY (hidden_schools)
							AND (
								EXISTS (
									SELECT 1 FROM platform.email_delivery AS delivery
									WHERE delivery.outbox_id = outbox.id
										AND delivery.account_id = ANY (visitor.visitor_parent_account_ids))
								OR LOWER(outbox.recipient->>'address') IN (
									SELECT LOWER(account.email) FROM auth.accounts AS account
									WHERE account.id = ANY (visitor.visitor_parent_account_ids))
							)
					), deleted_deliveries AS (
						DELETE FROM platform.email_delivery AS delivery
						WHERE delivery.tenant_id = ANY (hidden_schools)
							AND (delivery.account_id = ANY (visitor.visitor_parent_account_ids)
								OR delivery.outbox_id IN (SELECT id FROM visitor_outbox))
					)
					DELETE FROM platform.email_outbox AS outbox
					WHERE outbox.id IN (SELECT id FROM visitor_outbox);
					-- Keep a visitor account that belongs to another school, but never
					-- leave its sessions or grants in a retired demo school behind.
					DELETE FROM auth.tokens
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_roles
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_permissions
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.guardian_student_access
					WHERE account_id = ANY (visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.account_tenants
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					DELETE FROM auth.mfa_overrides
					WHERE account_id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND tenant_id = ANY (visitor.visitor_school_ids);
					SELECT COALESCE(array_agg(account.id), '{}') INTO erased_account_ids
					FROM auth.accounts AS account
					WHERE account.id = ANY (visitor.visitor_account_ids || visitor.visitor_parent_account_ids)
						AND NOT EXISTS (
							SELECT 1 FROM auth.account_tenants AS membership
							WHERE membership.account_id = account.id
								AND membership.tenant_id <> ALL (visitor.visitor_school_ids)
						)
						AND NOT EXISTS (
							SELECT 1 FROM users.persons AS person
							WHERE person.account_id = account.id AND person.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.guardian_profiles AS guardian
							WHERE guardian.account_id = account.id AND guardian.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM enrollment.requests AS request
							WHERE request.guardian_account_id = account.id AND request.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_messages AS message
							WHERE message.sender_account_id = account.id AND message.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_threads AS thread
							WHERE thread.guardian_account_id = account.id AND thread.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_message_reads AS read
							WHERE read.account_id = account.id AND read.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM audit.guardian_changes AS change
							WHERE change.actor_account_id = account.id AND change.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM users.parent_announcement_declaration_submissions AS submission
							WHERE submission.account_id = account.id AND submission.tenant_id <> ALL (visitor.visitor_school_ids)
							UNION ALL
							SELECT 1 FROM platform.email_delivery AS delivery
							WHERE delivery.account_id = account.id AND delivery.tenant_id <> ALL (visitor.visitor_school_ids)
						);
					IF cardinality(erased_account_ids) > 0 THEN
						DELETE FROM auth.tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_tokens WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_roles WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_permissions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.account_tenants WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_email_challenges WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_trusted_devices WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.mfa_overrides WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_credentials WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.passkey_sessions WHERE account_id = ANY (erased_account_ids);
						DELETE FROM auth.demo_accesses
						WHERE account_id = ANY (erased_account_ids) OR parent_account_id = ANY (erased_account_ids);
						DELETE FROM auth.password_reset_rate_limits
						WHERE email IN (SELECT email FROM auth.accounts WHERE id = ANY (erased_account_ids));
						UPDATE auth.accounts SET
							email = 'demo-besuch-' || id || '@example.invalid',
							username = NULL, active = FALSE, password_hash = NULL, is_password_otp = FALSE,
							last_login = NULL, avatar = NULL, calendar_feed_token = NULL
						WHERE id = ANY (erased_account_ids);
					END IF;
				END IF;
				DELETE FROM platform.demo_school_states WHERE name = visitor.name;
				erased := erased + 1;
			END LOOP;
			RETURN erased;
		END;
		$$;
`

func signaturesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSignatures(ctx, tx)
	if _, err := tx.ExecContext(ctx, signaturesUpSQL); err != nil {
		return fmt.Errorf("create signatures: %w", err)
	}
	if err := provisionTenantRLS(ctx, tx, "audit.signatures"); err != nil {
		return err
	}
	if err := installSchoolErasurePlan(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure plan: %w", err)
	}
	if err := installSchoolErasureDelete(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure delete: %w", err)
	}
	if err := installSchoolErasurePreflight(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preflight: %w", err)
	}
	if err := installSchoolErasurePreview(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure preview: %w", err)
	}
	if err := installSchoolErasureResidual(ctx, tx); err != nil {
		return fmt.Errorf("extend school erasure residual check: %w", err)
	}
	if _, err := tx.ExecContext(ctx, signaturesDemoVisitorErasureSQL); err != nil {
		return fmt.Errorf("extend demo visitor erasure: %w", err)
	}
	return tx.Commit()
}

// signaturesDown refuses while signed declarations exist: dropping the
// signatures would silently turn signed proof into unsigned proof.
func signaturesDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer rollbackSignatures(ctx, tx)
	// The lock keeps a signature from arriving between the count and the drop.
	if _, err := tx.ExecContext(ctx, `LOCK TABLE audit.signatures IN ACCESS EXCLUSIVE MODE`); err != nil {
		return fmt.Errorf("lock signatures: %w", err)
	}
	var signed int
	if err := tx.NewRaw(`SELECT COUNT(*) FROM audit.signatures`).Scan(ctx, &signed); err != nil {
		return fmt.Errorf("count signatures: %w", err)
	}
	if signed > 0 {
		return fmt.Errorf("cannot drop audit.signatures while %d signatures exist", signed)
	}
	if _, err := tx.ExecContext(ctx, signaturesDemoVisitorErasureDownSQL); err != nil {
		return fmt.Errorf("restore demo visitor erasure: %w", err)
	}
	if err := restoreSchoolErasureWithoutSignatures(ctx, tx); err != nil {
		return fmt.Errorf("restore school erasure without signatures: %w", err)
	}
	if _, err := tx.ExecContext(ctx, signaturesDownSQL); err != nil {
		return fmt.Errorf("revert signatures: %w", err)
	}
	return tx.Commit()
}

func rollbackSignatures(ctx context.Context, tx bun.Tx) {
	if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
		logRollbackFailure(ctx, err)
	}
}
