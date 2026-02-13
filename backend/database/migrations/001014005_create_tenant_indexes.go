package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	tenantIndexesVersion     = "1.14.5"
	tenantIndexesDescription = "Create tenant_id indexes on all tenant-scoped tables plus performance composite indexes"
)

func init() {
	MigrationRegistry[tenantIndexesVersion] = &Migration{
		Version:     tenantIndexesVersion,
		Description: tenantIndexesDescription,
		DependsOn:   []string{"1.14.2"}, // tenant_id columns exist
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createTenantIndexes(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackTenantIndexes(ctx, db)
		},
	)
}

// allTenantScopedTables lists all 58+1 tables that have a tenant_id column.
// Each gets a standard idx_{table}_tenant index for RLS filter performance.
var allTenantScopedTables = []struct {
	schema string
	table  string
}{
	// auth (6 NOT NULL + 1 NULLABLE)
	{"auth", "tokens"},
	{"auth", "invitation_tokens"},
	{"auth", "accounts_parents"},
	{"auth", "guardian_invitations"},
	{"auth", "account_roles"},
	{"auth", "account_permissions"},
	{"auth", "roles"}, // nullable tenant_id

	// users (12)
	{"users", "rfid_cards"},
	{"users", "persons"},
	{"users", "profiles"},
	{"users", "staff"},
	{"users", "teachers"},
	{"users", "guests"},
	{"users", "persons_guardians"},
	{"users", "students"},
	{"users", "guardian_profiles"},
	{"users", "students_guardians"},
	{"users", "privacy_consents"},
	{"users", "guardian_phone_numbers"},

	// education (6)
	{"education", "groups"},
	{"education", "group_teacher"},
	{"education", "group_substitution"},
	{"education", "grade_transitions"},
	{"education", "grade_transition_mappings"},
	{"education", "grade_transition_history"},

	// facilities (1)
	{"facilities", "rooms"},

	// activities (5)
	{"activities", "categories"},
	{"activities", "groups"},
	{"activities", "schedules"},
	{"activities", "supervisors_planned"},
	{"activities", "student_enrollments"},

	// active (10)
	{"active", "groups"},
	{"active", "visits"},
	{"active", "group_supervisors"},
	{"active", "combined_groups"},
	{"active", "group_mappings"},
	{"active", "attendance"},
	{"active", "scheduled_checkouts"},
	{"active", "work_sessions"},
	{"active", "work_session_breaks"},
	{"active", "staff_absences"},

	// schedule (6)
	{"schedule", "timeframes"},
	{"schedule", "dateframes"},
	{"schedule", "recurrence_rules"},
	{"schedule", "student_pickup_schedules"},
	{"schedule", "student_pickup_exceptions"},
	{"schedule", "student_pickup_notes"},

	// iot (1)
	{"iot", "devices"},

	// feedback (1)
	{"feedback", "entries"},

	// config (1)
	{"config", "settings"},

	// suggestions (5)
	{"suggestions", "posts"},
	{"suggestions", "votes"},
	{"suggestions", "comments"},
	{"suggestions", "comment_reads"},
	{"suggestions", "post_reads"},

	// audit (4)
	{"audit", "data_deletions"},
	{"audit", "auth_events"},
	{"audit", "data_imports"},
	{"audit", "work_session_edits"},
}

// compositePKTableSet lists the 18 tables from V1.14.4 that already have UNIQUE(tenant_id, id).
// Used to skip these when creating the regular (tenant_id, id) indexes.
var compositePKTableSet = map[string]bool{
	"users.persons":               true,
	"users.staff":                 true,
	"users.students":              true,
	"users.teachers":              true,
	"users.guardian_profiles":     true,
	"users.rfid_cards":            true,
	"education.groups":            true,
	"education.grade_transitions": true,
	"facilities.rooms":            true,
	"activities.categories":       true,
	"activities.groups":           true,
	"active.groups":               true,
	"active.combined_groups":      true,
	"active.work_sessions":        true,
	"iot.devices":                 true,
	"schedule.timeframes":         true,
	"suggestions.posts":           true,
	"auth.accounts_parents":       true,
}

