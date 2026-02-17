package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	compositeFKsVersion     = "1.15.2"
	compositeFKsDescription = "Convert single-column foreign keys to composite (tenant_id, column) form for cross-tenant FK safety"
)

func init() {
	MigrationRegistry[compositeFKsVersion] = &Migration{
		Version:     compositeFKsVersion,
		Description: compositeFKsDescription,
		DependsOn:   []string{"1.15.1", "1.14.4"}, // RLS enabled + UNIQUE(tenant_id, id) indexes exist
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createCompositeFKs(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackCompositeFKs(ctx, db)
		},
	)
}

// compositeFKSpec defines a foreign key to convert from single-column to composite form.
type compositeFKSpec struct {
	// Source table (schema.table) and column that holds the FK reference
	sourceTable  string
	sourceColumn string
	// Target table (schema.table) — id is always the target column
	targetTable string
	// New composite FK constraint name
	newFKName string
	// ON DELETE action (CASCADE, SET NULL, RESTRICT, or empty for default)
	onDelete string
}

// compositeFKSpecs lists all FKs that need composite conversion.
// Each references one of the 18 target tables with UNIQUE(tenant_id, id).
// Excluded: FKs to auth.accounts, platform.schools, auth.permissions, auth.role_permissions (non-tenant).
var compositeFKSpecs = []compositeFKSpec{
	// ─── FKs TO users.persons ───
	{sourceTable: "users.staff", sourceColumn: "person_id", targetTable: "users.persons",
		newFKName: "fk_staff_person_tenant", onDelete: "CASCADE"},
	{sourceTable: "iot.devices", sourceColumn: "registered_by_id", targetTable: "users.persons",
		newFKName: "fk_devices_registered_by_tenant", onDelete: "SET NULL"},

	// ─── FKs TO users.staff ───
	{sourceTable: "users.teachers", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_teachers_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "users.guests", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_guests_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "education.group_substitution", sourceColumn: "regular_staff_id", targetTable: "users.staff",
		newFKName: "fk_group_sub_regular_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "education.group_substitution", sourceColumn: "substitute_staff_id", targetTable: "users.staff",
		newFKName: "fk_group_sub_substitute_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "activities.supervisors", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_activity_supervisors_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "activities.groups", sourceColumn: "created_by", targetTable: "users.staff",
		newFKName: "fk_activity_groups_created_by_tenant", onDelete: ""},
	{sourceTable: "active.group_supervisors", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_supervision_staff_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.attendance", sourceColumn: "checked_in_by", targetTable: "users.staff",
		newFKName: "fk_attendance_checked_in_by_tenant", onDelete: "RESTRICT"},
	{sourceTable: "active.attendance", sourceColumn: "checked_out_by", targetTable: "users.staff",
		newFKName: "fk_attendance_checked_out_by_tenant", onDelete: "RESTRICT"},
	{sourceTable: "active.work_sessions", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_work_sessions_staff_tenant", onDelete: ""},
	{sourceTable: "active.work_sessions", sourceColumn: "created_by", targetTable: "users.staff",
		newFKName: "fk_work_sessions_created_by_tenant", onDelete: ""},
	{sourceTable: "active.work_sessions", sourceColumn: "updated_by", targetTable: "users.staff",
		newFKName: "fk_work_sessions_updated_by_tenant", onDelete: ""},
	{sourceTable: "active.staff_absences", sourceColumn: "staff_id", targetTable: "users.staff",
		newFKName: "fk_staff_absences_staff_tenant", onDelete: ""},
	{sourceTable: "active.staff_absences", sourceColumn: "approved_by", targetTable: "users.staff",
		newFKName: "fk_staff_absences_approved_by_tenant", onDelete: ""},
	{sourceTable: "active.staff_absences", sourceColumn: "created_by", targetTable: "users.staff",
		newFKName: "fk_staff_absences_created_by_tenant", onDelete: ""},
	{sourceTable: "schedule.student_pickup_schedules", sourceColumn: "created_by", targetTable: "users.staff",
		newFKName: "fk_pickup_schedules_created_by_tenant", onDelete: ""},
	{sourceTable: "schedule.student_pickup_exceptions", sourceColumn: "created_by", targetTable: "users.staff",
		newFKName: "fk_pickup_exceptions_created_by_tenant", onDelete: ""},

	// ─── FKs TO users.students ───
	{sourceTable: "users.students_guardians", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_students_guardians_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "users.privacy_consents", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_privacy_consents_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.visits", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_visits_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.attendance", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_attendance_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.scheduled_checkouts", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_scheduled_checkouts_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "activities.student_enrollments", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_enrollments_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "schedule.student_pickup_schedules", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_pickup_schedules_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "schedule.student_pickup_exceptions", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_pickup_exceptions_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "schedule.student_pickup_notes", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_pickup_notes_student_tenant", onDelete: "CASCADE"},
	{sourceTable: "feedback.entries", sourceColumn: "student_id", targetTable: "users.students",
		newFKName: "fk_feedback_student_tenant", onDelete: "CASCADE"},

	// ─── FKs TO users.teachers ───
	{sourceTable: "education.group_teacher", sourceColumn: "teacher_id", targetTable: "users.teachers",
		newFKName: "fk_group_teacher_teacher_tenant", onDelete: "CASCADE"},

	// ─── FKs TO users.guardian_profiles ───
	{sourceTable: "users.students_guardians", sourceColumn: "guardian_profile_id", targetTable: "users.guardian_profiles",
		newFKName: "fk_students_guardians_guardian_tenant", onDelete: "CASCADE"},
	{sourceTable: "users.guardian_phone_numbers", sourceColumn: "guardian_profile_id", targetTable: "users.guardian_profiles",
		newFKName: "fk_guardian_phones_guardian_tenant", onDelete: "CASCADE"},

	// ─── FKs TO users.rfid_cards ───
	// Note: users.persons.tag_id is TEXT referencing rfid_cards(id) which is also TEXT.
	// The composite FK references (tenant_id, id) on rfid_cards.
	{sourceTable: "users.persons", sourceColumn: "tag_id", targetTable: "users.rfid_cards",
		newFKName: "fk_persons_rfid_card_tenant", onDelete: "SET NULL"},

	// ─── FKs TO education.groups ───
	{sourceTable: "users.students", sourceColumn: "group_id", targetTable: "education.groups",
		newFKName: "fk_students_group_tenant", onDelete: "SET NULL"},
	{sourceTable: "education.group_teacher", sourceColumn: "group_id", targetTable: "education.groups",
		newFKName: "fk_group_teacher_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "education.group_substitution", sourceColumn: "group_id", targetTable: "education.groups",
		newFKName: "fk_group_sub_group_tenant", onDelete: "CASCADE"},

	// ─── FKs TO education.grade_transitions ───
	{sourceTable: "education.grade_transition_mappings", sourceColumn: "transition_id", targetTable: "education.grade_transitions",
		newFKName: "fk_grade_mappings_transition_tenant", onDelete: "CASCADE"},
	{sourceTable: "education.grade_transition_history", sourceColumn: "transition_id", targetTable: "education.grade_transitions",
		newFKName: "fk_grade_history_transition_tenant", onDelete: "CASCADE"},

	// ─── FKs TO facilities.rooms ───
	{sourceTable: "education.groups", sourceColumn: "room_id", targetTable: "facilities.rooms",
		newFKName: "fk_groups_room_tenant", onDelete: "SET NULL"},
	{sourceTable: "activities.groups", sourceColumn: "planned_room_id", targetTable: "facilities.rooms",
		newFKName: "fk_activity_groups_room_tenant", onDelete: "SET NULL"},
	{sourceTable: "active.groups", sourceColumn: "room_id", targetTable: "facilities.rooms",
		newFKName: "fk_active_groups_room_tenant", onDelete: "RESTRICT"},

	// ─── FKs TO activities.categories ───
	{sourceTable: "activities.groups", sourceColumn: "category_id", targetTable: "activities.categories",
		newFKName: "fk_activity_groups_category_tenant", onDelete: "RESTRICT"},

	// ─── FKs TO activities.groups ───
	{sourceTable: "activities.supervisors", sourceColumn: "group_id", targetTable: "activities.groups",
		newFKName: "fk_activity_supervisors_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "activities.student_enrollments", sourceColumn: "activity_group_id", targetTable: "activities.groups",
		newFKName: "fk_enrollments_activity_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "activities.schedules", sourceColumn: "activity_group_id", targetTable: "activities.groups",
		newFKName: "fk_schedules_activity_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.groups", sourceColumn: "group_id", targetTable: "activities.groups",
		newFKName: "fk_active_groups_group_tenant", onDelete: "CASCADE"},

	// ─── FKs TO active.groups ───
	{sourceTable: "active.visits", sourceColumn: "active_group_id", targetTable: "active.groups",
		newFKName: "fk_visits_active_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.group_supervisors", sourceColumn: "group_id", targetTable: "active.groups",
		newFKName: "fk_supervision_group_tenant", onDelete: "CASCADE"},
	{sourceTable: "active.group_mappings", sourceColumn: "active_group_id", targetTable: "active.groups",
		newFKName: "fk_group_mappings_active_group_tenant", onDelete: "CASCADE"},

	// ─── FKs TO active.combined_groups ───
	{sourceTable: "active.group_mappings", sourceColumn: "active_combined_group_id", targetTable: "active.combined_groups",
		newFKName: "fk_group_mappings_combined_group_tenant", onDelete: "CASCADE"},

	// ─── FKs TO active.work_sessions ───
	{sourceTable: "active.work_session_breaks", sourceColumn: "work_session_id", targetTable: "active.work_sessions",
		newFKName: "fk_work_breaks_session_tenant", onDelete: "CASCADE"},
	{sourceTable: "audit.work_session_edits", sourceColumn: "work_session_id", targetTable: "active.work_sessions",
		newFKName: "fk_session_edits_session_tenant", onDelete: "CASCADE"},

	// ─── FKs TO iot.devices ───
	{sourceTable: "active.groups", sourceColumn: "device_id", targetTable: "iot.devices",
		newFKName: "fk_active_groups_device_tenant", onDelete: "RESTRICT"},
	{sourceTable: "active.attendance", sourceColumn: "device_id", targetTable: "iot.devices",
		newFKName: "fk_attendance_device_tenant", onDelete: "RESTRICT"},

	// ─── FKs TO schedule.timeframes ───
	{sourceTable: "activities.schedules", sourceColumn: "timeframe_id", targetTable: "schedule.timeframes",
		newFKName: "fk_schedules_timeframe_tenant", onDelete: "SET NULL"},

	// ─── FKs TO suggestions.posts ───
	{sourceTable: "suggestions.votes", sourceColumn: "post_id", targetTable: "suggestions.posts",
		newFKName: "fk_votes_post_tenant", onDelete: "CASCADE"},
	{sourceTable: "suggestions.comments", sourceColumn: "post_id", targetTable: "suggestions.posts",
		newFKName: "fk_comments_post_tenant", onDelete: "CASCADE"},
	{sourceTable: "suggestions.comment_reads", sourceColumn: "post_id", targetTable: "suggestions.posts",
		newFKName: "fk_comment_reads_post_tenant", onDelete: "CASCADE"},
	{sourceTable: "suggestions.post_reads", sourceColumn: "post_id", targetTable: "suggestions.posts",
		newFKName: "fk_post_reads_post_tenant", onDelete: "CASCADE"},

	// ─── FKs TO auth.accounts_parents ───
	{sourceTable: "users.guardian_profiles", sourceColumn: "account_id", targetTable: "auth.accounts_parents",
		newFKName: "fk_guardian_profiles_account_tenant", onDelete: "SET NULL"},
	{sourceTable: "users.persons_guardians", sourceColumn: "guardian_account_id", targetTable: "auth.accounts_parents",
		newFKName: "fk_persons_guardians_guardian_tenant", onDelete: "CASCADE"},
}

func createCompositeFKs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.2: Converting foreign keys to composite (tenant_id, column) form...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for i, spec := range compositeFKSpecs {
		// Step 1: Find and drop the existing single-column FK constraint.
		// We look it up dynamically from pg_constraint since some FKs are unnamed (auto-generated).
		dropSQL := fmt.Sprintf(`
			DO $$
			DECLARE
				fk_name TEXT;
			BEGIN
				SELECT con.conname INTO fk_name
				FROM pg_constraint con
				JOIN pg_attribute att ON att.attrelid = con.conrelid
					AND att.attnum = ANY(con.conkey)
				WHERE con.conrelid = '%s'::regclass
					AND con.confrelid = '%s'::regclass
					AND con.contype = 'f'
					AND att.attname = '%s'
					AND array_length(con.conkey, 1) = 1;

				IF fk_name IS NOT NULL THEN
					EXECUTE format('ALTER TABLE %s DROP CONSTRAINT %%I', fk_name);
				END IF;
			END $$;`,
			spec.sourceTable, spec.targetTable, spec.sourceColumn, spec.sourceTable)

		_, err := tx.ExecContext(ctx, dropSQL)
		if err != nil {
			return fmt.Errorf("failed to drop old FK on %s(%s) → %s: %w",
				spec.sourceTable, spec.sourceColumn, spec.targetTable, err)
		}

		// Step 2: Create new composite FK (tenant_id, column) → (tenant_id, id)
		onDeleteClause := ""
		if spec.onDelete != "" {
			onDeleteClause = " ON DELETE " + spec.onDelete
		}

		addSQL := fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s FOREIGN KEY (tenant_id, %s) REFERENCES %s(tenant_id, id)%s`,
			spec.sourceTable, spec.newFKName, spec.sourceColumn, spec.targetTable, onDeleteClause)

		_, err = tx.ExecContext(ctx, addSQL)
		if err != nil {
			return fmt.Errorf("failed to create composite FK %s on %s(%s) → %s: %w",
				spec.newFKName, spec.sourceTable, spec.sourceColumn, spec.targetTable, err)
		}

		if (i+1)%10 == 0 {
			fmt.Printf("  ✓ %d/%d composite FKs created\n", i+1, len(compositeFKSpecs))
		}
	}

	fmt.Printf("Migration 1.15.2: Successfully converted %d foreign keys to composite form\n", len(compositeFKSpecs))
	return tx.Commit()
}

func rollbackCompositeFKs(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.2: Restoring single-column foreign keys...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil &&
			err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	for _, spec := range compositeFKSpecs {
		// Drop the composite FK
		_, _ = tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s`,
			spec.sourceTable, spec.newFKName))

		// Restore the original single-column FK
		onDeleteClause := ""
		if spec.onDelete != "" {
			onDeleteClause = " ON DELETE " + spec.onDelete
		}

		_, err := tx.ExecContext(ctx, fmt.Sprintf(
			`ALTER TABLE %s ADD CONSTRAINT %s_single FOREIGN KEY (%s) REFERENCES %s(id)%s`,
			spec.sourceTable, spec.newFKName, spec.sourceColumn, spec.targetTable, onDeleteClause))
		if err != nil {
			log.Printf("Warning: failed to restore single-column FK on %s(%s): %v",
				spec.sourceTable, spec.sourceColumn, err)
		}
	}

	fmt.Println("Migration 1.15.2: Successfully rolled back composite FKs")
	return tx.Commit()
}
