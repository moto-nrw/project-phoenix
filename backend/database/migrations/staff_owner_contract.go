package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
)

// The staff Contract (#2754) follows the student (#2760) and request-child
// (#2719) Contracts: deployment stops the application and verifies a complete
// release backup, preflight asks the integrity questions before downtime, and
// the same checks run again under locks right before the destructive DDL.
//
// It removes only the rollback shape the cutover (#2753) added: the
// users.staff view with its routing trigger, the users.staff_legacy archive
// with its owned id sequence, and both hit counters. The personnel-number
// trigger, the membership updated_at trigger and the index names the owners
// classify conflicts by are current behaviour and stay.

func staffOwnerContractPrecondition(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return nil // The replay has not created the staff schema yet.
	}
	return staffOwnerContractDataPreflight(ctx, db)
}

func staffOwnerContractUp(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return contractStaffOwnerStorageChecked(ctx, db, func(ctx context.Context, connection bun.IDB) error {
			var occupied bool
			err := connection.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.staff_legacy)
				OR EXISTS (SELECT 1 FROM users.staff_school_memberships)
				OR EXISTS (SELECT 1 FROM users.staff_employment_profiles)`).Scan(ctx, &occupied)
			if err != nil {
				return fmt.Errorf("staff contract: verify empty initial replay: %w", err)
			}
			if occupied {
				return errors.New("staff contract: initial replay contains staff data")
			}
			return nil
		})
	}
	return contractStaffOwnerStorageChecked(ctx, db, nil)
}

func staffOwnerContractDown(context.Context, *bun.DB) error {
	return errors.New("staff owner Contract is irreversible: restore the verified pre-Contract backup and its prior application image together; automatic Down cannot recreate removed rollback storage")
}

func contractStaffOwnerStorageChecked(ctx context.Context, db *bun.DB, check func(context.Context, bun.IDB) error) error {
	if db == nil {
		return errors.New("staff contract: database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	release, err := lockStorageBackfill(ctx, db, StaffOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE users.staff IN ACCESS EXCLUSIVE MODE;
			LOCK TABLE users.staff_legacy, users.staff_school_memberships,
				users.staff_employment_profiles IN ACCESS EXCLUSIVE MODE;
		`); err != nil {
			return fmt.Errorf("staff contract: lock storage: %w", err)
		}
		if err := staffOwnerContractDataPreflight(ctx, tx); err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, tx); err != nil {
				return err
			}
		}
		before, err := staffContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		// RESTRICT is intentional. An unknown dependency must abort and roll
		// back even an earlier DROP, never be silently removed by CASCADE.
		// The archive's owned id sequence goes with the archive.
		if _, err := tx.ExecContext(ctx, `
			DROP VIEW users.staff RESTRICT;
			DROP FUNCTION users.route_staff_compatibility() RESTRICT;
			DROP TABLE users.staff_legacy RESTRICT;
			DROP SEQUENCE users.staff_compatibility_reads, users.staff_compatibility_writes RESTRICT;
		`); err != nil {
			return fmt.Errorf("staff contract: remove obsolete storage: %w", err)
		}
		after, err := staffContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return errors.New("staff contract: owner rows or checksums changed during Contract")
		}
		return nil
	})
}

type staffContractFingerprint struct {
	Object   string
	TenantID int64
	Rows     int64
	Checksum string
}

// Every target column participates. Grouping by school prevents totals from
// masking tenant drift.
func staffContractFingerprints(ctx context.Context, db bun.IDB) ([]staffContractFingerprint, error) {
	var result []staffContractFingerprint
	if err := db.NewRaw(`
		SELECT 'users.staff_school_memberships' AS object, tenant_id, count(*) AS rows,
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex') AS checksum
		FROM users.staff_school_memberships r GROUP BY tenant_id
		UNION ALL
		SELECT 'users.staff_employment_profiles', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM users.staff_employment_profiles r GROUP BY tenant_id
		ORDER BY object, tenant_id
	`).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("staff contract: fingerprint owner storage: %w", err)
	}
	return result, nil
}

