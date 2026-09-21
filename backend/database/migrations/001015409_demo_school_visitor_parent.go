package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.409",
		Description: "Name the visitor's parent account of a demo school for the demo role parent (#3468)",
		DependsOn:   []string{"1.15.408"},
	})
	Migrations.MustRegister(demoSchoolVisitorParentUp, demoSchoolVisitorParentDown)
}

// The demo process names the parent who carries the visitor's name next to
// the caregiver; the serving backend signs the visitor in as that parent for
// the demo role parent. It may read the column, not write it.
func demoSchoolVisitorParentUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.demo_school_states ADD COLUMN IF NOT EXISTS visitor_parent_account_id BIGINT;
		GRANT SELECT (visitor_parent_account_id) ON platform.demo_school_states TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("name demo school visitor parent: %w", err)
	}
	return nil
}

func demoSchoolVisitorParentDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		REVOKE SELECT (visitor_parent_account_id) ON platform.demo_school_states FROM phoenix_admin;
		ALTER TABLE platform.demo_school_states DROP COLUMN IF EXISTS visitor_parent_account_id;
	`).Exec(ctx)
	return err
}
