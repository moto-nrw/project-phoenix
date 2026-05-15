package migrations

import (
	"context"
	"fmt"

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

func repairPickupScheduleTenantIDs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.39: Repairing pickup schedule tenant IDs...")

	for _, spec := range pickupTenantRepairSpecs {
		if err := ensurePickupTenantRepairIsSafe(ctx, db, spec); err != nil {
			return err
		}

		result, err := db.ExecContext(ctx, fmt.Sprintf(`
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

	fmt.Println("Migration 1.15.39: Pickup schedule tenant ID repair complete")
	return nil
}

func ensurePickupTenantRepairIsSafe(ctx context.Context, db bun.IDB, spec pickupTenantRepairSpec) error {
	var missingCreatedBy int64
	err := db.QueryRowContext(ctx, fmt.Sprintf(`
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
	err = db.QueryRowContext(ctx, fmt.Sprintf(`
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
	if err := db.QueryRowContext(ctx, spec.uniqueConflict).Scan(&conflicts); err != nil {
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
