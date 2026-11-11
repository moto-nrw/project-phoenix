package migrations

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

const dataReturnErasureOrderLockVersion = "1.15.919"

func init() {
	MigrationRegistry.Register(&Migration{
		Version:     dataReturnErasureOrderLockVersion,
		Description: "Datenrückgabe: Freigabe eines Trägerpakets und Start der Trägerlöschung sperren sich gegenseitig (#6665, ADR 0203)",
		DependsOn: []string{
			dataReturnGrantsVersion,       // data_return_issue and data_return_open_for; the newest on development (1.15.917)
			traegerRecordsHandoverVersion, // begin_organization_erasure as it stands
		},
	})
	Migrations.MustRegister(dataReturnErasureOrderLockUp, dataReturnErasureOrderLockDown)
}

// The release of a Träger package and the start of the Träger's erasure
// meet on the open erasure order (#6665). Before, the release read the order
// without a lock and the begin did not look for packages, so a release
// between the CLI's package check and the begin left a started erasure with
// a retrievable package.
//
//   - The release locks the open order FOR SHARE before it reads the start.
//     A begin that holds the order FOR UPDATE makes it wait; the check that
//     follows then sees the start and refuses.
//   - The begin counts the unpurged packages of the Träger once it holds the
//     order FOR UPDATE, started or not. A release that held the order makes
//     it wait; the count then sees the package and refuses with 55006, the
//     code of the delete trigger.
//
// The migration carries both functions whole, because the architecture
// check resolves the SQL of a migration from its own file. Without the two
// added blocks they are the functions of 1.15.917 and 1.15.879, which the
// rollback restores.
//
// Both functions are VOLATILE: under READ COMMITTED the statement after the
// lock reads what the other transaction committed. The release locks the
// order before the Träger row, like begin_organization_erasure and
// erase_organization, so it cannot deadlock with either.
const (
	// data_return_issue of migration 1.15.917 up to its release branch.
	dataReturnIssueOrderLockHeadSQL = `
CREATE OR REPLACE FUNCTION platform.data_return_issue(
	p_operator BIGINT, p_action TEXT, p_package TEXT, p_subject BIGINT, p_archive_name TEXT,
	p_archive_bytes BIGINT, p_archive_sha256 TEXT, p_manifest_sha256 TEXT, p_format_version INTEGER,
	p_key_check TEXT, p_recipient TEXT, p_recipient_name TEXT, p_token_lookup TEXT, p_wrapped_key BYTEA,
	p_expires_at TIMESTAMPTZ, p_keep_until TIMESTAMPTZ, p_max_downloads INTEGER)
RETURNS BIGINT LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, platform, pg_temp AS $function$
DECLARE
	v_id BIGINT;
	v_head platform.data_return_grants%ROWTYPE;
BEGIN
	PERFORM platform.data_return_require_operator(p_operator);
	PERFORM platform.data_return_lock_archive(p_archive_name);
	IF p_action = 'release' THEN
`
	dataReturnIssueOrderLockSQL = `		IF p_package = 'organization' THEN
			PERFORM 1 FROM platform.organization_erasure_orders
				WHERE organization_id = p_subject AND withdrawn_at IS NULL FOR SHARE;
		END IF;
`
	dataReturnIssueOrderLockTailSQL = `		IF p_package = 'school' THEN
			PERFORM 1 FROM platform.schools WHERE id = p_subject FOR KEY SHARE;
		ELSIF p_package = 'organization' THEN
			PERFORM 1 FROM platform.organizations WHERE id = p_subject FOR KEY SHARE;
		END IF;
		IF NOT FOUND THEN
			RAISE EXCEPTION 'data return: the school or Träger does not exist or is erased' USING ERRCODE = '55000';
		END IF;
		-- An erasure that has begun is no less final than one that is done:
		-- a school with an erasure order, a Träger whose order has started.
		-- A recorded Träger order in its grace period does not block: the
		-- customer may need the return then.
		IF p_package = 'school' AND EXISTS (SELECT 1 FROM platform.school_erasures WHERE school_id = p_subject)
			OR p_package = 'organization' AND EXISTS (SELECT 1 FROM platform.organization_erasure_orders
				WHERE organization_id = p_subject AND started_at IS NOT NULL AND withdrawn_at IS NULL) THEN
			RAISE EXCEPTION 'data return: the erasure of the school or Träger has begun' USING ERRCODE = '55000';
		END IF;
		IF EXISTS (SELECT 1 FROM platform.data_return_grants WHERE archive_name = p_archive_name) THEN
			RAISE EXCEPTION 'data return: archive already released' USING ERRCODE = '23505';
		END IF;
		IF p_keep_until <= now() OR p_keep_until > now() + interval '366 days' THEN
			RAISE EXCEPTION 'data return: the retrieval period ends within a year' USING ERRCODE = '22023';
		END IF;
		v_head.package := p_package; v_head.subject_id := p_subject; v_head.archive_name := p_archive_name;
		v_head.archive_bytes := p_archive_bytes; v_head.archive_sha256 := p_archive_sha256;
		v_head.manifest_sha256 := p_manifest_sha256; v_head.format_version := p_format_version;
		v_head.key_check := p_key_check; v_head.keep_until := p_keep_until;
	ELSIF p_action = 'grant' THEN
		SELECT * INTO v_head FROM platform.data_return_grants WHERE archive_name = p_archive_name
			AND purged_at IS NULL AND keep_until > now() AND key_check = p_key_check ORDER BY id LIMIT 1;
		IF NOT FOUND THEN
			RAISE EXCEPTION 'data return: archive purged, expired or unknown' USING ERRCODE = '55000';
		END IF;
	ELSE
		RAISE EXCEPTION 'data return: unknown action' USING ERRCODE = '22023';
	END IF;
	IF p_expires_at <= now() OR p_expires_at > now() + interval '30 days' OR p_expires_at > v_head.keep_until THEN
		RAISE EXCEPTION 'data return: a token lives up to 30 days, within the retrieval period' USING ERRCODE = '22023';
	END IF;
	INSERT INTO platform.data_return_grants (package, subject_id, archive_name, archive_bytes, archive_sha256,
		manifest_sha256, format_version, key_check, recipient, recipient_name, token_lookup, wrapped_key,
		issued_by, expires_at, keep_until, max_downloads)
	VALUES (v_head.package, v_head.subject_id, v_head.archive_name, v_head.archive_bytes, v_head.archive_sha256,
		v_head.manifest_sha256, v_head.format_version, v_head.key_check, p_recipient, p_recipient_name,
		p_token_lookup, p_wrapped_key, p_operator, p_expires_at, v_head.keep_until, p_max_downloads)
	RETURNING id INTO v_id;
	PERFORM platform.data_return_audit(p_operator, p_action, v_head.package, v_head.subject_id, jsonb_build_object(
		'grant_id', v_id, 'archive_name', v_head.archive_name, 'manifest_sha256', v_head.manifest_sha256,
		'recipient', p_recipient, 'expires_at', p_expires_at, 'keep_until', v_head.keep_until));
	RETURN v_id;
END $function$;`
	dataReturnIssueWithOrderLockSQL    = dataReturnIssueOrderLockHeadSQL + dataReturnIssueOrderLockSQL + dataReturnIssueOrderLockTailSQL
	dataReturnIssueWithoutOrderLockSQL = dataReturnIssueOrderLockHeadSQL + dataReturnIssueOrderLockTailSQL

	// begin_organization_erasure of migration 1.15.879 up to its start.
	organizationErasureBeginOrderLockedSQL = `
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
  END IF;
  -- Started or not: no school is erased without the customer's choice (#4720).
  IF v_order.records_choice IS NULL THEN
   RAISE EXCEPTION 'organization erasure requires the records hint and the customer choice on its order (#4720)' USING ERRCODE = '22023';
  END IF;`
	organizationErasureBeginDataReturnCheckSQL = `
  IF platform.data_return_open_for('organization', p_org) > 0 THEN
   RAISE EXCEPTION 'organization erasure: the Träger has a released return package that is not purged' USING ERRCODE = '55006';
  END IF;`
	organizationErasureBeginStartSQL = `
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
	organizationErasureBeginDataReturnSQL        = organizationErasureBeginOrderLockedSQL + organizationErasureBeginDataReturnCheckSQL + organizationErasureBeginStartSQL
	organizationErasureBeginWithoutDataReturnSQL = organizationErasureBeginOrderLockedSQL + organizationErasureBeginStartSQL
)

// CREATE OR REPLACE keeps the owner, the grants and the revokes.
func dataReturnErasureOrderLockUp(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, dataReturnIssueWithOrderLockSQL); err != nil {
			return fmt.Errorf("lock the erasure order in data_return_issue: %w", err)
		}
		if _, err := tx.ExecContext(ctx, organizationErasureBeginDataReturnSQL); err != nil {
			return fmt.Errorf("check return packages in begin_organization_erasure: %w", err)
		}
		return nil
	})
}

func dataReturnErasureOrderLockDown(ctx context.Context, db *bun.DB) error {
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, dataReturnIssueWithoutOrderLockSQL); err != nil {
			return fmt.Errorf("restore data_return_issue: %w", err)
		}
		if _, err := tx.ExecContext(ctx, organizationErasureBeginWithoutDataReturnSQL); err != nil {
			return fmt.Errorf("restore begin_organization_erasure: %w", err)
		}
		return nil
	})
}
