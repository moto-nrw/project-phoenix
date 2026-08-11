package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	repairSchoolIdentitiesVersion     = "1.15.284"
	repairSchoolIdentitiesDescription = "Repair accounts that hold a school role without a staff record (#2222)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     repairSchoolIdentitiesVersion,
		Description: repairSchoolIdentitiesDescription,
		DependsOn:   []string{rolesBaseRoleBackfillVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return repairSchoolIdentitiesUp(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return repairSchoolIdentitiesDown(ctx, db)
		},
	)
}

// staffTierRoleExists matches an account that holds at least one role of staff
// tier at the school. Mirrors RoleNeedsStaffRecord: everything that is not a
// guardian is personnel, and an unknown tier counts as personnel because a
// staff row grants nothing — withholding it is what breaks the account.
const staffTierRoleExists = `
	EXISTS (
		SELECT 1
		FROM auth.account_roles ar
		JOIN auth.roles r ON r.id = ar.role_id
		WHERE ar.account_id = p.account_id
		  AND ar.tenant_id = p.tenant_id
		  AND COALESCE(
				NULLIF(LOWER(TRIM(r.base_role)), ''),
				CASE WHEN r.is_system THEN LOWER(TRIM(r.name)) END,
				''
		      ) <> 'guardian'
	)`

