package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func restoreDirectPermissionsStorage(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, directPermissionsTableSQL+directPermissionsLegacyChecksSQL)
	return err
}

// The rollback recreates storage, constraints, fences and access control,
// never the deleted grants. It is not a data-recovery mechanism.
const directPermissionsTableSQL = `
CREATE TABLE auth.account_permissions (
 id BIGSERIAL PRIMARY KEY,
 created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
 account_id BIGINT NOT NULL CONSTRAINT fk_account_permissions_account REFERENCES auth.accounts(id) ON DELETE CASCADE,
 permission_id BIGINT NOT NULL CONSTRAINT fk_account_permissions_permission REFERENCES auth.permissions(id) ON DELETE CASCADE,
 granted BOOLEAN NOT NULL DEFAULT TRUE,
 tenant_id BIGINT NOT NULL DEFAULT 1 REFERENCES platform.schools(id),
 CONSTRAINT chk_account_permissions_tenant_id_not_null CHECK (tenant_id IS NOT NULL)
);
CREATE UNIQUE INDEX idx_account_permissions_tenant ON auth.account_permissions(account_id, permission_id, tenant_id);
CREATE INDEX idx_account_permissions_account_id ON auth.account_permissions(account_id);
CREATE INDEX idx_account_permissions_permission_id ON auth.account_permissions(permission_id);
CREATE INDEX idx_account_permissions_granted ON auth.account_permissions(granted);
CREATE INDEX idx_account_permissions_tenant_lead ON auth.account_permissions(tenant_id);
CREATE TRIGGER update_account_permissions_updated_at BEFORE UPDATE ON auth.account_permissions
 FOR EACH ROW EXECUTE FUNCTION update_modified_column();
CREATE TRIGGER lock_live_account_reference BEFORE INSERT OR UPDATE ON auth.account_permissions
 FOR EACH ROW EXECUTE FUNCTION auth.lock_live_account_reference('account_id');
GRANT SELECT ON auth.account_permissions TO phoenix_auth;
GRANT SELECT, INSERT, UPDATE, DELETE ON auth.account_permissions TO phoenix_tenant;
GRANT ALL ON auth.account_permissions TO phoenix_admin;
GRANT USAGE ON SEQUENCE auth.account_permissions_id_seq TO phoenix_tenant;
GRANT ALL ON SEQUENCE auth.account_permissions_id_seq TO phoenix_admin;
GRANT USAGE ON SEQUENCE auth.account_permissions_id_seq TO phoenix_auth;
`

// Unused legacy permission helpers installed by 1.0.8, restored only for a
// schema rollback. Up drops them rather than retaining broken table reads.
const directPermissionsLegacyChecksSQL = `
		-- Create a function to check if an account has a specific permission
		CREATE OR REPLACE FUNCTION auth.has_permission(account_id BIGINT, permission_name TEXT)
		RETURNS BOOLEAN AS $$
		DECLARE
			permission_granted BOOLEAN;
		BEGIN
			-- First check for direct denials (these override everything else)
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.name = $2
				AND ap.granted = FALSE
			) INTO permission_granted;

			IF permission_granted THEN
				RETURN FALSE;
			END IF;

			-- Then check for direct grants
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.name = $2
				AND ap.granted = TRUE
			) INTO permission_granted;

			IF permission_granted THEN
				RETURN TRUE;
			END IF;

			-- Finally check role-based permissions
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.role_permissions rp ON ar.role_id = rp.role_id
				JOIN auth.permissions p ON rp.permission_id = p.id
				WHERE ar.account_id = $1
				AND p.name = $2
			) INTO permission_granted;

			RETURN permission_granted;
		END;
		$$ LANGUAGE plpgsql;

		-- Create a function to check if an account has access to a resource
		CREATE OR REPLACE FUNCTION auth.has_resource_permission(account_id BIGINT, resource TEXT, action TEXT)
		RETURNS BOOLEAN AS $$
		DECLARE
			permission_granted BOOLEAN;
		BEGIN
			-- First check for direct denials (these override everything else)
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.resource = $2
				AND p.action = $3
				AND ap.granted = FALSE
			) INTO permission_granted;

			IF permission_granted THEN
				RETURN FALSE;
			END IF;

			-- Then check for direct grants
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_permissions ap
				JOIN auth.permissions p ON ap.permission_id = p.id
				WHERE ap.account_id = $1
				AND p.resource = $2
				AND p.action = $3
				AND ap.granted = TRUE
			) INTO permission_granted;

			IF permission_granted THEN
				RETURN TRUE;
			END IF;

			-- Finally check role-based permissions
			SELECT EXISTS (
				SELECT 1
				FROM auth.account_roles ar
				JOIN auth.role_permissions rp ON ar.role_id = rp.role_id
				JOIN auth.permissions p ON rp.permission_id = p.id
				WHERE ar.account_id = $1
				AND p.resource = $2
				AND p.action = $3
			) INTO permission_granted;

			RETURN permission_granted;
		END;
		$$ LANGUAGE plpgsql;`
