package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const schoolErasureVersion = "1.15.481"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:      schoolErasureVersion,
		Description:  "Erase a school's database with an isolated role and durable phase evidence (#3837)",
		DependsOn:    []string{guestViewOnlyVersion},
		Precondition: schoolErasurePrecondition,
	})
	Migrations.MustRegister(schoolErasureUp, schoolErasureDown)
}

func schoolErasureUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, installSchoolErasure)
}

func installSchoolErasure(ctx context.Context, tx bun.Tx) error {
	if err := requireSchoolErasureReferences(ctx, tx); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, schoolErasureSchemaSQL); err != nil {
		return fmt.Errorf("create school erasure evidence: %w", err)
	}
	if err := installSchoolErasurePlan(ctx, tx); err != nil {
		return fmt.Errorf("install school erasure plan: %w", err)
	}
	if err := installSchoolErasurePreflight(ctx, tx); err != nil {
		return fmt.Errorf("install school erasure preflight: %w", err)
	}
	if err := installSchoolErasureResidual(ctx, tx); err != nil {
		return fmt.Errorf("install school erasure residual check: %w", err)
	}
	if err := installSchoolErasureDelete(ctx, tx); err != nil {
		return fmt.Errorf("install school erasure deletion: %w", err)
	}
	if _, err := tx.ExecContext(ctx, schoolErasureFunctionsSQL); err != nil {
		return fmt.Errorf("install school erasure functions: %w", err)
	}
	if err := installSchoolErasureBilling(ctx, tx); err != nil {
		return fmt.Errorf("restrict retained billing evidence: %w", err)
	}
	return nil
}

const schoolErasureSchemaSQL = `
 DO $role$ BEGIN
  IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = 'phoenix_school_erasure') THEN
   CREATE ROLE phoenix_school_erasure NOLOGIN NOINHERIT NOBYPASSRLS;
  END IF;
 END $role$;
 GRANT USAGE ON SCHEMA platform TO phoenix_school_erasure;
 -- Every tenant writer must be fenced by the school root. Otherwise a
 -- transaction queued behind the erasure locks could recreate personal rows
 -- after the school has disappeared. Validate existing references; orphaned
 -- legacy data blocks rollout rather than being silently reassigned/deleted.
 ALTER TABLE active.presence_backfill_batches ADD CONSTRAINT fk_presence_backfill_batches_school_erasure
  FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
 ALTER TABLE audit.room_color_migration_backup ADD CONSTRAINT fk_room_color_backup_school_erasure
  FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
 ALTER TABLE audit.wc_alias_migration_backup ADD CONSTRAINT fk_wc_alias_backup_school_erasure
  FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
 ALTER TABLE auth.mfa_email_challenges ADD CONSTRAINT fk_mfa_email_challenges_school_erasure
  FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);
 ALTER TABLE auth.parent_declaration_permission_grants ADD CONSTRAINT fk_parent_declaration_grants_school_erasure
  FOREIGN KEY (tenant_id) REFERENCES platform.schools(id);

 -- 1.15.92 gave PUBLIC DML on MFA overrides. PUBLIC applies even to an
 -- isolated NOINHERIT role. Preserve the existing runtime roles' rights and
 -- RLS while preventing this new CLI role from inheriting generic writes.
 GRANT SELECT, INSERT, UPDATE, DELETE ON auth.mfa_overrides
  TO phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo;
 GRANT USAGE ON SEQUENCE auth.mfa_overrides_id_seq
  TO phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_demo;
 REVOKE ALL ON auth.mfa_overrides FROM PUBLIC;
 REVOKE ALL ON SEQUENCE auth.mfa_overrides_id_seq FROM PUBLIC;


 -- No FKs: evidence must survive removal of the school, organization and
 -- operator. IDs are references, not names or copies of their records.
 CREATE TABLE platform.school_erasures (
  id BIGSERIAL PRIMARY KEY,
  school_id BIGINT NOT NULL CHECK (school_id > 0),
  organization_id BIGINT NOT NULL CHECK (organization_id > 0),
  operator_id BIGINT NOT NULL CHECK (operator_id > 0),
  order_reference VARCHAR(120) NOT NULL CHECK (length(btrim(order_reference)) > 0),
  catalog_version INTEGER NOT NULL DEFAULT 1,
  procedure_version TEXT NOT NULL DEFAULT '1.15.481',
  status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','db_done','files_done','completed','failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  db_started_at TIMESTAMPTZ,
  last_attempt_at TIMESTAMPTZ,
  db_done_at TIMESTAMPTZ,
  files_done_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  last_error_code VARCHAR(5),
  row_counts JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(row_counts) = 'object'),
  CHECK (status NOT IN ('db_done','files_done','completed') OR db_done_at IS NOT NULL),
  CHECK (status NOT IN ('files_done','completed') OR files_done_at IS NOT NULL),
  CHECK ((status = 'completed') = (completed_at IS NOT NULL)),
  CHECK (status <> 'failed' OR (failed_at IS NOT NULL AND last_error_code IS NOT NULL))
 );
 CREATE UNIQUE INDEX school_erasures_one_order_per_school ON platform.school_erasures(school_id);
 CREATE INDEX school_erasures_expiry ON platform.school_erasures(completed_at) WHERE status = 'completed';
 COMMENT ON TABLE platform.school_erasures IS
  'ADR 0092: internal erasure order and phase evidence; no names, secrets or personal free text. Completed evidence expires after three years. DB success is not file or overall success.';
 REVOKE ALL ON platform.school_erasures FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
 REVOKE ALL ON SEQUENCE platform.school_erasures_id_seq FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
`

