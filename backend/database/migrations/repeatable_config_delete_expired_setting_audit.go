package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("config.delete_expired_setting_audit", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatableConfigDeleteExpiredSettingAuditSQL)
		return err
	})
}

// repeatableConfigDeleteExpiredSettingAuditSQL is the current definition of
// config.delete_expired_setting_audit() (ADR 0223). It is the only way the
// tenant role removes setting changes (#6844): the changes of the calling
// school older than three years (settings.SettingAuditRetentionYears, #4044),
// on the database clock. audit.authorize_retention_tenant binds the school to
// the transaction's tenant.
const repeatableConfigDeleteExpiredSettingAuditSQL = `CREATE OR REPLACE FUNCTION config.delete_expired_setting_audit(p_tenant_id BIGINT)
	RETURNS BIGINT
	LANGUAGE plpgsql
	VOLATILE
	SECURITY DEFINER
	SET search_path = pg_catalog, config
	AS $$
	DECLARE
		deleted_count BIGINT;
	BEGIN
		PERFORM audit.authorize_retention_tenant(p_tenant_id);
		DELETE FROM config.setting_audit
		WHERE tenant_id = p_tenant_id
		  AND changed_at < clock_timestamp() - INTERVAL '3 years';
		GET DIAGNOSTICS deleted_count = ROW_COUNT;
		RETURN deleted_count;
	END;
	$$;

REVOKE ALL ON FUNCTION config.delete_expired_setting_audit(BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION config.delete_expired_setting_audit(BIGINT) TO phoenix_tenant;
`
