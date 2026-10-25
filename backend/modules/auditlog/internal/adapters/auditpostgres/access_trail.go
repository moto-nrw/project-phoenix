package audit

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// AccessTrail reads the access trail of a school set (#4655) in the caller's
// transaction. Every branch of every statement filters on the school set,
// bound as the first argument `?0`.
type AccessTrail struct {
	runtime Runtime
}

// NewAccessTrail builds the trail over the caller's transaction. It never
// opens one: the Träger projection calls it inside its administrative unit
// (ADR 0111).
func NewAccessTrail(runtime Runtime) *AccessTrail {
	return &AccessTrail{runtime: requireRuntime(runtime)}
}

// accessTrailEntries is one row per read of audit.data_access_log and per
// change of the ledgers that record an actor on personal data. The values of
// the change data match the auditlog.AccessTrail* constants. The actor is an
// account ID, or a staff ID where the ledger stores that: always for
// personnel number changes, and for staff master data outside the sections
// that record the account (bank-steuer, dokumente, school-classes). A
// recorded 0 is the system actor and becomes NULL.
const accessTrailEntries = `SELECT log.id, log.tenant_id, log.accessed_at AS occurred_at, NULLIF(log.actor_account_id, 0) AS actor_account_id,
		NULL::bigint AS actor_staff_id, ''::text AS actor_name, 'read'::text AS kind, log.resource_type AS data
	FROM audit.data_access_log AS log WHERE log.tenant_id = ANY(?0)
	UNION ALL SELECT edit.id, edit.tenant_id, edit.created_at, NULLIF(edit.edited_by, 0), NULL, edit.edited_by_name, 'change', 'student_record'
	FROM audit.student_field_edits AS edit WHERE edit.tenant_id = ANY(?0)
	UNION ALL SELECT guardian.id, guardian.tenant_id, guardian.changed_at, NULLIF(guardian.actor_account_id, 0), NULL,
		COALESCE(guardian.actor_name_snapshot, ''), 'change', 'guardian_record'
	FROM audit.guardian_changes AS guardian WHERE guardian.tenant_id = ANY(?0)
	UNION ALL SELECT bank.id, bank.tenant_id, bank.occurred_at, NULLIF(bank.changed_by, 0), NULL, '', 'change', 'guardian_bank_details'
	FROM audit.guardian_financial_change_ledger AS bank WHERE bank.tenant_id = ANY(?0)
	UNION ALL SELECT consent.id, consent.tenant_id, consent.created_at, NULLIF(consent.actor_account_id, 0), NULL, '', 'change', 'student_consent'
	FROM audit.student_consent_changes AS consent WHERE consent.tenant_id = ANY(?0)
	UNION ALL SELECT staff.id, staff.tenant_id, staff.occurred_at,
		CASE WHEN staff.section IN ('bank-steuer', 'dokumente', 'school-classes') THEN NULLIF(staff.changed_by, 0) END,
		CASE WHEN staff.section NOT IN ('bank-steuer', 'dokumente', 'school-classes') THEN NULLIF(staff.changed_by, 0) END,
		'', 'change', 'staff_record'
	FROM audit.staff_master_data_changes AS staff WHERE staff.tenant_id = ANY(?0)
	UNION ALL SELECT number.id, number.tenant_id, number.occurred_at, NULL, NULLIF(number.changed_by, 0), '', 'change', 'staff_personnel_number'
	FROM audit.personnel_number_changes AS number WHERE number.tenant_id = ANY(?0)
	UNION ALL SELECT correction.id, correction.tenant_id, correction.created_at, NULLIF(correction.actor_account_id, 0), NULL,
		COALESCE(correction.actor_name_snapshot, ''), 'change', 'attendance_correction'
	FROM audit.attendance_corrections AS correction WHERE correction.tenant_id = ANY(?0)
	UNION ALL SELECT deletion.id, deletion.tenant_id, deletion.deleted_at, NULLIF(deletion.actor_account_id, 0), NULL, '', 'change', 'student_deletion'
	FROM audit.student_deletions AS deletion WHERE deletion.tenant_id = ANY(?0)`

// accessTrailQuery lists the entries of a filter, newest first; the
// optional filters are off at 0 or the empty kind. A person filter matches
// the account and the staff IDs of that account (?8).
const accessTrailQuery = `SELECT trail.id, trail.tenant_id, trail.occurred_at, trail.actor_account_id, trail.actor_staff_id,
		trail.actor_name, trail.kind, trail.data
	FROM (` + accessTrailEntries + `) AS trail
	WHERE trail.occurred_at >= ?1 AND trail.occurred_at < ?2
		AND (?3 = 0 OR trail.tenant_id = ?3)
		AND (?4 = 0 OR trail.actor_account_id = ?4 OR trail.actor_staff_id = ANY(?8))
		AND (?5 = '' OR trail.kind = ?5)
	ORDER BY trail.occurred_at DESC, trail.tenant_id, trail.kind, trail.data, trail.id DESC
	LIMIT ?6 OFFSET ?7`

