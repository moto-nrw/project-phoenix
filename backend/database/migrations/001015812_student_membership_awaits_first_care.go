package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentMembershipAwaitsFirstCareVersion = "1.15.812"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentMembershipAwaitsFirstCareVersion,
		Description: "Mitgliedschaft merkt sich, ob ein vorgemerktes Kind noch nie betreut wurde (#6041)",
		DependsOn: []string{
			traegerDocumentsRoleVersion, // the last migration before this one
			studentOwnerCutoverVersion,  // the membership table and its updated_at trigger
		},
	})
	Migrations.MustRegister(studentMembershipAwaitsFirstCareUp, studentMembershipAwaitsFirstCareDown)
}

// studentMembershipAwaitsFirstCareUpSQL stores School Membership's fact that a
// membership still waits for its first care (#6041). A scheduled approval
// inserts a new child pending with its start; a resumption or a renewal
// replaces the interval of a known child the same way. Only the insert knows
// the difference, so the trigger records it there and every later write can
// only clear it:
//
//   - set on insert of a pending row with a start, when the child has no
//     other (also soft-deleted) membership at this school;
//   - kept while the row stays pending with a start, and the start moves
//     earlier or moves while it still lies ahead (Berlin day): before an
//     unreached start the child cannot have been cared for;
//   - cleared for good otherwise (activation, end, resumption, a reached
//     start moved later, any status but pending).
//
// The trigger owns the column: writers cannot set it, and no path forgets it.
//
// The backfill marks only rows that provably never changed since their
// insert: pending with a start, updated_at equal to created_at (the
// updated_at trigger bumps every later statement; the Cutover copy kept the
// source timestamps) and no other membership of the child. Every other row
// stays unknown and keeps Care Plan's protective today boundary.
const studentMembershipAwaitsFirstCareUpSQL = `
ALTER TABLE users.student_school_memberships
	ADD COLUMN IF NOT EXISTS awaits_first_care BOOLEAN NOT NULL DEFAULT false;
COMMENT ON COLUMN users.student_school_memberships.awaits_first_care IS
	'True while the membership was inserted pending with its start and never had care since (#6041). Maintained only by trg_student_school_memberships_first_care.';

ALTER TABLE users.student_school_memberships DISABLE TRIGGER update_student_school_memberships_updated_at;
UPDATE users.student_school_memberships AS m SET awaits_first_care = true
WHERE m.deleted_at IS NULL AND m.status = 'pending' AND m.enrolled_from IS NOT NULL
	AND m.updated_at = m.created_at
	AND NOT EXISTS (
		SELECT 1 FROM users.student_school_memberships AS other
		WHERE other.tenant_id = m.tenant_id AND other.student_profile_id = m.student_profile_id AND other.id <> m.id
	);
ALTER TABLE users.student_school_memberships ENABLE TRIGGER update_student_school_memberships_updated_at;

CREATE OR REPLACE FUNCTION users.student_school_membership_first_care() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
	IF TG_OP = 'INSERT' THEN
		NEW.awaits_first_care := NEW.status = 'pending' AND NEW.enrolled_from IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM users.student_school_memberships AS other
				WHERE other.tenant_id = NEW.tenant_id AND other.student_profile_id = NEW.student_profile_id
			);
	ELSE
		NEW.awaits_first_care := OLD.awaits_first_care AND NEW.status = 'pending'
			AND NEW.enrolled_from IS NOT NULL AND OLD.enrolled_from IS NOT NULL
			AND (NEW.enrolled_from <= OLD.enrolled_from
				OR OLD.enrolled_from > (now() AT TIME ZONE 'Europe/Berlin')::date);
	END IF;
	RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS trg_student_school_memberships_first_care ON users.student_school_memberships;
CREATE TRIGGER trg_student_school_memberships_first_care
	BEFORE INSERT OR UPDATE ON users.student_school_memberships
	FOR EACH ROW EXECUTE FUNCTION users.student_school_membership_first_care();
`

const studentMembershipAwaitsFirstCareDownSQL = `
DROP TRIGGER IF EXISTS trg_student_school_memberships_first_care ON users.student_school_memberships;
DROP FUNCTION IF EXISTS users.student_school_membership_first_care();
ALTER TABLE users.student_school_memberships DROP COLUMN IF EXISTS awaits_first_care;
`

func studentMembershipAwaitsFirstCareUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentMembershipAwaitsFirstCareUpSQL); err != nil {
			return fmt.Errorf("record whether a student membership awaits its first care: %w", err)
		}
		return nil
	})
}

func studentMembershipAwaitsFirstCareDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentMembershipAwaitsFirstCareDownSQL); err != nil {
		return fmt.Errorf("drop the first-care fact of student memberships: %w", err)
	}
	return nil
}
