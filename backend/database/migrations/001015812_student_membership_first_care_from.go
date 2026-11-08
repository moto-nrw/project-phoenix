package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentMembershipFirstCareFromVersion = "1.15.812"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentMembershipFirstCareFromVersion,
		Description: "Mitgliedschaft merkt sich den frühesten möglichen Betreuungstag eines neuen Kindes (#6041)",
		DependsOn: []string{
			demoVisitorErasureContractPushVersion, // the last migration before this one
			studentOwnerCutoverVersion,            // the membership table and its updated_at trigger
		},
	})
	Migrations.MustRegister(studentMembershipFirstCareFromUp, studentMembershipFirstCareFromDown)
}

// studentMembershipFirstCareFromUpSQL adds School Membership's first-care
// bound (#6041, ADR 0166): the earliest day a child can have been cared for at
// this school, NULL when unknown. School Membership's writes keep it on the
// application's day; nothing else writes it.
//
// The backfill sets it only where it is provable: a pending row with a start,
// unchanged since its insert (updated_at equal to created_at; the updated_at
// trigger bumps every later statement, the Cutover copy kept the source
// timestamps), and no other membership of the child. Such a row was inserted
// as a new child waiting for that start. Every other row stays unknown and
// keeps Care Plan's earlier behavior. The backfill keeps updated_at.
const studentMembershipFirstCareFromUpSQL = `
ALTER TABLE users.student_school_memberships ADD COLUMN IF NOT EXISTS first_care_from DATE;
COMMENT ON COLUMN users.student_school_memberships.first_care_from IS
	'Earliest day the child can have been cared for at this school; NULL when unknown (#6041, ADR 0166). Written only by School Membership.';

ALTER TABLE users.student_school_memberships DISABLE TRIGGER update_student_school_memberships_updated_at;
UPDATE users.student_school_memberships AS m SET first_care_from = m.enrolled_from
WHERE m.deleted_at IS NULL AND m.status = 'pending' AND m.enrolled_from IS NOT NULL
	AND m.first_care_from IS NULL AND m.updated_at = m.created_at
	AND NOT EXISTS (
		SELECT 1 FROM users.student_school_memberships AS other
		WHERE other.tenant_id = m.tenant_id AND other.student_profile_id = m.student_profile_id AND other.id <> m.id
	);
ALTER TABLE users.student_school_memberships ENABLE TRIGGER update_student_school_memberships_updated_at;
`

const studentMembershipFirstCareFromDownSQL = `
ALTER TABLE users.student_school_memberships DROP COLUMN IF EXISTS first_care_from;
`

func studentMembershipFirstCareFromUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, studentMembershipFirstCareFromUpSQL); err != nil {
			return fmt.Errorf("add the first-care bound of student memberships: %w", err)
		}
		return nil
	})
}

func studentMembershipFirstCareFromDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentMembershipFirstCareFromDownSQL); err != nil {
		return fmt.Errorf("drop the first-care bound of student memberships: %w", err)
	}
	return nil
}
