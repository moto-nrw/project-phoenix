package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	classListEntriesVersion     = "1.15.306"
	classListEntriesDescription = "Minimal class-list-only entries (Name + Klasse) for children without an OGS record (#2382)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classListEntriesVersion,
		Description: classListEntriesDescription,
		DependsOn:   []string{classTeachersVersion},
	})

	Migrations.MustRegister(classListEntriesUp, classListEntriesDown)
}

// classListEntriesUp creates users.class_list_entries (#2382): one row is a
// child of the class cohort that has NO OGS record — only first name, last
// name and the free-text school class. The row exists so class lists and the
// Lehrkraft class-day view can show the complete Klassenverband; it is
// deliberately NOT a student: no person, no guardians, no attendance, no care
// planning can ever reference it. The class is the same free-text value as
// users.students.school_class, matched via LOWER(BTRIM(...)).
//
// audit.class_list_entry_changes is the append-only trail: entries are
// personal data of children, so create / update / delete / assign must leave
// a trace (same contract as audit.staff_master_data_changes).
//
// education.grade_transition_class_list_entries is the per-transition ledger
// of what a grade-transition apply did to the entries — 'removed' rows it
// deleted (with their original display form), 'created' rows it inserted.
// The revert replays this ledger instead of guessing a reverse rename from
// the mappings, which cannot distinguish an entry the apply renamed into
// "2a" from a pre-existing "2a" entry it never touched (mirror of
// education.grade_transition_class_teachers).
func classListEntriesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.306: Class-list-only entries...")

	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS users.class_list_entries (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			first_name   TEXT NOT NULL,
			last_name    TEXT NOT NULL,
			school_class TEXT NOT NULL,
			created_by   BIGINT,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_class_list_entries_first_name CHECK (BTRIM(first_name) <> ''),
			CONSTRAINT chk_class_list_entries_last_name CHECK (BTRIM(last_name) <> ''),
			CONSTRAINT chk_class_list_entries_school_class CHECK (BTRIM(school_class) <> '')
		);

		-- One child appears once per class: an identical name in the same
		-- class is an input error, never a second child (twins differ in the
		-- first name). Race-safe backstop for the service-level guard.
		CREATE UNIQUE INDEX IF NOT EXISTS uniq_class_list_entries_name_class
			ON users.class_list_entries (tenant_id, LOWER(BTRIM(first_name)), LOWER(BTRIM(last_name)), LOWER(BTRIM(school_class)));

		CREATE INDEX IF NOT EXISTS idx_class_list_entries_tenant_class
			ON users.class_list_entries (tenant_id, LOWER(BTRIM(school_class)));

		DROP TRIGGER IF EXISTS update_class_list_entries_updated_at ON users.class_list_entries;
		CREATE TRIGGER update_class_list_entries_updated_at
		BEFORE UPDATE ON users.class_list_entries
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE users.class_list_entries ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.class_list_entries FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_users_class_list_entries ON users.class_list_entries;
		CREATE POLICY tenant_isolation_users_class_list_entries ON users.class_list_entries
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.class_list_entries TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.class_list_entries_id_seq TO phoenix_tenant;

		CREATE TABLE IF NOT EXISTS audit.class_list_entry_changes (
			id                 BIGSERIAL PRIMARY KEY,
			tenant_id          BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			entry_id           BIGINT NOT NULL,
			action             TEXT NOT NULL,
			old_value          TEXT NOT NULL DEFAULT '',
			new_value          TEXT NOT NULL DEFAULT '',
			matched_student_id BIGINT,
			changed_by         BIGINT NOT NULL,
			occurred_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_class_list_entry_changes_action
				CHECK (action IN ('created', 'updated', 'deleted', 'assigned'))
		);

		CREATE INDEX IF NOT EXISTS idx_class_list_entry_changes_entry
			ON audit.class_list_entry_changes (tenant_id, entry_id, occurred_at DESC);

		ALTER TABLE audit.class_list_entry_changes ENABLE ROW LEVEL SECURITY;
		ALTER TABLE audit.class_list_entry_changes FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_class_list_entry_changes ON audit.class_list_entry_changes;
		CREATE POLICY tenant_isolation_class_list_entry_changes ON audit.class_list_entry_changes
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		-- Append-only from the application's point of view: the app role gets
		-- no DELETE. No retention cleanup consumes this table yet (same as
		-- audit.staff_master_data_changes); a future cleanup would run on the
		-- superuser CLI connection and needs no extra grant here.
		GRANT SELECT, INSERT ON audit.class_list_entry_changes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE audit.class_list_entry_changes_id_seq TO phoenix_tenant;

		CREATE TABLE IF NOT EXISTS education.grade_transition_class_list_entries (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			transition_id BIGINT NOT NULL REFERENCES education.grade_transitions(id) ON DELETE CASCADE,
			first_name    TEXT NOT NULL,
			last_name     TEXT NOT NULL,
			school_class  TEXT NOT NULL,
			action        TEXT NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_gtcle_first_name CHECK (BTRIM(first_name) <> ''),
			CONSTRAINT chk_gtcle_last_name CHECK (BTRIM(last_name) <> ''),
			CONSTRAINT chk_gtcle_school_class CHECK (BTRIM(school_class) <> ''),
			CONSTRAINT chk_gtcle_action CHECK (action IN ('removed', 'created'))
		);

		CREATE INDEX IF NOT EXISTS idx_gtcle_tenant_transition
			ON education.grade_transition_class_list_entries (tenant_id, transition_id);

		DROP TRIGGER IF EXISTS update_grade_transition_class_list_entries_updated_at ON education.grade_transition_class_list_entries;
		CREATE TRIGGER update_grade_transition_class_list_entries_updated_at
		BEFORE UPDATE ON education.grade_transition_class_list_entries
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE education.grade_transition_class_list_entries ENABLE ROW LEVEL SECURITY;
		ALTER TABLE education.grade_transition_class_list_entries FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_education_grade_transition_class_list_entries ON education.grade_transition_class_list_entries;
		CREATE POLICY tenant_isolation_education_grade_transition_class_list_entries ON education.grade_transition_class_list_entries
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.grade_transition_class_list_entries TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.grade_transition_class_list_entries_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating users.class_list_entries: %w", err)
	}

	return nil
}

func classListEntriesDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.306: Dropping users.class_list_entries...")

	_, err := db.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.grade_transition_class_list_entries;
		DROP TABLE IF EXISTS audit.class_list_entry_changes;
		DROP TABLE IF EXISTS users.class_list_entries;
	`)
	if err != nil {
		return fmt.Errorf("error dropping users.class_list_entries: %w", err)
	}

	return nil
}
