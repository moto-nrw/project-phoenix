package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.412",
		Description: "Let the demo process expire demo accesses and hide their demo schools (#3470)",
		DependsOn:   []string{"1.15.411"},
	})
	Migrations.MustRegister(demoExpiryUp, demoExpiryDown)
}

// The demo process deletes demo accesses 14 days after their last use and
// soft-deletes the demo schools nobody can enter any more. Its role sees only
// the columns that decide this: never an address or a name of a prospect,
// never a school's data. The serving role hides a school when a visitor
// starts over; it already holds every right on platform.schools.
func demoExpiryUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		GRANT USAGE ON SCHEMA auth TO phoenix_demo;
		GRANT SELECT (id, school_slug, expires_at), DELETE ON auth.demo_accesses TO phoenix_demo;
		GRANT SELECT (id, deleted_at), UPDATE (deleted_at) ON platform.schools TO phoenix_demo;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("grant demo expiry: %w", err)
	}
	return nil
}

func demoExpiryDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		REVOKE SELECT (id, deleted_at), UPDATE (deleted_at) ON platform.schools FROM phoenix_demo;
		REVOKE SELECT (id, school_slug, expires_at), DELETE ON auth.demo_accesses FROM phoenix_demo;
		REVOKE USAGE ON SCHEMA auth FROM phoenix_demo;
	`).Exec(ctx)
	return err
}
