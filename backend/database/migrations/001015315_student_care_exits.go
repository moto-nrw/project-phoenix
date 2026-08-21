package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentCareExitsVersion     = "1.15.315"
	studentCareExitsDescription = "Create users.student_care_exits and allow the care_ended terminal state on the four parent request queues (#2487)."
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentCareExitsVersion,
		Description: studentCareExitsDescription,
		DependsOn:   []string{templateSourceSchoolClassesVersion},
	})

	Migrations.MustRegister(studentCareExitsUp, studentCareExitsDown)
}

// careEndedStatusTables lists the parent request queues whose open rows are
// closed when a child's care ends (#2487). Each needs its status CHECK widened
// by the new terminal value; the existing values are re-listed per table
// because they differ (the Stammdaten queue has auto_applied and no withdrawn).
var careEndedStatusTables = []struct {
	Schema   string
	Table    string
	Existing []string
}{
	{"users", "student_data_change_requests", []string{"auto_applied", "pending", "approved", "rejected"}},
	{"active", "excused_absence_requests", []string{"pending", "approved", "rejected", "withdrawn"}},
	{"enrollment", "offering_change_requests", []string{"pending", "approved", "rejected", "withdrawn"}},
	{"schedule", "care_schedule_change_requests", []string{"pending", "approved", "rejected", "withdrawn"}},
}

// studentCareExitsUp creates the record behind "Betreuung beenden".
//
// The LAST CARE DAY itself is deliberately NOT stored here: users.students
// .enrolled_until already is the business time boundary every reader honours,
// and a second copy would be one more thing that can drift out of sync with it.
// This table carries only what the interval cannot express — why the care
// ended, the optional short note, and who recorded it — and it is read behind
// the users:delete gate, which is why the reason never lives on the student
// row that half the app reads.
func studentCareExitsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.315: Creating users.student_care_exits...")
	if _, err := db.NewRaw(`
		CREATE TABLE IF NOT EXISTS users.student_care_exits (
			id BIGSERIAL PRIMARY KEY,
			tenant_id BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			student_id BIGINT NOT NULL REFERENCES users.students(id) ON DELETE CASCADE,
			reason TEXT NOT NULL
				CHECK (reason IN ('moved_away', 'no_care_needed', 'other')),
			reason_note TEXT,
			recorded_by BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

			-- Only "Anderer Grund" carries free text. Enforced here as well as
			-- in the model so an import or a hand-written UPDATE cannot leave a
			-- note attached to a categorised reason, which the archive view
			-- would then show under a heading that contradicts it.
			CONSTRAINT chk_student_care_exits_note
				CHECK ((reason = 'other') OR reason_note IS NULL)
		);

		-- One exit per child: the current one. Changing a planned exit rewrites
		-- this row, cancelling it deletes it; the history lives in
		-- audit.student_field_edits, which is append-only.
		CREATE UNIQUE INDEX IF NOT EXISTS uq_student_care_exits_student
			ON users.student_care_exits (tenant_id, student_id);

		ALTER TABLE users.student_care_exits ENABLE ROW LEVEL SECURITY;
		ALTER TABLE users.student_care_exits FORCE ROW LEVEL SECURITY;

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_policies
				WHERE schemaname = 'users'
					AND tablename = 'student_care_exits'
					AND policyname = 'tenant_isolation_users_student_care_exits'
			) THEN
				CREATE POLICY tenant_isolation_users_student_care_exits
					ON users.student_care_exits
					FOR ALL
					USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
					WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);
			END IF;
		END $$;

		GRANT SELECT, INSERT, UPDATE, DELETE ON users.student_care_exits TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE users.student_care_exits_id_seq TO phoenix_tenant;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating users.student_care_exits: %w", err)
	}

	// The enrolled_until index that already exists is partial on status =
	// 'active'. The archive view reads the opposite side — children whose
	// interval has run out — so it gets its own partial index.
	if _, err := db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_tenant_enrolled_until_ended
			ON users.students (tenant_id, enrolled_until)
			WHERE enrolled_until IS NOT NULL AND status <> 'alumnus';
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed creating idx_students_tenant_enrolled_until_ended: %w", err)
	}

	for _, target := range careEndedStatusTables {
		if err := widenStatusCheck(ctx, db, target.Schema, target.Table, append(target.Existing, careEndedStatus)); err != nil {
			return err
		}
	}
	return nil
}

// careEndedStatus is the terminal state a request reaches when the child's
// care ended before anybody decided it. Named in one place so the model
// constants and this migration cannot drift.
const careEndedStatus = "care_ended"

// widenStatusCheck replaces whatever CHECK constraint currently governs the
// table's `status` column with one that allows exactly `values`.
//
// The constraints were created inline and are therefore named by PostgreSQL,
// differently per table and (for the two repair migrations) potentially per
// deployment. They are located by the column they constrain rather than by
// name: a single-column CHECK on `status`. Nothing else matches that shape on
// these tables — the absence_status CHECK added in 1.15.310 constrains a
// different column and is left alone.
func widenStatusCheck(ctx context.Context, db *bun.DB, schema, table string, values []string) error {
	literals := ""
	for i, v := range values {
		if i > 0 {
			literals += ", "
		}
		literals += "'" + v + "'"
	}
	stmt := fmt.Sprintf(`
		DO $$
		DECLARE
			existing RECORD;
		BEGIN
			IF to_regclass('%[1]s.%[2]s') IS NULL THEN
				RETURN;
			END IF;
			FOR existing IN
				SELECT c.conname
				FROM pg_constraint c
				JOIN pg_attribute a
					ON a.attrelid = c.conrelid AND a.attnum = ANY (c.conkey)
				WHERE c.conrelid = '%[1]s.%[2]s'::regclass
					AND c.contype = 'c'
					AND array_length(c.conkey, 1) = 1
					AND a.attname = 'status'
			LOOP
				EXECUTE format('ALTER TABLE %[1]s.%[2]s DROP CONSTRAINT %%I', existing.conname);
			END LOOP;
			ALTER TABLE %[1]s.%[2]s
				ADD CONSTRAINT chk_%[2]s_status CHECK (status IN (%[3]s));
		END $$;
	`, schema, table, literals)
	if _, err := db.ExecContext(ctx, stmt); err != nil {
		return fmt.Errorf("failed widening %s.%s status check: %w", schema, table, err)
	}
	return nil
}

func studentCareExitsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.315: Dropping users.student_care_exits...")

	// Rows in the new terminal state would violate the narrowed constraint.
	// Refuse loudly rather than silently rewriting parent-visible decisions.
	for _, target := range careEndedStatusTables {
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`
			DO $$
			BEGIN
				IF to_regclass('%[1]s.%[2]s') IS NULL THEN
					RETURN;
				END IF;
				IF EXISTS (SELECT 1 FROM %[1]s.%[2]s WHERE status = '%[3]s') THEN
					RAISE EXCEPTION 'cannot narrow %[1]s.%[2]s.status while %[3]s rows exist';
				END IF;
			END $$;
		`, target.Schema, target.Table, careEndedStatus)); err != nil {
			return err
		}
		if err := widenStatusCheck(ctx, db, target.Schema, target.Table, target.Existing); err != nil {
			return err
		}
	}

	if _, err := db.NewRaw(`
		DROP INDEX IF EXISTS users.idx_students_tenant_enrolled_until_ended;
		DROP TABLE IF EXISTS users.student_care_exits CASCADE;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("failed dropping users.student_care_exits: %w", err)
	}
	return nil
}
