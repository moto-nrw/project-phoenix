package migrations

import (
	"context"
	"errors"
	"fmt"

	"github.com/uptrace/bun"
)

const traegerRecordsHandoverVersion = "1.15.870"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     traegerRecordsHandoverVersion,
		Description: "Löschauftrag eines Trägers: Hinweis auf aufbewahrungspflichtige Unterlagen und Rückgabe oder Wahl der Löschung (#4720)",
		DependsOn: []string{
			familyChildrenLinkAndEnrollmentVersion, // the newest migration on development
			traegerErasureOrderVersion,             // the erasure order the hint belongs to
		},
	})
	Migrations.MustRegister(traegerRecordsHandoverUp, traegerRecordsHandoverDown)
}

// traegerRecordsHandoverSQL records, on the erasure order of a Träger, that
// the moto team pointed the Träger to its records subject to retention and
// what the customer chose (#4720 Slice 3, AVV Ziff. 8.2 and 8.4):
//
//   - handed_over: the Rückgabe export was handed over on records_handed_over_on;
//     records_export_sha256 optionally names the COMPLETE checksum of the
//     handed-over export.
//   - erasure_chosen: the customer chose lawful erasure after the hint. No
//     handover day, no export.
//
// Who recorded it is recorded_by, when recorded_at. The values carry no
// personal data. The application requires both for every new order. Orders
// recorded before this migration have neither, so the erasure procedure
// refuses to start, resume or complete them: begin_organization_erasure
// checks the choice even on a started order, and the trigger below refuses
// every start or completion without it. organization_erasure_check reports
// records_choice_missing. The operator records the hint and the choice on
// the open order afterwards (phoenix_admin may set the four columns); a
// started order cannot be withdrawn. Once recorded, the choice is evidence
// and never changes.
const traegerRecordsHandoverSQL = `
ALTER TABLE platform.organization_erasure_orders
	ADD COLUMN records_notice_shown_at TIMESTAMPTZ,
	ADD COLUMN records_choice VARCHAR(16),
	ADD COLUMN records_handed_over_on DATE,
	ADD COLUMN records_export_sha256 CHAR(64),
	ADD CONSTRAINT chk_organization_erasure_orders_records_choice
		CHECK (records_choice IS NULL OR records_choice IN ('handed_over', 'erasure_chosen')),
	ADD CONSTRAINT chk_organization_erasure_orders_records_notice
		CHECK ((records_choice IS NULL) = (records_notice_shown_at IS NULL)),
	ADD CONSTRAINT chk_organization_erasure_orders_records_handover
		CHECK ((records_choice = 'handed_over') = (records_handed_over_on IS NOT NULL)),
	ADD CONSTRAINT chk_organization_erasure_orders_records_export
		CHECK (records_export_sha256 IS NULL OR (records_choice = 'handed_over' AND records_export_sha256 ~ '^[0-9a-f]{64}$'));
COMMENT ON COLUMN platform.organization_erasure_orders.records_choice IS
	'#4720, AVV Ziff. 8.4: after the hint on records subject to retention the customer took over the Rückgabe (handed_over) or chose lawful erasure (erasure_chosen).';
COMMENT ON COLUMN platform.organization_erasure_orders.records_export_sha256 IS
	'#4720: SHA-256 in the COMPLETE file of the handed-over Rückgabe export (tenant export --organization); no contents.';

GRANT UPDATE (records_notice_shown_at, records_choice, records_handed_over_on, records_export_sha256)
	ON platform.organization_erasure_orders TO phoenix_admin;

CREATE FUNCTION platform.refuse_organization_erasure_without_records_choice() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $body$
BEGIN
 IF OLD.records_choice IS NOT NULL AND ROW(NEW.records_notice_shown_at, NEW.records_choice, NEW.records_handed_over_on, NEW.records_export_sha256)
   IS DISTINCT FROM ROW(OLD.records_notice_shown_at, OLD.records_choice, OLD.records_handed_over_on, OLD.records_export_sha256) THEN
  RAISE EXCEPTION 'the recorded records choice of an erasure order never changes (#4720)' USING ERRCODE = '22023';
 END IF;
 IF NEW.started_at IS NOT NULL AND NEW.records_choice IS NULL
   AND (OLD.started_at IS DISTINCT FROM NEW.started_at OR OLD.completed_at IS DISTINCT FROM NEW.completed_at) THEN
  RAISE EXCEPTION 'organization erasure requires the records hint and the customer choice on its order (#4720)' USING ERRCODE = '22023';
 END IF;
 RETURN NEW;
END $body$;
CREATE TRIGGER trg_organization_erasure_orders_records_choice
 BEFORE UPDATE OF started_at, completed_at, records_notice_shown_at, records_choice, records_handed_over_on, records_export_sha256
 ON platform.organization_erasure_orders
 FOR EACH ROW EXECUTE FUNCTION platform.refuse_organization_erasure_without_records_choice();
`

