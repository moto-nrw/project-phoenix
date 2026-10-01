package migrations

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
)

// The guardian Contract (#2757) follows the student (#2760), request-child
// (#2719) and staff (#2754) Contracts: deployment stops the application and
// verifies a complete release backup, preflight asks the integrity questions
// before downtime, and the same checks run again under locks right before the
// destructive DDL.
//
// It removes only the rollback shape the cutover (#2756) left behind: the
// users.students_guardians mirror with its routing, single-primary and
// updated_at triggers and its owned id sequence, the three mirror triggers on
// the owner tables, the write counter, and the two pre-Cutover grant
// invalidation functions the mirror's old triggers called. The owner updated_at
// triggers, the account binding, the grant invalidation on the access row and
// the repointed grant foreign keys are current behaviour and stay.
//
// The mirror triggers never mirrored a standalone DELETE of a pickup or access
// row. Instead of closing that gap, the Contract refuses while a relationship
// lacks either row (incomplete links) or differs from the mirror (mirror drift).

func guardianOwnerContractPrecondition(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return nil // The replay has not created the guardian schema yet.
	}
	return guardianOwnerContractDataPreflight(ctx, db)
}

func guardianOwnerContractUp(ctx context.Context, db *bun.DB) error {
	if fresh, _ := ctx.Value(freshStudentStorageKey{}).(bool); fresh {
		return contractGuardianOwnerStorageChecked(ctx, db, func(ctx context.Context, connection bun.IDB) error {
			var occupied bool
			err := connection.NewRaw(`SELECT EXISTS (SELECT 1 FROM users.students_guardians)
				OR EXISTS (SELECT 1 FROM users.student_guardian_relationships)
				OR EXISTS (SELECT 1 FROM users.student_guardian_pickup_permissions)
				OR EXISTS (SELECT 1 FROM auth.guardian_student_access)`).Scan(ctx, &occupied)
			if err != nil {
				return fmt.Errorf("guardian contract: verify empty initial replay: %w", err)
			}
			if occupied {
				return errors.New("guardian contract: initial replay contains guardian links")
			}
			return nil
		})
	}
	return contractGuardianOwnerStorageChecked(ctx, db, nil)
}

func guardianOwnerContractDown(context.Context, *bun.DB) error {
	return errors.New("guardian owner Contract is irreversible: restore the verified pre-Contract backup and its prior application image together; automatic Down cannot recreate the removed rollback mirror")
}

func contractGuardianOwnerStorageChecked(ctx context.Context, db *bun.DB, check func(context.Context, bun.IDB) error) error {
	if db == nil {
		return errors.New("guardian contract: database is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	release, err := lockStorageBackfill(ctx, db, GuardianOwnerBackfillName)
	if err != nil {
		return err
	}
	defer release()
	return db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		// The cutover's lock order. users.guardian_profiles is locked because
		// the binding check reads its account.
		if _, err := tx.ExecContext(ctx, `
			SET LOCAL lock_timeout = '5s';
			SET LOCAL statement_timeout = '60s';
			LOCK TABLE users.students_guardians, users.student_guardian_relationships,
				users.student_guardian_pickup_permissions, auth.guardian_student_access,
				users.guardian_profiles
				IN ACCESS EXCLUSIVE MODE;
		`); err != nil {
			return fmt.Errorf("guardian contract: lock storage: %w", err)
		}
		if err := guardianOwnerContractDataPreflight(ctx, tx); err != nil {
			return err
		}
		if check != nil {
			if err := check(ctx, tx); err != nil {
				return err
			}
		}
		before, err := guardianContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		// RESTRICT is intentional. An unknown dependency must abort and roll
		// back even an earlier DROP, never be silently removed by CASCADE.
		// The mirror's triggers and owned id sequence go with the mirror.
		if _, err := tx.ExecContext(ctx, `
			DROP TRIGGER student_guardian_relationships_mirror ON users.student_guardian_relationships RESTRICT;
			DROP TRIGGER student_guardian_pickup_permissions_mirror ON users.student_guardian_pickup_permissions RESTRICT;
			DROP TRIGGER guardian_student_access_mirror ON auth.guardian_student_access RESTRICT;
			DROP TABLE users.students_guardians RESTRICT;
			DROP FUNCTION users.mirror_student_guardian_relationship() RESTRICT;
			DROP FUNCTION users.mirror_student_guardian_pickup_permission() RESTRICT;
			DROP FUNCTION auth.mirror_guardian_student_access() RESTRICT;
			DROP FUNCTION users.route_students_guardians_compatibility() RESTRICT;
			DROP FUNCTION users.enforce_single_primary_student_guardian() RESTRICT;
			DROP FUNCTION meta.invalidate_parent_student_consent_permission_grant() RESTRICT;
			DROP FUNCTION meta.invalidate_meal_participation_permission_grant() RESTRICT;
			DROP SEQUENCE users.students_guardians_compatibility_writes RESTRICT;
		`); err != nil {
			return fmt.Errorf("guardian contract: remove obsolete storage: %w", err)
		}
		after, err := guardianContractFingerprints(ctx, tx)
		if err != nil {
			return err
		}
		if !slices.Equal(before, after) {
			return errors.New("guardian contract: owner rows or checksums changed during Contract")
		}
		return nil
	})
}

