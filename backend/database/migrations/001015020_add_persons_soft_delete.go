package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

const (
	addPersonsSoftDeleteVersion     = "1.15.20"
	addPersonsSoftDeleteDescription = "Add deleted_at column to users.persons for soft delete support"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     addPersonsSoftDeleteVersion,
		Description: addPersonsSoftDeleteDescription,
		DependsOn:   []string{"1.15.19"},
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()

			// Add deleted_at column for BUN soft delete
			_, err = tx.ExecContext(ctx, `
				ALTER TABLE users.persons
				ADD COLUMN deleted_at TIMESTAMPTZ
			`)
			if err != nil {
				return err
			}

			// Replace tenant-scoped unique indexes with partial unique indexes
			// that exclude soft-deleted rows. This allows re-use of tag_id and
			// account_id after a person is soft-deleted.
			_, err = tx.ExecContext(ctx, `
				DROP INDEX IF EXISTS users.idx_persons_tenant_tag;
				CREATE UNIQUE INDEX idx_persons_tenant_tag
					ON users.persons(tenant_id, tag_id)
					WHERE deleted_at IS NULL;

				DROP INDEX IF EXISTS users.idx_persons_tenant_account;
				CREATE UNIQUE INDEX idx_persons_tenant_account
					ON users.persons(tenant_id, account_id)
					WHERE deleted_at IS NULL;
			`)
			if err != nil {
				return err
			}

			return tx.Commit()
		},
		func(ctx context.Context, db *bun.DB) error {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return err
			}
			defer func() { _ = tx.Rollback() }()

			// Restore original tenant-scoped unique indexes (without partial filter)
			_, err = tx.ExecContext(ctx, `
				DROP INDEX IF EXISTS users.idx_persons_tenant_tag;
				CREATE UNIQUE INDEX idx_persons_tenant_tag
					ON users.persons(tenant_id, tag_id);

				DROP INDEX IF EXISTS users.idx_persons_tenant_account;
				CREATE UNIQUE INDEX idx_persons_tenant_account
					ON users.persons(tenant_id, account_id);
			`)
			if err != nil {
				return err
			}

			_, err = tx.ExecContext(ctx, `
				ALTER TABLE users.persons
				DROP COLUMN IF EXISTS deleted_at
			`)
			if err != nil {
				return err
			}

			return tx.Commit()
		},
	)
}
