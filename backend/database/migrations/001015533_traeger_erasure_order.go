package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerErasureOrderVersion = "1.15.533"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerErasureOrderVersion,
		Description: "Träger erasure order with written confirmation, grace period and root deletion (#4657)",
		DependsOn: []string{
			accountAnalyticsConsentVersion, // the last migration before this one
			traegerPortalVersion,           // memberships, Träger roles and Träger sessions the root deletion cascades to
			"1.15.490",                     // the school phases of an organization order (school_erasure_schools)
		},
	})
	Migrations.MustRegister(traegerErasureOrderUp, traegerErasureOrderDown)
}

// traegerErasureOrderSQL completes the tenant erasure of ADR 0092 for the
// Träger root (#4657, #3837):
//
//   - platform.organization_erasure_orders records the moto team's erasure
//     order for a deactivated Träger: the non-personal order reference, the
//     date of the written confirmation and the end of the grace period. The
//     operator records and withdraws it in the operator portal; it has no FK,
//     so it survives the root as evidence, like platform.school_erasures.
//   - platform.organization_erasure_check reports, without changes, whether
//     the order is due. The CLI asks it before it erases any school.
//   - platform.begin_organization_erasure marks a due order as started before
//     the first school goes. From then on the order cannot be withdrawn, so a
//     run that fails halfway is resumed with the same order, never replaced.
//   - platform.erase_organization deletes the Träger root after its schools
//     are erased: the organization row and, by its cascading FKs, the Träger
//     memberships, Träger roles and Träger sessions. It writes the operator
//     audit entry and completes the order in the same transaction.
//
// Both functions run only under the isolated erasure role, like the school
// erasure (SECURITY DEFINER, fixed search path).
const traegerErasureOrderSQL = `
 CREATE TABLE platform.organization_erasure_orders (
  id BIGSERIAL PRIMARY KEY,
  organization_id BIGINT NOT NULL CHECK (organization_id > 0),
  order_reference VARCHAR(40) NOT NULL CHECK (order_reference ~ '^[A-Z][A-Z0-9-]{2,39}$'),
  confirmed_on DATE NOT NULL,
  erase_not_before DATE NOT NULL,
  recorded_by BIGINT NOT NULL CHECK (recorded_by > 0),
  recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  started_at TIMESTAMPTZ,
  withdrawn_by BIGINT CHECK (withdrawn_by > 0),
  withdrawn_at TIMESTAMPTZ,
  completed_by BIGINT CHECK (completed_by > 0),
  completed_at TIMESTAMPTZ,
  row_counts JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(row_counts) = 'object'),
  CONSTRAINT chk_organization_erasure_orders_grace CHECK (erase_not_before > confirmed_on),
  CONSTRAINT chk_organization_erasure_orders_withdrawn CHECK ((withdrawn_at IS NULL) = (withdrawn_by IS NULL)),
  CONSTRAINT chk_organization_erasure_orders_completed CHECK ((completed_at IS NULL) = (completed_by IS NULL)),
  CONSTRAINT chk_organization_erasure_orders_final CHECK (withdrawn_at IS NULL OR completed_at IS NULL),
  CONSTRAINT chk_organization_erasure_orders_started CHECK (started_at IS NULL OR withdrawn_at IS NULL),
  CONSTRAINT chk_organization_erasure_orders_completed_started CHECK (completed_at IS NULL OR started_at IS NOT NULL)
 );
 CREATE UNIQUE INDEX organization_erasure_orders_one_open
  ON platform.organization_erasure_orders(organization_id) WHERE withdrawn_at IS NULL;
 COMMENT ON TABLE platform.organization_erasure_orders IS
  'ADR 0092, #4657: erasure order of a Träger with written confirmation and grace period; no names, no FKs. Completed and withdrawn orders expire after three years.';
 REVOKE ALL ON platform.organization_erasure_orders FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
 REVOKE ALL ON SEQUENCE platform.organization_erasure_orders_id_seq FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin, phoenix_school_erasure;
 GRANT SELECT, INSERT ON platform.organization_erasure_orders TO phoenix_admin;
 GRANT UPDATE (withdrawn_by, withdrawn_at) ON platform.organization_erasure_orders TO phoenix_admin;
 GRANT USAGE ON SEQUENCE platform.organization_erasure_orders_id_seq TO phoenix_admin;

 CREATE FUNCTION platform.organization_erasure_check(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
 RETURNS JSONB LANGUAGE plpgsql STABLE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_order platform.organization_erasure_orders%ROWTYPE;
  v_active BOOLEAN;
  v_status TEXT;
 BEGIN
  IF p_org IS NULL OR p_org <= 0 OR p_reference IS NULL THEN
   RAISE EXCEPTION 'organization erasure check requires an organization and a reference' USING ERRCODE = '22023';
  END IF;
  IF NOT EXISTS (SELECT FROM platform.operators WHERE id = p_operator AND active) THEN
   RAISE EXCEPTION 'active operator required' USING ERRCODE = '42501';
  END IF;
  SELECT * INTO v_order FROM platform.organization_erasure_orders
   WHERE organization_id = p_org AND withdrawn_at IS NULL;
  SELECT active INTO v_active FROM platform.organizations WHERE id = p_org;
  v_status := CASE
   WHEN v_order.id IS NULL THEN 'missing'
   WHEN v_order.order_reference <> btrim(p_reference) THEN 'reference_mismatch'
   WHEN v_order.completed_at IS NOT NULL THEN 'completed'
   WHEN v_active IS NULL THEN 'root_missing'
   WHEN v_active THEN 'active'
   WHEN v_order.erase_not_before > (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date THEN 'grace_period'
   ELSE 'due' END;
  RETURN jsonb_build_object(
   'organization_id', p_org, 'order_id', v_order.id, 'status', v_status,
   'erase_not_before', v_order.erase_not_before, 'started_at', v_order.started_at,
   'row_counts', jsonb_build_object(
    'platform.organization_accounts', (SELECT count(*) FROM platform.organization_accounts WHERE organization_id = p_org),
    'auth.organization_account_roles', (SELECT count(*) FROM auth.organization_account_roles WHERE organization_id = p_org),
    'auth.tokens', (SELECT count(*) FROM auth.tokens WHERE organization_id = p_org),
    'platform.schools', (SELECT count(*) FROM platform.schools WHERE organization_id = p_org)));
 END $body$;

 CREATE FUNCTION platform.begin_organization_erasure(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
 RETURNS JSONB LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE
  v_order platform.organization_erasure_orders%ROWTYPE;
  v_active BOOLEAN;
 BEGIN
  IF p_org IS NULL OR p_org <= 0 OR p_operator IS NULL OR p_operator <= 0 OR p_reference IS NULL THEN
   RAISE EXCEPTION 'organization erasure requires an organization, operator and reference' USING ERRCODE = '22023';
  END IF;
  PERFORM 1 FROM platform.operators WHERE id = p_operator AND active FOR KEY SHARE;
  IF NOT FOUND THEN
   RAISE EXCEPTION 'organization erasure requires an active operator' USING ERRCODE = '42501';
  END IF;
  SELECT * INTO v_order FROM platform.organization_erasure_orders
   WHERE organization_id = p_org AND withdrawn_at IS NULL FOR UPDATE;
  IF NOT FOUND OR v_order.order_reference <> btrim(p_reference) THEN
   RAISE EXCEPTION 'organization erasure requires its recorded open order' USING ERRCODE = '22023';
  END IF;
  IF v_order.started_at IS NULL THEN
   IF v_order.erase_not_before > (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date THEN
    RAISE EXCEPTION 'organization erasure grace period has not ended' USING ERRCODE = '22023';
   END IF;
   SELECT active INTO v_active FROM platform.organizations WHERE id = p_org FOR SHARE;
   IF NOT FOUND OR v_active THEN
    RAISE EXCEPTION 'organization erasure requires the deactivated organization' USING ERRCODE = '22023';
   END IF;
   UPDATE platform.organization_erasure_orders SET started_at = clock_timestamp()
    WHERE id = v_order.id RETURNING * INTO v_order;
  END IF;
  RETURN to_jsonb(v_order);
 END $body$;

 CREATE FUNCTION platform.erase_organization(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
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
  v_counts := jsonb_build_object(
   'platform.organizations', 1,
   'platform.organization_accounts', (SELECT count(*) FROM platform.organization_accounts WHERE organization_id = p_org),
   'auth.organization_account_roles', (SELECT count(*) FROM auth.organization_account_roles WHERE organization_id = p_org),
   'auth.tokens', (SELECT count(*) FROM auth.tokens WHERE organization_id = p_org),
   'platform.school_erasures', (SELECT count(*) FROM platform.school_erasures
    WHERE organization_id = p_org AND order_reference = v_order.order_reference AND status = 'completed'));
  -- The FKs of memberships, Träger roles and Träger sessions cascade.
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

 CREATE FUNCTION platform.delete_expired_organization_erasure_orders()
 RETURNS BIGINT LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path = pg_catalog
 AS $body$
 DECLARE v_count BIGINT;
 BEGIN
  DELETE FROM platform.organization_erasure_orders
   WHERE COALESCE(completed_at, withdrawn_at) < clock_timestamp() - interval '3 years';
  GET DIAGNOSTICS v_count = ROW_COUNT;
  RETURN v_count;
 END $body$;

 REVOKE ALL ON FUNCTION platform.organization_erasure_check(BIGINT,BIGINT,TEXT), platform.begin_organization_erasure(BIGINT,BIGINT,TEXT),
  platform.erase_organization(BIGINT,BIGINT,TEXT), platform.delete_expired_organization_erasure_orders()
  FROM PUBLIC, phoenix_auth, phoenix_tenant, phoenix_admin;
 GRANT EXECUTE ON FUNCTION platform.organization_erasure_check(BIGINT,BIGINT,TEXT), platform.begin_organization_erasure(BIGINT,BIGINT,TEXT),
  platform.erase_organization(BIGINT,BIGINT,TEXT), platform.delete_expired_organization_erasure_orders() TO phoenix_school_erasure;
`

func traegerErasureOrderUp(ctx context.Context, db *bun.DB) error {
	if _, err := db.ExecContext(ctx, traegerErasureOrderSQL); err != nil {
		return fmt.Errorf("create Träger erasure order: %w", err)
	}
	return nil
}

func traegerErasureOrderDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// An order is evidence of a confirmation or an erasure; never discard it.
		var count int
		if err := tx.NewRaw(`SELECT count(*) FROM platform.organization_erasure_orders`).Scan(ctx, &count); err != nil {
			return err
		}
		if count != 0 {
			return fmt.Errorf("organization erasure order rollback would discard evidence; use a forward migration")
		}
		_, err := tx.ExecContext(ctx, `
   DROP FUNCTION platform.delete_expired_organization_erasure_orders();
   DROP FUNCTION platform.erase_organization(BIGINT,BIGINT,TEXT);
   DROP FUNCTION platform.begin_organization_erasure(BIGINT,BIGINT,TEXT);
   DROP FUNCTION platform.organization_erasure_check(BIGINT,BIGINT,TEXT);
   DROP TABLE platform.organization_erasure_orders;
  `)
		return err
	})
}
