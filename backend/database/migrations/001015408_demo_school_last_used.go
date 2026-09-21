package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.408",
		Description: "Note when a demo school was last entered, so the simulation serves only schools in use (#3464)",
		DependsOn:   []string{"1.15.406"},
	})
	Migrations.MustRegister(demoSchoolLastUsedUp, demoSchoolLastUsedDown)
}

// The serving backend notes every entry into a demo school; the demo process
// ticks only schools entered in the last minutes. The column grant keeps the
// serving role's reach as narrow as before: it may stamp the time, nothing else.
func demoSchoolLastUsedUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		ALTER TABLE platform.demo_school_states ADD COLUMN IF NOT EXISTS last_used_at TIMESTAMPTZ;
		GRANT UPDATE (last_used_at) ON platform.demo_school_states TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("note demo school use: %w", err)
	}
	return nil
}

func demoSchoolLastUsedDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		REVOKE UPDATE (last_used_at) ON platform.demo_school_states FROM phoenix_admin;
		ALTER TABLE platform.demo_school_states DROP COLUMN IF EXISTS last_used_at;
	`).Exec(ctx)
	return err
}
