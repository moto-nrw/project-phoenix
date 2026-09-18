package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

// studentOwnerSourceProjection and studentOwnerTargetProjection are the
// canonical row shapes compared during verification. Column names and order
// must match so that to_jsonb renders identical text for identical data.
//
// The three targets each carry their own created_at/updated_at and the
// membership its own deleted_at, so the projection names all of them and holds
// each against the one source row they were copied from. The target side joins
// on the preserved identity as well as on the owning link, which is what turns
// a drifted profile, membership or care id into a missing row rather than a
// silently matching one.
const studentOwnerSourceProjection = `
	SELECT s.id, s.tenant_id, s.person_id,
	       s.address_street, s.address_city, s.address_postal_code, s.extra_info,
	       s.photo_path, s.photo_consent_given_at, s.photo_consent_given_by,
	       s.agb_accepted_at, s.data_processing_accepted_at, s.email_contact_accepted_at,
	       s.school_class, s.group_id, s.status, s.enrolled_from, s.enrolled_until,
	       s.supervisor_notes, s.health_info, s.pickup_status, s.departure_days,
	       s.allowed_departure_modes, s.departure_companion_note, s.pickup_days, s.bus_days,
	       NULL::timestamptz AS membership_deleted_at,
	       s.created_at AS profile_created_at, s.updated_at AS profile_updated_at,
	       s.created_at AS membership_created_at, s.updated_at AS membership_updated_at,
	       s.created_at AS care_created_at, s.updated_at AS care_updated_at
	FROM users.students AS s
	WHERE s.tenant_id = ?`

const studentOwnerTargetProjection = `
	SELECT p.id, p.tenant_id, p.person_id,
	       p.address_street, p.address_city, p.address_postal_code, p.extra_info,
	       p.photo_path, p.photo_consent_given_at, p.photo_consent_given_by,
	       p.agb_accepted_at, p.data_processing_accepted_at, p.email_contact_accepted_at,
	       m.school_class, m.group_id, m.status, m.enrolled_from, m.enrolled_until,
	       c.supervisor_notes, c.health_info, c.pickup_status, c.departure_days,
	       c.allowed_departure_modes, c.departure_companion_note, c.pickup_days, c.bus_days,
	       m.deleted_at AS membership_deleted_at,
	       p.created_at AS profile_created_at, p.updated_at AS profile_updated_at,
	       m.created_at AS membership_created_at, m.updated_at AS membership_updated_at,
	       c.created_at AS care_created_at, c.updated_at AS care_updated_at
	FROM users.student_profiles AS p
	JOIN users.student_school_memberships AS m
		ON m.tenant_id = p.tenant_id AND m.id = p.id AND m.student_profile_id = p.id
	JOIN users.student_care_profiles AS c
		ON c.tenant_id = m.tenant_id AND c.membership_id = m.id
	WHERE p.tenant_id = ?`

const studentOwnerSourceChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + studentOwnerSourceProjection + `) AS r`

const studentOwnerTargetChecksum = `
	SELECT count(*), encode(sha256(coalesce(string_agg(sha256(convert_to(to_jsonb(r)::text, 'UTF8')), ''::bytea ORDER BY r.id), ''::bytea)), 'hex')
	FROM (` + studentOwnerTargetProjection + `) AS r`

const studentOwnerMismatch = `
	SELECT count(*) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t)),
	       min(s.profile_updated_at) FILTER (WHERE to_jsonb(s) IS DISTINCT FROM to_jsonb(t))
	FROM (` + studentOwnerSourceProjection + `) AS s
	FULL JOIN (` + studentOwnerTargetProjection + `) AS t ON t.id = s.id`