// repairSchoolIdentitiesUp adds the users.staff rows that the invitation flow
// failed to create.
//
// Until #2222 the staff record was created only for platform roles. An account
// invited with a school's own role got a person and nothing else: it holds the
// role and logs in, but the chain account → users.persons → users.staff is
// broken, so GetCurrentStaff fails and everything reading through it is dead
// for that account — no entry in the staff list, no /api/me/staff, no starting
// a spontaneous activity.
//
// The repair attaches the staff record to the person that already carries
// account_id. That is the whole point: creating the staff member through the UI
// instead would need a second person for the same account at the same school,
// which the partial unique index on (tenant_id, account_id) refuses outright.
//
// Scope, deliberately narrow:
//   - only persons linked to an account, not soft-deleted
//   - only where that account still has ACTIVE access to that school
//   - only where the account holds a staff-tier role there
//   - never a person that is a student
//
// Caregiver profiles (users.teachers) are created under the same rule the
// provisioning now applies: caregiver tier gets one, as does the retired
// platform 'teacher' role that predates the tier column, and the class-scoped
// Lehrkraft role never does (#1772). Without it the repaired account would show
// up in the staff list and still find its groups and supervisions empty.
//
// Idempotent — every insert is guarded by NOT EXISTS.
func repairSchoolIdentitiesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.284: Repairing accounts with a school role but no staff record...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO users.staff (tenant_id, person_id, staff_notes, created_at, updated_at)
			SELECT p.tenant_id, p.id, '', NOW(), NOW()
			FROM users.persons p
			WHERE p.account_id IS NOT NULL
			  AND p.deleted_at IS NULL
			  AND EXISTS (
				SELECT 1 FROM auth.account_tenants at
				WHERE at.account_id = p.account_id
				  AND at.tenant_id = p.tenant_id
				  AND at.status = 'active'
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM users.staff s
				WHERE s.person_id = p.id AND s.deleted_at IS NULL
			  )
			  AND NOT EXISTS (
				SELECT 1 FROM users.students st WHERE st.person_id = p.id
			  )
			  AND `+staffTierRoleExists+`
		`)
		if err != nil {
			return fmt.Errorf("error repairing staff records: %w", err)
		}
		if affected, affErr := res.RowsAffected(); affErr == nil {
			fmt.Printf("  Created %d missing staff record(s)\n", affected)
		}

		res, err = tx.ExecContext(ctx, `
			INSERT INTO users.teachers (tenant_id, staff_id, specialization, created_at, updated_at)
			SELECT s.tenant_id, s.id, '', NOW(), NOW()
			FROM users.staff s
			JOIN users.persons p ON p.id = s.person_id AND p.deleted_at IS NULL
			WHERE s.deleted_at IS NULL
			  AND p.account_id IS NOT NULL
			  AND NOT EXISTS (
				SELECT 1 FROM users.teachers t
				WHERE t.staff_id = s.id AND t.deleted_at IS NULL
			  )
			  AND EXISTS (
				SELECT 1 FROM auth.account_tenants at
				WHERE at.account_id = p.account_id
				  AND at.tenant_id = p.tenant_id
				  AND at.status = 'active'
			  )
			  AND EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.roles r ON r.id = ar.role_id
				WHERE ar.account_id = p.account_id
				  AND ar.tenant_id = p.tenant_id
				  AND (
					COALESCE(
						NULLIF(LOWER(TRIM(r.base_role)), ''),
						CASE WHEN r.is_system THEN LOWER(TRIM(r.name)) END,
						''
					) = 'user'
					-- The retired platform 'teacher' role predates base_role and
					-- was never backfilled, so its tier reads as unknown.
					-- RoleNeedsCaregiverProfile matches it by name; without the
					-- same name match here the accounts still holding it get a
					-- staff record and no caregiver profile, which is the half
					-- of the bug this migration exists to repair.
					OR (r.is_system AND LOWER(TRIM(r.name)) = 'teacher')
				  )
				  AND NOT (r.is_system AND LOWER(TRIM(r.name)) = 'lehrkraft')
			  )
			  AND NOT EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.roles r ON r.id = ar.role_id
				WHERE ar.account_id = p.account_id
				  AND ar.tenant_id = p.tenant_id
				  AND r.is_system
				  AND LOWER(TRIM(r.name)) = 'lehrkraft'
			  )
		`)
		if err != nil {
			return fmt.Errorf("error repairing caregiver profiles: %w", err)
		}
		if affected, affErr := res.RowsAffected(); affErr == nil {
			fmt.Printf("  Created %d missing caregiver profile(s)\n", affected)
		}

		return reportUnrepairableSchoolIdentities(ctx, tx)
	})
}

// reportUnrepairableSchoolIdentities lists the accounts the repair cannot
// reach: they have school access and a staff-tier role but no person at all, so
// there is no name to build an identity from. Creating one would mean inventing
// personal data. They are printed so a human can complete them through the
// staff UI, which links the new person to the account.
func reportUnrepairableSchoolIdentities(ctx context.Context, tx bun.Tx) error {
	var rows []struct {
		AccountID int64  `bun:"account_id"`
		TenantID  int64  `bun:"tenant_id"`
		Email     string `bun:"email"`
	}

	if err := tx.NewRaw(`
		SELECT at.account_id, at.tenant_id, a.email
		FROM auth.account_tenants at
		JOIN auth.accounts a ON a.id = at.account_id
		WHERE at.status = 'active'
		  AND NOT EXISTS (
			SELECT 1 FROM users.persons p
			WHERE p.account_id = at.account_id
			  AND p.tenant_id = at.tenant_id
			  AND p.deleted_at IS NULL
		  )
		  AND EXISTS (
			SELECT 1
			FROM auth.account_roles ar
			JOIN auth.roles r ON r.id = ar.role_id
			WHERE ar.account_id = at.account_id
			  AND ar.tenant_id = at.tenant_id
			  AND COALESCE(
					NULLIF(LOWER(TRIM(r.base_role)), ''),
					CASE WHEN r.is_system THEN LOWER(TRIM(r.name)) END,
					''
			      ) <> 'guardian'
		  )
		ORDER BY at.tenant_id, at.account_id
	`).Scan(ctx, &rows); err != nil {
		return fmt.Errorf("error listing unrepairable accounts: %w", err)
	}

	if len(rows) == 0 {
		return nil
	}

	fmt.Printf("  %d account(s) hold school access without any person record and need a name:\n", len(rows))
	for _, row := range rows {
		fmt.Printf("    school %d: account %d (%s)\n", row.TenantID, row.AccountID, row.Email)
	}

	return nil
}

// repairSchoolIdentitiesDown is a no-op on purpose. The rows this migration
// created are indistinguishable from the ones the fixed provisioning creates,
// and deleting staff records would take their historical references with them.
func repairSchoolIdentitiesDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.284: nothing to undo (repair only, see doc comment)")
	return nil
}
