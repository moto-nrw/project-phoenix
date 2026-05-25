package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	studentLifecycleVersion     = "1.15.56"
	studentLifecycleDescription = "Add status, enrolled_from, enrolled_until columns to users.students for parent-enrollment lifecycle"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentLifecycleVersion,
		Description: studentLifecycleDescription,
		DependsOn: []string{
			UsersStudentsVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return studentLifecycleUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return studentLifecycleDown(ctx, db)
		},
	)
}

func studentLifecycleUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.56: Adding lifecycle columns (status, enrolled_from, enrolled_until) to users.students...")

	_, err := db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'
			CHECK (status IN ('pending', 'active', 'inactive', 'alumnus'));
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding status column to users.students: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS enrolled_from DATE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding enrolled_from column to users.students: %w", err)
	}

	_, err = db.NewRaw(`
		ALTER TABLE users.students
		ADD COLUMN IF NOT EXISTS enrolled_until DATE;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed adding enrolled_until column to users.students: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_tenant_status
			ON users.students(tenant_id, status);
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating idx_students_tenant_status: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_tenant_enrolled_from_pending
			ON users.students(tenant_id, enrolled_from)
			WHERE status = 'pending';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating idx_students_tenant_enrolled_from_pending: %w", err)
	}

	_, err = db.NewRaw(`
		CREATE INDEX IF NOT EXISTS idx_students_tenant_enrolled_until_active
			ON users.students(tenant_id, enrolled_until)
			WHERE status = 'active';
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed creating idx_students_tenant_enrolled_until_active: %w", err)
	}

	return nil
}

func studentLifecycleDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.56: Removing lifecycle columns from users.students...")

	_, err := db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_tenant_enrolled_until_active;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping idx_students_tenant_enrolled_until_active: %w", err)
	}

	_, err = db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_tenant_enrolled_from_pending;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping idx_students_tenant_enrolled_from_pending: %w", err)
	}

	_, err = db.NewRaw(`DROP INDEX IF EXISTS users.idx_students_tenant_status;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping idx_students_tenant_status: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS enrolled_until;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping enrolled_until column: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS enrolled_from;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping enrolled_from column: %w", err)
	}

	_, err = db.NewRaw(`ALTER TABLE users.students DROP COLUMN IF EXISTS status;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed dropping status column: %w", err)
	}

	return nil
}
