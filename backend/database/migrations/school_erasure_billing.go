package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func installSchoolErasureBilling(ctx context.Context, tx bun.Tx) error {
	_, err := tx.ExecContext(ctx, schoolErasureBillingSQL)
	return err
}

// phoenix_admin bypasses RLS. A restricted view plus revoked base-table SELECT
// is therefore the boundary, not a policy that this runtime role could bypass.
// The invoice's captured names remain immutable references (1.15.416).
const schoolErasureBillingSQL = `
 DO $role$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_billing_retention') THEN
   CREATE ROLE phoenix_billing_retention NOLOGIN NOINHERIT NOBYPASSRLS;
  END IF;
 END $role$;
 GRANT USAGE ON SCHEMA platform TO phoenix_billing_retention;
 CREATE VIEW platform.live_billing_key_date_counts WITH (security_barrier = true) AS
  SELECT r.* FROM platform.billing_key_date_counts r
  WHERE EXISTS (SELECT FROM platform.schools s WHERE s.id = r.school_id);
 REVOKE SELECT, INSERT ON platform.billing_key_date_counts FROM phoenix_admin;
 REVOKE ALL ON platform.live_billing_key_date_counts FROM PUBLIC, phoenix_auth, phoenix_tenant;
 GRANT SELECT ON platform.live_billing_key_date_counts TO phoenix_admin;
 GRANT SELECT ON platform.billing_key_date_counts TO phoenix_billing_retention;

 -- ON CONFLICT needs SELECT on the base relation. Keep those rights inside
 -- this narrow writer instead of restoring the operator's archive read access.
 CREATE FUNCTION platform.capture_billing_key_date_counts(
  p_school_ids BIGINT[], p_periods TEXT[], p_key_dates TEXT[], p_school_names TEXT[],
  p_organization_names TEXT[], p_students INTEGER[], p_terminals INTEGER[])
 RETURNS BIGINT LANGUAGE sql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
  WITH inserted AS (INSERT INTO platform.billing_key_date_counts
   (school_id,period,key_date,school_name,organization_name,active_students,active_terminals)
  SELECT school_id,period::date,key_date::date,school_name,organization_name,students,terminals
  FROM unnest(p_school_ids,p_periods,p_key_dates,p_school_names,p_organization_names,p_students,p_terminals)
   AS captured(school_id,period,key_date,school_name,organization_name,students,terminals)
  ON CONFLICT (school_id,period) DO NOTHING RETURNING 1)
  SELECT count(*) FROM inserted;
 $body$;
 REVOKE ALL ON FUNCTION platform.capture_billing_key_date_counts(BIGINT[],TEXT[],TEXT[],TEXT[],TEXT[],INTEGER[],INTEGER[])
  FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_school_erasure, phoenix_billing_retention;
 GRANT EXECUTE ON FUNCTION platform.capture_billing_key_date_counts(BIGINT[],TEXT[],TEXT[],TEXT[],TEXT[],INTEGER[],INTEGER[]) TO phoenix_admin;

 CREATE FUNCTION platform.delete_expired_billing_key_date_counts()
 RETURNS BIGINT LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE v_count BIGINT;
 BEGIN
  DELETE FROM platform.billing_key_date_counts
   WHERE key_date < (date_trunc('year', clock_timestamp() AT TIME ZONE 'Europe/Berlin') - interval '10 years')::date;
  GET DIAGNOSTICS v_count = ROW_COUNT;
  RETURN v_count;
 END $body$;
 REVOKE ALL ON FUNCTION platform.delete_expired_billing_key_date_counts()
  FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
 GRANT EXECUTE ON FUNCTION platform.delete_expired_billing_key_date_counts() TO phoenix_billing_retention;
 COMMENT ON TABLE platform.billing_key_date_counts IS
  'ADR 0092: immutable organizational invoice references, retained 10 years after the key_date calendar year. After school deletion readable only by the restricted billing role; no third-party export. Legal confirmation and scheduled expiry execution remain operational follow-ups.';
`
