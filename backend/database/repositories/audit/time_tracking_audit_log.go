package audit

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type timeTrackingAuditLogRepository struct {
	db *bun.DB
}

func NewTimeTrackingAuditLogRepository(db *bun.DB) auditModels.TimeTrackingAuditLogRepository {
	return &timeTrackingAuditLogRepository{db: db}
}

// auditLogBranches maps each source to its branch SELECT. Every branch
// yields the same envelope columns:
//
//	occurred_at, source, entry_id, staff_id (display; NULL for school-wide
//	events), staff_ids (filter; all affected staff), actor_staff_id,
//	actor_is_system, reason, detail (jsonb)
//
// Grouping happens here, not in the service, because the keyset cursor must
// paginate over final events: session edits of one save action collapse to
// one event (same grouping key the detail accordion uses — shared
// created_at), and a school-wide month close collapses to one event per
// close action instead of one row per account.
//
// The absence trail stores actor as an account id (active.staff_absence_audit
// FK to auth.accounts); the account → person → staff join resolves it so the
// actor filter and name resolution are uniform. A NULL result (actor without
// a staff row) stays NULL and renders as unknown.
//
// The %s in every branch is the tenant predicate — defense-in-depth on top
// of RLS, same as the generic TenantScoped path.
var auditLogBranches = map[string]string{
	auditModels.AuditLogSourceSessionEdit: `
		SELECT e.created_at AS occurred_at, 'session_edit' AS source, MIN(e.id) AS entry_id,
			e.staff_id AS staff_id, ARRAY[e.staff_id] AS staff_ids,
			NULLIF(e.edited_by, 0) AS actor_staff_id, e.edited_by = 0 AS actor_is_system,
			COALESCE(MAX(NULLIF(e.notes, '')), '') AS reason,
			jsonb_build_object(
				'session_id', e.session_id,
				'session_date', to_char(ws.date, 'YYYY-MM-DD'),
				'fields', jsonb_agg(jsonb_build_object(
					'field', e.field_name, 'old', e.old_value, 'new', e.new_value
				) ORDER BY e.id)
			) AS detail
		FROM audit.work_session_edits e
		JOIN active.work_sessions ws ON ws.id = e.session_id
		WHERE e.tenant_id = %s
		GROUP BY e.session_id, e.staff_id, e.edited_by, e.created_at, ws.date`,
	auditModels.AuditLogSourceAbsence: `
		SELECT saa.changed_at AS occurred_at, 'absence' AS source, saa.id AS entry_id,
			sa.staff_id AS staff_id, ARRAY[sa.staff_id] AS staff_ids,
			st.id AS actor_staff_id, FALSE AS actor_is_system,
			COALESCE(saa.note, '') AS reason,
			jsonb_build_object(
				'absence_id', sa.id, 'absence_type', sa.absence_type,
				'date_start', to_char(sa.date_start, 'YYYY-MM-DD'),
				'date_end', to_char(sa.date_end, 'YYYY-MM-DD'),
				'from_status', saa.from_status, 'to_status', saa.to_status
			) AS detail
		FROM active.staff_absence_audit saa
		JOIN active.staff_absences sa ON sa.id = saa.absence_id
		LEFT JOIN users.persons p ON p.account_id = saa.actor_id
		LEFT JOIN users.staff st ON st.person_id = p.id
		WHERE saa.tenant_id = %s`,
	auditModels.AuditLogSourceAdjustment: `
		SELECT ba.decided_at AS occurred_at, 'adjustment' AS source, ba.id AS entry_id,
			ba.staff_id AS staff_id, ARRAY[ba.staff_id] AS staff_ids,
			ba.decided_by AS actor_staff_id, FALSE AS actor_is_system,
			ba.note AS reason,
			jsonb_build_object(
				'adjustment_id', ba.id, 'type', ba.type,
				'minutes_delta', ba.minutes_delta,
				'effective_date', to_char(ba.effective_date, 'YYYY-MM-DD')
			) AS detail
		FROM active.staff_balance_adjustments ba
		WHERE ba.tenant_id = %s`,
	auditModels.AuditLogSourceMonthClose: `
		SELECT s.closed_at AS occurred_at, 'month_close' AS source, MIN(s.id) AS entry_id,
			NULL::bigint AS staff_id, array_agg(s.staff_id) AS staff_ids,
			s.closed_by AS actor_staff_id, FALSE AS actor_is_system,
			s.close_reason AS reason,
			jsonb_build_object(
				'year', s.year, 'month', s.month, 'account_count', COUNT(*)
			) AS detail
		FROM active.staff_month_balance_snapshots s
		WHERE s.tenant_id = %s
		GROUP BY s.closed_at, s.closed_by, s.year, s.month, s.close_reason`,
	auditModels.AuditLogSourceMonthReopen: `
		SELECT s.reopened_at AS occurred_at, 'month_reopen' AS source, s.id AS entry_id,
			s.staff_id AS staff_id, ARRAY[s.staff_id] AS staff_ids,
			s.reopened_by AS actor_staff_id, FALSE AS actor_is_system,
			s.reopen_reason AS reason,
			jsonb_build_object(
				'year', s.year, 'month', s.month,
				'closing_balance_minutes', s.closing_balance_minutes
			) AS detail
		FROM active.staff_month_balance_snapshots s
		WHERE s.reopened_at IS NOT NULL AND s.tenant_id = %s`,
	auditModels.AuditLogSourceDeletion: `
		SELECT d.occurred_at AS occurred_at, 'deletion' AS source, d.id AS entry_id,
			d.staff_id AS staff_id, ARRAY[d.staff_id] AS staff_ids,
			d.deleted_by AS actor_staff_id, FALSE AS actor_is_system,
			d.note AS reason,
			jsonb_build_object(
				'deleted_source', d.source, 'source_id', d.source_id, 'payload', d.payload
			) AS detail
		FROM audit.time_tracking_deletions d
		WHERE d.tenant_id = %s`,
}

