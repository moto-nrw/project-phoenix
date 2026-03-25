package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addInvitationRevocationFieldsVersion     = "1.15.19"
	addInvitationRevocationFieldsDescription = "Add revoked_at and revoked_by to auth.invitation_tokens"
)

func init() {
	MigrationRegistry[addInvitationRevocationFieldsVersion] = &Migration{
		Version:     addInvitationRevocationFieldsVersion,
		Description: addInvitationRevocationFieldsDescription,
		DependsOn:   []string{"1.15.18"},
	}

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				ALTER TABLE auth.invitation_tokens
				ADD COLUMN IF NOT EXISTS revoked_at TIMESTAMPTZ,
				ADD COLUMN IF NOT EXISTS revoked_by BIGINT,
				ADD CONSTRAINT fk_invitation_revoked_by
					FOREIGN KEY (revoked_by) REFERENCES auth.accounts(id) ON DELETE SET NULL
			`)
			if err != nil {
				return err
			}

			_, err = db.ExecContext(ctx, `
				CREATE INDEX IF NOT EXISTS idx_invitation_tokens_revoked_at
				ON auth.invitation_tokens(revoked_at)
			`)
			return err
		},
		func(ctx context.Context, db *bun.DB) error {
			_, err := db.ExecContext(ctx, `
				DROP INDEX IF EXISTS idx_invitation_tokens_revoked_at;
				ALTER TABLE auth.invitation_tokens
				DROP CONSTRAINT IF EXISTS fk_invitation_revoked_by,
				DROP COLUMN IF EXISTS revoked_by,
				DROP COLUMN IF EXISTS revoked_at
			`)
			return err
		},
	)
}
