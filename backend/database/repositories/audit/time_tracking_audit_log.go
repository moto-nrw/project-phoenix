package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

const auditLogDefaultLimit = 50

const timeTrackingAuditLogViewQuery = `
SELECT ev.occurred_at, ev.source, ev.entry_id, ev.staff_id,
       ev.actor_staff_id, ev.actor_is_system, ev.reason, ev.detail
FROM audit.time_tracking_audit_log ev
WHERE ev.tenant_id = ?
  AND ((? AND ev.source = 'session_edit')
    OR (? AND ev.source = 'absence')
    OR (? AND ev.source = 'adjustment')
    OR (? AND ev.source = 'month_close')
    OR (? AND ev.source = 'month_reopen')
    OR (? AND ev.source = 'deletion')
    OR (? AND ev.source = 'personnel_number')
    OR (? AND ev.source = 'vacation_opening')
    OR (? AND ev.source = 'absence_type_allowance'))
  AND (?::timestamptz IS NULL OR ev.occurred_at >= ?)
  AND (?::timestamptz IS NULL OR ev.occurred_at < ?)
  AND (?::bigint = 0 OR ? = ANY(ev.staff_ids))
  AND (?::bigint = 0 OR ev.actor_staff_id = ?)
  AND (?::timestamptz IS NULL OR (ev.occurred_at, ev.source, ev.entry_id) < (?, ?, ?))
ORDER BY ev.occurred_at DESC, ev.source DESC, ev.entry_id DESC
LIMIT ?`

type timeTrackingAuditLogRepository struct {
	runtime Runtime
}

func NewTimeTrackingAuditLogRepository(runtime Runtime) auditModels.TimeTrackingAuditLogRepository {
	return &timeTrackingAuditLogRepository{runtime: requireRuntime(runtime)}
}

// ListEntries reads the Audit-owned projection and paginates its final events
// with a descending (occurred_at, source, entry_id) keyset.
func (r *timeTrackingAuditLogRepository) ListEntries(ctx context.Context, filter auditModels.TimeTrackingAuditLogFilter) ([]*auditModels.TimeTrackingAuditLogEntry, error) {
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return nil, fmt.Errorf("time tracking audit log requires a tenant context")
	}

	selected := make([]bool, len(auditModels.ValidAuditLogSources))
	selectedCount := 0
	for index, source := range auditModels.ValidAuditLogSources {
		selected[index] = len(filter.Sources) == 0 || containsSource(filter.Sources, source)
		if selected[index] {
			selectedCount++
		}
	}
	if selectedCount == 0 {
		return nil, fmt.Errorf("no valid audit log sources selected")
	}

	var from, to, cursorAt any
	if filter.From != nil {
		from = filter.From.BerlinMidnight()
	}
	if filter.To != nil {
		to = filter.To.AddDays(1).BerlinMidnight()
	}
	cursorSource := ""
	var cursorID int64
	if filter.Cursor != nil {
		cursorAt = filter.Cursor.OccurredAt
		cursorSource = filter.Cursor.Source
		cursorID = filter.Cursor.EntryID
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = auditLogDefaultLimit
	}

	var entries []*auditModels.TimeTrackingAuditLogEntry
	if err := runtimeDB(ctx, r.runtime).NewRaw(timeTrackingAuditLogViewQuery,
		tenantID,
		selected[0], selected[1], selected[2], selected[3], selected[4], selected[5], selected[6], selected[7], selected[8],
		from, from, to, to,
		filter.StaffID, filter.StaffID,
		filter.ActorStaffID, filter.ActorStaffID,
		cursorAt, cursorAt, cursorSource, cursorID, limit,
	).Scan(ctx, &entries); err != nil {
		return nil, wrapDatabase("list time tracking audit log", err)
	}
	return entries, nil
}

func containsSource(sources []string, candidate string) bool {
	for _, source := range sources {
		if source == candidate {
			return true
		}
	}
	return false
}
