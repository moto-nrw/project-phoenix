package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	repairPickupScheduleTenantIDsVersion     = "1.15.39"
	repairPickupScheduleTenantIDsDescription = "Repair tenant IDs on existing pickup schedule data"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairPickupScheduleTenantIDsVersion,
		Description: repairPickupScheduleTenantIDsDescription,
		DependsOn:   []string{"1.15.38"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return repairPickupScheduleTenantIDs(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return nil
		},
	)
}

type pickupTenantRepairSpec struct {
	table          string
	tableAlias     string
	uniqueConflict string
}

var pickupTenantRepairSpecs = []pickupTenantRepairSpec{
	{
		table:      "schedule.student_pickup_schedules",
		tableAlias: "pickup_schedule",
		uniqueConflict: `
			SELECT COUNT(*)
			FROM schedule.student_pickup_schedules p
			JOIN users.students s ON s.id = p.student_id
			JOIN schedule.student_pickup_schedules existing
				ON existing.tenant_id = s.tenant_id
				AND existing.student_id = p.student_id
				AND existing.weekday = p.weekday
				AND existing.id <> p.id
			WHERE p.tenant_id <> s.tenant_id`,
	},
	{
		table:      "schedule.student_pickup_exceptions",
		tableAlias: "pickup_exception",
		uniqueConflict: `
			SELECT COUNT(*)
			FROM schedule.student_pickup_exceptions p
			JOIN users.students s ON s.id = p.student_id
			JOIN schedule.student_pickup_exceptions existing
				ON existing.tenant_id = s.tenant_id
				AND existing.student_id = p.student_id
				AND existing.exception_date = p.exception_date
				AND existing.id <> p.id
			WHERE p.tenant_id <> s.tenant_id`,
	},
	{
		table:          "schedule.student_pickup_notes",
		tableAlias:     "pickup_note",
		uniqueConflict: "",
	},
}

var pickupCompositeFKStatements = []struct {
	name string
	add  string
}{
	{
		name: "fk_pickup_schedules_created_by_tenant",
		add:  `ALTER TABLE schedule.student_pickup_schedules ADD CONSTRAINT fk_pickup_schedules_created_by_tenant FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff(tenant_id, id)`,
	},
	{
		name: "fk_pickup_schedules_student_tenant",
		add:  `ALTER TABLE schedule.student_pickup_schedules ADD CONSTRAINT fk_pickup_schedules_student_tenant FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE`,
	},
	{
		name: "fk_pickup_exceptions_created_by_tenant",
		add:  `ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT fk_pickup_exceptions_created_by_tenant FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff(tenant_id, id)`,
	},
	{
		name: "fk_pickup_exceptions_student_tenant",
		add:  `ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT fk_pickup_exceptions_student_tenant FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE`,
	},
	{
		name: "fk_pickup_notes_created_by_tenant",
		add:  `ALTER TABLE schedule.student_pickup_notes ADD CONSTRAINT fk_pickup_notes_created_by_tenant FOREIGN KEY (tenant_id, created_by) REFERENCES users.staff(tenant_id, id)`,
	},
	{
		name: "fk_pickup_notes_student_tenant",
		add:  `ALTER TABLE schedule.student_pickup_notes ADD CONSTRAINT fk_pickup_notes_student_tenant FOREIGN KEY (tenant_id, student_id) REFERENCES users.students(tenant_id, id) ON DELETE CASCADE`,
	},
}