// studentOwnerGuardianMismatch counts the students whose legacy guardian
// columns hold a value that no guardian linked to that child carries. Those
// four columns have no target: users.guardian_profiles and
// users.guardian_phone_numbers own contact people, and the split must not copy
// a second, diverging copy of them into student-owned storage. Reporting the
// unreconciled ones is what keeps the values from being dropped silently.
//
// A value counts as reconciled when a linked guardian carries it: the name
// against "first last" case-insensitively, the e-mail against the profile
// e-mail, the phone against any of the guardian's phone numbers reduced to
// digits, and the legacy free-text contact against any of the three, because
// historically it held whichever the school had. Comparison is exact after that
// normalization; a differently written phone or name is reported rather than
// assumed equal, so the correction stays a human decision.
const studentOwnerGuardianMismatch = `
	WITH legacy AS (
		SELECT s.id, s.updated_at,
		       nullif(lower(btrim(coalesce(s.guardian_name, ''))), '') AS guardian_name,
		       nullif(lower(btrim(coalesce(s.guardian_contact, ''))), '') AS guardian_contact,
		       nullif(lower(btrim(coalesce(s.guardian_email, ''))), '') AS guardian_email,
		       nullif(regexp_replace(coalesce(s.guardian_phone, ''), '\D', '', 'g'), '') AS guardian_phone
		FROM users.students AS s
		WHERE s.tenant_id = ?
	),
	links AS (
		SELECT sg.tenant_id, sg.student_id, sg.guardian_profile_id
		FROM users.students_guardians AS sg
		WHERE sg.tenant_id = ?
	),
	names AS (
		SELECT l.student_id, lower(btrim(coalesce(g.first_name, '') || ' ' || coalesce(g.last_name, ''))) AS value
		FROM links AS l
		JOIN users.guardian_profiles AS g ON g.tenant_id = l.tenant_id AND g.id = l.guardian_profile_id
	),
	emails AS (
		SELECT l.student_id, lower(btrim(g.email)) AS value
		FROM links AS l
		JOIN users.guardian_profiles AS g ON g.tenant_id = l.tenant_id AND g.id = l.guardian_profile_id
		WHERE g.email IS NOT NULL AND btrim(g.email) <> ''
	),
	phones AS (
		SELECT l.student_id, regexp_replace(n.phone_number, '\D', '', 'g') AS value
		FROM links AS l
		JOIN users.guardian_phone_numbers AS n
			ON n.tenant_id = l.tenant_id AND n.guardian_profile_id = l.guardian_profile_id
		WHERE regexp_replace(n.phone_number, '\D', '', 'g') <> ''
	)
	SELECT count(*), min(l.updated_at)
	FROM legacy AS l
	WHERE (l.guardian_name IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM names AS v WHERE v.student_id = l.id AND v.value = l.guardian_name))
	   OR (l.guardian_email IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM emails AS v WHERE v.student_id = l.id AND v.value = l.guardian_email))
	   OR (l.guardian_phone IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM phones AS v WHERE v.student_id = l.id AND v.value = l.guardian_phone))
	   OR (l.guardian_contact IS NOT NULL
			AND NOT EXISTS (SELECT 1 FROM names AS v WHERE v.student_id = l.id AND v.value = l.guardian_contact)
			AND NOT EXISTS (SELECT 1 FROM emails AS v WHERE v.student_id = l.id AND v.value = l.guardian_contact)
			AND NOT EXISTS (SELECT 1 FROM phones AS v WHERE v.student_id = l.id
				AND v.value = nullif(regexp_replace(l.guardian_contact, '\D', '', 'g'), '')))`

