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
// tier at the school named by the given columns. Mirrors RoleNeedsStaffRecord:
// everything that is not a guardian is personnel, and an unknown tier counts as
// personnel because a staff row grants nothing — withholding it is what breaks
// the account.
//
// The column names are the caller's own aliases, never input.
func staffTierRoleExists(accountColumn, tenantColumn string) string {
	return `
	EXISTS (
		SELECT 1
		FROM auth.account_roles ar
		JOIN auth.roles r ON r.id = ar.role_id
		WHERE ar.account_id = ` + accountColumn + `
		  AND ar.tenant_id = ` + tenantColumn + `
		  AND COALESCE(
				NULLIF(LOWER(TRIM(r.base_role)), ''),
				CASE WHEN r.is_system THEN LOWER(TRIM(r.name)) END,
				''
		      ) <> 'guardian'
	)`
}

// unambiguousNameFromMappedSchools yields, per account, the one name it carries
// at the schools it is ACTIVELY mapped to.
//
// This is the SQL twin of loadPersonNamesForTenant's fallback: the name shown
// to such an account today already comes from another of its schools, which is
// why the broken state looks healthy in the header. Only schools the account
// genuinely belongs to are consulted, and only a name they all agree on is
// used — two different names is an ambiguity this migration is no more
// entitled to resolve than the login is, so those accounts fall through to the
// report instead.
//
// Student rows are never a name source: a person that is a student is a child,
// not the account holder's identity.
const unambiguousNameFromMappedSchools = `
	SELECT p.account_id,
	       MIN(TRIM(p.first_name)) AS first_name,
	       MIN(TRIM(p.last_name))  AS last_name
	FROM users.persons p
	JOIN auth.account_tenants at2
	  ON at2.account_id = p.account_id
	 AND at2.tenant_id = p.tenant_id
	 AND at2.status = 'active'
	WHERE p.account_id IS NOT NULL
	  AND p.deleted_at IS NULL
	  AND TRIM(p.first_name) <> ''
	  AND TRIM(p.last_name) <> ''
	  AND NOT EXISTS (
		SELECT 1 FROM users.students st WHERE st.person_id = p.id
	  )
	GROUP BY p.account_id
	HAVING COUNT(DISTINCT (TRIM(p.first_name), TRIM(p.last_name))) = 1`

