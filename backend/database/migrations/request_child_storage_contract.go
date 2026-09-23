package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
)

// The request-child Contract (#2719) follows the student Contract (#2760):
// deployment stops the application and verifies a complete release backup,
// preflight asks the integrity questions before downtime, and the same checks
// run again under locks right before the destructive DDL.

func requestChildStorageContractPrecondition(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return nil // The replay has not created the enrollment schema yet.
	}
	return requestChildStorageContractDataPreflight(ctx, db)
}

func requestChildStorageContractUp(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return contractRequestChildStorageChecked(ctx, db, func(ctx context.Context, connection bun.IDB) error {
			var occupied bool
			err := connection.NewRaw(`SELECT EXISTS (SELECT 1 FROM enrollment.request_child_offerings_legacy)
				OR EXISTS (SELECT 1 FROM enrollment.care_offering_bookings)
				OR EXISTS (SELECT 1 FROM enrollment.request_child_offering_selections)`).Scan(ctx, &occupied)
			if err != nil {
				return fmt.Errorf("request child contract: verify empty initial replay: %w", err)
			}
			if occupied {
				return errors.New("request child contract: initial replay contains request-child data")
			}
			return nil
		})
	}
	return contractRequestChildStorageChecked(ctx, db, nil)
}

func requestChildStorageContractDown(context.Context, *bun.DB) error {
	return errors.New("request child Contract is irreversible: restore the verified pre-Contract backup and its prior application image together; automatic Down cannot recreate removed rollback storage")
}

