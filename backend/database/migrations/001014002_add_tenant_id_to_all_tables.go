package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"github.com/uptrace/bun"
)

const (
	addTenantIDVersion     = "1.14.2"
	addTenantIDDescription = "Add tenant_id column (DEFAULT 1) to all 58+1 tenant-scoped tables"
)

func init() {
	MigrationRegistry[addTenantIDVersion] = &Migration{
		Version:     addTenantIDVersion,
		Description: addTenantIDDescription,
		DependsOn:   []string{"1.14.1", "1.13.1"}, // Roles exist, platform.schools for FK
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return addTenantIDToAllTables(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackTenantIDFromAllTables(ctx, db)
		},
	)
}

// tablesWithNotNullTenantID lists all 58 tables that get NOT NULL tenant_id with DEFAULT 1.
var tablesWithNotNullTenantID = []string{
	// auth (6 tables)
	"auth.tokens",
	"auth.invitation_tokens",
	"auth.accounts_parents",
	"auth.guardian_invitations",
	"auth.account_roles",
	"auth.account_permissions",

	// users (12 tables)
	"users.rfid_cards",
	"users.persons",
	"users.profiles",
	"users.staff",
	"users.teachers",
	"users.guests",
	"users.persons_guardians",
	"users.students",
	"users.guardian_profiles",
	"users.students_guardians",
	"users.privacy_consents",
	"users.guardian_phone_numbers",

	// education (6 tables)
	"education.groups",
	"education.group_teacher",
	"education.group_substitution",
	"education.grade_transitions",
	"education.grade_transition_mappings",
	"education.grade_transition_history",

	// facilities (1 table)
	"facilities.rooms",

	// activities (5 tables)
	"activities.categories",
	"activities.groups",
	"activities.schedules",
	"activities.supervisors_planned",
	"activities.student_enrollments",

	// active (10 tables)
	"active.groups",
	"active.visits",
	"active.group_supervisors",
	"active.combined_groups",
	"active.group_mappings",
	"active.attendance",
	"active.scheduled_checkouts",
	"active.work_sessions",
	"active.work_session_breaks",
	"active.staff_absences",

	// schedule (6 tables)
	"schedule.timeframes",
	"schedule.dateframes",
	"schedule.recurrence_rules",
	"schedule.student_pickup_schedules",
	"schedule.student_pickup_exceptions",
	"schedule.student_pickup_notes",

	// iot (1 table)
	"iot.devices",

	// feedback (1 table)
	"feedback.entries",

	// config (1 table)
	"config.settings",

	// suggestions (5 tables)
	"suggestions.posts",
	"suggestions.votes",
	"suggestions.comments",
	"suggestions.comment_reads",
	"suggestions.post_reads",

	// audit (4 tables)
	"audit.data_deletions",
	"audit.auth_events",
	"audit.data_imports",
	"audit.work_session_edits",
}

func addTenantIDToAllTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.14.2: Adding tenant_id to all tenant-scoped tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Ensure default organization and school exist BEFORE adding FK constraints.
	// The FK REFERENCES platform.schools(id) with DEFAULT 1 requires school id=1
	// to exist, otherwise PostgreSQL rejects existing rows on FK validation.
	_, err = tx.ExecContext(ctx, `
		INSERT INTO platform.organizations (id, name, slug, active)
		VALUES (1, 'Default Organization', 'default', true)
		ON CONFLICT (id) DO NOTHING;

		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active)
		VALUES (1, 1, 'Default School', 'default', 'default', true)
		ON CONFLICT (id) DO NOTHING;

		-- Advance sequences past the explicitly inserted id=1 so that future
		-- auto-generated IDs (nextval) do not collide with the default rows.
		SELECT setval(pg_get_serial_sequence('platform.organizations', 'id'),
			GREATEST(nextval(pg_get_serial_sequence('platform.organizations', 'id')),
				(SELECT COALESCE(MAX(id), 1) FROM platform.organizations)));
		SELECT setval(pg_get_serial_sequence('platform.schools', 'id'),
			GREATEST(nextval(pg_get_serial_sequence('platform.schools', 'id')),
				(SELECT COALESCE(MAX(id), 1) FROM platform.schools)));
	`)
	if err != nil {
		return fmt.Errorf("error ensuring default school for FK target: %w", err)
	}

	// Add NOT NULL tenant_id to 58 tables using the safe 5-step pattern
	for _, table := range tablesWithNotNullTenantID {
		fmt.Printf("  Adding tenant_id to %s...\n", table)

		// Step 1: Add column with DEFAULT 1 (instant metadata change in PG 11+)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				ADD COLUMN IF NOT EXISTS tenant_id BIGINT DEFAULT 1
				REFERENCES platform.schools(id);
		`, table))
		if err != nil {
			return fmt.Errorf("error adding tenant_id to %s: %w", table, err)
		}

		// Step 2: Backfill (instant since column was just added with DEFAULT)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			UPDATE %s SET tenant_id = 1 WHERE tenant_id IS NULL;
		`, table))
		if err != nil {
			return fmt.Errorf("error backfilling tenant_id on %s: %w", table, err)
		}

		// Step 3: CHECK NOT VALID (instant, no table scan)
		constraintName := constraintNameForTable(table)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				ADD CONSTRAINT %s
				CHECK (tenant_id IS NOT NULL) NOT VALID;
		`, table, constraintName))
		if err != nil {
			return fmt.Errorf("error adding CHECK constraint to %s: %w", table, err)
		}

		// Step 4: Validate constraint (scan, no exclusive lock)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				VALIDATE CONSTRAINT %s;
		`, table, constraintName))
		if err != nil {
			return fmt.Errorf("error validating CHECK constraint on %s: %w", table, err)
		}

		// Step 5: SET NOT NULL (instant when valid CHECK exists in PG 12+)
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s
				ALTER COLUMN tenant_id SET NOT NULL;
		`, table))
		if err != nil {
			return fmt.Errorf("error setting NOT NULL on %s.tenant_id: %w", table, err)
		}
	}

	// Special case: auth.roles gets NULLABLE tenant_id (D13)
	// System roles (admin, user, guest, guardian) keep NULL = globally visible
	fmt.Println("  Adding NULLABLE tenant_id to auth.roles...")
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.roles
			ADD COLUMN IF NOT EXISTS tenant_id BIGINT REFERENCES platform.schools(id);
	`)
	if err != nil {
		return fmt.Errorf("error adding nullable tenant_id to auth.roles: %w", err)
	}

	fmt.Println("Migration 1.14.2: Successfully added tenant_id to all 58+1 tables")
	return tx.Commit()
}

// constraintNameForTable generates the CHECK constraint name for a table.
// e.g., "auth.tokens" → "chk_tokens_tenant_id_not_null"
func constraintNameForTable(schemaTable string) string {
	// Extract just the table name (after the dot)
	for i := len(schemaTable) - 1; i >= 0; i-- {
		if schemaTable[i] == '.' {
			return fmt.Sprintf("chk_%s_tenant_id_not_null", schemaTable[i+1:])
		}
	}
	return fmt.Sprintf("chk_%s_tenant_id_not_null", schemaTable)
}

func rollbackTenantIDFromAllTables(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.14.2: Removing tenant_id from all tables...")

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			log.Printf("Error rolling back transaction: %v", err)
		}
	}()

	// Drop tenant_id from all NOT NULL tables
	for _, table := range tablesWithNotNullTenantID {
		_, err = tx.ExecContext(ctx, fmt.Sprintf(`
			ALTER TABLE %s DROP COLUMN IF EXISTS tenant_id CASCADE;
		`, table))
		if err != nil {
			return fmt.Errorf("error dropping tenant_id from %s: %w", table, err)
		}
	}

	// Drop tenant_id from auth.roles (nullable)
	_, err = tx.ExecContext(ctx, `
		ALTER TABLE auth.roles DROP COLUMN IF EXISTS tenant_id CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping tenant_id from auth.roles: %w", err)
	}

	fmt.Println("Migration 1.14.2: Successfully removed tenant_id from all tables")
	return tx.Commit()
}
