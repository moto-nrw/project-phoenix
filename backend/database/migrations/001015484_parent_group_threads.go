package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	parentGroupThreadsVersion     = "1.15.484"
	parentGroupThreadsDescription = "Create parent group threads: the OGS writes to a group, class or AG; recipients are frozen per message; parents answer only the OGS in their child's conversation (#4784)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentGroupThreadsVersion,
		Description: parentGroupThreadsDescription,
		DependsOn: []string{
			parentMessagingVersion,         // users.parent_messages gains the thread reference
			maskParentHealthHistoryVersion, // previous head
		},
	})
	Migrations.MustRegister(parentGroupThreadsUp, parentGroupThreadsDown)
}

// parentGroupThreadsUp creates the group threads (#4784).
//
// A group thread is a one-way announcement channel with a reply path: only
// the OGS writes into it. Its target (group, class or AG) is resolved to the
// reachable guardians anew for every message and frozen in
// parent_group_message_recipients, so a family that joins later does not see
// earlier messages and a family that left sees nothing new. A guardian's
// answer is an ordinary message in the 1:1 conversation of the child and
// carries group_thread_id as its reference; no guardian ever reads another
// guardian's answer, name or count.
func parentGroupThreadsUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, parentGroupThreadTablesSQL); err != nil {
			return fmt.Errorf("create parent group thread tables: %w", err)
		}
		if _, err := tx.ExecContext(ctx, parentGroupThreadSecuritySQL); err != nil {
			return fmt.Errorf("secure parent group thread tables: %w", err)
		}
		if _, err := tx.ExecContext(ctx, parentGroupThreadSequencesSQL); err != nil {
			return fmt.Errorf("grant parent group thread sequences: %w", err)
		}
		if err := installSchoolErasurePlan(ctx, tx); err != nil {
			return fmt.Errorf("update school erasure plan for parent group threads: %w", err)
		}
		if err := installSchoolErasurePreflight(ctx, tx); err != nil {
			return fmt.Errorf("update school erasure preflight for parent group threads: %w", err)
		}
		if err := installSchoolErasureResidual(ctx, tx); err != nil {
			return fmt.Errorf("update school erasure residual check for parent group threads: %w", err)
		}
		if err := installSchoolErasureDelete(ctx, tx); err != nil {
			return fmt.Errorf("update school erasure deletion for parent group threads: %w", err)
		}
		if _, err := tx.ExecContext(ctx, schoolErasureFunctionsSQL); err != nil {
			return fmt.Errorf("update school erasure functions for parent group threads: %w", err)
		}
		return nil
	})
}

// parentGroupThreadTablesSQL creates the threads, their messages, the
// recipients frozen per message, the read cursors and the answer reference.
const parentGroupThreadTablesSQL = `
			CREATE TABLE IF NOT EXISTS users.parent_group_threads (
				id                    BIGSERIAL PRIMARY KEY,
				tenant_id             BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				target_type           TEXT NOT NULL,
				target_ref_id         BIGINT,
				target_ref_text       TEXT,
				target_label          TEXT NOT NULL,
				title                 TEXT NOT NULL,
				created_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				closed_at             TIMESTAMPTZ,
				closed_by_account_id  BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				last_message_at       TIMESTAMPTZ,
				created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				CONSTRAINT chk_parent_group_threads_target CHECK (
					(target_type = 'class' AND target_ref_text IS NOT NULL AND target_ref_id IS NULL)
					OR (target_type IN ('group', 'activity_group') AND target_ref_id IS NOT NULL AND target_ref_text IS NULL)
				),
				CONSTRAINT chk_parent_group_threads_title_not_blank CHECK (length(btrim(title)) > 0)
			);
			CREATE INDEX IF NOT EXISTS idx_parent_group_threads_tenant_activity
				ON users.parent_group_threads (tenant_id, last_message_at DESC, id DESC);

			-- created_at stamps the insert itself, not the transaction start,
			-- because it orders the messages and the read cursor (see
			-- users.parent_messages).
			CREATE TABLE IF NOT EXISTS users.parent_group_messages (
				id                BIGSERIAL PRIMARY KEY,
				tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				thread_id         BIGINT NOT NULL REFERENCES users.parent_group_threads(id) ON DELETE CASCADE,
				sender_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
				sender_name       TEXT NOT NULL,
				body              TEXT NOT NULL,
				created_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
				CONSTRAINT chk_parent_group_messages_body_not_blank CHECK (length(btrim(body)) > 0)
			);
			CREATE INDEX IF NOT EXISTS idx_parent_group_messages_thread
				ON users.parent_group_messages (tenant_id, thread_id, id);

			-- One row per child and guardian a message reached. A child without
			-- a reachable guardian is not stored: nobody could read it.
			CREATE TABLE IF NOT EXISTS users.parent_group_message_recipients (
				tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				thread_id           BIGINT NOT NULL REFERENCES users.parent_group_threads(id) ON DELETE CASCADE,
				message_id          BIGINT NOT NULL REFERENCES users.parent_group_messages(id) ON DELETE CASCADE,
				student_id          BIGINT NOT NULL,
				guardian_account_id BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
				PRIMARY KEY (message_id, student_id, guardian_account_id),
				FOREIGN KEY (tenant_id, student_id)
					REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE
			);
			CREATE INDEX IF NOT EXISTS idx_parent_group_message_recipients_guardian
				ON users.parent_group_message_recipients (tenant_id, guardian_account_id, thread_id);
			CREATE INDEX IF NOT EXISTS idx_parent_group_message_recipients_thread
				ON users.parent_group_message_recipients (tenant_id, thread_id, student_id);

			-- The guardian's read cursor per thread: the newest message read.
			CREATE TABLE IF NOT EXISTS users.parent_group_thread_reads (
				tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
				thread_id            BIGINT NOT NULL REFERENCES users.parent_group_threads(id) ON DELETE CASCADE,
				account_id           BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
				last_read_message_id BIGINT NOT NULL,
				read_at              TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
				PRIMARY KEY (thread_id, account_id)
			);

			ALTER TABLE users.parent_messages
				ADD COLUMN IF NOT EXISTS group_thread_id BIGINT
					REFERENCES users.parent_group_threads(id) ON DELETE SET NULL;

			DROP TRIGGER IF EXISTS update_parent_group_threads_updated_at ON users.parent_group_threads;
			CREATE TRIGGER update_parent_group_threads_updated_at
			BEFORE UPDATE ON users.parent_group_threads
			FOR EACH ROW EXECUTE FUNCTION update_modified_column();
		`

