package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("config.delete_expired_traeger_setting_audit", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatableConfigDeleteExpiredTraegerSettingAuditSQL)
		return err
	})
}

// repeatableConfigDeleteExpiredTraegerSettingAuditSQL is the current definition of
// config.delete_expired_traeger_setting_audit() (ADR 0223). It is the only way the
// tenant role removes entries of the Träger setting changes (#6844): those of the
// Träger of the calling transaction (app.current_org_id) older than three
// years, on the database clock. The Träger transaction prunes through it
// before each new entry.
const repeatableConfigDeleteExpiredTraegerSettingAuditSQL = `CREATE OR REPLACE FUNCTION config.delete_expired_traeger_setting_audit(p_organization_id BIGINT)
	RETURNS BIGINT
	LANGUAGE plpgsql
	VOLATILE
	SECURITY DEFINER
	SET search_path = pg_catalog, config
	AS $$
	DECLARE
		current_organization_id BIGINT;
		invoker_is_superuser BOOLEAN;
		deleted_count BIGINT;
	BEGIN
		IF p_organization_id IS NULL OR p_organization_id <= 0 THEN
			RAISE EXCEPTION 'Träger setting changes retention requires a Träger' USING ERRCODE = '22023';
		END IF;
		current_organization_id := NULLIF(current_setting('app.current_org_id', true), '')::BIGINT;
		SELECT rolsuper INTO invoker_is_superuser FROM pg_roles WHERE rolname = session_user;
		IF current_organization_id IS NULL AND NOT COALESCE(invoker_is_superuser, FALSE) THEN
			RAISE EXCEPTION 'Träger setting changes retention requires a Träger transaction' USING ERRCODE = '42501';
		END IF;
		IF current_organization_id IS NOT NULL AND p_organization_id IS DISTINCT FROM current_organization_id THEN
			RAISE EXCEPTION 'Träger setting changes retention Träger does not match transaction Träger' USING ERRCODE = '42501';
		END IF;

		DELETE FROM config.traeger_setting_audit
		WHERE organization_id = p_organization_id
		  AND changed_at < clock_timestamp() - INTERVAL '3 years';
		GET DIAGNOSTICS deleted_count = ROW_COUNT;
		RETURN deleted_count;
	END;
	$$;

REVOKE ALL ON FUNCTION config.delete_expired_traeger_setting_audit(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION config.delete_expired_traeger_setting_audit(BIGINT) TO phoenix_tenant;
`
