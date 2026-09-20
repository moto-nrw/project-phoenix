package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
)

func studentOwnerContractDown(context.Context, *bun.DB) error {
	return errors.New("student owner Contract is irreversible: restore the verified pre-Contract backup and its prior application image together; automatic Down cannot recreate removed rollback storage")
}

// contractStudentOwnerStorageWithEvidence checks operational evidence before
// acquiring locks, then checks it again inside the destructive transaction.
func contractStudentOwnerStorageWithEvidence(ctx context.Context, db *bun.DB, evidence StudentContractEvidence, policy StudentContractPolicy, release string) error {
	if db == nil {
		return errors.New("student contract: database is required")
	}
	check := func(ctx context.Context, connection bun.IDB) error {
		return validateStudentContractLiveEvidence(ctx, connection, evidence, policy, release, time.Now().UTC())
	}
	if err := check(ctx, db); err != nil {
		return err
	}
	return contractStudentOwnerStorageChecked(ctx, db, check)
}

func contractStudentOwnerStorageChecked(ctx context.Context, db *bun.DB, check func(context.Context, bun.IDB) error) error {
	if db == nil {
		return errors.New("student contract: database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	release, err := lockStorageBackfill(ctx, db, StudentOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE users.students IN ACCESS EXCLUSIVE MODE;
			LOCK TABLE users.student_profiles, users.students_legacy,
				users.student_school_memberships, users.student_care_profiles IN ACCESS EXCLUSIVE MODE;
			LOCK TABLE users.students_guardians, users.guardian_profiles,
				users.guardian_phone_numbers IN SHARE MODE;
		`); err != nil {
			return fmt.Errorf("student contract: lock storage: %w", err)
		}
		if err := studentOwnerContractDataPreflight(ctx, tx); err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, tx); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, studentContractVisitCountFunction); err != nil {
			return fmt.Errorf("student contract: rebind visit count to owner storage: %w", err)
		}
		before, err := studentContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		// RESTRICT is intentional. An unknown dependency must abort and roll
		// back even an earlier DROP, never be silently removed by CASCADE.
		if _, err := tx.ExecContext(ctx, `
			DROP VIEW users.expired_privacy_consents RESTRICT;
			DROP VIEW users.students RESTRICT;
			DROP FUNCTION users.route_student_compatibility() RESTRICT;
			DROP TABLE users.students_legacy RESTRICT;
			DROP SEQUENCE users.student_compatibility_reads, users.student_compatibility_writes RESTRICT;
		`); err != nil {
			return fmt.Errorf("student contract: remove obsolete storage: %w", err)
		}
		after, err := studentContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return errors.New("student contract: owner rows or checksums changed during Contract")
		}
		return nil
	})
}

type studentContractFingerprint struct {
	Object   string
	TenantID int64
	Rows     int64
	Checksum string
}

// Every target column participates, including live absence and consent
// timestamps. Grouping by school prevents totals from masking tenant drift.
func studentContractFingerprints(ctx context.Context, db bun.IDB) ([]studentContractFingerprint, error) {
	var result []studentContractFingerprint
	if err := db.NewRaw(`
		SELECT 'users.student_profiles' AS object, tenant_id, count(*) AS rows,
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex') AS checksum
		FROM users.student_profiles r GROUP BY tenant_id
		UNION ALL
		SELECT 'users.student_school_memberships', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM users.student_school_memberships r GROUP BY tenant_id
		UNION ALL
		SELECT 'users.student_care_profiles', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM users.student_care_profiles r GROUP BY tenant_id
		ORDER BY object, tenant_id
	`).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("student contract: fingerprint owner storage: %w", err)
	}
	return result, nil
}

// studentOwnerContractDataPreflight never selects the compatibility view or
// changes rows/counters. Run it again under the Contract lock: a pre-deployment
// observation alone cannot exclude writes between preflight and DDL.
func studentOwnerContractDataPreflight(ctx context.Context, db bun.IDB) error {
	var ready bool
	if err := db.NewRaw(`SELECT
		(SELECT relkind = 'v' FROM pg_class WHERE oid = to_regclass('users.students'))
		AND (SELECT relkind = 'r' FROM pg_class WHERE oid = to_regclass('users.students_legacy'))
		AND (SELECT count(*) = 4 FROM pg_attribute
			WHERE attrelid = to_regclass('users.student_care_profiles') AND NOT attisdropped
			AND attname IN ('sick', 'sick_since', 'excused', 'excused_since'))`).Scan(ctx, &ready); err != nil {
		return fmt.Errorf("student contract: inspect cutover schema: %w", err)
	}
	if !ready {
		return errors.New("student contract: requires completed cutover and Care Plan absence migration")
	}
	var reads, writes int64
	if err := db.NewRaw(`SELECT
		coalesce(pg_sequence_last_value('users.student_compatibility_reads'), 0),
		coalesce(pg_sequence_last_value('users.student_compatibility_writes'), 0)`).Scan(ctx, &reads, &writes); err != nil {
		return fmt.Errorf("student contract: inspect compatibility counters: %w", err)
	}
	if reads != 0 || writes != 0 {
		return fmt.Errorf("student contract: compatibility hits prevent Contract (reads=%d writes=%d)", reads, writes)
	}
	var functionName string
	if err := db.NewRaw(`SELECT coalesce((SELECT p.oid::regprocedure::text
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		AND replace(p.prosrc, '"', '') ~* '(^|[^[:alnum:]_])users[.](students|students_legacy|expired_privacy_consents)([^[:alnum:]_]|$)'
		AND p.oid NOT IN ('users.route_student_compatibility()'::regprocedure,
			'active.count_student_visits_for_deletion(bigint,bigint)'::regprocedure)
		ORDER BY p.oid LIMIT 1), '')`).Scan(ctx, &functionName); err != nil {
		return fmt.Errorf("student contract: inspect stored function callers: %w", err)
	}
	if functionName != "" {
		return fmt.Errorf("student contract: stored function still references retired storage: %s", functionName)
	}
	var counts [7]int64
	if err := db.NewRaw(`SELECT
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND NOT convalidated AND
			(confrelid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass, 'users.student_care_profiles'::regclass)
			 OR conrelid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass, 'users.student_care_profiles'::regclass))),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND confrelid = 'users.students_legacy'::regclass),
		(SELECT count(*) FROM pg_class
			WHERE oid IN ('users.student_profiles'::regclass, 'users.student_school_memberships'::regclass, 'users.student_care_profiles'::regclass)
			AND (NOT relrowsecurity OR NOT relforcerowsecurity)),
		(SELECT count(*) FROM users.student_school_memberships m
			WHERE m.deleted_at IS NULL AND NOT EXISTS (SELECT FROM users.student_care_profiles c
				WHERE c.tenant_id = m.tenant_id AND c.membership_id = m.id)),
		(SELECT count(*) FROM users.student_school_memberships m
			WHERE NOT EXISTS (SELECT FROM users.student_profiles p WHERE p.tenant_id = m.tenant_id AND p.id = m.student_profile_id)),
		(SELECT count(*) FROM users.student_care_profiles c
			WHERE NOT EXISTS (SELECT FROM users.student_school_memberships m WHERE m.tenant_id = c.tenant_id AND m.id = c.membership_id)),
		(SELECT count(*) FROM (
			SELECT tenant_id, student_profile_id FROM users.student_school_memberships
			WHERE deleted_at IS NULL GROUP BY tenant_id, student_profile_id HAVING count(*) > 1
		) duplicates)
	`).Scan(ctx, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6]); err != nil {
		return fmt.Errorf("student contract: inspect owner integrity: %w", err)
	}
	for index, name := range []string{
		"unvalidated owner foreign keys", "foreign keys still referencing archive", "owner RLS disabled",
		"live memberships without care", "memberships without profiles", "care rows without memberships",
		"duplicate live memberships",
	} {
		if counts[index] != 0 {
			return fmt.Errorf("student contract: %s: %d", name, counts[index])
		}
	}
	return studentOwnerContractGuardianPreflight(ctx, db)
}

// SQL-language function bodies stored as strings are not tracked by DROP
// RESTRICT. Preserve the old view's live-membership/care presence semantics and
// the existing SECURITY DEFINER tenant guard while removing that dependency.
const studentContractVisitCountFunction = `
CREATE OR REPLACE FUNCTION active.count_student_visits_for_deletion(
    p_tenant_id BIGINT, p_student_id BIGINT
)
RETURNS INTEGER LANGUAGE sql STABLE SECURITY DEFINER
SET search_path = pg_catalog, active, users
AS $$
    SELECT COUNT(*)::INTEGER
    FROM active.visits AS visit
    JOIN users.student_profiles AS student ON student.id = visit.student_id
    JOIN users.student_school_memberships AS membership
      ON membership.student_profile_id = student.id
      AND membership.tenant_id = student.tenant_id AND membership.deleted_at IS NULL
    JOIN users.student_care_profiles AS care
      ON care.membership_id = membership.id AND care.tenant_id = membership.tenant_id
    WHERE visit.student_id = p_student_id
      AND student.tenant_id = p_tenant_id
      AND (
        NULLIF(current_setting('app.current_tenant_id', true), '') IS NULL
        OR student.tenant_id = NULLIF(current_setting('app.current_tenant_id', true), '')::BIGINT
      );
$$;
REVOKE ALL ON FUNCTION active.count_student_visits_for_deletion(BIGINT, BIGINT) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION active.count_student_visits_for_deletion(BIGINT, BIGINT) TO phoenix_tenant;
`
