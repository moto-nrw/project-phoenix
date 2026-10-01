package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentNotesVersion     = "1.15.438"
	studentNotesDescription = "Kindnotizen: Kartei je Kind mit Bezug, Sichtbarkeit und dauerhaften Hinweisen"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentNotesVersion,
		Description: studentNotesDescription,
		DependsOn:   []string{settingsCleanupVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentNotesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentNotesDown(ctx, db)
		},
	)
}

// studentNotesUp creates users.student_notes — the note card ("Kartei") a
// school keeps for one child.
//
// One table, not three. A note carries at most one subject reference, and that
// reference is what distinguishes the three cases the OGS asked for: no
// reference at all is the general note that used to live in the Stammdaten
// field, an activity reference is the note about a child in one course, and a
// subject_date is the note about one day. Splitting these into separate tables
// would triple the read path of a single chronological timeline without
// separating anything the domain treats differently.
//
// kind separates the two lifetimes on the same row. A 'permanent' note is the
// durable hint that belongs with the master data and is shown on the Stammdaten
// tab; a 'journal' note is one dated entry in the chronicle. Promoting an entry
// is therefore an UPDATE of one column, not a copy into a second table.
//
// visibility is deliberately per note and not derived from the reference: the
// same course note may be a team-wide hint one day and a matter for the group
// leads the next. The three values map onto predicates that already exist —
// 'all_staff' is today's read scope (authorize.CanReadStudent), 'care_team' is
// the staff whose own groups, own activities or own school classes include the
// child, and 'group_leads' is the leadership of the referenced group. NOTE:
// CONTEXT.md states that groups do not govern the visibility of child data.
// Notes are the first deliberate exception to that sentence; see the glossary
// entry added with this change.
func studentNotesUp(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.student_notes (
			id                   BIGSERIAL PRIMARY KEY,
			tenant_id            BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id           BIGINT NOT NULL,
			author_account_id    BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			origin               VARCHAR(20) NOT NULL DEFAULT 'staff',
			kind                 VARCHAR(20) NOT NULL DEFAULT 'journal',
			visibility           VARCHAR(20) NOT NULL DEFAULT 'all_staff',
			category             VARCHAR(40),
			body                 TEXT NOT NULL,
			subject_date         DATE,
			activity_group_id    BIGINT REFERENCES activities.groups(id) ON DELETE SET NULL,
			education_group_id   BIGINT REFERENCES education.groups(id) ON DELETE SET NULL,
			deleted_at           TIMESTAMPTZ,
			deleted_by_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT chk_student_notes_body CHECK (length(btrim(body)) > 0),
			CONSTRAINT chk_student_notes_origin CHECK (origin IN ('staff', 'master_data')),
			CONSTRAINT chk_student_notes_kind CHECK (kind IN ('permanent', 'journal')),
			CONSTRAINT chk_student_notes_visibility CHECK (
				visibility IN ('all_staff', 'care_team', 'group_leads')
			),
			-- At most one group reference. A note is about the child in
			-- general, about one activity, or about one group — never about two
			-- at once, because the group_leads audience would then have two
			-- different leaderships to ask.
			--
			-- One occurrence of an activity needs no reference of its own: it
			-- is the activity plus the day (activity_group_id + subject_date),
			-- which is also how the timetable already separates its durable
			-- Wochennotiz from its Tagesnotiz.
			CONSTRAINT chk_student_notes_single_reference CHECK (
				activity_group_id IS NULL OR education_group_id IS NULL
			),
			-- 'group_leads' asks the leadership of a referenced group. Without
			-- a reference there is no leadership to ask, and the note would be
			-- visible to nobody but its author.
			CONSTRAINT chk_student_notes_group_leads_reference CHECK (
				visibility <> 'group_leads'
				OR activity_group_id IS NOT NULL
				OR education_group_id IS NOT NULL
			),
			-- A durable hint has no day, while every chronicle entry has one.
			-- These two lifetimes must not overlap: a dated hint belongs in the
			-- journal and an undated entry cannot be placed in its chronology.
			CONSTRAINT chk_student_notes_permanent_undated CHECK (
				(kind = 'permanent' AND subject_date IS NULL)
				OR (kind = 'journal' AND subject_date IS NOT NULL)
			),
			CONSTRAINT chk_student_notes_deletion CHECK (
				(deleted_at IS NULL) = (deleted_by_account_id IS NULL)
			),
			-- The tenant travels into the foreign key, as on every table the
			-- student split created: a note can only ever point at a child of
			-- its own school, enforced by the constraint and not only by RLS.
			CONSTRAINT fk_student_notes_student FOREIGN KEY (tenant_id, student_id)
				REFERENCES users.student_profiles(tenant_id, id) ON DELETE CASCADE
		);

		COMMENT ON TABLE users.student_notes IS 'Kindnotizen (Kartei): dauerhafte Hinweise und datierte Einträge je Kind, mit eigener Sichtbarkeit je Notiz.';
		COMMENT ON COLUMN users.student_notes.origin IS 'staff = von einer Person geschrieben; master_data = beim Umstieg aus users.students.supervisor_notes übernommen, daher ohne Autor.';

		-- The timeline of one child is the only read that matters for the
		-- detail page; the partial predicate keeps soft-deleted rows out of the
		-- index the timeline scans.
		CREATE INDEX IF NOT EXISTS idx_student_notes_student_created
			ON users.student_notes (tenant_id, student_id, created_at DESC)
			WHERE deleted_at IS NULL;

		-- The Stammdaten tab reads only the durable hints of one child.
		CREATE INDEX IF NOT EXISTS idx_student_notes_permanent
			ON users.student_notes (tenant_id, student_id)
			WHERE kind = 'permanent' AND deleted_at IS NULL;

		DROP TRIGGER IF EXISTS update_student_notes_updated_at ON users.student_notes;
		CREATE TRIGGER update_student_notes_updated_at
		BEFORE UPDATE ON users.student_notes
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_notes TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_notes_id_seq TO phoenix_tenant;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error creating users.student_notes: %w", err)
	}

	// Carry the former free-text field over as one durable hint per child. It
	// runs before RLS is provisioned so the migration's superuser connection
	// reads every tenant's rows without a tenant setting.
	if err := studentNotesBackfill(ctx, tx); err != nil {
		return err
	}

	if err := provisionTenantRLS(ctx, tx, "users.student_notes"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student notes migration: %w", err)
	}
	return nil
}