const auditLogDefaultLimit = 50

// ListEntries merges the selected branches with UNION ALL and paginates the
// result with a (occurred_at, source, entry_id) descending keyset. A raw
// query is unavoidable here: the feed is a cross-schema union with per-branch
// grouping that no single model maps to.
func (r *timeTrackingAuditLogRepository) ListEntries(ctx context.Context, filter auditModels.TimeTrackingAuditLogFilter) ([]*auditModels.TimeTrackingAuditLogEntry, error) {
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return nil, fmt.Errorf("time tracking audit log requires a tenant context")
	}

	sources := filter.Sources
	if len(sources) == 0 {
		sources = auditModels.ValidAuditLogSources
	}
	branches := make([]string, 0, len(sources))
	args := make([]any, 0, 16)
	for _, src := range auditModels.ValidAuditLogSources {
		branch, ok := auditLogBranches[src]
		if !ok || !slices.Contains(sources, src) {
			continue
		}
		branches = append(branches, fmt.Sprintf(branch, "?"))
		args = append(args, tenantID)
	}
	if len(branches) == 0 {
		return nil, fmt.Errorf("no valid audit log sources selected")
	}

	var sb strings.Builder
	sb.WriteString("SELECT ev.occurred_at, ev.source, ev.entry_id, ev.staff_id, ev.actor_staff_id, ev.actor_is_system, ev.reason, ev.detail FROM (")
	sb.WriteString(strings.Join(branches, "\n\t\tUNION ALL\n"))
	sb.WriteString(") ev WHERE 1=1")

	if filter.From != nil {
		sb.WriteString(" AND ev.occurred_at >= ?")
		args = append(args, filter.From.BerlinMidnight())
	}
	if filter.To != nil {
		sb.WriteString(" AND ev.occurred_at <= ?")
		args = append(args, filter.To.EndOfDay())
	}
	if filter.StaffID > 0 {
		sb.WriteString(" AND ? = ANY(ev.staff_ids)")
		args = append(args, filter.StaffID)
	}
	if filter.ActorStaffID > 0 {
		sb.WriteString(" AND ev.actor_staff_id = ?")
		args = append(args, filter.ActorStaffID)
	}
	if filter.Cursor != nil {
		sb.WriteString(" AND (ev.occurred_at, ev.source, ev.entry_id) < (?, ?, ?)")
		args = append(args, filter.Cursor.OccurredAt, filter.Cursor.Source, filter.Cursor.EntryID)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = auditLogDefaultLimit
	}
	sb.WriteString(" ORDER BY ev.occurred_at DESC, ev.source DESC, ev.entry_id DESC LIMIT ?")
	args = append(args, limit)

	var entries []*auditModels.TimeTrackingAuditLogEntry
	if err := base.GetDB(ctx, r.db).NewRaw(sb.String(), args...).Scan(ctx, &entries); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list time tracking audit log", Err: err}
	}
	return entries, nil
}