// repairSchoolIdentitiesUp completes the identity chain for accounts that hold
// school access as personnel without carrying it.
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
// An account can also be missing the person itself. That is the second source
// of the same state: /auth/register and /auth/link-to-tenant created account,
// mapping and role, and left the identity to two follow-up requests from the
// browser. Where such an account is mapped to more than one school, the name is
// not lost — it sits on its person at another of its schools, and the login
// already serves it from there, which is why the header shows a name while the
// school's own staff list does not. The first step below writes that name into
// a person at this school, under the same unambiguity rule the login applies,
// so the staff step can then do its work.
//
// Scope, deliberately narrow:
//   - only persons linked to an account, not soft-deleted
//   - only where that account still has ACTIVE access to that school
//   - only where the account holds a staff-tier role there
//   - never a person that is a student, neither as target nor as name source
//
// Caregiver profiles (users.teachers) are created under the same rule the
// provisioning now applies, and like it the rule is decided PER ROLE: caregiver
// tier gets one, as does the retired platform 'teacher' role that predates the
// tier column, and the class-scoped Lehrkraft role never does (#1772) — so an
// account that holds Lehrkraft alongside a real caregiver role still gets the
// profile that role requires. Without it the repaired account would show up in
// the staff list and still find its groups and supervisions empty.
//
// Idempotent — every insert is guarded by NOT EXISTS.
func repairSchoolIdentitiesUp(ctx context.Context, db *bun.DB) error {
	fmt.Println("Migration 1.15.284: Repairing accounts with a school role but no staff record...")

	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		res, err := tx.ExecContext(ctx, `
			INSERT INTO users.persons (tenant_id, account_id, first_name, last_name, created_at, updated_at)
			SELECT at.tenant_id, at.account_id, n.first_name, n.last_name, NOW(), NOW()
			FROM auth.account_tenants at
			JOIN (`+unambiguousNameFromMappedSchools+`) n ON n.account_id = at.account_id
			WHERE at.status = 'active'
			  AND NOT EXISTS (
				SELECT 1 FROM users.persons p
				WHERE p.account_id = at.account_id
				  AND p.tenant_id = at.tenant_id
				  AND p.deleted_at IS NULL
			  )
			  AND `+staffTierRoleExists("at.account_id", "at.tenant_id")+`
		`)
		if err != nil {
			return fmt.Errorf("error repairing person records: %w", err)
		}
		if affected, affErr := res.RowsAffected(); affErr == nil {
			fmt.Printf("  Created %d missing person record(s) from a name the account carries at another school\n", affected)
		}

		res, err = tx.ExecContext(ctx, `
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
			  AND `+staffTierRoleExists("p.account_id", "p.tenant_id")+`
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
			  -- Same refusal as the staff step, and needed independently of it:
			  -- this step starts from users.staff, so a staff row that legacy
			  -- data already put on a child's person would otherwise collect a
			  -- caregiver profile here even though the staff step declined to
			  -- create one. A child is never personnel (see
			  -- ErrSchoolIdentityPersonIsStudent).
			  AND NOT EXISTS (
				SELECT 1 FROM users.students st WHERE st.person_id = p.id
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
				  -- Lehrkraft carries caregiver tier for grant classification
				  -- but is class_day-read-only by design (#1772), so it never
				  -- earns a profile on its own. Excluding it HERE and not for
				  -- the account as a whole is the point: an account whose only
				  -- caregiver-tier role is Lehrkraft matches nothing and keeps
				  -- no profile, while one that also holds a real caregiver role
				  -- gets the profile that role has always required. Blanket-
				  -- skipping every account with a Lehrkraft role would leave
				  -- those dual-role accounts in exactly the half-written state
				  -- this migration exists to end — and unreported, since the
				  -- staff record does get created. RoleNeedsCaregiverProfile
				  -- decides per role too.
				  AND NOT (r.is_system AND LOWER(TRIM(r.name)) = 'lehrkraft')
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

// reportUnrepairableSchoolIdentities lists every account holding school access
// as personnel that the repair could not put right. Two causes reach it:
//
//   - No person at this school and no name at another of their schools the
//     first step could have used — either there is no other school, or those
//     schools disagree on the name. Inventing one would mean making up personal
//     data.
//   - The person carrying the account at this school is a child's record. Every
//     step above refuses it on purpose: filing a child as personnel is worse
//     than leaving the account incomplete, and it cannot be undone afterwards
//     because nothing tells the resulting rows apart from legitimate ones. Such
//     an account needs its own person, which means unlinking it from the child.
//
// The child case is reported whether or not a staff row exists, and that is not
// belt and braces. Legacy data can already carry a staff row on a child's
// person: the staff step then has nothing to add, the caregiver step still
// refuses (a child is never personnel), and asking only "is a staff record
// missing?" would let the whole account pass unmentioned — the one invalid
// identity in this file's scope, silently. A missing staff row is a gap; a
// child filed as personnel is a wrong answer, and it stays on the list until a
// human separates the two records.
//
// Printed with the reason so a human can finish the job in the staff UI, which
// links the new person to the account.
func reportUnrepairableSchoolIdentities(ctx context.Context, tx bun.Tx) error {
	rows, err := listUnrepairableSchoolIdentities(ctx, tx)
	if err != nil {
		return err
	}

	if len(rows) == 0 {
		return nil
	}

	fmt.Printf("  %d account(s) hold school access as personnel with an incomplete or invalid identity and need a human:\n", len(rows))
	for _, row := range rows {
		fmt.Printf("    school %d: account %d (%s) — %s\n", row.TenantID, row.AccountID, row.Email, row.Reason)
	}

	return nil
}

// studentLinkedIdentityExists matches an account whose live person at the
// school named by at.tenant_id is a child's record — the state every insert
// above refuses, and the one the report must name whether or not a staff row
// happens to exist. Written against the report's own aliases.
const studentLinkedIdentityExists = `
	EXISTS (
		SELECT 1
		FROM users.persons p
		JOIN users.students st ON st.person_id = p.id
		WHERE p.account_id = at.account_id
		  AND p.tenant_id = at.tenant_id
		  AND p.deleted_at IS NULL
	)`

// unrepairableSchoolIdentity is one account the repair left incomplete.
type unrepairableSchoolIdentity struct {
	AccountID int64  `bun:"account_id"`
	TenantID  int64  `bun:"tenant_id"`
	Email     string `bun:"email"`
	Reason    string `bun:"reason"`
}

// listUnrepairableSchoolIdentities is the query behind the report, split out so
// it can be asserted on directly.
func listUnrepairableSchoolIdentities(ctx context.Context, db bun.IDB) ([]unrepairableSchoolIdentity, error) {
	var rows []unrepairableSchoolIdentity

	if err := db.NewRaw(`
		SELECT at.account_id, at.tenant_id, a.email,
		       CASE WHEN `+studentLinkedIdentityExists+` THEN 'linked to a child''s person record'
		            ELSE 'no person record and no name at another school'
		       END AS reason
		FROM auth.account_tenants at
		JOIN auth.accounts a ON a.id = at.account_id
		WHERE at.status = 'active'
		  AND `+staffTierRoleExists("at.account_id", "at.tenant_id")+`
		  AND (
			NOT EXISTS (
				SELECT 1
				FROM users.persons p
				JOIN users.staff s ON s.person_id = p.id AND s.deleted_at IS NULL
				WHERE p.account_id = at.account_id
				  AND p.tenant_id = at.tenant_id
				  AND p.deleted_at IS NULL
			)
			-- A staff row on a child's person is not a repaired account, it is
			-- an invalid one. Asking only whether a staff record is missing
			-- would drop it off this list entirely (see the doc comment).
			OR `+studentLinkedIdentityExists+`
		  )
		ORDER BY at.tenant_id, at.account_id
	`).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("error listing unrepairable accounts: %w", err)
	}

	return rows, nil
}

// repairSchoolIdentitiesDown is a no-op on purpose. The rows this migration
// created are indistinguishable from the ones the fixed provisioning creates,
// and deleting staff records would take their historical references with them.
func repairSchoolIdentitiesDown(_ context.Context, _ *bun.DB) error {
	fmt.Println("Rolling back migration 1.15.284: nothing to undo (repair only, see doc comment)")
	return nil
}