// accessTrailActorsQuery lists each account or staff member with an entry
// at a school of the set in the window, with the newest name a ledger
// stored for it.
const accessTrailActorsQuery = `SELECT trail.tenant_id, COALESCE(trail.actor_account_id, 0) AS actor_account_id,
		COALESCE(trail.actor_staff_id, 0) AS actor_staff_id,
		COALESCE((array_agg(NULLIF(trail.actor_name, '') ORDER BY trail.occurred_at DESC)
			FILTER (WHERE trail.actor_name <> ''))[1], '') AS actor_name
	FROM (` + accessTrailEntries + `) AS trail
	WHERE trail.occurred_at >= ?1 AND trail.occurred_at < ?2
		AND (trail.actor_account_id IS NOT NULL OR trail.actor_staff_id IS NOT NULL)
	GROUP BY trail.tenant_id, trail.actor_account_id, trail.actor_staff_id
	ORDER BY trail.tenant_id, actor_account_id, actor_staff_id`

type accessTrailRow struct {
	ID             int64     `bun:"id"`
	TenantID       int64     `bun:"tenant_id"`
	OccurredAt     time.Time `bun:"occurred_at"`
	ActorAccountID *int64    `bun:"actor_account_id"`
	ActorStaffID   *int64    `bun:"actor_staff_id"`
	ActorName      string    `bun:"actor_name"`
	Kind           string    `bun:"kind"`
	Data           string    `bun:"data"`
}

// ListAccessTrail lists the entries of the filter, newest first.
func (s *AccessTrail) ListAccessTrail(ctx context.Context, filter auditlog.AccessTrailFilter) ([]auditlog.AccessTrailEntry, error) {
	if len(filter.Schools) == 0 {
		return nil, auditlog.ErrAccessTrailSchoolsRequired
	}
	result := []auditlog.AccessTrailEntry{}
	if filter.Limit <= 0 {
		return result, nil
	}
	db, err := runtimeDB(ctx, s.runtime)
	if err != nil {
		return nil, err
	}
	var rows []accessTrailRow
	if err := db.NewRaw(accessTrailQuery, pgdialect.Array(filter.Schools), filter.From, filter.To, filter.SchoolID,
		filter.AccountID, filter.Kind, filter.Limit, max(filter.Offset, 0), pgdialect.Array(nonNilIDs(filter.StaffIDs))).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("audit access trail: list entries: %w", err)
	}
	for _, row := range rows {
		result = append(result, auditlog.AccessTrailEntry{
			ID: row.ID, SchoolID: row.TenantID, OccurredAt: row.OccurredAt, AccountID: row.ActorAccountID, StaffID: row.ActorStaffID,
			ActorNameSnapshot: row.ActorName, Kind: row.Kind, Data: row.Data,
		})
	}
	return result, nil
}

// ListAccessTrailActors lists the accounts with an entry in the window.
func (s *AccessTrail) ListAccessTrailActors(ctx context.Context, schools []int64, from, to time.Time) ([]auditlog.AccessTrailActor, error) {
	if len(schools) == 0 {
		return nil, auditlog.ErrAccessTrailSchoolsRequired
	}
	db, err := runtimeDB(ctx, s.runtime)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		TenantID       int64  `bun:"tenant_id"`
		ActorAccountID int64  `bun:"actor_account_id"`
		ActorStaffID   int64  `bun:"actor_staff_id"`
		ActorName      string `bun:"actor_name"`
	}
	if err := db.NewRaw(accessTrailActorsQuery, pgdialect.Array(schools), from, to).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("audit access trail: list actors: %w", err)
	}
	result := make([]auditlog.AccessTrailActor, 0, len(rows))
	for _, row := range rows {
		result = append(result, auditlog.AccessTrailActor{
			SchoolID: row.TenantID, AccountID: row.ActorAccountID, StaffID: row.ActorStaffID, ActorNameSnapshot: row.ActorName,
		})
	}
	return result, nil
}

// nonNilIDs binds an absent ID list as the empty array, never as NULL.
func nonNilIDs(ids []int64) []int64 {
	if ids == nil {
		return []int64{}
	}
	return ids
}
