package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	vacationWorkflowVersion     = "1.15.105"
	vacationWorkflowDescription = "Add vacation request workflow fields + staff_vacation_quota table"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     vacationWorkflowVersion,
		Description: vacationWorkflowDescription,
		DependsOn:   []string{"1.10.7"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addVacationWorkflow(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return removeVacationWorkflow(ctx, db)
		},
	)
}

func addVacationWorkflow(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.105: Adding vacation workflow fields + quota table...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Extend staff_absences with workflow fields. The existing CHECK constraint
	// on `status` (reported|approved|declined) gets relaxed to include the
	// vacation-request flow statuses (requested|canceled). 'reported' stays
	// for admin-direct entries that bypass approval (sick days, training).
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD COLUMN IF NOT EXISTS working_days       NUMERIC(4,1),
			ADD COLUMN IF NOT EXISTS decision_note      TEXT,
			ADD COLUMN IF NOT EXISTS requested_at       TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			ADD COLUMN IF NOT EXISTS substitute_staff_id BIGINT REFERENCES users.staff(id) ON DELETE SET NULL;
	`)
	if err != nil {
		return fmt.Errorf("error extending staff_absences: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS staff_absences_status_check;
	`)
	if err != nil {
		return fmt.Errorf("error dropping old status check: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT staff_absences_status_check
			CHECK (status IN ('reported','requested','approved','declined','canceled'));
	`)
	if err != nil {
		return fmt.Errorf("error adding new status check: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_staff_absences_status_staff
			ON active.staff_absences(status, staff_id)
			WHERE status IN ('requested','approved');
	`)
	if err != nil {
		return fmt.Errorf("error creating status index: %w", err)
	}

	// Per-staff annual quota. Resturlaub = entitled + carryover − approved − reserved.
	// 'reserved' is computed live from rows with status='requested'.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.staff_vacation_quota (
			id              BIGSERIAL PRIMARY KEY,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			staff_id        BIGINT NOT NULL REFERENCES users.staff(id) ON DELETE CASCADE,
			year            INT NOT NULL,
			entitled_days   NUMERIC(4,1) NOT NULL DEFAULT 30,
			carryover_days  NUMERIC(4,1) NOT NULL DEFAULT 0,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (staff_id, year)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_vacation_quota: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_vacation_quota ENABLE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_staff_vacation_quota ON active.staff_vacation_quota
			USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
		ALTER TABLE active.staff_vacation_quota FORCE ROW LEVEL SECURITY;
	`)
	if err != nil {
		return fmt.Errorf("error enabling RLS on staff_vacation_quota: %w", err)
	}

	// Audit trail for status transitions. Append-only, no updates.
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS active.staff_absence_audit (
			id           BIGSERIAL PRIMARY KEY,
			tenant_id    BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			absence_id   BIGINT NOT NULL REFERENCES active.staff_absences(id) ON DELETE CASCADE,
			from_status  TEXT,
			to_status    TEXT NOT NULL,
			actor_id     BIGINT NOT NULL REFERENCES auth.accounts(id),
			note         TEXT,
			changed_at   TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_staff_absence_audit_absence
			ON active.staff_absence_audit(absence_id, changed_at DESC);
	`)
	if err != nil {
		return fmt.Errorf("error creating staff_absence_audit: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absence_audit ENABLE ROW LEVEL SECURITY;
		CREATE POLICY tenant_isolation_staff_absence_audit ON active.staff_absence_audit
			USING (tenant_id = current_setting('app.current_tenant_id')::BIGINT);
		ALTER TABLE active.staff_absence_audit FORCE ROW LEVEL SECURITY;
	`)
	if err != nil {
		return fmt.Errorf("error enabling RLS on staff_absence_audit: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.permissions (name, description, resource, action)
		VALUES ('vacation:approve', 'Approve or deny vacation requests', 'vacation', 'approve')
		ON CONFLICT (name) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error inserting vacation:approve permission: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO auth.role_permissions (role_id, permission_id)
		SELECT r.id, p.id
		FROM auth.roles r
		CROSS JOIN auth.permissions p
		WHERE p.name = 'vacation:approve' AND r.name = 'admin'
		ON CONFLICT (role_id, permission_id) DO NOTHING;
	`)
	if err != nil {
		return fmt.Errorf("error granting vacation:approve to admin: %w", err)
	}

	fmt.Println("Migration 1.15.105: vacation workflow ready")
	return tx.Commit()
}

func removeVacationWorkflow(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.105: Removing vacation workflow...")

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
		DELETE FROM auth.role_permissions
		WHERE permission_id IN (SELECT id FROM auth.permissions WHERE name = 'vacation:approve');
		DELETE FROM auth.permissions WHERE name = 'vacation:approve';
	`)
	if err != nil {
		return fmt.Errorf("error removing vacation:approve permission: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		DROP TABLE IF EXISTS active.staff_absence_audit CASCADE;
		DROP TABLE IF EXISTS active.staff_vacation_quota CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping audit/quota tables: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.staff_absences
			DROP CONSTRAINT IF EXISTS staff_absences_status_check;
		ALTER TABLE active.staff_absences
			ADD CONSTRAINT staff_absences_status_check
			CHECK (status IN ('reported','approved','declined'));
		DROP INDEX IF EXISTS active.idx_staff_absences_status_staff;
		ALTER TABLE active.staff_absences
			DROP COLUMN IF EXISTS working_days,
			DROP COLUMN IF EXISTS decision_note,
			DROP COLUMN IF EXISTS requested_at,
			DROP COLUMN IF EXISTS substitute_staff_id;
	`)
	if err != nil {
		return fmt.Errorf("error reverting staff_absences extensions: %w", err)
	}

	fmt.Println("Migration 1.15.105 rollback complete")
	return tx.Commit()
}
