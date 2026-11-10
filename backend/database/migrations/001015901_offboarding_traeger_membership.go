package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const offboardingTraegerMembershipVersion = "1.15.901"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     offboardingTraegerMembershipVersion,
		Description: "Preserve Träger access when the last school offboards an account (#5974)",
		DependsOn:   []string{traegerPortalVersion, authTenantRLSVersion},
	})
	Migrations.MustRegister(offboardingTraegerMembershipUp, offboardingTraegerMembershipDown)
}

// The tenant receives only the retention fact for an account active at its
// own school. No organization, role or foreign school details leave the
// function. Pending/active membership retains the account independently of
// the organization's lifecycle, matching auth.account_is_unused.
const offboardingTraegerMembershipUpSQL = `
CREATE OR REPLACE FUNCTION platform.account_has_retained_traeger_membership(p_account_id BIGINT)
RETURNS BOOLEAN
LANGUAGE SQL STABLE SECURITY DEFINER
SET search_path = pg_catalog, platform, auth
AS $function$
	SELECT EXISTS (
		SELECT 1 FROM auth.account_tenants AS own
		WHERE own.account_id = p_account_id AND own.status = 'active'
			AND own.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::bigint
	) AND EXISTS (
		SELECT 1 FROM platform.organization_accounts AS membership
		WHERE membership.account_id = p_account_id
			AND membership.status IN ('pending', 'active')
	)
$function$;
REVOKE ALL ON FUNCTION platform.account_has_retained_traeger_membership(BIGINT)
	FROM PUBLIC, phoenix_auth, phoenix_admin;
GRANT EXECUTE ON FUNCTION platform.account_has_retained_traeger_membership(BIGINT) TO phoenix_tenant;
`

func offboardingTraegerMembershipUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, offboardingTraegerMembershipUpSQL); err != nil {
		return fmt.Errorf("create tenant Träger retention capability: %w", err)
	}
	return nil
}

func offboardingTraegerMembershipDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `DROP FUNCTION IF EXISTS platform.account_has_retained_traeger_membership(BIGINT)`); err != nil {
		return fmt.Errorf("drop tenant Träger retention capability: %w", err)
	}
	return nil
}
