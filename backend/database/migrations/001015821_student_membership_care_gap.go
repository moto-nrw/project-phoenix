package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const studentMembershipCareGapVersion = "1.15.821"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     studentMembershipCareGapVersion,
		Description: "Mitgliedschaft merkt sich die Lücke zwischen früherem Ende und Wiederaufnahme (#6073)",
		DependsOn: []string{
			traegerWithdrawalContactVersion,       // the last migration before this one
			studentMembershipFirstCareFromVersion, // the first-care bound this one mirrors
		},
	})
	Migrations.MustRegister(studentMembershipCareGapUp, studentMembershipCareGapDown)
}

// studentMembershipCareGapUpSQL adds School Membership's care gap of a
// returning child (#6073, ADR 0177): care_paused_after is the last care day
// before a restart, care_resumed_from the earliest day the child can have
// been cared for again. Days strictly between them are no care days. Both
// NULL means no known gap. School Membership's writes keep them on the
// application's day; nothing else writes them.
//
// No backfill: the earlier end of a past restart is stored nowhere durable,
// so existing rows keep Care Plan's earlier behavior.
const studentMembershipCareGapUpSQL = `
ALTER TABLE users.student_school_memberships
	ADD COLUMN IF NOT EXISTS care_paused_after DATE,
	ADD COLUMN IF NOT EXISTS care_resumed_from DATE;
COMMENT ON COLUMN users.student_school_memberships.care_paused_after IS
	'Last care day before the latest restart of care; NULL when no gap is known (#6073, ADR 0177). Written only by School Membership.';
COMMENT ON COLUMN users.student_school_memberships.care_resumed_from IS
	'Earliest day the child can have been cared for again after care_paused_after; NULL when no gap is known (#6073, ADR 0177). Written only by School Membership.';
`

const studentMembershipCareGapDownSQL = `
ALTER TABLE users.student_school_memberships
	DROP COLUMN IF EXISTS care_resumed_from,
	DROP COLUMN IF EXISTS care_paused_after;
`

func studentMembershipCareGapUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentMembershipCareGapUpSQL); err != nil {
		return fmt.Errorf("add the care gap of student memberships: %w", err)
	}
	return nil
}

func studentMembershipCareGapDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, studentMembershipCareGapDownSQL); err != nil {
		return fmt.Errorf("drop the care gap of student memberships: %w", err)
	}
	return nil
}
