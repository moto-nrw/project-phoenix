package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	classTeacherTransitionHistoryVersion     = "1.15.279"
	classTeacherTransitionHistoryDescription = "Ledger of class-teacher assignment rewrites per grade transition (#1772)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classTeacherTransitionHistoryVersion,
		Description: classTeacherTransitionHistoryDescription,
		DependsOn:   []string{lehrkraftRoleVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return classTeacherTransitionHistoryUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return classTeacherTransitionHistoryDown(ctx, db)
		},
	)
}

// classTeacherTransitionHistoryUp creates
// education.grade_transition_class_teachers (#1772): the exact ledger of what
// a grade-transition apply did to education.class_teachers — 'removed' rows
// it deleted (with their original display form), 'created' rows it inserted.
// The revert replays this ledger instead of guessing a reverse rename from
// the mappings, which cannot distinguish a row the apply renamed into "2a"
// from a pre-existing "2a" assignment it never touched. Mirrors
// education.grade_transition_history on the student side.
func classTeacherTransitionHistoryUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.279: Class-teacher transition ledger...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS education.grade_transition_class_teachers (
			id            BIGSERIAL PRIMARY KEY,
			tenant_id     BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			transition_id BIGINT NOT NULL REFERENCES education.grade_transitions(id) ON DELETE CASCADE,
			staff_id      BIGINT NOT NULL,
			school_class  TEXT NOT NULL,
			action        TEXT NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_gtct_staff FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_gtct_school_class CHECK (BTRIM(school_class) <> ''),
			CONSTRAINT chk_gtct_action CHECK (action IN ('removed', 'created'))
		);

		CREATE INDEX IF NOT EXISTS idx_gtct_tenant_transition
			ON education.grade_transition_class_teachers (tenant_id, transition_id);

		DROP TRIGGER IF EXISTS update_grade_transition_class_teachers_updated_at ON education.grade_transition_class_teachers;
		CREATE TRIGGER update_grade_transition_class_teachers_updated_at
		BEFORE UPDATE ON education.grade_transition_class_teachers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE education.grade_transition_class_teachers ENABLE ROW LEVEL SECURITY;
		ALTER TABLE education.grade_transition_class_teachers FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_education_grade_transition_class_teachers ON education.grade_transition_class_teachers;
		CREATE POLICY tenant_isolation_education_grade_transition_class_teachers ON education.grade_transition_class_teachers
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.grade_transition_class_teachers TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.grade_transition_class_teachers_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating education.grade_transition_class_teachers: %w", err)
	}

	return tx.Commit()
}

func classTeacherTransitionHistoryDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.279: Dropping education.grade_transition_class_teachers...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS education.grade_transition_class_teachers;
	`)
	if err != nil {
		return fmt.Errorf("error dropping education.grade_transition_class_teachers: %w", err)
	}

	return tx.Commit()
}
