package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const diagnosticsReaderVersion = "1.15.488"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     diagnosticsReaderVersion,
		Description: "Add a tenant-scoped diagnostics reader without financial, health or audit access (#3840)",
		DependsOn:   []string{enableRLSVersion, schoolErasureVersion},
	})
	Migrations.MustRegister(diagnosticsReaderUp, diagnosticsReaderDown)
}

// Explicit column grants also exclude future columns, free text and names.
// No membership in runtime roles, no default grants, no RLS bypass. Existing
// PUBLIC tenant policies apply; the operator selects one school per transaction.
const diagnosticsReaderUpSQL = `
DO $$ BEGIN
    IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_diagnostics') THEN
        CREATE ROLE phoenix_diagnostics NOLOGIN NOINHERIT NOSUPERUSER
            NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS;
    END IF;
    -- Roles are cluster-wide (including local migration templates). Reuse only
    -- the expected unprivileged group, never silently adopt an elevated role.
    IF EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_diagnostics'
        AND (rolcanlogin OR rolinherit OR rolsuper OR rolcreatedb OR rolcreaterole
             OR rolreplication OR rolbypassrls))
       OR EXISTS (SELECT FROM pg_auth_members
                  WHERE member = 'phoenix_diagnostics'::regrole) THEN
        RAISE EXCEPTION 'phoenix_diagnostics must be an unprivileged NOLOGIN group without role memberships';
    END IF;
END $$;
-- 1.15.481 owns the MFA PUBLIC revocations and runtime replacement grants.
-- Leave them untouched in both directions, including diagnostics rollback.
GRANT USAGE ON SCHEMA facilities, education, activities TO phoenix_diagnostics;
GRANT SELECT (id, tenant_id, created_at, updated_at)
    ON facilities.rooms, education.groups, activities.groups TO phoenix_diagnostics;
`

// Revoke column ACLs explicitly: a table-level REVOKE does not remove them.
// DROP ROLE fails closed on unexpected grants/ownership, including dependencies
// in another database; never DROP OWNED or remove someone else's objects.
const diagnosticsReaderDownSQL = `
REVOKE SELECT (id, tenant_id, created_at, updated_at)
    ON facilities.rooms, education.groups, activities.groups FROM phoenix_diagnostics;
REVOKE USAGE ON SCHEMA facilities, education, activities FROM phoenix_diagnostics;
DROP ROLE phoenix_diagnostics;
`

func diagnosticsReaderUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, diagnosticsReaderUpSQL); err != nil {
			return fmt.Errorf("create diagnostics reader: %w", err)
		}
		return nil
	})
}

func diagnosticsReaderDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, diagnosticsReaderDownSQL); err != nil {
			return fmt.Errorf("remove diagnostics reader: %w", err)
		}
		return nil
	})
}
