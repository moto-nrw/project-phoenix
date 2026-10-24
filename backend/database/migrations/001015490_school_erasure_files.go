package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	MigrationRegistry.Register(&Migration{Version: "1.15.490", Description: "Tenant erasure inventory, preview and file phase evidence (#3837)", DependsOn: []string{schoolErasureVersion}})
	Migrations.MustRegister(schoolErasureFilesUp, schoolErasureFilesDown)
}

func schoolErasureFilesUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, schoolErasurePreviewSQL); err != nil {
		return err
	}
	_, err := db.ExecContext(ctx, schoolErasureFilesSQL)
	return err
}

func schoolErasureFilesDown(ctx context.Context, db *bun.DB) error {
	_, err := db.ExecContext(ctx, `
 DO $guard$ BEGIN
 IF EXISTS (SELECT FROM platform.school_erasures WHERE files_inventory_at IS NOT NULL) THEN
 RAISE EXCEPTION 'file phase evidence requires a forward fix'; END IF; END $guard$;
 DROP FUNCTION platform.school_erasure_preview(bigint,bigint);
 DROP FUNCTION platform.school_erasure_schools(bigint,bigint,text);
 DROP FUNCTION platform.school_erasure_file_phase(bigint,text,jsonb);
 ALTER TABLE platform.school_erasures DROP COLUMN files_inventory_at, DROP COLUMN file_counts;
 `)
	return err
}

const schoolErasureFilesSQL = `
 ALTER TABLE platform.school_erasures ADD COLUMN files_inventory_at TIMESTAMPTZ,
 ADD COLUMN file_counts JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(file_counts)='object');

 CREATE FUNCTION platform.school_erasure_schools(p_org BIGINT,p_operator BIGINT,p_reference TEXT)
 RETURNS JSONB LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path=pg_catalog AS $body$
 BEGIN
 IF NOT EXISTS (SELECT FROM platform.operators WHERE id=p_operator AND active) THEN
 RAISE EXCEPTION 'active operator required' USING ERRCODE='42501'; END IF;
 IF NOT EXISTS (SELECT FROM platform.organizations WHERE id=p_org) THEN
 RAISE EXCEPTION 'existing organization required' USING ERRCODE='22023'; END IF;
 RETURN (SELECT COALESCE(jsonb_agg(id ORDER BY id),'[]') FROM (
 SELECT id FROM platform.schools WHERE organization_id=p_org
 UNION SELECT school_id FROM platform.school_erasures WHERE organization_id=p_org AND operator_id=p_operator AND order_reference=p_reference
 ) s);
 END $body$;

 CREATE FUNCTION platform.school_erasure_file_phase(p_id BIGINT,p_phase TEXT,p_counts JSONB)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog AS $body$
 DECLARE v_order platform.school_erasures%ROWTYPE;
 BEGIN
 SELECT * INTO v_order FROM platform.school_erasures WHERE id=p_id FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'existing erasure required' USING ERRCODE='22023'; END IF;
 IF v_order.completed_at IS NOT NULL THEN RETURN to_jsonb(v_order); END IF;
 IF p_phase='inventory' THEN
 IF v_order.files_inventory_at IS NULL THEN
 IF v_order.db_done_at IS NOT NULL OR p_counts IS NULL OR jsonb_typeof(p_counts)<>'object' THEN
 RAISE EXCEPTION 'inventory required before database deletion' USING ERRCODE='22023'; END IF;
 IF (SELECT count(*) FROM jsonb_each(p_counts))<>9 OR EXISTS (
 SELECT FROM jsonb_each(p_counts) e WHERE e.key NOT IN ('files','announcement-attachments','student-documents','staff-documents','student-photos','login-images','enrollment-legal-documents','enrollment-form-legal-documents','avatars')
 OR jsonb_typeof(e.value)<>'number' OR e.value::text !~ '^[0-9]+$') THEN
 RAISE EXCEPTION 'bounded file counts required' USING ERRCODE='22023'; END IF;
 UPDATE platform.school_erasures SET files_inventory_at=clock_timestamp(),file_counts=p_counts,procedure_version='1.15.490' WHERE id=p_id;
 END IF;
 ELSIF p_phase='failed' THEN
 UPDATE platform.school_erasures SET status='failed',failed_at=clock_timestamp(),last_error_code='F0001' WHERE id=p_id;
 ELSIF p_phase='files_done' AND v_order.db_done_at IS NOT NULL AND v_order.files_inventory_at IS NOT NULL THEN
 UPDATE platform.school_erasures SET status='files_done',files_done_at=COALESCE(files_done_at,clock_timestamp()),failed_at=NULL,last_error_code=NULL WHERE id=p_id;
 ELSIF p_phase='completed' AND v_order.status='files_done' THEN
 UPDATE platform.school_erasures SET status='completed',completed_at=clock_timestamp() WHERE id=p_id;
 ELSE RAISE EXCEPTION 'invalid file phase transition' USING ERRCODE='22023'; END IF;
 RETURN platform.read_school_erasure(p_id);
 END $body$;
 REVOKE ALL ON FUNCTION platform.school_erasure_preview(bigint,bigint),platform.school_erasure_schools(bigint,bigint,text),platform.school_erasure_file_phase(bigint,text,jsonb) FROM PUBLIC,phoenix_auth,phoenix_tenant,phoenix_admin;
 GRANT EXECUTE ON FUNCTION platform.school_erasure_preview(bigint,bigint),platform.school_erasure_schools(bigint,bigint,text),platform.school_erasure_file_phase(bigint,text,jsonb) TO phoenix_school_erasure;
`
