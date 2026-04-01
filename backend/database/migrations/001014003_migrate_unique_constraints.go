package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	migrateUniqueConstraintsVersion     = "1.14.3"
	migrateUniqueConstraintsDescription = "Migrate UNIQUE constraints to include tenant_id for multi-tenancy"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     migrateUniqueConstraintsVersion,
		Description: migrateUniqueConstraintsDescription,
		DependsOn:   []string{"1.14.2"}, // tenant_id columns exist
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return migrateUniqueConstraints(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackUniqueConstraints(ctx, db)
		},
	)
}

func migrateUniqueConstraints(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.3: Migrating UNIQUE constraints to include tenant_id...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// ===================================================================
	// 13 FUNCTIONALLY NECESSARY CHANGES (§2.4.1)
	// These are required for correct multi-tenant behavior.
	// ===================================================================

	fmt.Println("  Migrating functionally necessary constraints...")

	// 1. facilities.rooms: UNIQUE(name) → UNIQUE(tenant_id, name)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE facilities.rooms DROP CONSTRAINT IF EXISTS rooms_name_key;
		CREATE UNIQUE INDEX idx_rooms_tenant_name ON facilities.rooms(tenant_id, name);
	`)
	if err != nil {
		return fmt.Errorf("error migrating facilities.rooms unique constraint: %w", err)
	}

	// 2. education.groups: UNIQUE(name) → UNIQUE(tenant_id, name)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE education.groups DROP CONSTRAINT IF EXISTS groups_name_key;
		CREATE UNIQUE INDEX idx_groups_tenant_name ON education.groups(tenant_id, name);
	`)
	if err != nil {
		return fmt.Errorf("error migrating education.groups unique constraint: %w", err)
	}

	// 3. activities.categories: UNIQUE(name) → UNIQUE(tenant_id, name)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE activities.categories DROP CONSTRAINT IF EXISTS categories_name_key;
		CREATE UNIQUE INDEX idx_categories_tenant_name ON activities.categories(tenant_id, name);
	`)
	if err != nil {
		return fmt.Errorf("error migrating activities.categories unique constraint: %w", err)
	}

	// 4. config.settings: UNIQUE(key) → UNIQUE(tenant_id, key)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE config.settings DROP CONSTRAINT IF EXISTS settings_key_key;
		CREATE UNIQUE INDEX idx_settings_tenant_key ON config.settings(tenant_id, key);
	`)
	if err != nil {
		return fmt.Errorf("error migrating config.settings unique constraint: %w", err)
	}

	// 5. users.persons: UNIQUE(account_id) → UNIQUE(tenant_id, account_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons DROP CONSTRAINT IF EXISTS persons_account_id_key;
		CREATE UNIQUE INDEX idx_persons_tenant_account ON users.persons(tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.persons account_id constraint: %w", err)
	}

	// 6. users.persons: UNIQUE(tag_id) → UNIQUE(tenant_id, tag_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons DROP CONSTRAINT IF EXISTS persons_tag_id_key;
		CREATE UNIQUE INDEX idx_persons_tenant_tag ON users.persons(tenant_id, tag_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.persons tag_id constraint: %w", err)
	}

	// 7. users.profiles: UNIQUE(account_id) → UNIQUE(tenant_id, account_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.profiles DROP CONSTRAINT IF EXISTS profiles_account_id_key;
		CREATE UNIQUE INDEX idx_profiles_tenant_account ON users.profiles(tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.profiles unique constraint: %w", err)
	}

	// 8. users.guardian_profiles: UNIQUE(email) → UNIQUE(tenant_id, email)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles DROP CONSTRAINT IF EXISTS guardian_profiles_email_key;
		CREATE UNIQUE INDEX idx_guardian_profiles_tenant_email ON users.guardian_profiles(tenant_id, email);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.guardian_profiles email constraint: %w", err)
	}

	// 9. users.guardian_profiles: UNIQUE(account_id) → UNIQUE(tenant_id, account_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guardian_profiles DROP CONSTRAINT IF EXISTS guardian_profiles_account_id_key;
		CREATE UNIQUE INDEX idx_guardian_profiles_tenant_account ON users.guardian_profiles(tenant_id, account_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.guardian_profiles account_id constraint: %w", err)
	}

	// 10. auth.accounts_parents: UNIQUE INDEX on email → UNIQUE(tenant_id, email)
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_accounts_parents_email;
		CREATE UNIQUE INDEX idx_accounts_parents_tenant_email ON auth.accounts_parents(tenant_id, email);
	`)
	if err != nil {
		return fmt.Errorf("error migrating auth.accounts_parents email constraint: %w", err)
	}

	// 11. auth.accounts_parents: UNIQUE(username) → UNIQUE(tenant_id, username)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.accounts_parents DROP CONSTRAINT IF EXISTS accounts_parents_username_key;
		CREATE UNIQUE INDEX idx_accounts_parents_tenant_username ON auth.accounts_parents(tenant_id, username);
	`)
	if err != nil {
		return fmt.Errorf("error migrating auth.accounts_parents username constraint: %w", err)
	}

	// 12. auth.account_roles: UNIQUE(account_id, role_id) → UNIQUE(account_id, role_id, tenant_id)
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_account_roles_account_role;
		CREATE UNIQUE INDEX idx_account_roles_tenant ON auth.account_roles(account_id, role_id, tenant_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating auth.account_roles unique constraint: %w", err)
	}

	// 13. auth.account_permissions: UNIQUE(account_id, permission_id) → UNIQUE(account_id, permission_id, tenant_id)
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_account_permissions_account_permission;
		CREATE UNIQUE INDEX idx_account_permissions_tenant ON auth.account_permissions(account_id, permission_id, tenant_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating auth.account_permissions unique constraint: %w", err)
	}

	// ===================================================================
	// auth.roles SPECIAL CASE (nullable tenant_id — D13)
	// System roles (NULL tenant_id) must be unique globally.
	// Tenant-specific roles must be unique per tenant.
	// ===================================================================

	fmt.Println("  Migrating auth.roles special case (nullable tenant_id)...")

	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_roles_name;
		CREATE UNIQUE INDEX idx_roles_name_system ON auth.roles(name) WHERE tenant_id IS NULL;
		CREATE UNIQUE INDEX idx_roles_name_tenant ON auth.roles(tenant_id, name) WHERE tenant_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error migrating auth.roles unique constraints: %w", err)
	}

	// ===================================================================
	// 18 DEFENSE-IN-DEPTH CHANGES (§2.4.2)
	// These add tenant_id to uniqueness for cross-tenant safety.
	// ===================================================================

	fmt.Println("  Migrating defense-in-depth constraints...")

	// 1. users.staff: UNIQUE(person_id) → UNIQUE(tenant_id, person_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.staff DROP CONSTRAINT IF EXISTS staff_person_id_key;
		CREATE UNIQUE INDEX idx_staff_tenant_person ON users.staff(tenant_id, person_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.staff unique constraint: %w", err)
	}

	// 2. users.teachers: UNIQUE(staff_id) → UNIQUE(tenant_id, staff_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.teachers DROP CONSTRAINT IF EXISTS teachers_staff_id_key;
		CREATE UNIQUE INDEX idx_teachers_tenant_staff ON users.teachers(tenant_id, staff_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.teachers unique constraint: %w", err)
	}

	// 3. users.guests: UNIQUE(staff_id) → UNIQUE(tenant_id, staff_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.guests DROP CONSTRAINT IF EXISTS guests_staff_id_key;
		CREATE UNIQUE INDEX idx_guests_tenant_staff ON users.guests(tenant_id, staff_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.guests unique constraint: %w", err)
	}

	// 4. users.students: UNIQUE(person_id) → UNIQUE(tenant_id, person_id)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students DROP CONSTRAINT IF EXISTS students_person_id_key;
		CREATE UNIQUE INDEX idx_students_tenant_person ON users.students(tenant_id, person_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.students unique constraint: %w", err)
	}

	// 5. users.students_guardians: UNIQUE(student_id, guardian_profile_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.students_guardians DROP CONSTRAINT IF EXISTS unique_student_guardian;
		CREATE UNIQUE INDEX idx_students_guardians_tenant ON users.students_guardians(tenant_id, student_id, guardian_profile_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.students_guardians unique constraint: %w", err)
	}

	// 6. users.persons_guardians: UNIQUE(person_id, guardian_account_id, relationship_type) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE users.persons_guardians DROP CONSTRAINT IF EXISTS unique_person_guardian_relationship;
		CREATE UNIQUE INDEX idx_persons_guardians_tenant ON users.persons_guardians(tenant_id, person_id, guardian_account_id, relationship_type);
	`)
	if err != nil {
		return fmt.Errorf("error migrating users.persons_guardians unique constraint: %w", err)
	}

	// 7. education.group_teacher: UNIQUE(group_id, teacher_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE education.group_teacher DROP CONSTRAINT IF EXISTS uk_group_teacher;
		CREATE UNIQUE INDEX idx_group_teacher_tenant ON education.group_teacher(tenant_id, group_id, teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating education.group_teacher unique constraint: %w", err)
	}

	// 8. education.group_substitution: Partial UNIQUE(group_id, substitute_staff_id, start_date) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS education.idx_no_duplicate_group_transfers;
		CREATE UNIQUE INDEX idx_no_duplicate_group_transfers_tenant
			ON education.group_substitution(tenant_id, group_id, substitute_staff_id, start_date)
			WHERE regular_staff_id IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error migrating education.group_substitution unique constraint: %w", err)
	}

	// 9. education.grade_transition_mappings: UNIQUE(transition_id, from_class) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE education.grade_transition_mappings
			DROP CONSTRAINT IF EXISTS grade_transition_mappings_transition_id_from_class_key;
		CREATE UNIQUE INDEX idx_grade_transition_mappings_tenant
			ON education.grade_transition_mappings(tenant_id, transition_id, from_class);
	`)
	if err != nil {
		return fmt.Errorf("error migrating education.grade_transition_mappings unique constraint: %w", err)
	}

	// 10. activities.student_enrollments: UNIQUE(student_id, activity_group_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE activities.student_enrollments DROP CONSTRAINT IF EXISTS uk_student_activity_enrollment;
		CREATE UNIQUE INDEX idx_student_enrollments_tenant ON activities.student_enrollments(tenant_id, student_id, activity_group_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating activities.student_enrollments unique constraint: %w", err)
	}

	// 11. activities.supervisors: UNIQUE(staff_id, group_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE activities.supervisors DROP CONSTRAINT IF EXISTS uq_activity_supervisors_staff_group;
		CREATE UNIQUE INDEX idx_supervisors_planned_tenant ON activities.supervisors(tenant_id, staff_id, group_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating activities.supervisors unique constraint: %w", err)
	}

	// 12. activities.schedules: Partial UNIQUE(weekday, timeframe_id, activity_group_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_activity_schedules_unique;
		CREATE UNIQUE INDEX idx_activity_schedules_tenant_unique
			ON activities.schedules(tenant_id, weekday, timeframe_id, activity_group_id)
			WHERE timeframe_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error migrating activities.schedules unique constraint: %w", err)
	}

	// 13. active.group_supervisors: Partial UNIQUE(staff_id, group_id, role) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.unique_active_staff_group_role;
		CREATE UNIQUE INDEX unique_active_staff_group_role_tenant
			ON active.group_supervisors(tenant_id, staff_id, group_id, role)
			WHERE end_date IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error migrating active.group_supervisors unique constraint: %w", err)
	}

	// 14. active.group_mappings: UNIQUE(active_combined_group_id, active_group_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE active.group_mappings DROP CONSTRAINT IF EXISTS uq_active_group_mappings;
		CREATE UNIQUE INDEX idx_group_mappings_tenant ON active.group_mappings(tenant_id, active_combined_group_id, active_group_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating active.group_mappings unique constraint: %w", err)
	}

	// 15. active.scheduled_checkouts: Partial UNIQUE(student_id, status) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_scheduled_checkouts_unique_pending;
		CREATE UNIQUE INDEX idx_scheduled_checkouts_tenant_pending
			ON active.scheduled_checkouts(tenant_id, student_id, status)
			WHERE status = 'pending';
	`)
	if err != nil {
		return fmt.Errorf("error migrating active.scheduled_checkouts unique constraint: %w", err)
	}

	// 16. schedule.student_pickup_schedules: UNIQUE(student_id, weekday) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE schedule.student_pickup_schedules DROP CONSTRAINT IF EXISTS unique_student_weekday;
		CREATE UNIQUE INDEX idx_pickup_schedules_tenant ON schedule.student_pickup_schedules(tenant_id, student_id, weekday);
	`)
	if err != nil {
		return fmt.Errorf("error migrating schedule.student_pickup_schedules unique constraint: %w", err)
	}

	// 17. schedule.student_pickup_exceptions: UNIQUE(student_id, exception_date) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE schedule.student_pickup_exceptions DROP CONSTRAINT IF EXISTS unique_student_exception_date;
		CREATE UNIQUE INDEX idx_pickup_exceptions_tenant ON schedule.student_pickup_exceptions(tenant_id, student_id, exception_date);
	`)
	if err != nil {
		return fmt.Errorf("error migrating schedule.student_pickup_exceptions unique constraint: %w", err)
	}

	// 18. suggestions.votes: UNIQUE(post_id, voter_id) → +tenant_id
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE suggestions.votes DROP CONSTRAINT IF EXISTS votes_post_id_voter_id_key;
		CREATE UNIQUE INDEX idx_votes_tenant ON suggestions.votes(tenant_id, post_id, voter_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating suggestions.votes unique constraint: %w", err)
	}

	// ===================================================================
	// FIX-9: IoT device_id per-tenant
	// device_id becomes per-tenant unique; api_key stays globally unique
	// ===================================================================

	fmt.Println("  Applying FIX-9: IoT device_id per-tenant unique...")

	_, err = tx.ExecContext(ctx, `
		ALTER TABLE iot.devices DROP CONSTRAINT IF EXISTS devices_device_id_key;
		CREATE UNIQUE INDEX idx_devices_tenant_device_id ON iot.devices(tenant_id, device_id);
	`)
	if err != nil {
		return fmt.Errorf("error migrating iot.devices device_id constraint: %w", err)
	}

	fmt.Println("Migration 1.14.3: Successfully migrated all UNIQUE constraints")
	return tx.Commit()
}

func rollbackUniqueConstraints(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.3: Restoring original UNIQUE constraints...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// ===================================================================
	// Restore FIX-9: IoT device_id
	// ===================================================================
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS iot.idx_devices_tenant_device_id;
		ALTER TABLE iot.devices ADD CONSTRAINT devices_device_id_key UNIQUE (device_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring iot.devices constraint: %w", err)
	}

	// ===================================================================
	// Restore defense-in-depth constraints (reverse order)
	// ===================================================================

	// 18. suggestions.votes
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS suggestions.idx_votes_tenant;
		ALTER TABLE suggestions.votes ADD CONSTRAINT votes_post_id_voter_id_key UNIQUE (post_id, voter_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring suggestions.votes constraint: %w", err)
	}

	// 17. schedule.student_pickup_exceptions
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS schedule.idx_pickup_exceptions_tenant;
		ALTER TABLE schedule.student_pickup_exceptions ADD CONSTRAINT unique_student_exception_date UNIQUE (student_id, exception_date);
	`)
	if err != nil {
		return fmt.Errorf("error restoring schedule.student_pickup_exceptions constraint: %w", err)
	}

	// 16. schedule.student_pickup_schedules
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS schedule.idx_pickup_schedules_tenant;
		ALTER TABLE schedule.student_pickup_schedules ADD CONSTRAINT unique_student_weekday UNIQUE (student_id, weekday);
	`)
	if err != nil {
		return fmt.Errorf("error restoring schedule.student_pickup_schedules constraint: %w", err)
	}

	// 15. active.scheduled_checkouts
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_scheduled_checkouts_tenant_pending;
		CREATE UNIQUE INDEX idx_scheduled_checkouts_unique_pending ON active.scheduled_checkouts(student_id, status) WHERE status = 'pending';
	`)
	if err != nil {
		return fmt.Errorf("error restoring active.scheduled_checkouts constraint: %w", err)
	}

	// 14. active.group_mappings
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.idx_group_mappings_tenant;
		ALTER TABLE active.group_mappings ADD CONSTRAINT uq_active_group_mappings UNIQUE (active_combined_group_id, active_group_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring active.group_mappings constraint: %w", err)
	}

	// 13. active.group_supervisors
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS active.unique_active_staff_group_role_tenant;
		CREATE UNIQUE INDEX unique_active_staff_group_role ON active.group_supervisors(staff_id, group_id, role) WHERE end_date IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring active.group_supervisors constraint: %w", err)
	}

	// 12. activities.schedules
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_activity_schedules_tenant_unique;
		CREATE UNIQUE INDEX idx_activity_schedules_unique ON activities.schedules(weekday, timeframe_id, activity_group_id) WHERE timeframe_id IS NOT NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring activities.schedules constraint: %w", err)
	}

	// 11. activities.supervisors
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_supervisors_planned_tenant;
		ALTER TABLE activities.supervisors ADD CONSTRAINT uq_activity_supervisors_staff_group UNIQUE (staff_id, group_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring activities.supervisors constraint: %w", err)
	}

	// 10. activities.student_enrollments
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_student_enrollments_tenant;
		ALTER TABLE activities.student_enrollments ADD CONSTRAINT uk_student_activity_enrollment UNIQUE (student_id, activity_group_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring activities.student_enrollments constraint: %w", err)
	}

	// 9. education.grade_transition_mappings
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS education.idx_grade_transition_mappings_tenant;
		ALTER TABLE education.grade_transition_mappings ADD CONSTRAINT grade_transition_mappings_transition_id_from_class_key UNIQUE (transition_id, from_class);
	`)
	if err != nil {
		return fmt.Errorf("error restoring education.grade_transition_mappings constraint: %w", err)
	}

	// 8. education.group_substitution
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS education.idx_no_duplicate_group_transfers_tenant;
		CREATE UNIQUE INDEX idx_no_duplicate_group_transfers ON education.group_substitution(group_id, substitute_staff_id, start_date) WHERE regular_staff_id IS NULL;
	`)
	if err != nil {
		return fmt.Errorf("error restoring education.group_substitution constraint: %w", err)
	}

	// 7. education.group_teacher
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS education.idx_group_teacher_tenant;
		ALTER TABLE education.group_teacher ADD CONSTRAINT uk_group_teacher UNIQUE (group_id, teacher_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring education.group_teacher constraint: %w", err)
	}

	// 6. users.persons_guardians
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_guardians_tenant;
		ALTER TABLE users.persons_guardians ADD CONSTRAINT unique_person_guardian_relationship UNIQUE (person_id, guardian_account_id, relationship_type);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.persons_guardians constraint: %w", err)
	}

	// 5. users.students_guardians
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_students_guardians_tenant;
		ALTER TABLE users.students_guardians ADD CONSTRAINT unique_student_guardian UNIQUE (student_id, guardian_profile_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.students_guardians constraint: %w", err)
	}

	// 4. users.students
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_students_tenant_person;
		ALTER TABLE users.students ADD CONSTRAINT students_person_id_key UNIQUE (person_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.students constraint: %w", err)
	}

	// 3. users.guests
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_guests_tenant_staff;
		ALTER TABLE users.guests ADD CONSTRAINT guests_staff_id_key UNIQUE (staff_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.guests constraint: %w", err)
	}

	// 2. users.teachers
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_teachers_tenant_staff;
		ALTER TABLE users.teachers ADD CONSTRAINT teachers_staff_id_key UNIQUE (staff_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.teachers constraint: %w", err)
	}

	// 1. users.staff
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_staff_tenant_person;
		ALTER TABLE users.staff ADD CONSTRAINT staff_person_id_key UNIQUE (person_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.staff constraint: %w", err)
	}

	// ===================================================================
	// Restore auth.roles special case
	// ===================================================================
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_roles_name_system;
		DROP INDEX IF EXISTS auth.idx_roles_name_tenant;
		CREATE UNIQUE INDEX idx_roles_name ON auth.roles(name);
	`)
	if err != nil {
		return fmt.Errorf("error restoring auth.roles constraint: %w", err)
	}

	// ===================================================================
	// Restore functionally necessary constraints (reverse order)
	// ===================================================================

	// 13. auth.account_permissions
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_account_permissions_tenant;
		CREATE UNIQUE INDEX idx_account_permissions_account_permission ON auth.account_permissions(account_id, permission_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring auth.account_permissions constraint: %w", err)
	}

	// 12. auth.account_roles
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_account_roles_tenant;
		CREATE UNIQUE INDEX idx_account_roles_account_role ON auth.account_roles(account_id, role_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring auth.account_roles constraint: %w", err)
	}

	// 11. auth.accounts_parents username
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_accounts_parents_tenant_username;
		ALTER TABLE auth.accounts_parents ADD CONSTRAINT accounts_parents_username_key UNIQUE (username);
	`)
	if err != nil {
		return fmt.Errorf("error restoring auth.accounts_parents username constraint: %w", err)
	}

	// 10. auth.accounts_parents email
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS auth.idx_accounts_parents_tenant_email;
		CREATE UNIQUE INDEX idx_accounts_parents_email ON auth.accounts_parents(email);
	`)
	if err != nil {
		return fmt.Errorf("error restoring auth.accounts_parents email constraint: %w", err)
	}

	// 9. users.guardian_profiles account_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_guardian_profiles_tenant_account;
		ALTER TABLE users.guardian_profiles ADD CONSTRAINT guardian_profiles_account_id_key UNIQUE (account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.guardian_profiles account_id constraint: %w", err)
	}

	// 8. users.guardian_profiles email
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_guardian_profiles_tenant_email;
		ALTER TABLE users.guardian_profiles ADD CONSTRAINT guardian_profiles_email_key UNIQUE (email);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.guardian_profiles email constraint: %w", err)
	}

	// 7. users.profiles
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_profiles_tenant_account;
		ALTER TABLE users.profiles ADD CONSTRAINT profiles_account_id_key UNIQUE (account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.profiles constraint: %w", err)
	}

	// 6. users.persons tag_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_tenant_tag;
		ALTER TABLE users.persons ADD CONSTRAINT persons_tag_id_key UNIQUE (tag_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.persons tag_id constraint: %w", err)
	}

	// 5. users.persons account_id
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_persons_tenant_account;
		ALTER TABLE users.persons ADD CONSTRAINT persons_account_id_key UNIQUE (account_id);
	`)
	if err != nil {
		return fmt.Errorf("error restoring users.persons account_id constraint: %w", err)
	}

	// 4. config.settings
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS config.idx_settings_tenant_key;
		ALTER TABLE config.settings ADD CONSTRAINT settings_key_key UNIQUE (key);
	`)
	if err != nil {
		return fmt.Errorf("error restoring config.settings constraint: %w", err)
	}

	// 3. activities.categories
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS activities.idx_categories_tenant_name;
		ALTER TABLE activities.categories ADD CONSTRAINT categories_name_key UNIQUE (name);
	`)
	if err != nil {
		return fmt.Errorf("error restoring activities.categories constraint: %w", err)
	}

	// 2. education.groups
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS education.idx_groups_tenant_name;
		ALTER TABLE education.groups ADD CONSTRAINT groups_name_key UNIQUE (name);
	`)
	if err != nil {
		return fmt.Errorf("error restoring education.groups constraint: %w", err)
	}

	// 1. facilities.rooms
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS facilities.idx_rooms_tenant_name;
		ALTER TABLE facilities.rooms ADD CONSTRAINT rooms_name_key UNIQUE (name);
	`)
	if err != nil {
		return fmt.Errorf("error restoring facilities.rooms constraint: %w", err)
	}

	fmt.Println("Migration 1.14.3: Successfully restored original UNIQUE constraints")
	return tx.Commit()
}
