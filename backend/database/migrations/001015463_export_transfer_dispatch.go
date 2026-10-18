package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const exportTransferDispatchVersion = "1.15.463"

func init() {
	MigrationRegistry.Register(&Migration{Version: exportTransferDispatchVersion,
		Description: "Fence committed export transfer attempts with one-shot dispatch leases",
		DependsOn:   []string{exportTransferAuditVersion, "1.15.353"}})
	Migrations.MustRegister(exportTransferDispatchUp, exportTransferDispatchDown)
}

func exportTransferDispatchUp(ctx context.Context, db *bun.DB) error {
	_, err := db.NewRaw(`
 ALTER TABLE audit.export_transfers
 ADD COLUMN dispatch_state TEXT NOT NULL DEFAULT 'terminal' CHECK(dispatch_state IN ('terminal','pending','dispatching','unknown')),
 ADD COLUMN dispatch_token TEXT,
 ADD COLUMN dispatch_expires_at TIMESTAMPTZ,
 ADD CONSTRAINT export_transfer_dispatch_fence CHECK (
 (dispatch_state='terminal' AND dispatch_token IS NULL) OR
 (dispatch_state IN ('pending','unknown') AND dispatch_token IS NULL AND dispatch_expires_at IS NOT NULL) OR
 (dispatch_state='dispatching' AND dispatch_token IS NOT NULL AND dispatch_expires_at IS NOT NULL));
 CREATE INDEX idx_export_transfer_dispatch_expiry ON audit.export_transfers(dispatch_expires_at,id)
 WHERE dispatch_state IN ('pending','dispatching');

 -- No general UPDATE grant: only the one-shot outcome can change, never
 -- the committed actor, destination, filename, size or creation evidence.
 CREATE FUNCTION audit.claim_export_transfer(p_tenant BIGINT,p_id BIGINT) RETURNS TEXT
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,audit AS $$
 DECLARE token TEXT;
 BEGIN
 PERFORM audit.authorize_retention_tenant(p_tenant);
 token := public.uuid_generate_v4()::text;
 UPDATE audit.export_transfers SET dispatch_state='dispatching',dispatch_token=token,
 dispatch_expires_at=clock_timestamp()+INTERVAL '2 minutes',updated_at=clock_timestamp()
 WHERE tenant_id=p_tenant AND id=p_id AND dispatch_state='pending' AND dispatch_expires_at>clock_timestamp();
 IF NOT FOUND THEN RETURN ''; END IF;
 RETURN token;
 END; $$;

 CREATE FUNCTION audit.complete_export_transfer(p_tenant BIGINT,p_id BIGINT,p_token TEXT,p_success BOOLEAN,p_reason TEXT) RETURNS BOOLEAN
 LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,audit AS $$
 BEGIN
 PERFORM audit.authorize_retention_tenant(p_tenant);
 IF p_success IS NULL OR p_reason IS NULL OR
 (p_success AND p_reason<>'') OR (NOT p_success AND p_reason NOT IN
 ('address_denied','host_key_mismatch','authentication_rejected','connection_failed','upload_failed','file_too_large','internal_error')) THEN
 RAISE EXCEPTION 'invalid transfer outcome' USING ERRCODE='22023';
 END IF;
 UPDATE audit.export_transfers SET dispatch_state='terminal',dispatch_token=NULL,
 status=CASE WHEN p_success THEN 'success' ELSE 'failed' END,failure_reason=p_reason,updated_at=clock_timestamp()
 WHERE tenant_id=p_tenant AND id=p_id AND dispatch_state='dispatching' AND dispatch_token=p_token AND dispatch_expires_at>clock_timestamp();
 RETURN FOUND;
 END; $$;
 REVOKE ALL ON FUNCTION audit.claim_export_transfer(BIGINT,BIGINT) FROM PUBLIC;
 REVOKE ALL ON FUNCTION audit.complete_export_transfer(BIGINT,BIGINT,TEXT,BOOLEAN,TEXT) FROM PUBLIC;
 GRANT EXECUTE ON FUNCTION audit.claim_export_transfer(BIGINT,BIGINT) TO phoenix_tenant;
 GRANT EXECUTE ON FUNCTION audit.complete_export_transfer(BIGINT,BIGINT,TEXT,BOOLEAN,TEXT) TO phoenix_tenant;
 `).Exec(ctx)
	if err != nil {
		return fmt.Errorf("create export transfer dispatch fences: %w", err)
	}
	return nil
}

func exportTransferDispatchDown(ctx context.Context, db *bun.DB) error {
	var active int
	if err := db.NewRaw(`SELECT count(*) FROM audit.export_transfers WHERE dispatch_state IN ('pending','dispatching')`).Scan(ctx, &active); err != nil {
		return err
	}
	if active > 0 {
		return fmt.Errorf("export transfer rollback requires settling %d unfinished attempts", active)
	}
	_, err := db.NewRaw(`DROP FUNCTION audit.complete_export_transfer(BIGINT,BIGINT,TEXT,BOOLEAN,TEXT);
 DROP FUNCTION audit.claim_export_transfer(BIGINT,BIGINT);
 DROP INDEX audit.idx_export_transfer_dispatch_expiry;
 ALTER TABLE audit.export_transfers DROP CONSTRAINT export_transfer_dispatch_fence,
 DROP COLUMN dispatch_state,DROP COLUMN dispatch_token,DROP COLUMN dispatch_expires_at;`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("remove export transfer dispatch fences: %w", err)
	}
	return nil
}
