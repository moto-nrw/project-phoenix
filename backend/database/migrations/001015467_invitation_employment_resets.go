package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.467", Description: "Store optional employment resets on invitations (#3406)", DependsOn: []string{"1.15.466"}})
	Migrations.MustRegister(invitationEmploymentResetsUp, invitationEmploymentResetsDown)
}

func invitationEmploymentResetsUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.invitation_tokens
 ADD COLUMN reset_master_data BOOLEAN NOT NULL DEFAULT FALSE,
 ADD COLUMN reset_qualifications BOOLEAN NOT NULL DEFAULT FALSE,
 ADD COLUMN reset_work_time BOOLEAN NOT NULL DEFAULT FALSE,
 ADD CONSTRAINT invitation_resets_require_resume CHECK (employment_action = 'resume' OR NOT (reset_master_data OR reset_qualifications OR reset_work_time))`)
	if err != nil {
		return fmt.Errorf("add invitation employment resets: %w", err)
	}
	return nil
}
func invitationEmploymentResetsDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `ALTER TABLE auth.invitation_tokens DROP CONSTRAINT invitation_resets_require_resume, DROP COLUMN reset_master_data, DROP COLUMN reset_qualifications, DROP COLUMN reset_work_time`)
	if err != nil {
		return fmt.Errorf("drop invitation employment resets: %w", err)
	}
	return nil
}
