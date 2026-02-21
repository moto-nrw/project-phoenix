package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	relaxVisitsStudentFKVersion     = "1.15.6"
	relaxVisitsStudentFKDescription = "Relax active.visits student FK from composite to simple for cross-tenant visits (Ferienbetreuung)"
)

func init() {
	MigrationRegistry[relaxVisitsStudentFKVersion] = &Migration{
		Version:     relaxVisitsStudentFKVersion,
		Description: relaxVisitsStudentFKDescription,
		DependsOn:   []string{"1.15.5"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.6: Relaxing active.visits student FK for cross-tenant visits...")

			// During Ferienbetreuung (holiday care), students from School A visit
			// School B's OGS. The visit record has tenant_id = School B (hosting),
			// but the student has tenant_id = School A (home). The composite FK
			// (tenant_id, student_id) blocks this, so we replace it with a simple
			// FK on student_id only.
			_, err := db.ExecContext(ctx, `
				-- Drop the composite FK (tenant_id, student_id) → (tenant_id, id)
				ALTER TABLE active.visits
					DROP CONSTRAINT IF EXISTS fk_visits_student_tenant;

				-- Add a simple FK on student_id → students.id (cross-tenant safe)
				ALTER TABLE active.visits
					ADD CONSTRAINT fk_visits_student
					FOREIGN KEY (student_id) REFERENCES users.students(id)
					ON DELETE CASCADE;
			`)
			if err != nil {
				return fmt.Errorf("migration 1.15.6: %w", err)
			}

			fmt.Println("Migration 1.15.6: Done — active.visits.student_id now allows cross-tenant references")
			return nil
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rollback 1.15.6: Restoring composite FK on active.visits...")

			_, err := db.ExecContext(ctx, `
				-- Drop the simple FK
				ALTER TABLE active.visits
					DROP CONSTRAINT IF EXISTS fk_visits_student;

				-- Restore the composite FK
				ALTER TABLE active.visits
					ADD CONSTRAINT fk_visits_student_tenant
					FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id)
					ON DELETE CASCADE;
			`)
			if err != nil {
				return fmt.Errorf("rollback 1.15.6: %w", err)
			}

			return nil
		},
	)
}