// The check and the begin of migration 1.15.533 and, with the status
// records_choice_missing and the refusal for an open order without a
// recorded choice, started or not, those of this migration. Constants keep the SQL
// statically resolvable for the architecture check.
const (
	organizationErasureCheckHeadSQL = `
 CREATE OR REPLACE FUNCTION platform.organization_erasure_check(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
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
   WHEN v_order.erase_not_before > (clock_timestamp() AT TIME ZONE 'Europe/Berlin')::date THEN 'grace_period'`
	organizationErasureCheckTailSQL = `
   ELSE 'due' END;
  RETURN jsonb_build_object(
   'organization_id', p_org, 'order_id', v_order.id, 'status', v_status,
   'erase_not_before', v_order.erase_not_before, 'started_at', v_order.started_at,
   'row_counts', jsonb_build_object(
    'platform.organization_accounts', (SELECT count(*) FROM platform.organization_accounts WHERE organization_id = p_org),
    'auth.organization_account_roles', (SELECT count(*) FROM auth.organization_account_roles WHERE organization_id = p_org),
    'auth.tokens', (SELECT count(*) FROM auth.tokens WHERE organization_id = p_org),
    'platform.schools', (SELECT count(*) FROM platform.schools WHERE organization_id = p_org)),
   -- Schools this order already erased, for every operator: a repeated run
   -- replays their file phase after an upload restore.
   'school_ids', (SELECT COALESCE(jsonb_agg(school_id ORDER BY school_id), '[]'::jsonb)
    FROM platform.school_erasures
    WHERE organization_id = p_org AND order_reference = v_order.order_reference));
 END $body$;`
	organizationErasureCheckRecordsSQL = organizationErasureCheckHeadSQL + `
   WHEN v_order.records_choice IS NULL THEN 'records_choice_missing'` + organizationErasureCheckTailSQL
	organizationErasureCheckLegacySQL = organizationErasureCheckHeadSQL + organizationErasureCheckTailSQL

	organizationErasureBeginHeadSQL = `
 CREATE OR REPLACE FUNCTION platform.begin_organization_erasure(p_org BIGINT, p_operator BIGINT, p_reference TEXT)
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
  END IF;`
	organizationErasureBeginTailSQL = `
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
 END $body$;`
	organizationErasureBeginRecordsSQL = organizationErasureBeginHeadSQL + `
  -- Started or not: no school is erased without the customer's choice (#4720).
  IF v_order.records_choice IS NULL THEN
   RAISE EXCEPTION 'organization erasure requires the records hint and the customer choice on its order (#4720)' USING ERRCODE = '22023';
  END IF;` + organizationErasureBeginTailSQL
	organizationErasureBeginLegacySQL = organizationErasureBeginHeadSQL + organizationErasureBeginTailSQL
)

func traegerRecordsHandoverUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, traegerRecordsHandoverSQL); err != nil {
			return fmt.Errorf("add Träger records handover: %w", err)
		}
		// CREATE OR REPLACE keeps the owner, the grants and the revokes.
		if _, err := tx.ExecContext(ctx, organizationErasureCheckRecordsSQL); err != nil {
			return fmt.Errorf("check the Träger records choice before erasure: %w", err)
		}
		if _, err := tx.ExecContext(ctx, organizationErasureBeginRecordsSQL); err != nil {
			return fmt.Errorf("require the Träger records choice to begin or resume erasure: %w", err)
		}
		return nil
	})
}

func traegerRecordsHandoverDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// A recorded choice is evidence of the hint and the handover; never discard it.
		var count int
		if err := tx.NewRaw(`SELECT count(*) FROM platform.organization_erasure_orders WHERE records_choice IS NOT NULL`).Scan(ctx, &count); err != nil {
			return err
		}
		if count != 0 {
			return errors.New("records handover rollback would discard evidence; use a forward migration")
		}
		if _, err := tx.ExecContext(ctx, organizationErasureCheckLegacySQL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, organizationErasureBeginLegacySQL); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DROP TRIGGER trg_organization_erasure_orders_records_choice ON platform.organization_erasure_orders;
			DROP FUNCTION platform.refuse_organization_erasure_without_records_choice()`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `ALTER TABLE platform.organization_erasure_orders
			DROP COLUMN records_export_sha256,
			DROP COLUMN records_handed_over_on,
			DROP COLUMN records_choice,
			DROP COLUMN records_notice_shown_at`)
		return err
	})
}
