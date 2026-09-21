package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.405",
		Description: "Index demo accesses by address for the lookup of every public request (#3465)",
		DependsOn:   []string{"1.15.404"},
	})
	Migrations.MustRegister(demoAccessesEmailIndexUp, demoAccessesEmailIndexDown)
}

func demoAccessesEmailIndexUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`CREATE INDEX IF NOT EXISTS idx_demo_accesses_email_expires_at
		ON auth.demo_accesses (email, expires_at)`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("index demo accesses by address: %w", err)
	}
	return nil
}

func demoAccessesEmailIndexDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`DROP INDEX IF EXISTS auth.idx_demo_accesses_email_expires_at`).Exec(ctx)
	return err
}