// studentNotesBackfill copies every non-empty Betreuernotiz into the note card
// as one durable hint per child.
//
// The text lives in Care Plan's users.student_care_profiles since the student
// split (#2717); the join walks care profile → membership → child because the
// note belongs to the child, not to one enrollment of it. DISTINCT ON picks the
// newest membership for a child that has more than one, so a re-enrolled child
// gets one hint and not two.
//
// author_account_id stays NULL: the column held a text, not an authorship, and
// inventing one would put a name under words that person may never have
// written. origin = 'master_data' is what the UI renders instead.
//
// Idempotent by predicate, not by constraint: re-running the migration after a
// partial failure must not duplicate the hints, and a school that deliberately
// deleted a carried-over note must not have it resurrected either — hence the
// soft-deleted rows count as present.
//
// Extracted from the Up step so a test can run exactly this statement against
// real rows instead of re-typing it.
func studentNotesBackfill(ctx context.Context, db bun.IDB) error {
	_, err := db.NewRaw(`
		INSERT INTO users.student_notes (
			tenant_id, student_id, origin, kind, visibility, body, created_at, updated_at
		)
		SELECT DISTINCT ON (profile.tenant_id, profile.id)
			profile.tenant_id, profile.id, 'master_data', 'permanent', 'all_staff',
			btrim(care.supervisor_notes), NOW(), NOW()
		FROM users.student_care_profiles AS care
		JOIN users.student_school_memberships AS membership
			ON membership.tenant_id = care.tenant_id AND membership.id = care.membership_id
		JOIN users.student_profiles AS profile
			ON profile.tenant_id = membership.tenant_id AND profile.id = membership.student_profile_id
		WHERE care.supervisor_notes IS NOT NULL
		  AND length(btrim(care.supervisor_notes)) > 0
		  AND NOT EXISTS (
			SELECT 1 FROM users.student_notes AS existing
			WHERE existing.tenant_id = profile.tenant_id
			  AND existing.student_id = profile.id
			  AND existing.origin = 'master_data'
		  )
		ORDER BY profile.tenant_id, profile.id, membership.id DESC;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("error carrying supervisor notes into users.student_notes: %w", err)
	}
	return nil
}

// studentNotesDown drops the table. The carried-over hints are safe to lose
// here: the rollback target still has
// users.student_care_profiles.supervisor_notes, which this migration
// deliberately left in place and untouched.
func studentNotesDown(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	if _, err := tx.NewRaw(`DROP TABLE IF EXISTS users.student_notes;`).Exec(ctx); err != nil {
		return fmt.Errorf("error dropping users.student_notes: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit student notes rollback: %w", err)
	}
	return nil
}
