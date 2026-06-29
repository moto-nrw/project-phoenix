package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	parentMessagingVersion     = "1.15.149"
	parentMessagingDescription = "Create parent-OGS messaging tables (threads, messages, reads), backfill from student_parent_notes, then drop that table. Fully idempotent: a production/staging environment that already ran this under the burned 001015148 bun name re-runs it as a safe no-op."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     parentMessagingVersion,
		Description: parentMessagingDescription,
		DependsOn: []string{
			"1.15.114", // users.student_parent_notes (backfill source)
			"1.3.5",    // users.students
			"1.3.5.1",  // users.guardian_profiles (note-backfill join, lines ~198/216)
			"1.3.6",    // users.students_guardians (linked-guardian backfill filter)
			"1.0.1",    // auth.accounts
			"1.13.2",   // auth.account_tenants (read-cursor seeding source)
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return parentMessagingDown(ctx, db)
		},
	)
}

func parentMessagingUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.149: Creating parent messaging tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// A thread is the ONE continuous conversation between the OGS and a single
	// guardian about one child (chat model, no subject). Exactly one thread per
	// (tenant, student, guardian) — enforced by a unique constraint so the
	// "send" path is get-or-create. last_sender_kind lets the inbox flag
	// "awaiting OGS reply" without scanning the messages.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_message_threads (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id          BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			guardian_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			last_message_at     TIMESTAMPTZ,
			last_message_id     BIGINT,
			last_sender_kind    TEXT,
			last_message_body   TEXT NOT NULL DEFAULT '',
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT uq_parent_message_threads_student_guardian UNIQUE (tenant_id, student_id, guardian_account_id),
			CONSTRAINT chk_parent_message_threads_last_sender CHECK (last_sender_kind IN ('guardian','staff'))
		);

		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_inbox
			ON users.parent_message_threads (tenant_id, last_message_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_guardian
			ON users.parent_message_threads (tenant_id, guardian_account_id, last_message_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_message_threads_student
			ON users.parent_message_threads (tenant_id, student_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_message_threads: %w", err)
	}

	// Append-only message log. sender_name is denormalized at send time so
	// the chat history stays stable even if an account/profile is renamed
	// or removed, and reads need no joins. student_id is denormalized for
	// the inbox ACL filter (student -> group).
	//
	// created_at is the read cursor for the whole feature (MarkReadUpTo advances
	// to the newest returned message's created_at), so it MUST stamp at actual
	// insert time, not transaction start. The send runs inside the request-wide
	// tenant transaction and the message INSERT is its LAST statement (after
	// load-thread / settings / guardian-link checks / get-or-create-thread), so
	// NOW() (= transaction_timestamp) would back-date the message to before all
	// that work — widening the [created_at, commit] window during which a
	// concurrent reader can advance its cursor past this still-uncommitted
	// message and silently mark it read. clock_timestamp() stamps at the insert
	// itself, near commit, shrinking that window to the bare tail of the txn.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_messages (
			id                BIGSERIAL PRIMARY KEY,
			tenant_id         BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id         BIGINT NOT NULL REFERENCES users.parent_message_threads(id) ON DELETE CASCADE,
			student_id        BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			sender_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			sender_kind       TEXT NOT NULL,
			sender_name       TEXT NOT NULL,
			body              TEXT NOT NULL,
			created_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			updated_at        TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
			CONSTRAINT chk_parent_messages_sender_kind CHECK (sender_kind IN ('guardian','staff')),
			CONSTRAINT chk_parent_messages_body_not_blank CHECK (length(btrim(body)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_messages_thread_created
			ON users.parent_messages (tenant_id, thread_id, created_at DESC);
		CREATE INDEX IF NOT EXISTS idx_parent_messages_student_created
			ON users.parent_messages (tenant_id, student_id, created_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_messages: %w", err)
	}

	// Per-reader read cursor (account = guardian or staff). Unread is any
	// message after the cursor not sent by the reader. The cursor is the COMPOSITE
	// (last_read_at, last_read_message_id): two messages in a thread can share a
	// created_at (clock_timestamp() is microsecond-precision, so rapid/concurrent
	// sends can tie), and the message list breaks those ties by id DESC. A
	// timestamp-only cursor would treat a tied message that committed after the
	// reader's snapshot as already read (created_at NOT > last_read_at), silently
	// dropping it from the unread badge — so the cursor and every unread predicate
	// carry the same id tie-breaker. Defaults to 0 ("before any message").
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.parent_message_reads (
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id),
			thread_id            BIGINT NOT NULL REFERENCES users.parent_message_threads(id) ON DELETE CASCADE,
			account_id           BIGINT NOT NULL REFERENCES auth.accounts(id),
			last_read_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_read_message_id BIGINT NOT NULL DEFAULT 0,
			PRIMARY KEY (thread_id, account_id)
		);

		CREATE INDEX IF NOT EXISTS idx_parent_message_reads_account
			ON users.parent_message_reads (tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error creating users.parent_message_reads: %w", err)
	}

	// updated_at triggers
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_parent_message_threads_updated_at ON users.parent_message_threads;
		CREATE TRIGGER update_parent_message_threads_updated_at
		BEFORE UPDATE ON users.parent_message_threads
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();

		DROP TRIGGER IF EXISTS update_parent_messages_updated_at ON users.parent_messages;
		CREATE TRIGGER update_parent_messages_updated_at
		BEFORE UPDATE ON users.parent_messages
		FOR EACH ROW EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at triggers: %w", err)
	}

	// RLS — tenant isolation, same shape as student_parent_notes. CREATE POLICY has
	// no IF NOT EXISTS in PostgreSQL, so guard it with a pg_policies existence check
	// (mirrors 001015148_enrollment_change_requests): an environment that already ran
	// parent messaging under the burned 001015148 bun name re-runs this without
	// "policy already exists". ENABLE/FORCE RLS and GRANT are naturally idempotent.
	for _, table := range []string{"parent_message_threads", "parent_messages", "parent_message_reads"} {
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
		GRANT USAGE ON SEQUENCE users.parent_message_threads_id_seq TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.parent_messages_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error granting sequence usage: %w", err)
	}

	// Backfill only when the legacy source table still exists. The existence of
	// users.student_parent_notes IS the "backfill not yet done" flag: on a fresh DB
	// and on an environment that ran *enrollment* under the burned 001015148 bun
	// name (notes left intact) it is present -> run the one-time backfill below; on
	// the environment that already ran *parent messaging* under 001015148 the table
	// was dropped at the end of that run, so it is absent here -> skip the whole
	// backfill. This makes the re-run safe (no "relation does not exist") and
	// non-duplicating (the threads/messages/cursors from the first run stay as-is).
	var notesExist bool
	if err = tx.QueryRowContext(ctx,
		`SELECT to_regclass('users.student_parent_notes') IS NOT NULL`,
	).Scan(&notesExist); err != nil {
		return fmt.Errorf("error checking for users.student_parent_notes: %w", err)
	}

	if notesExist {
		// Backfill from existing parent notes. The migration runs as the superuser
		// (bypasses RLS), so it sees every tenant's notes. One thread per
		// (tenant, student, guardian); each note becomes a guardian message with
		// the guardian's name frozen from their profile at migration time.
		//
		// Two things the naive backfill got wrong:
		//   1. last_sender_kind='guardian' on every thread flooded the staff inbox
		//      with "awaiting OGS reply" for notes already handled in the old UI.
		//      Seed it NULL (a neutral "nobody is waiting" state the CHECK permits)
		//      instead.
		//   2. No read cursor meant every historical note counted as unread, so a
		//      school that used notes for months saw its inbox badge inflated by the
		//      entire backlog. Seed a read cursor (below) so migrated history starts
		//      read for everyone who already has access.
		//
		// Backfill EVERY note (no students_guardians filter): a guardian deliberately
		// unlinked from the child since writing a note must not have that free-text
		// silently destroyed when the source table is dropped below. The thread is
		// still created, but the parent-portal read paths gate on a live link with
		// parent_portal.access (guardianStillLinked), so an unlinked guardian never
		// sees the resurrected thread — staff keep the history, nothing is lost.
		_, err = tx.ExecContext(ctx, `
		INSERT INTO users.parent_message_threads
			(tenant_id, student_id, guardian_account_id, last_message_at, last_sender_kind, created_at, updated_at)
		SELECT n.tenant_id, n.student_id, n.guardian_account_id,
		       MAX(n.created_at), NULL, MIN(n.created_at), MAX(n.created_at)
		FROM users.student_parent_notes n
		GROUP BY n.tenant_id, n.student_id, n.guardian_account_id;

		INSERT INTO users.parent_messages
			(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, created_at, updated_at)
		SELECT n.tenant_id, t.id, n.student_id, n.guardian_account_id, 'guardian',
		       COALESCE(NULLIF(btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')), ''), 'Elternteil'),
		       n.body, n.created_at, n.updated_at
		FROM users.student_parent_notes n
		JOIN users.parent_message_threads t
			ON t.tenant_id = n.tenant_id
			AND t.student_id = n.student_id
			AND t.guardian_account_id = n.guardian_account_id
		LEFT JOIN users.guardian_profiles gp
			ON gp.account_id = n.guardian_account_id
			AND gp.tenant_id = n.tenant_id;
	`)
		if err != nil {
			return fmt.Errorf("error backfilling parent messages from notes: %w", err)
		}

		// Denormalize the inbox preview (last_message_body) and the composite
		// tie-breaker (last_message_id) onto each backfilled thread so the staff
		// inbox projection reads them off the thread row instead of a correlated
		// subquery. Pick the newest message per thread (created_at, then id as a
		// deterministic tiebreaker), mirroring the projection's ORDER BY created_at
		// DESC, id DESC — and seed last_message_id from that SAME row so the
		// TouchLastMessage monotonic guard has the right (created_at, id) baseline.
		_, err = tx.ExecContext(ctx, `
		UPDATE users.parent_message_threads t
		SET last_message_body = lm.body,
		    last_message_id = lm.id
		FROM (
			SELECT DISTINCT ON (m.thread_id) m.thread_id, m.body, m.id
			FROM users.parent_messages m
			ORDER BY m.thread_id, m.created_at DESC, m.id DESC
		) lm
		WHERE lm.thread_id = t.id;
	`)
		if err != nil {
			return fmt.Errorf("error backfilling thread last_message_body: %w", err)
		}

		// Seed a read cursor at each backfilled thread's newest message for every
		// account currently active in the tenant. Migrated history therefore starts
		// READ for all existing staff (no backlog badge) and for the guardian
		// themselves; only messages sent AFTER the migration count as unread. New
		// accounts created later have no cursor and will see migrated history as
		// unread on first open — acceptable, since they have not reviewed it yet.
		//
		// Accepted tradeoff: the parent portal's "OGS hat gelesen" indicator derives
		// from the newest read cursor of a STAFF account (LatestReadCursorByOther gates
		// positively on users.staff membership at the thread's tenant), so seeding
		// staff cursors here makes every migrated note render as read by the OGS. That
		// is truthful — staff saw these notes in the old notes UI before migration.
		// Seeding guardian / other-parent cursors too is harmless to that indicator
		// (they are not staff, so they are never counted); they only serve the
		// backlog-badge suppression. The two indicators share this single cursor
		// table, so suppressing the staff backlog badge (above) and hiding the parent
		// indicator cannot both be satisfied. The badge suppression is the more
		// important UX, so it wins.
		// Seed last_read_message_id alongside last_read_at at the thread's newest
		// message (by created_at DESC, id DESC — the same ordering the inbox preview
		// backfill above uses), so the seeded cursor is the exact composite the unread
		// predicates compare against. Threads always have a backfilled message here,
		// but COALESCE keeps the seed safe (0) if somehow none exists.
		_, err = tx.ExecContext(ctx, `
		INSERT INTO users.parent_message_reads (tenant_id, thread_id, account_id, last_read_at, last_read_message_id)
		SELECT t.tenant_id, t.id, atn.account_id,
		       COALESCE(t.last_message_at, t.created_at),
		       COALESCE(lm.id, 0)
		FROM users.parent_message_threads t
		JOIN auth.account_tenants atn
			ON atn.tenant_id = t.tenant_id
			AND atn.status = 'active'
		LEFT JOIN LATERAL (
			SELECT m.id
			FROM users.parent_messages m
			WHERE m.thread_id = t.id
			ORDER BY m.created_at DESC, m.id DESC
			LIMIT 1
		) lm ON true
		ON CONFLICT (thread_id, account_id) DO NOTHING;
	`)
		if err != nil {
			return fmt.Errorf("error seeding parent message read cursors: %w", err)
		}
	} // end if notesExist

	// Drop the old one-way notes table now that ALL of its content lives in
	// parent_messages — the backfill above migrated every note (including those
	// from since-unlinked guardians), so dropping the table loses no free-text.
	// No production Go code references users.student_parent_notes after this PR.
	// The down migration recreates the table AND backfills the guardian free-text
	// back into it from parent_messages before dropping the messaging tables, so a
	// rollback recovers every guardian-authored note rather than shedding it.
	_, err = tx.ExecContext(ctx, `DROP TABLE IF EXISTS users.student_parent_notes CASCADE;`)
	if err != nil {
		return fmt.Errorf("error dropping users.student_parent_notes: %w", err)
	}

	return tx.Commit()
}

func parentMessagingDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.149: Dropping parent messaging tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Recreate users.student_parent_notes FIRST (schema, index, trigger, RLS,
	// grants exactly as migration 1.15.114 created it), so the guardian free-text
	// can be backfilled into it BEFORE the messaging tables — its only home — are
	// dropped. The up path CASCADE-dropped this table after migrating its content
	// into parent_messages; rolling back must restore it so the pre-branch binary's
	// parent-note routes don't fail with "relation does not exist".
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.student_parent_notes (
			id                  BIGSERIAL PRIMARY KEY,
			tenant_id           BIGINT NOT NULL REFERENCES platform.schools(id),
			student_id          BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			guardian_account_id BIGINT NOT NULL REFERENCES auth.accounts(id),
			body                TEXT NOT NULL,
			created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_student_parent_notes_body_not_blank CHECK (length(btrim(body)) > 0)
		);

		CREATE INDEX IF NOT EXISTS idx_student_parent_notes_student_created
			ON users.student_parent_notes (tenant_id, student_id, created_at DESC);

		DROP TRIGGER IF EXISTS update_student_parent_notes_updated_at ON users.student_parent_notes;
		CREATE TRIGGER update_student_parent_notes_updated_at
		BEFORE UPDATE ON users.student_parent_notes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.student_parent_notes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_parent_notes FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_student_parent_notes ON users.student_parent_notes;
		CREATE POLICY tenant_isolation_users_student_parent_notes ON users.student_parent_notes
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_parent_notes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_parent_notes_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.student_parent_notes: %w", err)
	}

	// Backfill the guardian free-text BACK into student_parent_notes before the
	// messaging tables are dropped, so a `migrate down` / `migrate reset` is NOT a
	// silent data-loss footgun: every guardian-authored plain message (which is
	// exactly what the old one-way notes table held — the migrated historical notes
	// plus any new guardian messages) is recovered. Staff replies, structured
	// requests and system events have no representation in the old one-way schema
	// and are necessarily shed on rollback; only guardian free-text round-trips.
	// The chk_..._body_not_blank CHECK on both tables guarantees every copied body
	// is non-blank.
	//
	// Filter ONLY on sender_kind = 'guardian'. The `kind` column does not exist at
	// this migration's own schema level — it is added by the later #1672 requests
	// migration (expected 1.15.154), whose down migration (which DependsOn 1.15.149,
	// so it runs first on a reverse-order rollback) has already DROP COLUMN'd it by
	// the time this runs. Referencing m.kind here would fail with "column m.kind does
	// not exist" and abort the whole rollback, restoring zero notes. At the 1.15.149
	// schema every guardian row is a plain message; the few guardian-authored request
	// rows (added by 1.15.154) carry
	// human-readable German bodies ("Anfrage: …"), so restoring them as notes is
	// harmless and there is no surviving column to distinguish them anyway.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO users.student_parent_notes
			(tenant_id, student_id, guardian_account_id, body, created_at, updated_at)
		SELECT m.tenant_id, m.student_id, m.sender_account_id, m.body, m.created_at, m.updated_at
		FROM users.parent_messages m
		WHERE m.sender_kind = 'guardian';
	`)
	if err != nil {
		return fmt.Errorf("error restoring guardian notes from parent messages: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_parent_messages_updated_at ON users.parent_messages;
		DROP TRIGGER IF EXISTS update_parent_message_threads_updated_at ON users.parent_message_threads;
		DROP TABLE IF EXISTS users.parent_message_reads CASCADE;
		DROP TABLE IF EXISTS users.parent_messages CASCADE;
		DROP TABLE IF EXISTS users.parent_message_threads CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping parent messaging tables: %w", err)
	}

	return tx.Commit()
}