// staffOwnerContractDataPreflight never selects the compatibility view or
// changes rows/counters. It runs again under the Contract lock: a
// pre-deployment observation alone cannot exclude writes between preflight
// and DDL.
func staffOwnerContractDataPreflight(ctx context.Context, db bun.IDB) error {
	var ready bool
	if err := db.NewRaw(`SELECT
		coalesce((SELECT relkind = 'v' FROM pg_class WHERE oid = to_regclass('users.staff')), false)
		AND coalesce((SELECT relkind = 'r' FROM pg_class WHERE oid = to_regclass('users.staff_legacy')), false)
		AND to_regclass('users.staff_compatibility_reads') IS NOT NULL
		AND to_regclass('users.staff_compatibility_writes') IS NOT NULL
		AND to_regprocedure('users.route_staff_compatibility()') IS NOT NULL`).Scan(ctx, &ready); err != nil {
		return fmt.Errorf("staff contract: inspect cutover schema: %w", err)
	}
	if !ready {
		return errors.New("staff contract: requires the completed cutover with its compatibility schema")
	}
	// Historical compatibility counters are diagnostic, not a cleanup gate.
	// Deployment stops the old application before migration and restores the
	// complete release backup on failure; only current integrity matters here.
	var functionName string
	if err := db.NewRaw(`SELECT coalesce((SELECT p.oid::regprocedure::text
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		AND replace(p.prosrc, '"', '') ~* '(^|[^[:alnum:]_])users[.]staff(_legacy)?([^[:alnum:]_]|$)'
		AND p.oid <> 'users.route_staff_compatibility()'::regprocedure
		ORDER BY p.oid LIMIT 1), '')`).Scan(ctx, &functionName); err != nil {
		return fmt.Errorf("staff contract: inspect stored function callers: %w", err)
	}
	if functionName != "" {
		return fmt.Errorf("staff contract: stored function still references retired storage: %s", functionName)
	}
	var counts [9]int64
	if err := db.NewRaw(`SELECT
		(SELECT count(*) FROM pg_depend d JOIN pg_rewrite r ON r.oid = d.objid
			WHERE d.classid = 'pg_rewrite'::regclass
			AND d.refobjid IN ('users.staff'::regclass, 'users.staff_legacy'::regclass)
			AND r.ev_class <> 'users.staff'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND confrelid = 'users.staff_legacy'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND NOT convalidated AND
			(confrelid IN ('users.staff_school_memberships'::regclass, 'users.staff_employment_profiles'::regclass)
			 OR conrelid IN ('users.staff_school_memberships'::regclass, 'users.staff_employment_profiles'::regclass))),
		(SELECT count(*) FROM pg_class
			WHERE oid IN ('users.staff_school_memberships'::regclass, 'users.staff_employment_profiles'::regclass)
			AND (NOT relrowsecurity OR NOT relforcerowsecurity)),
		(SELECT count(*) FROM users.staff_school_memberships m
			WHERE NOT EXISTS (SELECT FROM users.staff_employment_profiles p
				WHERE p.tenant_id = m.tenant_id AND p.membership_id = m.id)),
		(SELECT count(*) FROM users.staff_employment_profiles p
			WHERE NOT EXISTS (SELECT FROM users.staff_school_memberships m
				WHERE m.tenant_id = p.tenant_id AND m.id = p.membership_id)),
		(SELECT count(*) FROM users.staff_school_memberships m
			WHERE NOT EXISTS (SELECT FROM users.persons p WHERE p.tenant_id = m.tenant_id AND p.id = m.person_id)),
		(SELECT count(*) FROM (
			SELECT tenant_id, person_id FROM users.staff_school_memberships
			WHERE deleted_at IS NULL GROUP BY tenant_id, person_id HAVING count(*) > 1
		) duplicates),
		(SELECT count(*) FROM (
			SELECT p.tenant_id, p.personnel_number FROM users.staff_employment_profiles p
			JOIN users.staff_school_memberships m ON m.tenant_id = p.tenant_id AND m.id = p.membership_id
			WHERE p.personnel_number IS NOT NULL AND m.deleted_at IS NULL
			GROUP BY p.tenant_id, p.personnel_number HAVING count(*) > 1
		) duplicates)
	`).Scan(ctx, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6], &counts[7], &counts[8]); err != nil {
		return fmt.Errorf("staff contract: inspect owner integrity: %w", err)
	}
	for index, name := range []string{
		"unexpected views depend on retired storage", "foreign keys still referencing archive",
		"unvalidated owner foreign keys", "owner RLS disabled",
		"memberships without employment profiles", "employment profiles without memberships",
		"memberships without persons", "duplicate live memberships", "duplicate live personnel numbers",
	} {
		if counts[index] != 0 {
			return fmt.Errorf("staff contract: %s: %d", name, counts[index])
		}
	}
	return nil
}
