package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const accountTraegerParentAccessFactVersion = "1.15.960"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     accountTraegerParentAccessFactVersion,
		Description: "Schul-Admin erfährt, ob ein Konto der eigenen Schule auch Eltern-Zugang bei einem Träger hat (#6862)",
		DependsOn: []string{
			holidayRegistrationStatusTokenVersion, // the newest migration on development (1.15.959)
			traegerParentAccessVersion,            // auth.traeger_parent_access
			authTenantRLSVersion,                  // the school scope the function reads
		},
	})
	Migrations.MustRegister(accountTraegerParentAccessFactUp, accountTraegerParentAccessFactDown)
}

// A school administrator changes an account's login address and second
// factor only while the school alone holds the account (#6862). The
// parents-portal access at a Träger (ADR 0216) is a further holder, and its
// table carries no tenant. The school receives only the fact, and only for
// an account active at its own school; no Träger leaves the function.
const accountTraegerParentAccessFactUpSQL = `
CREATE OR REPLACE FUNCTION auth.account_has_traeger_parent_access(p_account_id BIGINT)
RETURNS BOOLEAN
LANGUAGE SQL STABLE SECURITY DEFINER
SET search_path = pg_catalog, auth
AS $function$
	SELECT EXISTS (
		SELECT 1 FROM auth.account_tenants AS own
		WHERE own.account_id = p_account_id AND own.status = 'active'
			AND own.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
	) AND EXISTS (
		SELECT 1 FROM auth.traeger_parent_access AS access
		WHERE access.account_id = p_account_id
	)
$function$;
REVOKE ALL ON FUNCTION auth.account_has_traeger_parent_access(BIGINT)
	FROM PUBLIC, phoenix_auth, phoenix_admin;
GRANT EXECUTE ON FUNCTION auth.account_has_traeger_parent_access(BIGINT) TO phoenix_tenant;
`

func accountTraegerParentAccessFactUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, accountTraegerParentAccessFactUpSQL); err != nil {
		return fmt.Errorf("create tenant Träger parent access fact: %w", err)
	}
	return nil
}

func accountTraegerParentAccessFactDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS auth.account_has_traeger_parent_access(BIGINT)`); err != nil {
		return fmt.Errorf("drop tenant Träger parent access fact: %w", err)
	}
	return nil
}