// studentOwnerCareStateMismatch counts the students whose legacy sick/excused
// flag is raised without an equivalent open day in active.student_status_days.
// The flags are not copied: the status days are the authority for scheduled
// absence, and after Cutover they are the only source of the effective state
// that today reads as "flag OR open day". Equivalence therefore means the flag
// implies an open day, and a raised flag without one is the exact case that
// would silently lose a child's absence.
//
// class_trip counts as excused, matching how the effective absence count reads
// the two today. The day is a calendar date in the school's timezone, so it is
// derived from Europe/Berlin rather than from the session's UTC.
//
// Sick outranks excused: the effective read is an if/else-if chain, so a child
// who is sick after the split reaches the excused branch neither before nor
// after and their excused flag is inert. Nothing about that child is lost, so
// the excused term only applies while no open sick day covers the day — without
// that guard a doubly flagged child would be reported although their state is
// identical on both sides, and an operator would have to clear a flag to
// satisfy the verifier rather than to correct the data.
const studentOwnerCareStateMismatch = `
	SELECT count(*), min(s.updated_at)
	FROM users.students AS s
	WHERE s.tenant_id = ?
	  AND ((s.sick IS TRUE AND NOT EXISTS (
			SELECT 1 FROM active.student_status_days AS d
			WHERE d.tenant_id = s.tenant_id AND d.student_id = s.id
			  AND d.date = (now() AT TIME ZONE 'Europe/Berlin')::date
			  AND d.cleared_at IS NULL AND d.status = 'sick'))
		OR (s.excused IS TRUE AND NOT EXISTS (
			SELECT 1 FROM active.student_status_days AS d
			WHERE d.tenant_id = s.tenant_id AND d.student_id = s.id
			  AND d.date = (now() AT TIME ZONE 'Europe/Berlin')::date
			  AND d.cleared_at IS NULL AND d.status IN ('excused', 'class_trip'))
			AND NOT EXISTS (
			SELECT 1 FROM active.student_status_days AS d
			WHERE d.tenant_id = s.tenant_id AND d.student_id = s.id
			  AND d.date = (now() AT TIME ZONE 'Europe/Berlin')::date
			  AND d.cleared_at IS NULL AND d.status = 'sick')))`

// verify compares per-tenant counts, canonical checksums and row-wise
// mismatches between the old table and the joined targets, reconciles the
// legacy guardian values and the effective care state, proves the row-level
// security of the three targets against the tenant role, then persists the
// evidence. The pass is stable when it changed nothing and everything matches.
func (r *studentOwnerTenantRun) verify(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
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
// and after Cutover these rows carry the tenant boundary for every student.
//
// It runs in its own transaction: the role switch would otherwise strip the
// verification transaction of the grants its checkpoint update needs.
// RowsVisibleToTenant is recorded rather than only asserted, so an operator can
// see the isolation was measured and not merely assumed.
func (r *studentOwnerTenantRun) verifyTenantIsolation(ctx context.Context, cp *StudentOwnerBackfillCheckpoint) error {
	var owned, visible, foreign int64
	err := r.db.RunInTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead}, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout = '5s'; SET LOCAL statement_timeout = '60s'`); err != nil {
			return err
		}
		if err := tx.NewRaw(studentOwnerOwnedRows, r.tenantID, r.tenantID, r.tenantID).Scan(ctx, &owned); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_tenant_id', ?, true)`,
			strconv.FormatInt(r.tenantID, 10)); err != nil {
			return err
		}
		if err := tx.NewRaw(studentOwnerVisibleRows).Scan(ctx, &visible, &foreign); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `RESET ROLE`)
		return err
	})
	if err != nil {
		return fmt.Errorf("student owner backfill: tenant %d row-level security: %w", r.tenantID, err)
	}
	if foreign != 0 || visible != owned {
		return fmt.Errorf(
			"student owner backfill: tenant %d row-level security leaked: phoenix_tenant sees %d of %d own rows and %d foreign rows",
			r.tenantID, visible, owned, foreign)
	}
	cp.RowsVisibleToTenant = visible
	return nil
}

// studentOwnerOwnedRows counts this school's target rows on the superuser
// connection, which no policy filters.
const studentOwnerOwnedRows = `
	SELECT (SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?)
	     + (SELECT count(*) FROM users.student_school_memberships WHERE tenant_id = ?)
	     + (SELECT count(*) FROM users.student_care_profiles WHERE tenant_id = ?)`

// studentOwnerVisibleRows counts the same three tables under the tenant policy.
// The second column must stay zero: a row the policy lets through while its
// tenant_id names another school is a cross-tenant leak, not a miscount.
const studentOwnerVisibleRows = `
	WITH rows AS (
		SELECT tenant_id FROM users.student_profiles
		UNION ALL SELECT tenant_id FROM users.student_school_memberships
		UNION ALL SELECT tenant_id FROM users.student_care_profiles
	)
	SELECT count(*),
	       count(*) FILTER (
		WHERE tenant_id IS DISTINCT FROM NULLIF(current_setting('app.current_tenant_id', true), '')::bigint)
	FROM rows`

