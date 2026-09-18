package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	createAccountTenantsVersion     = "1.13.2"
	createAccountTenantsDescription = "Create auth.account_tenants table for multi-tenancy account-school mapping"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     createAccountTenantsVersion,
		Description: createAccountTenantsDescription,
		DependsOn:   []string{"1.13.1", "1.0.1"}, // Depends on platform.schools and auth.accounts
	})

	Migrations.MustRegister(
		func(ctx context.Context, db *bun.DB) error {
			return createAccountTenants(ctx, db)
		},
		func(ctx context.Context, db *bun.DB) error {
			return rollbackAccountTenants(ctx, db)
		},
	)
}

func createAccountTenants(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	// Create account_tenants table
	_, err = tx.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS auth.account_tenants (
			id              BIGSERIAL PRIMARY KEY,
			account_id      BIGINT NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
			tenant_id       BIGINT NOT NULL REFERENCES platform.schools(id) ON DELETE CASCADE,
			status          TEXT NOT NULL DEFAULT 'active'
			                CHECK (status IN ('pending', 'active', 'inactive')),
			invited_at      TIMESTAMPTZ DEFAULT NOW(),
			activated_at    TIMESTAMPTZ,
			deactivated_at  TIMESTAMPTZ,
			created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE(account_id, tenant_id)
		);
	`)
	if err != nil {
		return fmt.Errorf("error creating auth.account_tenants table: %w", err)
	}

	// Create indexes
	_, err = tx.ExecContext(ctx, `
		CREATE INDEX IF NOT EXISTS idx_account_tenants_account ON auth.account_tenants(account_id);
		CREATE INDEX IF NOT EXISTS idx_account_tenants_tenant ON auth.account_tenants(tenant_id);
		CREATE INDEX IF NOT EXISTS idx_account_tenants_active ON auth.account_tenants(account_id, tenant_id)
			WHERE status = 'active';
	`)
	if err != nil {
		return fmt.Errorf("error creating indexes for auth.account_tenants: %w", err)
	}

	// Create updated_at trigger
	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_auth_account_tenants_updated_at ON auth.account_tenants;
		CREATE TRIGGER update_auth_account_tenants_updated_at
		BEFORE UPDATE ON auth.account_tenants
		FOR EACH ROW
		EXECUTE FUNCTION update_modified_column();
	`)
	if err != nil {
		return fmt.Errorf("error creating updated_at trigger for auth.account_tenants: %w", err)
	}

	return tx.Commit()
}

func rollbackAccountTenants(ctx context.Context, db *bun.DB) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err := tx.Rollback(); err != nil && err.Error() != "sql: transaction has already been committed or rolled back" {
			logRollbackFailure(ctx, err)
		}
	}()

	_, err = tx.ExecContext(ctx, `
		DROP TRIGGER IF EXISTS update_auth_account_tenants_updated_at ON auth.account_tenants;
		DROP TABLE IF EXISTS auth.account_tenants CASCADE;
	`)
	if err != nil {
		return fmt.Errorf("error dropping auth.account_tenants table: %w", err)
	}

	return tx.Commit()
}
