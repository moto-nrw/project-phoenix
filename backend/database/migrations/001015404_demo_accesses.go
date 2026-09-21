package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     "1.15.404",
		Description: "Store demo accesses of the public demo by token fingerprint (#3462)",
		DependsOn:   []string{"1.14.1", "1.15.403"},
	})
	Migrations.MustRegister(demoAccessesUp, demoAccessesDown)
}

// The table holds contact data of prospects. Only the administrative role
// reaches it; the demo flows open an administrative transaction.
func demoAccessesUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
		CREATE TABLE auth.demo_accesses (
			id BIGSERIAL PRIMARY KEY,
			email TEXT NOT NULL,
			person_name TEXT NOT NULL,
			school_name TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT '',
			contact_opt_in BOOLEAN NOT NULL DEFAULT FALSE,
			token_hash TEXT NOT NULL UNIQUE,
			account_id BIGINT REFERENCES auth.accounts(id) ON DELETE SET NULL,
			use_count INTEGER NOT NULL DEFAULT 0,
			last_used_at TIMESTAMPTZ,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX idx_demo_accesses_account_id ON auth.demo_accesses (account_id) WHERE account_id IS NOT NULL;
		REVOKE ALL ON auth.demo_accesses FROM PUBLIC, phoenix_auth, phoenix_tenant;
		GRANT SELECT, INSERT, UPDATE, DELETE ON auth.demo_accesses TO phoenix_admin;
		GRANT USAGE ON SEQUENCE auth.demo_accesses_id_seq TO phoenix_admin;
	`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create demo accesses: %w", err)
	}
	return nil
}

func demoAccessesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`DROP TABLE IF EXISTS auth.demo_accesses`).Exec(ctx)
	return err
}
