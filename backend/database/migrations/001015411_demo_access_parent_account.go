package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.411",
		Description: "Note the parent account a demo access signs in for the demo role parent (#3468)",
		DependsOn:   []string{"1.15.410"},
	})
	Migrations.MustRegister(demoAccessParentAccountUp, demoAccessParentAccountDown)
}

// The access keeps naming the caregiver in account_id; the parent the role
// parent signs in gets its own column, so the session cap and the tenant lock
// of the public demo know that account too (#3462).
func demoAccessParentAccountUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE auth.demo_accesses ADD COLUMN IF NOT EXISTS parent_account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL;
		CREATE INDEX IF NOT EXISTS idx_demo_accesses_parent_account_id ON auth.demo_accesses (parent_account_id) WHERE parent_account_id IS NOT NULL;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("note demo access parent account: %w", err)
	}
	return nil
}

func demoAccessParentAccountDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		DROP INDEX IF EXISTS auth.idx_demo_accesses_parent_account_id;
		ALTER TABLE auth.demo_accesses DROP COLUMN IF EXISTS parent_account_id;
	`).Exec(ctx)
	return err
}
