package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	auditRetentionCapabilitiesVersion     = "1.15.352"
	auditRetentionCapabilitiesDescription = "Move audit retention deletes behind tenant-bound database capabilities (#2850)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditRetentionCapabilitiesVersion,
		Description: auditRetentionCapabilitiesDescription,
		DependsOn: []string{
			deviationEventsAuditVersion,
			studentFieldEditsVersion,
			unregisteredTagScansVersion,
		},
	})
	Migrations.MustRegister(auditRetentionCapabilitiesUp, auditRetentionCapabilitiesDown)
}

const auditRetentionCapabilitiesSQL = `
		CREATE OR REPLACE FUNCTION audit.authorize_retention_tenant(p_tenant_id BIGINT)
		RETURNS BOOLEAN
		LANGUAGE plpgsql
		STABLE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			current_tenant_id BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_tenant_id IS NULL OR p_tenant_id <= 0 THEN
				RAISE EXCEPTION 'audit retention requires a tenant' USING ERRCODE = '22023';
			END IF;

			-- app.current_tenant_id is the same transaction identity used by every
			-- tenant RLS policy. This capability narrows what that established
			-- identity may delete; it does not invent a second tenant authority.
			current_tenant_id := NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT;
			SELECT rolsuper INTO invoker_is_superuser FROM pg_roles WHERE rolname = session_user;
			IF current_tenant_id IS NULL AND NOT COALESCE(invoker_is_superuser, FALSE) THEN
				RAISE EXCEPTION 'audit retention requires a tenant transaction' USING ERRCODE = '42501';
			END IF;
			IF current_tenant_id IS NOT NULL AND p_tenant_id IS DISTINCT FROM current_tenant_id THEN
				RAISE EXCEPTION 'audit retention tenant does not match transaction tenant' USING ERRCODE = '42501';
			END IF;
			RETURN COALESCE(invoker_is_superuser, FALSE);
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_deviation_events(
			p_tenant_id BIGINT,
			p_cutoff DATE
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			retention_days INTEGER;
			latest_allowed_cutoff DATE;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'deviation event retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			SELECT COALESCE(
				(SELECT (value #>> '{}')::INTEGER
				 FROM config.setting_values
				 WHERE tenant_id = p_tenant_id
				   AND setting_key = 'gdpr.timetable_retention_days'),
				365
			) INTO retention_days;
			IF retention_days < 30 OR retention_days > 1825 THEN
				RAISE EXCEPTION 'invalid timetable retention setting' USING ERRCODE = '22023';
			END IF;

			latest_allowed_cutoff := (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::DATE - retention_days;
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'deviation event cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.deviation_events
			WHERE tenant_id = p_tenant_id
			  AND occurrence_date < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=deviation_events, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_student_field_edits(
			p_tenant_id BIGINT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			retention_days INTEGER;
			latest_allowed_cutoff TIMESTAMPTZ;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'student field edit retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			SELECT COALESCE(
				(SELECT (value #>> '{}')::INTEGER
				 FROM config.setting_values
				 WHERE tenant_id = p_tenant_id
				   AND setting_key = 'gdpr.student_change_log_retention_days'),
				90
			) INTO retention_days;
			IF retention_days < 30 OR retention_days > 365 THEN
				RAISE EXCEPTION 'invalid student change-log retention setting' USING ERRCODE = '22023';
			END IF;

			latest_allowed_cutoff := (
				((clock_timestamp() AT TIME ZONE 'Europe/Berlin')::DATE - retention_days)::TIMESTAMP
				AT TIME ZONE 'Europe/Berlin'
			);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'student field edit cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.student_field_edits
			WHERE tenant_id = p_tenant_id
			  AND created_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=student_field_edits, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_unregistered_tag_scans(
			p_tenant_id BIGINT,
			p_cutoff TIMESTAMPTZ
		)
		RETURNS BIGINT
		LANGUAGE plpgsql
		VOLATILE
		SECURITY DEFINER
		SET search_path = pg_catalog, audit
		AS $$
		DECLARE
			latest_allowed_cutoff TIMESTAMPTZ;
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'unregistered tag scan retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			latest_allowed_cutoff := clock_timestamp() - INTERVAL '90 days';
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'unregistered tag scan cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.unregistered_tag_scans
			WHERE tenant_id = p_tenant_id
			  AND scanned_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=unregistered_tag_scans, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION audit.authorize_retention_tenant(BIGINT) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_deviation_events(BIGINT, DATE) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_student_field_edits(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_unregistered_tag_scans(BIGINT, TIMESTAMPTZ) FROM PUBLIC;

		GRANT EXECUTE ON FUNCTION audit.delete_expired_deviation_events(BIGINT, DATE) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_student_field_edits(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_unregistered_tag_scans(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

		REVOKE DELETE ON audit.deviation_events FROM phoenix_tenant;
		REVOKE DELETE ON audit.student_field_edits FROM phoenix_tenant;
		REVOKE DELETE ON audit.unregistered_tag_scans FROM phoenix_tenant;
`

func auditRetentionCapabilitiesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auditRetentionCapabilitiesSQL); err != nil {
		return fmt.Errorf("create audit retention capabilities: %w", err)
	}
	return nil
}

func auditRetentionCapabilitiesDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, `
		GRANT DELETE ON audit.deviation_events TO phoenix_tenant;
		GRANT DELETE ON audit.student_field_edits TO phoenix_tenant;
		GRANT DELETE ON audit.unregistered_tag_scans TO phoenix_tenant;

		DROP FUNCTION IF EXISTS audit.delete_expired_deviation_events(BIGINT, DATE);
		DROP FUNCTION IF EXISTS audit.delete_expired_student_field_edits(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS audit.delete_expired_unregistered_tag_scans(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS audit.authorize_retention_tenant(BIGINT);
	`); err != nil {
		return fmt.Errorf("drop audit retention capabilities: %w", err)
	}
	return nil
}
