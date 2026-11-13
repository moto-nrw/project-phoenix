package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("platform.delete_expired_traeger_master_data_audit", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatablePlatformDeleteExpiredTraegerMasterDataAuditSQL)
		return err
	})
}

// repeatablePlatformDeleteExpiredTraegerMasterDataAuditSQL is the current definition of
// platform.delete_expired_traeger_master_data_audit() (ADR 0223). It is the only way the
// tenant role removes entries of the Träger master data log (#6844): those of the
// Träger of the calling transaction (app.current_org_id) older than three
// years, on the database clock. The Träger transaction prunes through it
// before each new entry.
const repeatablePlatformDeleteExpiredTraegerMasterDataAuditSQL = `CREATE OR REPLACE FUNCTION platform.delete_expired_traeger_master_data_audit(p_organization_id BIGINT)
	RETURNS BIGINT
	LANGUAGE plpgsql
	VOLATILE
	SECURITY DEFINER
	SET search_path = pg_catalog, platform
	AS $$
	DECLARE
		current_organization_id BIGINT;
		invoker_is_superuser BOOLEAN;
		deleted_count BIGINT;
	BEGIN
		IF p_organization_id IS NULL OR p_organization_id <= 0 THEN
			RAISE EXCEPTION 'Träger master data log retention requires a Träger' USING ERRCODE = '22023';
		END IF;
		current_organization_id := NULLIF(current_setting('app.current_org_id', true), '')::BIGINT;
		SELECT rolsuper INTO invoker_is_superuser FROM pg_roles WHERE rolname = session_user;
		IF current_organization_id IS NULL AND NOT COALESCE(invoker_is_superuser, FALSE) THEN
			RAISE EXCEPTION 'Träger master data log retention requires a Träger transaction' USING ERRCODE = '42501';
		END IF;
		IF current_organization_id IS NOT NULL AND p_organization_id IS DISTINCT FROM current_organization_id THEN
			RAISE EXCEPTION 'Träger master data log retention Träger does not match transaction Träger' USING ERRCODE = '42501';
		END IF;

		DELETE FROM platform.traeger_master_data_audit
		WHERE organization_id = p_organization_id
		  AND occurred_at < clock_timestamp() - INTERVAL '3 years';
		GET DIAGNOSTICS deleted_count = ROW_COUNT;
		RETURN deleted_count;
	END;
	$$;

REVOKE ALL ON FUNCTION platform.delete_expired_traeger_master_data_audit(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION platform.delete_expired_traeger_master_data_audit(BIGINT) TO phoenix_tenant;
`