const schoolErasureFunctionsSQL = `
 CREATE OR REPLACE FUNCTION platform.begin_school_erasure(p_school_id BIGINT, p_operator_id BIGINT, p_order_reference TEXT)
 RETURNS BIGINT LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_organization_id BIGINT;
  v_order platform.school_erasures%ROWTYPE;
 BEGIN
  IF p_school_id IS NULL OR p_school_id <= 0 OR p_operator_id IS NULL OR p_operator_id <= 0
   OR p_order_reference IS NULL OR length(btrim(p_order_reference)) NOT BETWEEN 1 AND 120 THEN
   RAISE EXCEPTION 'school erasure requires a school, operator and bounded non-personal order reference' USING ERRCODE = '22023';
  END IF;
  PERFORM 1 FROM platform.operators WHERE id = p_operator_id AND active FOR KEY SHARE;
  IF NOT FOUND THEN
   RAISE EXCEPTION 'school erasure requires an active operator' USING ERRCODE = '42501';
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('school-erasure:' || p_school_id::text, 0));
  SELECT * INTO v_order FROM platform.school_erasures WHERE school_id = p_school_id FOR UPDATE;
  IF FOUND THEN
   IF v_order.order_reference <> btrim(p_order_reference) OR v_order.operator_id <> p_operator_id THEN
    RAISE EXCEPTION 'school already has a different erasure order' USING ERRCODE = '22023';
   END IF;
   RETURN v_order.id;
  END IF;
  SELECT organization_id INTO v_organization_id FROM platform.schools WHERE id = p_school_id FOR UPDATE;
  IF NOT FOUND THEN
   RAISE EXCEPTION 'school erasure requires an existing school' USING ERRCODE = '22023';
  END IF;
  INSERT INTO platform.school_erasures(school_id, organization_id, operator_id, order_reference)
  VALUES (p_school_id, v_organization_id, p_operator_id, btrim(p_order_reference))
  RETURNING id INTO v_order.id;
  RETURN v_order.id;
 END $body$;

 CREATE OR REPLACE FUNCTION platform.erase_school(p_school_id BIGINT, p_erasure_id BIGINT)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_order platform.school_erasures%ROWTYPE;
  v_counts JSONB := '{}'::jsonb;
  v_error TEXT;
 BEGIN
  IF p_school_id IS NULL OR p_school_id <= 0 OR p_erasure_id IS NULL OR p_erasure_id <= 0 THEN
   RAISE EXCEPTION 'school erasure requires matching positive references' USING ERRCODE = '22023';
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('school-erasure:' || p_school_id::text, 0));
  SELECT * INTO v_order FROM platform.school_erasures WHERE id = p_erasure_id FOR UPDATE;
  IF NOT FOUND OR v_order.school_id <> p_school_id THEN
   RAISE EXCEPTION 'school does not match erasure order' USING ERRCODE = '22023';
  END IF;
  -- Database completion is idempotent, even after a later file-phase failure.
  IF v_order.db_done_at IS NOT NULL THEN
   RETURN jsonb_build_object('status', v_order.status, 'row_counts', v_order.row_counts);
  END IF;
  UPDATE platform.school_erasures SET db_started_at = COALESCE(db_started_at, clock_timestamp()),
   last_attempt_at = clock_timestamp() WHERE id = p_erasure_id;

  -- This subtransaction contains EVERY deletion. A caught failure rolls it
  -- back before persisting failed status outside the block. Only SQLSTATE is
  -- stored; SQLERRM/detail can contain personal values and must never escape.
  BEGIN
   PERFORM platform.school_erasure_preflight(p_school_id,v_order.organization_id);
   v_counts := platform.school_erasure_delete(p_school_id);
   UPDATE platform.school_erasures SET status = 'db_done', db_done_at = clock_timestamp(),
    failed_at = NULL, last_error_code = NULL, row_counts = v_counts WHERE id = p_erasure_id;
  EXCEPTION WHEN OTHERS OR query_canceled THEN
   GET STACKED DIAGNOSTICS v_error = RETURNED_SQLSTATE;
   UPDATE platform.school_erasures SET status = 'failed', failed_at = clock_timestamp(),
    last_error_code = v_error, row_counts = '{}'::jsonb WHERE id = p_erasure_id;
  END;
  SELECT * INTO v_order FROM platform.school_erasures WHERE id = p_erasure_id;
  RETURN jsonb_build_object('status', v_order.status, 'row_counts', v_order.row_counts, 'error_code', v_order.last_error_code);
 END $body$;

 CREATE OR REPLACE FUNCTION platform.read_school_erasure(p_erasure_id BIGINT)
 RETURNS JSONB LANGUAGE sql STABLE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$ SELECT to_jsonb(e) FROM platform.school_erasures e WHERE e.id = p_erasure_id $body$;

 CREATE OR REPLACE FUNCTION platform.delete_expired_school_erasures()
 RETURNS BIGINT LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE v_count BIGINT;
 BEGIN
  DELETE FROM platform.school_erasures WHERE status = 'completed' AND completed_at < clock_timestamp() - interval '3 years';
  GET DIAGNOSTICS v_count = ROW_COUNT;
  RETURN v_count;
 END $body$;

 REVOKE ALL ON FUNCTION platform.begin_school_erasure(BIGINT,BIGINT,TEXT), platform.erase_school(BIGINT,BIGINT),
  platform.read_school_erasure(BIGINT), platform.delete_expired_school_erasures()
  FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
 GRANT EXECUTE ON FUNCTION platform.begin_school_erasure(BIGINT,BIGINT,TEXT), platform.erase_school(BIGINT,BIGINT),
  platform.read_school_erasure(BIGINT), platform.delete_expired_school_erasures() TO phoenix_school_erasure;
`

func schoolErasureDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// Erased records cannot be reconstructed. Never discard evidence on rollback.
		var count int
		if err := tx.NewRaw(`SELECT count(*) FROM platform.school_erasures`).Scan(ctx, &count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("school erasure rollback would discard evidence; use a forward migration")
		}
		_, err := tx.ExecContext(ctx, `
   DROP VIEW platform.live_billing_key_date_counts;
   DROP FUNCTION platform.delete_expired_billing_key_date_counts();
   REVOKE SELECT ON platform.billing_key_date_counts FROM phoenix_billing_retention;
   DROP FUNCTION platform.capture_billing_key_date_counts(BIGINT[],TEXT[],TEXT[],TEXT[],TEXT[],INTEGER[],INTEGER[]);
   GRANT SELECT, INSERT ON platform.billing_key_date_counts TO phoenix_admin;
   ALTER TABLE active.presence_backfill_batches DROP CONSTRAINT fk_presence_backfill_batches_school_erasure;
   ALTER TABLE audit.room_color_migration_backup DROP CONSTRAINT fk_room_color_backup_school_erasure;
   ALTER TABLE audit.wc_alias_migration_backup DROP CONSTRAINT fk_wc_alias_backup_school_erasure;
   ALTER TABLE auth.mfa_email_challenges DROP CONSTRAINT fk_mfa_email_challenges_school_erasure;
   ALTER TABLE auth.parent_declaration_permission_grants DROP CONSTRAINT fk_parent_declaration_grants_school_erasure;
   DROP FUNCTION platform.delete_expired_school_erasures();
   DROP FUNCTION platform.read_school_erasure(BIGINT);
   DROP FUNCTION platform.erase_school(BIGINT,BIGINT);
   DROP FUNCTION platform.school_erasure_delete(BIGINT);
   DROP FUNCTION platform.school_erasure_assert_empty(BIGINT,JSONB);
   DROP FUNCTION platform.school_erasure_preflight(BIGINT,BIGINT);
   DROP FUNCTION platform.begin_school_erasure(BIGINT,BIGINT,TEXT);
   DROP FUNCTION platform.school_erasure_plan();
   DROP TABLE platform.school_erasures;
  `)
		return err
	})
}
