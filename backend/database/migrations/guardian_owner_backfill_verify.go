package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

// guardianOwnerSourceProjection and guardianOwnerTargetProjection are the
// canonical row shapes compared during verification. Column names and order
// must match so that to_jsonb renders identical text for identical data.
//
// The source side carries the guardian's account binding from
// users.guardian_profiles, which the Identity target mirrors, so a guardian who
// gains or loses a portal account after the copy shows up as a mismatch. The
// join is outer: a source row whose guardian is missing or belongs to another
// school still counts on the source side and, having been rejected by the copy,
// is reported as a missing target row rather than disappearing from the count.
//
// The three targets each carry their own created_at/updated_at, so the
// projection names all of them and holds each against the one source row they
// were copied from. The target side joins on the preserved identity, which is
// what turns a drifted pickup or access row into a missing row rather than a
// silently matching one.
const guardianOwnerSourceProjection = `
	SELECT sg.id, sg.tenant_id, sg.student_id, sg.guardian_profile_id, sg.relationship_type,
	       sg.guardian_role, sg.is_primary, sg.is_emergency_contact, sg.emergency_priority, sg.is_payer,
	       sg.can_pickup, sg.pickup_notes, g.account_id, sg.permissions,
	       sg.created_at AS relationship_created_at, sg.updated_at AS relationship_updated_at,
	       sg.created_at AS pickup_created_at, sg.updated_at AS pickup_updated_at,
	       sg.created_at AS access_created_at, sg.updated_at AS access_updated_at
	FROM users.students_guardians AS sg
	LEFT JOIN users.guardian_profiles AS g ON g.tenant_id = sg.tenant_id AND g.id = sg.guardian_profile_id
	WHERE sg.tenant_id = ?`

const guardianOwnerTargetProjection = `
	SELECT r.id, r.tenant_id, r.student_id, r.guardian_profile_id, r.relationship_type,
	       r.guardian_role, r.is_primary, r.is_emergency_contact, r.emergency_priority, r.is_payer,
	       p.can_pickup, p.pickup_notes, a.account_id, a.permissions,
	       r.created_at AS relationship_created_at, r.updated_at AS relationship_updated_at,
	       p.created_at AS pickup_created_at, p.updated_at AS pickup_updated_at,
	       a.created_at AS access_created_at, a.updated_at AS access_updated_at
	FROM users.student_guardian_relationships AS r
	JOIN users.student_guardian_pickup_permissions AS p
		ON p.tenant_id = r.tenant_id AND p.relationship_id = r.id
	JOIN auth.guardian_student_access AS a
		ON a.tenant_id = r.tenant_id AND a.relationship_id = r.id
	WHERE r.tenant_id = ?`

const guardianOwnerSourceChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + guardianOwnerSourceProjection + `) AS r`

const guardianOwnerTargetChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + guardianOwnerTargetProjection + `) AS r`

const guardianOwnerMismatch = `
	SELECT count(*) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t)),
	       min(s.relationship_updated_at) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t))
	FROM (` + guardianOwnerSourceProjection + `) AS s
	FULL JOIN (` + guardianOwnerTargetProjection + `) AS t ON t.id = s.id`

// guardianOwnerAccessMismatch counts the source relationships whose Identity
// row is missing, bound to a different account than the guardian's profile
// names today, or carrying different parent-portal permissions. It is the
// guardian-access verdict on its own: a linked or unlinked portal account edits
// users.guardian_profiles, not the source row, so the source's updated_at alone
// would not date such a drift. The age is therefore taken from whichever of the
// two rows changed last.
const guardianOwnerAccessMismatch = `
	SELECT count(*), min(greatest(sg.updated_at, g.updated_at))
	FROM users.students_guardians AS sg
	JOIN users.guardian_profiles AS g ON g.tenant_id = sg.tenant_id AND g.id = sg.guardian_profile_id
	LEFT JOIN auth.guardian_student_access AS a ON a.tenant_id = sg.tenant_id AND a.relationship_id = sg.id
	WHERE sg.tenant_id = ?
	  AND (a.id IS NULL
	       OR a.account_id IS DISTINCT FROM g.account_id
	       OR a.permissions IS DISTINCT FROM sg.permissions)`

// guardianOwnerShapes is the verdict of one school's old and owner shapes.
type guardianOwnerShapes struct {
	SourceCount, TargetCount       int64
	SourceChecksum, TargetChecksum string
	Mismatches, AccessMismatches   int64
}

