package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	preserveStudentDeletionAuditVersion     = "1.15.244"
	preserveStudentDeletionAuditDescription = "Preserve data-deletion audit records after permanent student deletion (#2068)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     preserveStudentDeletionAuditVersion,
		Description: preserveStudentDeletionAuditDescription,
		DependsOn:   []string{studentDeletionsAuditVersion},
	})

	Migrations.MustRegister(preserveStudentDeletionAuditUp, preserveStudentDeletionAuditDown)
}

func preserveStudentDeletionAuditUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.244: Preserving data-deletion audit records for deleted students...")
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			DROP CONSTRAINT IF EXISTS fk_data_deletions_student;

		-- Ferienbetreuung visits belong to the hosting tenant and are therefore
		-- hidden by active.visits RLS from the child's home tenant. The function
		-- only returns a count and verifies that the requested child belongs to
		-- that home tenant before bypassing RLS.
		CREATE OR REPLACE FUNCTION active.count_student_visits_for_deletion(
			p_tenant_id BIGINT,
			p_student_id BIGINT
		)
		RETURNS INTEGER
		LANGUAGE sql
		STABLE
		SECURITY DEFINER
		SET search_path = pg_catalog, active, users
		AS $$
			SELECT COUNT(*)::INTEGER
			FROM active.visits AS visit
			JOIN users.students AS student ON student.id = visit.student_id
			WHERE visit.student_id = p_student_id
			  AND student.tenant_id = p_tenant_id
			  AND (
				NULLIF(current_setting('app.current_tenant_id', true), '') IS NULL
				OR student.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT
			  );
		$$;
		REVOKE ALL ON FUNCTION active.count_student_visits_for_deletion(BIGINT, BIGINT) FROM PUBLIC;
		GRANT EXECUTE ON FUNCTION active.count_student_visits_for_deletion(BIGINT, BIGINT) TO phoenix_tenant;
	`); err != nil {
		return fmt.Errorf("preserve student deletion audit records: %w", err)
	}
	return nil
}

func preserveStudentDeletionAuditDown(ctx context.Context, db *bun.DB) error {
	var orphanedRows int
	if err := db.NewRaw(`
		SELECT COUNT(*)
		FROM audit.data_deletions AS deletion
		LEFT JOIN users.students AS student ON student.id = deletion.student_id
		WHERE deletion.student_id IS NOT NULL AND student.id IS NULL
	`).Scan(ctx, &orphanedRows); err != nil {
		return fmt.Errorf("check preserved student deletion audit records: %w", err)
	}
	if orphanedRows > 0 {
		return fmt.Errorf("cannot restore data_deletions student foreign key while %d preserved deletion audit records exist", orphanedRows)
	}
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS active.count_student_visits_for_deletion(BIGINT, BIGINT);`); err != nil {
		return fmt.Errorf("drop cross-tenant visit count function: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		ALTER TABLE audit.data_deletions
			ADD CONSTRAINT fk_data_deletions_student
			FOREIGN KEY (student_id) REFERENCES users.students(id) ON DELETE CASCADE;
	`); err != nil {
		return fmt.Errorf("restore data_deletions student foreign key: %w", err)
	}
	return nil
}
