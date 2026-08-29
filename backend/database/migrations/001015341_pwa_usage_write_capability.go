package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	pwaUsageWriteCapabilityVersion     = "1.15.341"
	pwaUsageWriteCapabilityDescription = "Encapsulate PWA usage writes behind database capabilities (#2644)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     pwaUsageWriteCapabilityVersion,
		Description: pwaUsageWriteCapabilityDescription,
		DependsOn:   []string{pushSubscriptionsSchoolPortalVersion},
	})
	Migrations.MustRegister(pwaUsageWriteCapabilityUp, pwaUsageWriteCapabilityDown)
}

func pwaUsageWriteCapabilityUp(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		CREATE OR REPLACE FUNCTION iot.record_pwa_standalone_usage(
			p_tenant_id BIGINT,
			p_account_id BIGINT,
			p_portal TEXT
		)
		RETURNS VOID
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, iot
		AS $$
		BEGIN
			IF p_tenant_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT THEN
				RAISE EXCEPTION 'PWA usage tenant does not match transaction tenant' USING ERRCODE = '42501';
			END IF;
			INSERT INTO iot.pwa_standalone_usage (tenant_id, account_id, portal)
			VALUES (p_tenant_id, p_account_id, p_portal)
			ON CONFLICT (tenant_id, account_id, portal) DO UPDATE
			SET last_seen_at = NOW(), updated_at = NOW();
		END;
		$$;

		CREATE OR REPLACE FUNCTION iot.delete_pwa_standalone_usage_before(
			p_tenant_id BIGINT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS INTEGER
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, iot
		AS $$
		DECLARE
			deleted_count INTEGER;
		BEGIN
			IF p_tenant_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT THEN
				RAISE EXCEPTION 'PWA usage tenant does not match transaction tenant' USING ERRCODE = '42501';
			END IF;
			DELETE FROM iot.pwa_standalone_usage
			WHERE tenant_id = p_tenant_id AND last_seen_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION iot.record_pwa_standalone_usage(BIGINT, BIGINT, TEXT) FROM PUBLIC;
		REVOKE ALL ON FUNCTION iot.delete_pwa_standalone_usage_before(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		REVOKE INSERT, UPDATE, DELETE ON iot.pwa_standalone_usage FROM phoenix_tenant, phoenix_admin;
		GRANT EXECUTE ON FUNCTION iot.record_pwa_standalone_usage(BIGINT, BIGINT, TEXT) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION iot.delete_pwa_standalone_usage_before(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
	`)
	if err != nil {
		return fmt.Errorf("create PWA usage write capabilities: %w", err)
	}
	return nil
}

func pwaUsageWriteCapabilityDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
		GRANT INSERT, UPDATE, DELETE ON iot.pwa_standalone_usage TO phoenix_tenant, phoenix_admin;
		DROP FUNCTION IF EXISTS iot.delete_pwa_standalone_usage_before(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS iot.record_pwa_standalone_usage(BIGINT, BIGINT, TEXT);
	`)
	if err != nil {
		return fmt.Errorf("drop PWA usage write capabilities: %w", err)
	}
	return nil
}