func (r *studentOwnerTenantRun) verifySnapshot(ctx context.Context, tx bun.Tx, cp *StudentOwnerBackfillCheckpoint) error {
	var source, target struct {
		Count    int64
		Checksum string
	}
	if err := tx.NewRaw(studentOwnerSourceChecksum, r.tenantID).
		Scan(ctx, &source.Count, &source.Checksum); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d source checksum: %w", r.tenantID, err)
	}
	if r.opts.afterSourceVerification != nil {
		if err := r.opts.afterSourceVerification(ctx, tx); err != nil {
			return err
		}
	}
	if err := tx.NewRaw(studentOwnerTargetChecksum, r.tenantID).
		Scan(ctx, &target.Count, &target.Checksum); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d target checksum: %w", r.tenantID, err)
	}
	var mismatches int64
	var oldest sql.NullTime
	if err := tx.NewRaw(studentOwnerMismatch, r.tenantID, r.tenantID).Scan(ctx, &mismatches, &oldest); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d mismatches: %w", r.tenantID, err)
	}
	var guardianMismatches int64
	var guardianOldest sql.NullTime
	if err := tx.NewRaw(studentOwnerGuardianMismatch, r.tenantID, r.tenantID).Scan(ctx, &guardianMismatches, &guardianOldest); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d guardian reconciliation: %w", r.tenantID, err)
	}
	var careMismatches int64
	var careOldest sql.NullTime
	if err := tx.NewRaw(studentOwnerCareStateMismatch, r.tenantID).Scan(ctx, &careMismatches, &careOldest); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d care state: %w", r.tenantID, err)
	}
	var now time.Time
	if err := tx.NewRaw(`SELECT clock_timestamp(), pg_current_snapshot()::text`).Scan(ctx, &now, &cp.VerificationSnapshot); err != nil {
		return fmt.Errorf("student owner backfill: verification snapshot: %w", err)
	}
	cp.SourceCount, cp.SourceChecksum = source.Count, source.Checksum
	cp.TargetCount, cp.TargetChecksum = target.Count, target.Checksum
	cp.MismatchCount = mismatches
	cp.GuardianMismatchCount = guardianMismatches
	cp.CareStateMismatchCount = careMismatches
	cp.OldestUnmigratedAt = oldestUnmigrated(oldest, guardianOldest, careOldest)
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
			mismatch_count = ?, guardian_mismatch_count = ?, care_state_mismatch_count = ?,
			rows_visible_to_tenant = ?, oldest_unmigrated_at = ?, verified_at = ?, stable = ?, stable_at = ?,
			batch_p95_ms = ?, batch_max_ms = ?, pool_wait_ms = ?, verification_snapshot = ?, pass_completed = true, updated_at = now()
		WHERE backfill = ? AND tenant_id = ?`,
		cp.SourceCount, cp.TargetCount, cp.SourceChecksum, cp.TargetChecksum,
		cp.MismatchCount, cp.GuardianMismatchCount, cp.CareStateMismatchCount,
		cp.RowsVisibleToTenant, cp.OldestUnmigratedAt, cp.VerifiedAt, cp.Stable, cp.StableAt,
		cp.BatchP95Ms, cp.BatchMaxMs, cp.PoolWaitMs, cp.VerificationSnapshot, StudentOwnerBackfillName, r.tenantID); err != nil {
		return fmt.Errorf("student owner backfill: tenant %d persist verification: %w", r.tenantID, err)
	}
	r.opts.Logger.Info("student owner backfill pass verified",
		"tenant_id", r.tenantID,
		"pass", cp.Pass,
		"stable", cp.Stable,
		"source_count", cp.SourceCount,
		"target_count", cp.TargetCount,
		"mismatch_count", cp.MismatchCount,
		"guardian_mismatch_count", cp.GuardianMismatchCount,
		"care_state_mismatch_count", cp.CareStateMismatchCount,
		"rows_copied", cp.RowsCopied,
		"rows_rejected", cp.RowsRejected,
		"rows_removed", cp.RowsRemoved)
	return nil
}

// oldestUnmigrated is the earliest source change any of the three checks still
// reports, so one age answers "how far behind is this tenant" regardless of
// which kind of difference caused it.
func oldestUnmigrated(candidates ...sql.NullTime) *time.Time {
	var oldest *time.Time
	for _, candidate := range candidates {
		if !candidate.Valid {
			continue
		}
		if oldest == nil || candidate.Time.Before(*oldest) {
			oldest = &candidate.Time
		}
	}
	return oldest
}

// StudentOwnerVerification is the per-tenant verdict Cutover (#2759) records
// and refuses to switch without.
type StudentOwnerVerification struct {
	TenantID               int64  `json:"tenant_id"`
	SourceCount            int64  `json:"source_count"`
	TargetCount            int64  `json:"target_count"`
	SourceChecksum         string `json:"source_checksum"`
	TargetChecksum         string `json:"target_checksum"`
	MismatchCount          int64  `json:"mismatch_count"`
	GuardianMismatchCount  int64  `json:"guardian_mismatch_count"`
	CareStateMismatchCount int64  `json:"care_state_mismatch_count"`
}

// Equal reports whether the targets reproduce the old table exactly and the two
// column groups without a target are accounted for.
func (v StudentOwnerVerification) Equal() bool {
	return v.SourceCount == v.TargetCount && v.SourceChecksum == v.TargetChecksum &&
		v.MismatchCount == 0 && v.GuardianMismatchCount == 0 && v.CareStateMismatchCount == 0
}

// Describe names the failing verdicts so an operator sees which of them blocked
// the switch without reading the checkpoint row.
func (v StudentOwnerVerification) Describe() string {
	return fmt.Sprintf(
		"source %d rows %s, target %d rows %s, %d mismatches, %d unreconciled guardian values, %d absence flags without a status day",
		v.SourceCount, v.SourceChecksum, v.TargetCount, v.TargetChecksum,
		v.MismatchCount, v.GuardianMismatchCount, v.CareStateMismatchCount)
}

// verifyStudentOwnerTenant is Cutover's single-transaction verdict. It lives
// beside the projections above so it uses exactly the equality the resumable
// backfill used: one definition, not a second one that could drift from it.
func verifyStudentOwnerTenant(ctx context.Context, tx bun.Tx, tenantID int64) (StudentOwnerVerification, error) {
	verification := StudentOwnerVerification{TenantID: tenantID}
	if err := tx.NewRaw(studentOwnerSourceChecksum, tenantID).
		Scan(ctx, &verification.SourceCount, &verification.SourceChecksum); err != nil {
		return verification, fmt.Errorf("student owner cutover: tenant %d source checksum: %w", tenantID, err)
	}
	if err := tx.NewRaw(studentOwnerTargetChecksum, tenantID).
		Scan(ctx, &verification.TargetCount, &verification.TargetChecksum); err != nil {
		return verification, fmt.Errorf("student owner cutover: tenant %d target checksum: %w", tenantID, err)
	}
	var oldest sql.NullTime
	if err := tx.NewRaw(studentOwnerMismatch, tenantID, tenantID).
		Scan(ctx, &verification.MismatchCount, &oldest); err != nil {
		return verification, fmt.Errorf("student owner cutover: tenant %d mismatches: %w", tenantID, err)
	}
	if err := tx.NewRaw(studentOwnerGuardianMismatch, tenantID, tenantID).
		Scan(ctx, &verification.GuardianMismatchCount, &oldest); err != nil {
		return verification, fmt.Errorf("student owner cutover: tenant %d guardian reconciliation: %w", tenantID, err)
	}
	if err := tx.NewRaw(studentOwnerCareStateMismatch, tenantID).
		Scan(ctx, &verification.CareStateMismatchCount, &oldest); err != nil {
		return verification, fmt.Errorf("student owner cutover: tenant %d care state: %w", tenantID, err)
	}
	return verification, nil
}
