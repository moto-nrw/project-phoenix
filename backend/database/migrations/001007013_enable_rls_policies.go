package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

const (
	EnableRLSPoliciesVersion     = "1.8.3"
	EnableRLSPoliciesDescription = "Enable Row-Level Security on domain tables for tenant isolation"
)

// rlsTables lists all tables that need RLS policies.
// These are the primary domain tables that hold tenant-specific data.
// Must match the tables from 001007006_add_ogs_id_columns.go
var rlsTables = []string{
	"users.persons",
	"users.students",
	"users.staff",
	"users.teachers",
	"education.groups",
	"facilities.rooms",
	"iot.devices",
	"active.visits",
	"active.groups",
	"activities.groups",
	"activities.categories",
}

func init() {
	MigrationRegistry[EnableRLSPoliciesVersion] = &Migration{
		Version:     EnableRLSPoliciesVersion,
		Description: EnableRLSPoliciesDescription,
		DependsOn:   []string{"1.8.2"}, // After current_ogs_id() function
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return enableRLSPolicies(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return disableRLSPolicies(ctx, db)
		},
	)
}

// enableRLSPolicies enables Row-Level Security on all domain tables and creates
// isolation policies that filter data by ogs_id.
//
// Policy behavior:
// - FOR ALL: Applies to SELECT, INSERT, UPDATE, DELETE operations
// - USING clause: Filters existing rows (SELECT, UPDATE, DELETE)
// - WITH CHECK clause: Validates new/modified rows (INSERT, UPDATE)
//
// Security model:
//   - Rows are only visible if their ogs_id matches current_ogs_id()
//   - New rows must have ogs_id matching current_ogs_id()
//   - If app.ogs_id is not set, current_ogs_id() returns a non-matching UUID,
//     so no data is visible (safe default)
func enableRLSPolicies(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.8.3: Enabling Row-Level Security on domain tables...")

	for _, table := range rlsTables {
		// Generate policy name from table name
		// e.g., "users.students" -> "students_ogs_isolation"
		parts := strings.Split(table, ".")
		tableName := parts[len(parts)-1]
		policyName := tableName + "_ogs_isolation"

		// 1. Enable RLS on the table
		// This tells PostgreSQL to enforce policies on this table
		enableSQL := fmt.Sprintf(`ALTER TABLE %s ENABLE ROW LEVEL SECURITY`, table)
		_, err := db.ExecContext(ctx, enableSQL)
		if err != nil {
			return fmt.Errorf("error enabling RLS on %s: %w", table, err)
		}

		// 2. Drop existing policy if exists (idempotent re-run support)
		dropSQL := fmt.Sprintf(`DROP POLICY IF EXISTS %s ON %s`, policyName, table)
		_, _ = db.ExecContext(ctx, dropSQL) // Ignore error if policy doesn't exist

		// 3. Create policy for ALL operations (SELECT, INSERT, UPDATE, DELETE)
		// USING: Controls which existing rows are visible (for SELECT, UPDATE, DELETE)
		// WITH CHECK: Controls which new/modified rows are allowed (for INSERT, UPDATE)
		createPolicySQL := fmt.Sprintf(`
			CREATE POLICY %s ON %s
				FOR ALL
				USING (ogs_id = current_ogs_id())
				WITH CHECK (ogs_id = current_ogs_id())
		`, policyName, table)
		_, err = db.ExecContext(ctx, createPolicySQL)
		if err != nil {
			return fmt.Errorf("error creating policy %s on %s: %w", policyName, table, err)
		}

		fmt.Printf("  Enabled RLS on %s with policy %s\n", table, policyName)
	}

	fmt.Println("Migration 1.8.3: Successfully enabled RLS on all domain tables")
	fmt.Println("  NOTE: Table owners (e.g., postgres user) bypass RLS by default.")
	fmt.Println("  Application user should NOT be table owner to ensure RLS is enforced.")
	return nil
}

// disableRLSPolicies removes RLS policies and disables RLS on all domain tables.
// This restores the tables to their pre-RLS state.
func disableRLSPolicies(ctx context.Context, db *bun.DB) error {
	fmt.Println("Rolling back migration 1.8.3: Disabling Row-Level Security...")

	for _, table := range rlsTables {
		parts := strings.Split(table, ".")
		tableName := parts[len(parts)-1]
		policyName := tableName + "_ogs_isolation"

		// 1. Drop policy first (must be done before disabling RLS)
		dropSQL := fmt.Sprintf(`DROP POLICY IF EXISTS %s ON %s`, policyName, table)
		_, _ = db.ExecContext(ctx, dropSQL) // Ignore error if policy doesn't exist

		// 2. Disable RLS on the table
		disableSQL := fmt.Sprintf(`ALTER TABLE %s DISABLE ROW LEVEL SECURITY`, table)
		_, err := db.ExecContext(ctx, disableSQL)
		if err != nil {
			return fmt.Errorf("error disabling RLS on %s: %w", table, err)
		}

		fmt.Printf("  Disabled RLS on %s\n", table)
	}

	fmt.Println("Migration 1.8.3: Successfully disabled RLS on all domain tables")
	return nil
}