// compareGuardianOwnerShapes holds the old table against the joined owners
// with the backfill's own projections, inside the caller's transaction. The
// cutover (#2756) proves exactly what the backfill promised with it.
func compareGuardianOwnerShapes(ctx context.Context, tx bun.Tx, tenantID int64) (guardianOwnerShapes, error) {
	var shapes guardianOwnerShapes
	if err := tx.NewRaw(guardianOwnerSourceChecksum, tenantID).Scan(ctx, &shapes.SourceCount, &shapes.SourceChecksum); err != nil {
		return shapes, fmt.Errorf("source checksum: %w", err)
	}
	if err := tx.NewRaw(guardianOwnerTargetChecksum, tenantID).Scan(ctx, &shapes.TargetCount, &shapes.TargetChecksum); err != nil {
		return shapes, fmt.Errorf("target checksum: %w", err)
	}
	var oldest, accessOldest *time.Time
	if err := tx.NewRaw(guardianOwnerMismatch, tenantID, tenantID).Scan(ctx, &shapes.Mismatches, &oldest); err != nil {
		return shapes, fmt.Errorf("mismatches: %w", err)
	}
	if err := tx.NewRaw(guardianOwnerAccessMismatch, tenantID).Scan(ctx, &shapes.AccessMismatches, &accessOldest); err != nil {
		return shapes, fmt.Errorf("guardian access: %w", err)
	}
	return shapes, nil
}

// verify compares per-tenant counts, canonical checksums and row-wise
// mismatches between the old table and the joined targets, checks the guardian
// access binding, proves the row-level security of the three targets against
// the tenant role, then persists the evidence. The pass is stable when it
// changed nothing and everything matches.
func (r *guardianOwnerTenantRun) verify(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
	if err := r.verifyTenantIsolation(ctx, cp); err != nil {
		return err
	}
	return r.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL TIME ZONE 'UTC'; SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		return r.verifySnapshot(ctx, tx, cp)
	})
}