func repairPickupScheduleTenantIDs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.39: Repairing pickup schedule tenant IDs...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	if err := dropPickupCompositeFKsForRepair(ctx, tx); err != nil {
		return err
	}

	for _, spec := range pickupTenantRepairSpecs {
		if err := ensurePickupTenantRepairIsSafe(ctx, tx, spec); err != nil {
			return err
		}

		result, err := tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s p
			SET tenant_id = s.tenant_id
			FROM users.students s
			WHERE p.student_id = s.id
				AND p.tenant_id <> s.tenant_id;
		`, spec.table))
		if err != nil {
			return fmt.Errorf("repair tenant_id on %s: %w", spec.table, err)
		}

		rowsAffected, _ := result.RowsAffected()
		fmt.Printf("Migration 1.15.39: repaired %d row(s) in %s\n", rowsAffected, spec.table)
	}

	if err := restorePickupCompositeFKsAfterRepair(ctx, tx); err != nil {
		return err
	}

	fmt.Println("Migration 1.15.39: Pickup schedule tenant ID repair complete")
	return tx.Commit()
}

func ensurePickupTenantRepairIsSafe(ctx context.Context, tx bun.Tx, spec pickupTenantRepairSpec) error {
	var missingCreatedBy int64
	err := tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s p
		JOIN users.students s ON s.id = p.student_id
		LEFT JOIN users.staff st ON st.id = p.created_by
		WHERE p.tenant_id <> s.tenant_id
			AND st.id IS NULL;
	`, spec.table)).Scan(&missingCreatedBy)
	if err != nil {
		return fmt.Errorf("check created_by existence on %s: %w", spec.table, err)
	}
	if missingCreatedBy > 0 {
		return fmt.Errorf(
			"cannot repair %s: %d row(s) reference missing created_by staff",
			spec.table,
			missingCreatedBy,
		)
	}

	var badCreatedBy int64
	err = tx.QueryRowContext(ctx, fmt.Sprintf(`
		SELECT COUNT(*)
		FROM %s p
		JOIN users.students s ON s.id = p.student_id
		JOIN users.staff st ON st.id = p.created_by
		WHERE p.tenant_id <> s.tenant_id
			AND st.tenant_id <> s.tenant_id;
	`, spec.table)).Scan(&badCreatedBy)
	if err != nil {
		return fmt.Errorf("check created_by tenant safety on %s: %w", spec.table, err)
	}
	if badCreatedBy > 0 {
		return fmt.Errorf(
			"cannot repair %s: %d row(s) reference created_by staff from a different tenant than the owning student",
			spec.table,
			badCreatedBy,
		)
	}

	if spec.uniqueConflict == "" {
		return nil
	}

	var conflicts int64
	if err := tx.QueryRowContext(ctx, spec.uniqueConflict).Scan(&conflicts); err != nil {
		return fmt.Errorf("check unique conflicts on %s: %w", spec.table, err)
	}
	if conflicts > 0 {
		return fmt.Errorf(
			"cannot repair %s: %d row(s) would conflict with existing %s rows after tenant_id repair",
			spec.table,
			conflicts,
			spec.tableAlias,
		)
	}

	return nil
}

func dropPickupCompositeFKsForRepair(ctx context.Context, tx bun.Tx) error {
	for _, fk := range pickupCompositeFKStatements {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s", pickupFKTable(fk.name), fk.name)); err != nil {
			return fmt.Errorf("drop pickup composite FK %s: %w", fk.name, err)
		}
	}
	return nil
}

func restorePickupCompositeFKsAfterRepair(ctx context.Context, tx bun.Tx) error {
	for _, fk := range pickupCompositeFKStatements {
		if _, err := tx.ExecContext(ctx, fk.add); err != nil {
			return fmt.Errorf("restore pickup composite FK %s: %w", fk.name, err)
		}
	}
	return nil
}

func pickupFKTable(fkName string) string {
	switch fkName {
	case "fk_pickup_schedules_created_by_tenant", "fk_pickup_schedules_student_tenant":
		return "schedule.student_pickup_schedules"
	case "fk_pickup_exceptions_created_by_tenant", "fk_pickup_exceptions_student_tenant":
		return "schedule.student_pickup_exceptions"
	case "fk_pickup_notes_created_by_tenant", "fk_pickup_notes_student_tenant":
		return "schedule.student_pickup_notes"
	default:
		panic(fmt.Sprintf("unknown pickup FK %s", fkName))
	}
}
