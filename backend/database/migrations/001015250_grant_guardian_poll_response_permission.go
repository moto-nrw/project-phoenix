package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	grantGuardianPollResponsePermissionVersion     = "1.15.250"
	grantGuardianPollResponsePermissionDescription = "Grant parent_portal.poll.response to full guardian relationships"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     grantGuardianPollResponsePermissionVersion,
		Description: grantGuardianPollResponsePermissionDescription,
		DependsOn:   []string{parentAnnouncementPollsVersion},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Migration 1.15.250: Granting parent_portal.poll.response to full guardians...")
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = COALESCE(permissions, '{}'::jsonb)
					|| '{"parent_portal.poll.response": true}'::jsonb
				WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			`).Exec(ctx)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			fmt.Println("Rolling back migration 1.15.250: removing parent_portal.poll.response key...")
			_, err := db.NewRaw(`
				UPDATE users.students_guardians
				SET permissions = permissions - 'parent_portal.poll.response'
				WHERE guardian_role IN ('primary_guardian', 'legal_guardian', 'co_guardian')
			`).Exec(ctx)
			return err
		},
	)
}
