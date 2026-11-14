package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	RegisterRepeatable("platform.erase_organization", func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, repeatablePlatformEraseOrganizationSQL)
		return err
	})
}

// repeatablePlatformEraseOrganizationSQL is the current definition of
// platform.erase_organization(BIGINT, BIGINT, TEXT) (ADR 0223). Change the
// function here; numbered migrations no longer define it. Since #6955 the
// evidence also counts the devices the Träger-Portal remembered, which the
// deletion of the Träger takes along by cascade.
const repeatablePlatformEraseOrganizationSQL = `
 CREATE OR REPLACE FUNCTION platform.erase_organization(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_order platform.organization_erasure_orders%ROWTYPE;
  v_active BOOLEAN;
  v_counts JSONB;
 BEGIN
  IF p_org IS NULL OR p_org <= 0 OR p_operator IS NULL OR p_operator <= 0 OR p_reference IS NULL THEN
   RAISE EXCEPTION 'organization erasure requires an organization, operator and reference' USING ERRCODE = '22023';
  END IF;
  PERFORM 1 FROM platform.operators WHERE id = p_operator AND active FOR KEY SHARE;
  IF NOT FOUND THEN
   RAISE EXCEPTION 'organization erasure requires an active operator' USING ERRCODE = '42501';
  END IF;
  PERFORM pg_advisory_xact_lock(hashtextextended('organization-erasure:' || p_org::text, 0));
  SELECT * INTO v_order FROM platform.organization_erasure_orders
   WHERE organization_id = p_org AND withdrawn_at IS NULL FOR UPDATE;
  IF NOT FOUND OR v_order.order_reference <> btrim(p_reference) THEN
   RAISE EXCEPTION 'organization erasure requires its recorded open order' USING ERRCODE = '22023';
  END IF;
  -- Completion is idempotent: a repeated run returns the evidence.
  IF v_order.completed_at IS NOT NULL THEN
   RETURN to_jsonb(v_order);
  END IF;
  IF v_order.erase_not_before > (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date THEN
   RAISE EXCEPTION 'organization erasure grace period has not ended' USING ERRCODE = '22023';
  END IF;
  SELECT active INTO v_active FROM platform.organizations WHERE id = p_org FOR UPDATE;
  IF NOT FOUND OR v_active THEN
   RAISE EXCEPTION 'organization erasure requires the deactivated organization' USING ERRCODE = '22023';
  END IF;
  IF EXISTS (SELECT FROM platform.schools WHERE organization_id = p_org) THEN
   RAISE EXCEPTION 'organization erasure requires its schools to be erased first' USING ERRCODE = '22023';
  END IF;
  -- A school whose rows are gone but whose file phase did not finish, under
  -- any operator or reference, keeps the root: no completion over leftovers.
  IF EXISTS (SELECT FROM platform.school_erasures WHERE organization_id = p_org AND status <> 'completed') THEN
   RAISE EXCEPTION 'organization erasure requires every school erasure to be completed' USING ERRCODE = '22023';
  END IF;
  v_counts := jsonb_build_object(
   'platform.organizations', 1,
   'platform.organization_accounts', (SELECT count(*) FROM platform.organization_accounts WHERE organization_id = p_org),
   'auth.organization_account_roles', (SELECT count(*) FROM auth.organization_account_roles WHERE organization_id = p_org),
   'auth.tokens', (SELECT count(*) FROM auth.tokens WHERE organization_id = p_org),
   'auth.mfa_trusted_devices', (SELECT count(*) FROM auth.mfa_trusted_devices WHERE organization_id = p_org),
   'platform.school_erasures', (SELECT count(*) FROM platform.school_erasures
    WHERE organization_id = p_org AND order_reference = v_order.order_reference AND status = 'completed'));
  -- The FKs of memberships, Träger roles, Träger sessions and the devices
  -- the Träger-Portal remembered (#6955) cascade.
  DELETE FROM platform.organizations WHERE id = p_org;
  INSERT INTO platform.operator_audit_log (operator_id, action, resource_type, resource_id, changes)
  VALUES (p_operator, 'erase', 'organization', p_org,
   jsonb_build_object('order_id', v_order.id, 'order_reference', v_order.order_reference, 'row_counts', v_counts));
  UPDATE platform.organization_erasure_orders
   SET started_at = COALESCE(started_at, clock_timestamp()),
    completed_at = clock_timestamp(), completed_by = p_operator, row_counts = v_counts
   WHERE id = v_order.id RETURNING * INTO v_order;
  RETURN to_jsonb(v_order);
 END $body$;
`