// parentGroupThreadSecuritySQL isolates every table per school.
const parentGroupThreadSecuritySQL = `
			ALTER TABLE users.parent_group_threads ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.parent_group_threads FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_users_parent_group_threads ON users.parent_group_threads;
			CREATE POLICY tenant_isolation_users_parent_group_threads ON users.parent_group_threads
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			GRANT SELECT, INSERT, UPDATE, DELETE ON users.parent_group_threads TO phoenix_tenant;
			ALTER TABLE users.parent_group_messages ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.parent_group_messages FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_users_parent_group_messages ON users.parent_group_messages;
			CREATE POLICY tenant_isolation_users_parent_group_messages ON users.parent_group_messages
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			GRANT SELECT, INSERT, UPDATE, DELETE ON users.parent_group_messages TO phoenix_tenant;
			ALTER TABLE users.parent_group_message_recipients ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.parent_group_message_recipients FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_users_parent_group_message_recipients ON users.parent_group_message_recipients;
			CREATE POLICY tenant_isolation_users_parent_group_message_recipients ON users.parent_group_message_recipients
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			GRANT SELECT, INSERT, UPDATE, DELETE ON users.parent_group_message_recipients TO phoenix_tenant;
			ALTER TABLE users.parent_group_thread_reads ENABLE ROW LEVEL SECURITY;
			ALTER TABLE users.parent_group_thread_reads FORCE ROW LEVEL SECURITY;
			DROP POLICY IF EXISTS tenant_isolation_users_parent_group_thread_reads ON users.parent_group_thread_reads;
			CREATE POLICY tenant_isolation_users_parent_group_thread_reads ON users.parent_group_thread_reads
				FOR ALL
				USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
				WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			GRANT SELECT, INSERT, UPDATE, DELETE ON users.parent_group_thread_reads TO phoenix_tenant;
		`

// parentGroupThreadSequencesSQL lets the request role insert rows.
const parentGroupThreadSequencesSQL = `
			GRANT USAGE ON SEQUENCE users.parent_group_threads_id_seq TO phoenix_tenant;
			GRANT USAGE ON SEQUENCE users.parent_group_messages_id_seq TO phoenix_tenant;
		`

// parentGroupThreadsDown drops the group threads. The answers stay in the
// 1:1 conversations; only their reference goes.
func parentGroupThreadsDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			ALTER TABLE users.parent_messages DROP COLUMN IF EXISTS group_thread_id;
			DROP TABLE IF EXISTS users.parent_group_thread_reads;
			DROP TABLE IF EXISTS users.parent_group_message_recipients;
			DROP TABLE IF EXISTS users.parent_group_messages;
			DROP TABLE IF EXISTS users.parent_group_threads;
		`); err != nil {
			return fmt.Errorf("drop parent group thread tables: %w", err)
		}
		return nil
	})
}