// verifyTenantIsolation reads the three targets once as phoenix_tenant, scoped
// to this school, and holds what that role sees against what the superuser
// connection wrote. The copy runs as superuser and therefore bypasses the very
// policies the rows depend on, so nothing else in this run would notice a
// policy that Expand created and a later change dropped, disabled or widened —
// and after Cutover these rows decide which parent may see which child.
//
// It runs in its own transaction: the role switch would otherwise strip the
// verification transaction of the grants its checkpoint update needs.
// RowsVisibleToTenant is recorded rather than only asserted, so an operator can
// see the isolation was measured and not merely assumed.
func (r *guardianOwnerTenantRun) verifyTenantIsolation(ctx context.Context, cp *GuardianOwnerBackfillCheckpoint) error {
	var owned, visible, foreign int64
	err := r.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(guardianOwnerOwnedRows, r.tenantID, r.tenantID, r.tenantID).Scan(ctx, &owned); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`,
			strconv.FormatInt(r.tenantID, 10)); err != nil {
			return err
		}
		if err := tx.NewRaw(guardianOwnerVisibleRows).Scan(ctx, &visible, &foreign); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `RESET ROLE`)
		return err
	})
	if err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d row-level security: %w", r.tenantID, err)
	}
	if foreign != 0 || visible != owned {
		return fmt.Errorf(
			"guardian owner backfill: tenant %d row-level security leaked: phoenix_tenant sees %d of %d own rows and %d foreign rows",
			r.tenantID, visible, owned, foreign)
	}
	cp.RowsVisibleToTenant = visible
	return nil
}

// guardianOwnerOwnedRows counts this school's target rows on the superuser
// connection, which no policy filters.
const guardianOwnerOwnedRows = `
	SELECT (SELECT count(*) FROM users.student_guardian_relationships WHERE tenant_id = ?)
	     + (SELECT count(*) FROM users.student_guardian_pickup_permissions WHERE tenant_id = ?)
	     + (SELECT count(*) FROM auth.guardian_student_access WHERE tenant_id = ?)`

// guardianOwnerVisibleRows counts the same three tables under the tenant
// policy. The second column must stay zero: a row the policy lets through
// while its tenant_id names another school is a cross-tenant leak, not a
// miscount.
const guardianOwnerVisibleRows = `
	WITH rows AS (
		SELECT tenant_id FROM users.student_guardian_relationships
		UNION ALL SELECT tenant_id FROM users.student_guardian_pickup_permissions
		UNION ALL SELECT tenant_id FROM auth.guardian_student_access
	)
	SELECT count(*),
	       count(*) FILTER (
		WHERE tenant_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	FROM rows`

func (r *guardianOwnerTenantRun) verifySnapshot(ctx context.Context, tx bun.Tx, cp *GuardianOwnerBackfillCheckpoint) error {
	var source, target struct {
		Count    int64
		Checksum string
	}
	if err := tx.NewRaw(guardianOwnerSourceChecksum, r.tenantID).
		Scan(ctx, &source.Count, &source.Checksum); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d source checksum: %w", r.tenantID, err)
	}
	if r.opts.afterSourceVerification != nil {
		if err := r.opts.afterSourceVerification(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.NewRaw(guardianOwnerTargetChecksum, r.tenantID).
		Scan(ctx, &target.Count, &target.Checksum); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d target checksum: %w", r.tenantID, err)
	}
	var mismatches int64
	var oldest sql.NullTime
	if err := tx.NewRaw(guardianOwnerMismatch, r.tenantID, r.tenantID).Scan(ctx, &mismatches, &oldest); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d mismatches: %w", r.tenantID, err)
	}
	var accessMismatches int64
	var accessOldest sql.NullTime
	if err := tx.NewRaw(guardianOwnerAccessMismatch, r.tenantID).Scan(ctx, &accessMismatches, &accessOldest); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d guardian access: %w", r.tenantID, err)
	}
	var now time.Time
	if err := tx.NewRaw(`SELECT clock_timestamp(), pg_current_snapshot()::text`).Scan(ctx, &now, &cp.VerificationSnapshot); err != nil {
		return fmt.Errorf("guardian owner backfill: verification snapshot: %w", err)
	}
	cp.SourceCount, cp.SourceChecksum = source.Count, source.Checksum
	cp.TargetCount, cp.TargetChecksum = target.Count, target.Checksum
	cp.MismatchCount = mismatches
	cp.GuardianAccessMismatchCount = accessMismatches
	cp.OldestUnmigratedAt = oldestUnmigrated(oldest, accessOldest)
	cp.VerifiedAt = &now
	cp.PassCompleted = true
	cp.Stable = cp.PassWrites == 0 && cp.Verified()
	if cp.Stable {
		cp.StableAt = &now
	}
	p95, maxDuration := r.batchPercentiles()
	if maxDuration > cp.BatchMaxMs {
		cp.BatchMaxMs = maxDuration
	}
	if len(r.durations) > 0 {
		cp.BatchP95Ms = p95
	}
	cp.PoolWaitMs += r.poolWait.Milliseconds()
	r.durations, r.poolWait = nil, 0
	if _, err := tx.ExecContext(ctx, `
		UPDATE platform.storage_backfill_checkpoints SET
			source_count = ?, target_count = ?, source_checksum = ?, target_checksum = ?,
			mismatch_count = ?, guardian_access_mismatch_count = ?,
			rows_visible_to_tenant = ?, oldest_unmigrated_at = ?, verified_at = ?, stable = ?, stable_at = ?,
			batch_p95_ms = ?, batch_max_ms = ?, pool_wait_ms = ?, verification_snapshot = ?, pass_completed = true, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		cp.SourceCount, cp.TargetCount, cp.SourceChecksum, cp.TargetChecksum,
		cp.MismatchCount, cp.GuardianAccessMismatchCount,
		cp.RowsVisibleToTenant, cp.OldestUnmigratedAt, cp.VerifiedAt, cp.Stable, cp.StableAt,
		cp.BatchP95Ms, cp.BatchMaxMs, cp.PoolWaitMs, cp.VerificationSnapshot, GuardianOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("guardian owner backfill: tenant %d persist verification: %w", r.tenantID, err)
	}
	r.opts.Logger.Info("guardian owner backfill pass verified",
		"tenant_id", r.tenantID,
		"pass", cp.Pass,
		"stable", cp.Stable,
		"source_count", cp.SourceCount,
		"target_count", cp.TargetCount,
		"mismatch_count", cp.MismatchCount,
		"guardian_access_mismatch_count", cp.GuardianAccessMismatchCount,
		"rows_copied", cp.RowsCopied,
		"rows_rejected", cp.RowsRejected,
		"rows_removed", cp.RowsRemoved)
	return nil
}