func isCompositePKTable(schema, table string) bool {
	return compositePKTableSet[schema+"."+table]
}

func createTenantIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.5: Creating tenant indexes on all tenant-scoped tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Standard tenant_id index on every table
	for _, t := range allTenantScopedTables {
		indexName := fmt.Sprintf("idx_%s_tenant", t.table)
		fullTable := fmt.Sprintf("%s.%s", t.schema, t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s(tenant_id);
		`, indexName, fullTable))
		if err != nil {
			return fmt.Errorf("error creating tenant index on %s: %w", fullTable, err)
		}
	}

	// Composite (tenant_id, id) indexes on tables NOT already covered by V1.14.4.
	// V1.14.4 creates UNIQUE(tenant_id, id) on 18 FK-target tables. The remaining
	// 41 tables get a regular composite index for queries filtering on both tenant and PK.
	fmt.Println("  Creating composite (tenant_id, id) indexes on remaining tables...")
	for _, t := range allTenantScopedTables {
		if isCompositePKTable(t.schema, t.table) {
			continue // Already has UNIQUE(tenant_id, id) from V1.14.4
		}
		indexName := fmt.Sprintf("idx_%s_tenant_id", t.table)
		fullTable := fmt.Sprintf("%s.%s", t.schema, t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			CREATE INDEX IF NOT EXISTS %s ON %s(tenant_id, id);
		`, indexName, fullTable))
		if err != nil {
			return fmt.Errorf("error creating composite tenant+id index on %s: %w", fullTable, err)
		}
	}

	// Performance composite indexes for common query patterns
	fmt.Println("  Creating performance composite indexes...")

	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_students_tenant_class
			ON users.students(tenant_id, school_class);

		CREATE INDEX IF NOT EXISTS idx_visits_tenant_active
			ON active.visits(tenant_id, exit_time) WHERE exit_time IS NULL;

		CREATE INDEX IF NOT EXISTS idx_attendance_tenant_date
			ON active.attendance(tenant_id, date);

		CREATE INDEX IF NOT EXISTS idx_devices_tenant_status
			ON iot.devices(tenant_id, status);
	`)
	if err != nil {
		return fmt.Errorf("error creating performance composite indexes: %w", err)
	}

	fmt.Println("Migration 1.14.5: Successfully created all tenant indexes")
	return tx.Commit()
}

func rollbackTenantIndexes(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.5: Dropping tenant indexes...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop performance composite indexes
	_, err = tx.ExecContext(ctx, `
		DROP INDEX IF EXISTS users.idx_students_tenant_class;
		DROP INDEX IF EXISTS active.idx_visits_tenant_active;
		DROP INDEX IF EXISTS active.idx_attendance_tenant_date;
		DROP INDEX IF EXISTS iot.idx_devices_tenant_status;
	`)
	if err != nil {
		return fmt.Errorf("error dropping performance composite indexes: %w", err)
	}

	// Drop composite (tenant_id, id) indexes on non-PK tables
	for _, t := range allTenantScopedTables {
		if isCompositePKTable(t.schema, t.table) {
			continue // Owned by V1.14.4
		}
		indexName := fmt.Sprintf("idx_%s_tenant_id", t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DROP INDEX IF EXISTS %s.%s;
		`, t.schema, indexName))
		if err != nil {
			return fmt.Errorf("error dropping composite tenant+id index on %s.%s: %w", t.schema, t.table, err)
		}
	}

	// Drop standard tenant indexes
	for _, t := range allTenantScopedTables {
		indexName := fmt.Sprintf("idx_%s_tenant", t.table)

		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			DROP INDEX IF EXISTS %s.%s;
		`, t.schema, indexName))
		if err != nil {
			return fmt.Errorf("error dropping tenant index on %s.%s: %w", t.schema, t.table, err)
		}
	}

	fmt.Println("Migration 1.14.5: Successfully dropped all tenant indexes")
	return tx.Commit()
}