func contractRequestChildStorageChecked(ctx context.Context, db *bun.DB, check func(context.Context, bun.IDB) error) error {
	if db == nil {
		return errors.New("request child contract: database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	release, err := lockStorageBackfill(ctx, db, requestChildStorageBackfillName)
	if err != nil {
		return err
	}
	defer release()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE enrollment.request_child_offerings IN ACCESS EXCLUSIVE MODE;
			LOCK TABLE enrollment.request_child_offerings_legacy, enrollment.care_offering_bookings,
				enrollment.request_child_offering_selections IN ACCESS EXCLUSIVE MODE;
		`); err != nil {
			return fmt.Errorf("request child contract: lock storage: %w", err)
		}
		if err := requestChildStorageContractDataPreflight(ctx, tx); err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, tx); err != nil {
				return err
			}
		}
		before, err := requestChildContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		// RESTRICT is intentional. An unknown dependency must abort and roll
		// back even an earlier DROP, never be silently removed by CASCADE.
		// The archive's owned id sequence goes with the archive.
		if _, err := tx.ExecContext(ctx, `
			DROP VIEW enrollment.request_child_offerings RESTRICT;
			DROP FUNCTION enrollment.route_request_child_offering_compatibility() RESTRICT;
			DROP FUNCTION enrollment.request_child_legacy_manual(jsonb, jsonb, jsonb) RESTRICT;
			DROP FUNCTION enrollment.request_child_effective_days(jsonb, jsonb) RESTRICT;
			DROP TABLE enrollment.request_child_offerings_legacy RESTRICT;
			DROP SEQUENCE enrollment.request_child_compatibility_reads, enrollment.request_child_compatibility_writes RESTRICT;
		`); err != nil {
			return fmt.Errorf("request child contract: remove obsolete storage: %w", err)
		}
		after, err := requestChildContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return errors.New("request child contract: owner rows or checksums changed during Contract")
		}
		return nil
	})
}

type requestChildContractFingerprint struct {
	Object   string
	TenantID int64
	Rows     int64
	Checksum string
}

// Every target column participates. Grouping by school prevents totals from
// masking tenant drift.
func requestChildContractFingerprints(ctx context.Context, db bun.IDB) ([]requestChildContractFingerprint, error) {
	var result []requestChildContractFingerprint
	if err := db.NewRaw(`
		SELECT 'enrollment.request_child_offering_selections' AS object, tenant_id, count(*) AS rows,
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex') AS checksum
		FROM enrollment.request_child_offering_selections r GROUP BY tenant_id
		UNION ALL
		SELECT 'enrollment.care_offering_bookings', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM enrollment.care_offering_bookings r GROUP BY tenant_id
		ORDER BY object, tenant_id
	`).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("request child contract: fingerprint owner storage: %w", err)
	}
	return result, nil
}

// requestChildStorageContractDataPreflight never selects the compatibility
// view or changes rows/counters. It runs again under the Contract lock: a
// pre-deployment observation alone cannot exclude writes between preflight
// and DDL.
func requestChildStorageContractDataPreflight(ctx context.Context, db bun.IDB) error {
	var ready bool
	if err := db.NewRaw(`SELECT
		(SELECT relkind = 'v' FROM pg_class WHERE oid = to_regclass('enrollment.request_child_offerings'))
		AND (SELECT relkind = 'r' FROM pg_class WHERE oid = to_regclass('enrollment.request_child_offerings_legacy'))
		AND to_regclass('enrollment.request_child_compatibility_reads') IS NOT NULL
		AND to_regclass('enrollment.request_child_compatibility_writes') IS NOT NULL
		AND to_regprocedure('enrollment.route_request_child_offering_compatibility()') IS NOT NULL`).Scan(ctx, &ready); err != nil {
		return fmt.Errorf("request child contract: inspect cutover schema: %w", err)
	}
	if !ready {
		return errors.New("request child contract: requires the completed cutover with its compatibility schema")
	}
	// Historical compatibility counters are diagnostic, not a cleanup gate.
	// Deployment stops the old application before migration and restores the
	// complete release backup on failure; only current integrity matters here.
	var functionName string
	if err := db.NewRaw(`SELECT coalesce((SELECT p.oid::regprocedure::text
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		AND replace(p.prosrc, '"', '') ~* '(^|[^[:alnum:]_])enrollment[.]request_child_offerings(_legacy)?([^[:alnum:]_]|$)'
		AND p.oid <> 'enrollment.route_request_child_offering_compatibility()'::regprocedure
		ORDER BY p.oid LIMIT 1), '')`).Scan(ctx, &functionName); err != nil {
		return fmt.Errorf("request child contract: inspect stored function callers: %w", err)
	}
	if functionName != "" {
		return fmt.Errorf("request child contract: stored function still references retired storage: %s", functionName)
	}
	var counts [8]int64
	if err := db.NewRaw(`SELECT
		(SELECT count(*) FROM pg_depend d JOIN pg_rewrite r ON r.oid = d.objid
			WHERE d.classid = 'pg_rewrite'::regclass
			AND d.refobjid IN ('enrollment.request_child_offerings'::regclass, 'enrollment.request_child_offerings_legacy'::regclass)
			AND r.ev_class <> 'enrollment.request_child_offerings'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND confrelid = 'enrollment.request_child_offerings_legacy'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND NOT convalidated AND
			(confrelid IN ('enrollment.request_child_offering_selections'::regclass, 'enrollment.care_offering_bookings'::regclass)
			 OR conrelid IN ('enrollment.request_child_offering_selections'::regclass, 'enrollment.care_offering_bookings'::regclass))),
		(SELECT count(*) FROM pg_class
			WHERE oid IN ('enrollment.request_child_offering_selections'::regclass, 'enrollment.care_offering_bookings'::regclass)
			AND (NOT relrowsecurity OR NOT relforcerowsecurity)),
		(SELECT count(*) FROM enrollment.care_offering_bookings b
			WHERE NOT EXISTS (SELECT FROM enrollment.request_children c WHERE c.tenant_id = b.tenant_id AND c.id = b.request_child_id)),
		(SELECT count(*) FROM enrollment.request_child_offering_selections s
			WHERE NOT EXISTS (SELECT FROM enrollment.request_children c WHERE c.tenant_id = s.tenant_id AND c.id = s.request_child_id)),
		(SELECT count(*) FROM enrollment.care_offering_bookings b
			WHERE NOT EXISTS (SELECT FROM enrollment.care_offerings o WHERE o.tenant_id = b.tenant_id AND o.id = b.care_offering_id)),
		(SELECT count(*) FROM enrollment.request_child_offering_selections s
			WHERE NOT EXISTS (SELECT FROM enrollment.care_offerings o WHERE o.tenant_id = s.tenant_id AND o.id = s.care_offering_id))
	`).Scan(ctx, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6], &counts[7]); err != nil {
		return fmt.Errorf("request child contract: inspect owner integrity: %w", err)
	}
	for index, name := range []string{
		"unexpected views depend on retired storage", "foreign keys still referencing archive",
		"unvalidated owner foreign keys", "owner RLS disabled",
		"bookings without request children", "selections without request children",
		"bookings without care offerings", "selections without care offerings",
	} {
		if counts[index] != 0 {
			return fmt.Errorf("request child contract: %s: %d", name, counts[index])
		}
	}
	return nil
}
