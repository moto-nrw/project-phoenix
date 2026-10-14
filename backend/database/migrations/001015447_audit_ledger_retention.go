package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const (
	auditLedgerRetentionVersion     = "1.15.447"
	auditLedgerRetentionDescription = "Feste Löschfristen für Anmeldeereignisse, Zugriffsprotokoll und Löschprotokoll (#3956)"
)

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     auditLedgerRetentionVersion,
		Description: auditLedgerRetentionDescription,
		DependsOn:   []string{auditRetentionCapabilitiesVersion, statusDayRetentionPurgeVersion},
	})
	Migrations.MustRegister(auditLedgerRetentionUp, auditLedgerRetentionDown)
}

// auditLedgerRetentionSQL gives the three append-only audit ledgers of #3956
// their fixed retention, through the pattern of 1.15.353 (#2850): each
// function runs as its owner, accepts only the tenant of the calling
// transaction and refuses a cutoff newer than the window, so a tenant role
// can never delete young or foreign rows. Direct DELETE stays revoked.
//
//   - audit.auth_events: 90 days after the event.
//   - audit.data_access_log: until the end of the year after the access, so
//     the latest cutoff is 1 January of last year (Berlin).
//   - audit.data_deletions: three years after the deletion.
const auditLedgerRetentionSQL = `
		CREATE OR REPLACE FUNCTION audit.delete_expired_auth_events(
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
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'auth event retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '90 days' THEN
				RAISE EXCEPTION 'auth event cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.auth_events
			WHERE tenant_id = p_tenant_id
			  AND created_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=auth_events, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_data_access_log(
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
				RAISE EXCEPTION 'data access log retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);

			latest_allowed_cutoff := (
				date_trunc('year', clock_timestamp() AT TIME ZONE 'Europe/Berlin') - INTERVAL '1 year'
			) AT TIME ZONE 'Europe/Berlin';
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > latest_allowed_cutoff THEN
				RAISE EXCEPTION 'data access log cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.data_access_log
			WHERE tenant_id = p_tenant_id
			  AND accessed_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=data_access_log, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		CREATE OR REPLACE FUNCTION audit.delete_expired_data_deletions(
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
			deleted_count BIGINT;
			invoker_is_superuser BOOLEAN;
		BEGIN
			IF p_cutoff IS NULL THEN
				RAISE EXCEPTION 'deletion log retention requires a cutoff' USING ERRCODE = '22023';
			END IF;
			invoker_is_superuser := audit.authorize_retention_tenant(p_tenant_id);
			IF NOT COALESCE(invoker_is_superuser, FALSE) AND p_cutoff > clock_timestamp() - INTERVAL '3 years' THEN
				RAISE EXCEPTION 'deletion log cutoff exceeds retention window' USING ERRCODE = '42501';
			END IF;

			DELETE FROM audit.data_deletions
			WHERE tenant_id = p_tenant_id
			  AND deleted_at < p_cutoff;
			GET DIAGNOSTICS deleted_count = ROW_COUNT;
			RAISE LOG 'audit retention executed: tenant_id=%, capability=data_deletions, cutoff=%, rows_deleted=%',
				p_tenant_id, p_cutoff, deleted_count;
			RETURN deleted_count;
		END;
		$$;

		REVOKE ALL ON FUNCTION audit.delete_expired_auth_events(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_data_access_log(BIGINT, TIMESTAMPTZ) FROM PUBLIC;
		REVOKE ALL ON FUNCTION audit.delete_expired_data_deletions(BIGINT, TIMESTAMPTZ) FROM PUBLIC;

		GRANT EXECUTE ON FUNCTION audit.delete_expired_auth_events(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_data_access_log(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;
		GRANT EXECUTE ON FUNCTION audit.delete_expired_data_deletions(BIGINT, TIMESTAMPTZ) TO phoenix_tenant;

		REVOKE DELETE ON audit.auth_events FROM phoenix_tenant;
		REVOKE DELETE ON audit.data_access_log FROM phoenix_tenant;
		REVOKE DELETE ON audit.data_deletions FROM phoenix_tenant;
`

func auditLedgerRetentionUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auditLedgerRetentionSQL); err != nil {
		return fmt.Errorf("create audit ledger retention capabilities: %w", err)
	}
	return nil
}

const auditLedgerRetentionDownSQL = `
		GRANT DELETE ON audit.auth_events TO phoenix_tenant;
		GRANT DELETE ON audit.data_access_log TO phoenix_tenant;
		GRANT DELETE ON audit.data_deletions TO phoenix_tenant;

		DROP FUNCTION IF EXISTS audit.delete_expired_auth_events(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS audit.delete_expired_data_access_log(BIGINT, TIMESTAMPTZ);
		DROP FUNCTION IF EXISTS audit.delete_expired_data_deletions(BIGINT, TIMESTAMPTZ);
`

func auditLedgerRetentionDown(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, auditLedgerRetentionDownSQL); err != nil {
		return fmt.Errorf("drop audit ledger retention capabilities: %w", err)
	}
	return nil
}
