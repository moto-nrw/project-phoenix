package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	classTeachersVersion     = "1.15.277"
	classTeachersDescription = "Staff-to-school-class assignments for the Lehrkraft scope (#1772)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     classTeachersVersion,
		Description: classTeachersDescription,
		DependsOn:   []string{studentDocumentsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return classTeachersUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return classTeachersDown(ctx, db)
		},
	)
}

// classTeachersUp creates education.class_teachers (#1772): one row assigns a
// staff member to one school class. The class is the free-text
// users.students.school_class value — deliberately no class entity, matched
// via LOWER(BTRIM(...)) like the existing student class filters. The
// assignment is what scopes the read-only Lehrkraft class day view.
func classTeachersUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.277: Staff-to-school-class assignments...")

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
		CREATE TABLE IF NOT EXISTS education.class_teachers (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id     BIGINT NOT NULL,
			school_class TEXT NOT NULL,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT fk_class_teachers_staff FOREIGN KEY (tenant_id, staff_id)
				REFERENCES users.staff(tenant_id, id) ON DELETE CASCADE,
			CONSTRAINT chk_class_teachers_school_class CHECK (BTRIM(school_class) <> '')
		);

		CREATE UNIQUE INDEX IF NOT EXISTS uniq_class_teachers_staff_class
			ON education.class_teachers (tenant_id, staff_id, LOWER(BTRIM(school_class)));

		CREATE INDEX IF NOT EXISTS idx_class_teachers_tenant_class
			ON education.class_teachers (tenant_id, LOWER(BTRIM(school_class)));

		DROP TRIGGER IF EXISTS update_class_teachers_updated_at ON education.class_teachers;
		CREATE TRIGGER update_class_teachers_updated_at
		BEFORE UPDATE ON education.class_teachers
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();

		ALTER TABLE education.class_teachers ENABLE ROW LEVEL SECURITY;
		ALTER TABLE education.class_teachers FORCE ROW LEVEL SECURITY;

		DROP POLICY IF EXISTS tenant_isolation_education_class_teachers ON education.class_teachers;
		CREATE POLICY tenant_isolation_education_class_teachers ON education.class_teachers
			FOR ALL
			USING (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
			WITH CHECK (tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint);

		GRANT SELECT, INSERT, UPDATE, DELETE ON education.class_teachers TO phoenix_tenant;
		GRANT USAGE ON SEQUENCE education.class_teachers_id_seq TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("error creating education.class_teachers: %w", err)
	}

	return tx.Commit()
}

func classTeachersDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.277: Dropping education.class_teachers...")

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
		DROP TABLE IF EXISTS education.class_teachers;
	`)
	if err != nil {
		return fmt.Errorf("error dropping education.class_teachers: %w", err)
	}

	return tx.Commit()
}
