package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

const (
	activityInstancesCompositeFKsVersion     = "1.15.51"
	activityInstancesCompositeFKsDescription = "Upgrade timetable activity instance foreign keys to tenant-composite references"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     activityInstancesCompositeFKsVersion,
		Description: activityInstancesCompositeFKsDescription,
		DependsOn: []string{
			createActivityInstancesVersion,
			compositePKIndexesVersion,
		},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return activityInstancesCompositeFKsUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return activityInstancesCompositeFKsDown(ctx, db)
		},
	)
}

type activityInstanceCompositeFK struct {
	table        string
	column       string
	targetTable  string
	name         string
	onDelete     string
	setNullCols  string
	restoreName  string
	restoreTable string
}

var activityInstanceCompositeFKs = []activityInstanceCompositeFK{
	{
		table:        "schedule.activity_instances",
		column:       "activity_group_id",
		targetTable:  "activities.groups",
		name:         "fk_activity_instances_activity_group_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(activity_group_id)",
		restoreName:  "fk_activity_instances_activity_group_single",
		restoreTable: "activities.groups",
	},
	{
		table:        "schedule.activity_instances",
		column:       "room_id",
		targetTable:  "facilities.rooms",
		name:         "fk_activity_instances_room_tenant",
		onDelete:     "RESTRICT",
		restoreName:  "fk_activity_instances_room_single",
		restoreTable: "facilities.rooms",
	},
	{
		table:        "schedule.activity_instances",
		column:       "active_group_id",
		targetTable:  "active.groups",
		name:         "fk_activity_instances_active_group_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(active_group_id)",
		restoreName:  "fk_activity_instances_active_group_single",
		restoreTable: "active.groups",
	},
	{
		table:        "schedule.activity_instances",
		column:       "created_by",
		targetTable:  "users.staff",
		name:         "fk_activity_instances_created_by_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(created_by)",
		restoreName:  "fk_activity_instances_created_by_single",
		restoreTable: "users.staff",
	},
	{
		table:        "schedule.activity_instances",
		column:       "started_by",
		targetTable:  "users.staff",
		name:         "fk_activity_instances_started_by_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(started_by)",
		restoreName:  "fk_activity_instances_started_by_single",
		restoreTable: "users.staff",
	},
	{
		table:        "schedule.instance_staff",
		column:       "instance_id",
		targetTable:  "schedule.activity_instances",
		name:         "fk_instance_staff_instance_tenant",
		onDelete:     "CASCADE",
		restoreName:  "fk_instance_staff_instance_single",
		restoreTable: "schedule.activity_instances",
	},
	{
		table:        "schedule.instance_staff",
		column:       "staff_id",
		targetTable:  "users.staff",
		name:         "fk_instance_staff_staff_tenant",
		onDelete:     "RESTRICT",
		restoreName:  "fk_instance_staff_staff_single",
		restoreTable: "users.staff",
	},
	{
		table:        "schedule.instance_staff",
		column:       "room_id",
		targetTable:  "facilities.rooms",
		name:         "fk_instance_staff_room_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(room_id)",
		restoreName:  "fk_instance_staff_room_single",
		restoreTable: "facilities.rooms",
	},
	{
		table:        "schedule.instance_students",
		column:       "instance_id",
		targetTable:  "schedule.activity_instances",
		name:         "fk_instance_students_instance_tenant",
		onDelete:     "CASCADE",
		restoreName:  "fk_instance_students_instance_single",
		restoreTable: "schedule.activity_instances",
	},
	{
		table:        "schedule.instance_students",
		column:       "student_id",
		targetTable:  "users.students",
		name:         "fk_instance_students_student_tenant",
		onDelete:     "RESTRICT",
		restoreName:  "fk_instance_students_student_single",
		restoreTable: "users.students",
	},
	{
		table:        "schedule.instance_students",
		column:       "room_id",
		targetTable:  "facilities.rooms",
		name:         "fk_instance_students_room_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(room_id)",
		restoreName:  "fk_instance_students_room_single",
		restoreTable: "facilities.rooms",
	},
	{
		table:        "schedule.activity_exceptions",
		column:       "activity_group_id",
		targetTable:  "activities.groups",
		name:         "fk_activity_exceptions_activity_group_tenant",
		onDelete:     "CASCADE",
		restoreName:  "fk_activity_exceptions_activity_group_single",
		restoreTable: "activities.groups",
	},
	{
		table:        "schedule.activity_exceptions",
		column:       "room_id",
		targetTable:  "facilities.rooms",
		name:         "fk_activity_exceptions_room_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(room_id)",
		restoreName:  "fk_activity_exceptions_room_single",
		restoreTable: "facilities.rooms",
	},
	{
		table:        "schedule.activity_exceptions",
		column:       "created_by",
		targetTable:  "users.staff",
		name:         "fk_activity_exceptions_created_by_tenant",
		onDelete:     "SET NULL",
		setNullCols:  "(created_by)",
		restoreName:  "fk_activity_exceptions_created_by_single",
		restoreTable: "users.staff",
	},
}

func activityInstancesCompositeFKsUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.51: Upgrading timetable instance FKs to tenant-composite constraints...")

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
		ADD CONSTRAINT unique_activity_instance_tenant_id UNIQUE (tenant_id, id);
	`).Exec(ctx); err != nil && !constraintAlreadyExists(err) {
		return fmt.Errorf("add unique_activity_instance_tenant_id: %w", err)
	}

	for _, fk := range activityInstanceCompositeFKs {
		if err := dropSingleColumnFK(ctx, db, fk.table, fk.column, fk.targetTable); err != nil {
			return fmt.Errorf("drop old FK on %s.%s: %w", fk.table, fk.column, err)
		}
		if err := addCompositeFK(ctx, db, fk); err != nil {
			return fmt.Errorf("add composite FK %s: %w", fk.name, err)
		}
	}

	return nil
}

func activityInstancesCompositeFKsDown(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.51: Restoring timetable instance single-column FKs...")

	for i := len(activityInstanceCompositeFKs) - 1; i >= 0; i-- {
		fk := activityInstanceCompositeFKs[i]
		if _, err := db.NewRaw(fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`, fk.table, fk.name)).Exec(ctx); err != nil {
			return fmt.Errorf("drop composite FK %s: %w", fk.name, err)
		}
	}

	if _, err := db.NewRaw(`
		ALTER TABLE schedule.activity_instances
		DROP CONSTRAINT IF EXISTS unique_activity_instance_tenant_id;
	`).Exec(ctx); err != nil {
		return fmt.Errorf("drop unique_activity_instance_tenant_id: %w", err)
	}

	for _, fk := range activityInstanceCompositeFKs {
		if _, err := db.NewRaw(fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (%s) REFERENCES %s(id) ON DELETE %s`,
			fk.table,
			fk.restoreName,
			fk.column,
			fk.restoreTable,
			fk.onDelete,
		)).Exec(ctx); err != nil && !constraintAlreadyExists(err) {
			return fmt.Errorf("restore single-column FK %s: %w", fk.restoreName, err)
		}
	}

	return nil
}

func dropSingleColumnFK(ctx context.Context, db *bun.DB, table, column, targetTable string) error {
	_, err := db.NewRaw(fmt.Sprintf(`
		DO $$
		DECLARE
			fk record;
		BEGIN
			FOR fk IN
				SELECT con.conname
				FROM pg_constraint con
				JOIN pg_attribute att
					ON att.attrelid = con.conrelid
					AND att.attnum = con.conkey[1]
				WHERE con.conrelid = '%s'::regclass
					AND con.confrelid = '%s'::regclass
					AND con.contype = 'f'
					AND array_length(con.conkey, 1) = 1
					AND att.attname = '%s'
			LOOP
				EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %%I', fk.conname);
			END LOOP;
		END $$;
	`, table, targetTable, column, table)).Exec(ctx)
	return err
}

func addCompositeFK(ctx context.Context, db *bun.DB, fk activityInstanceCompositeFK) error {
	onDelete := fk.onDelete
	if fk.setNullCols != "" {
		onDelete = fmt.Sprintf("%s %s", onDelete, fk.setNullCols)
	}
	_, err := db.NewRaw(fmt.Sprintf(
		`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (tenant_id, %s) REFERENCES %s(tenant_id, id) ON DELETE %s`,
		fk.table,
		fk.name,
		fk.column,
		fk.targetTable,
		onDelete,
	)).Exec(ctx)
	if err != nil && constraintAlreadyExists(err) {
		return nil
	}
	return err
}

func constraintAlreadyExists(err error) bool {
	var pgErr pgdriver.Error
	return errors.As(err, &pgErr) && pgErr.Field('C') == "42710"
}