type guardianContractFingerprint struct {
	Object   string
	TenantID int64
	Rows     int64
	Checksum string
}

// Every target column participates. Grouping by school prevents totals from
// masking tenant drift.
func guardianContractFingerprints(ctx context.Context, db bun.IDB) ([]guardianContractFingerprint, error) {
	var result []guardianContractFingerprint
	if err := db.NewRaw(`
		SELECT 'users.student_guardian_relationships' AS object, tenant_id, count(*) AS rows,
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex') AS checksum
		FROM users.student_guardian_relationships r GROUP BY tenant_id
		UNION ALL
		SELECT 'users.student_guardian_pickup_permissions', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM users.student_guardian_pickup_permissions r GROUP BY tenant_id
		UNION ALL
		SELECT 'auth.guardian_student_access', tenant_id, count(*),
			encode(sha256(convert_to(string_agg(encode(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), 'hex'), '' ORDER BY to_jsonb(r)::text), 'UTF8')), 'hex')
		FROM auth.guardian_student_access r GROUP BY tenant_id
		ORDER BY object, tenant_id
	`).Scan(ctx, &result); err != nil {
		return nil, fmt.Errorf("guardian contract: fingerprint owner storage: %w", err)
	}
	return result, nil
}

// guardianOwnerContractDataPreflight reads the mirror but never changes rows
// or the counter. It runs again under the Contract lock: a pre-deployment
// observation alone cannot exclude writes between preflight and DDL.
func guardianOwnerContractDataPreflight(ctx context.Context, db bun.IDB) error {
	var ready bool
	if err := db.NewRaw(`SELECT
		coalesce((SELECT relkind = 'r' FROM pg_class WHERE oid = to_regclass('users.students_guardians')), false)
		AND to_regclass('users.students_guardians_compatibility_writes') IS NOT NULL
		AND to_regprocedure('users.route_students_guardians_compatibility()') IS NOT NULL
		AND to_regprocedure('users.mirror_student_guardian_relationship()') IS NOT NULL
		AND to_regprocedure('users.mirror_student_guardian_pickup_permission()') IS NOT NULL
		AND to_regprocedure('auth.mirror_guardian_student_access()') IS NOT NULL
		AND to_regprocedure('users.enforce_single_primary_student_guardian()') IS NOT NULL
		AND to_regprocedure('meta.invalidate_parent_student_consent_permission_grant()') IS NOT NULL
		AND to_regprocedure('meta.invalidate_meal_participation_permission_grant()') IS NOT NULL
		AND (SELECT count(*) FROM pg_trigger WHERE NOT tgisinternal AND tgname IN (
			'students_guardians_route_compatibility', 'student_guardian_relationships_mirror',
			'student_guardian_pickup_permissions_mirror', 'guardian_student_access_mirror')) = 4`).Scan(ctx, &ready); err != nil {
		return fmt.Errorf("guardian contract: inspect cutover schema: %w", err)
	}
	if !ready {
		return errors.New("guardian contract: requires the completed cutover with its compatibility mirror")
	}
	// The historical write counter is diagnostic, not a cleanup gate.
	// Deployment stops the old application before migration and restores the
	// complete release backup on failure; only current integrity matters here.
	var functionName string
	if err := db.NewRaw(`SELECT coalesce((SELECT p.oid::regprocedure::text
		FROM pg_proc p JOIN pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
		AND replace(p.prosrc, '"', '') ~* '(^|[^[:alnum:]_])users[.]students_guardians([^[:alnum:]_]|$)'
		AND p.oid NOT IN ('users.route_students_guardians_compatibility()'::regprocedure,
			'users.mirror_student_guardian_relationship()'::regprocedure,
			'users.mirror_student_guardian_pickup_permission()'::regprocedure,
			'auth.mirror_guardian_student_access()'::regprocedure,
			'users.enforce_single_primary_student_guardian()'::regprocedure)
		ORDER BY p.oid LIMIT 1), '')`).Scan(ctx, &functionName); err != nil {
		return fmt.Errorf("guardian contract: inspect stored function callers: %w", err)
	}
	if functionName != "" {
		return fmt.Errorf("guardian contract: stored function still references retired storage: %s", functionName)
	}
	var counts [7]int64
	if err := db.NewRaw(`WITH owners(oid) AS (VALUES
			('users.student_guardian_relationships'::regclass),
			('users.student_guardian_pickup_permissions'::regclass),
			('auth.guardian_student_access'::regclass))
		SELECT
		(SELECT count(*) FROM pg_depend d JOIN pg_rewrite r ON r.oid = d.objid
			WHERE d.classid = 'pg_rewrite'::regclass
			AND d.refobjid = 'users.students_guardians'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND confrelid = 'users.students_guardians'::regclass),
		(SELECT count(*) FROM pg_constraint
			WHERE contype = 'f' AND NOT convalidated
			AND (confrelid IN (SELECT oid FROM owners) OR conrelid IN (SELECT oid FROM owners))),
		(SELECT count(*) FROM pg_class
			WHERE oid IN (SELECT oid FROM owners) AND (NOT relrowsecurity OR NOT relforcerowsecurity)),
		(SELECT count(*) FROM users.student_guardian_relationships AS r
			WHERE NOT EXISTS (SELECT 1 FROM users.student_guardian_pickup_permissions AS p
				WHERE p.tenant_id = r.tenant_id AND p.relationship_id = r.id)
			OR NOT EXISTS (SELECT 1 FROM auth.guardian_student_access AS a
				WHERE a.tenant_id = r.tenant_id AND a.relationship_id = r.id)),
		(SELECT count(*) FROM auth.guardian_student_access AS a
			JOIN users.student_guardian_relationships AS r ON r.tenant_id = a.tenant_id AND r.id = a.relationship_id
			JOIN users.guardian_profiles AS g ON g.tenant_id = r.tenant_id AND g.id = r.guardian_profile_id
			WHERE a.account_id IS DISTINCT FROM g.account_id),
		(SELECT count(*) FROM users.student_guardian_relationships AS r
			LEFT JOIN users.student_guardian_pickup_permissions AS p
				ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
			LEFT JOIN auth.guardian_student_access AS a
				ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id
			FULL JOIN users.students_guardians AS sg
				ON sg.tenant_id = r.tenant_id AND sg.id = r.id
			WHERE ROW(r.student_id, r.guardian_profile_id, r.relationship_type, r.guardian_role, r.is_primary,
					r.is_emergency_contact, r.emergency_priority, r.is_payer, p.can_pickup, p.pickup_notes, a.permissions)
				IS DISTINCT FROM
				ROW(sg.student_id, sg.guardian_profile_id, sg.relationship_type, sg.guardian_role, sg.is_primary,
					sg.is_emergency_contact, sg.emergency_priority, sg.is_payer, sg.can_pickup, sg.pickup_notes, sg.permissions))
	`).Scan(ctx, &counts[0], &counts[1], &counts[2], &counts[3], &counts[4], &counts[5], &counts[6]); err != nil {
		return fmt.Errorf("guardian contract: inspect owner integrity: %w", err)
	}
	for index, name := range []string{
		"views depend on the mirror", "foreign keys still referencing the mirror",
		"unvalidated owner foreign keys", "owner RLS disabled",
		"incomplete links (relationship without pickup permission or access row)",
		"access rows not bound to the guardian's account", "mirror drift",
	} {
		if counts[index] != 0 {
			return fmt.Errorf("guardian contract: %s: %d", name, counts[index])
		}
	}
	return nil
}
