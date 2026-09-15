package services

import (
	"context"
	"slices"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type timeTrackingAuditReader struct {
	records audit.TimeTrackingAuditLogRepository
}

// NewTimeTrackingAuditReader adapts the audit-owned merged query for its consumer.
func NewTimeTrackingAuditReader(records audit.TimeTrackingAuditLogRepository) active.TimeTrackingAuditReader {
	return timeTrackingAuditReader{records: records}
}

func (r timeTrackingAuditReader) ValidSources() []string {
	return slices.Clone(audit.ValidAuditLogSources)
}

func (r timeTrackingAuditReader) ListEntries(ctx context.Context, filter active.TimeTrackingAuditFilter) ([]*active.TimeTrackingAuditEntry, error) {
	query := audit.TimeTrackingAuditLogFilter{
		From: auditFeedDate(filter.From), To: auditFeedDate(filter.To), StaffID: filter.StaffID,
		ActorStaffID: filter.ActorStaffID, Sources: filter.Sources, Limit: filter.Limit,
	}
	if filter.Cursor != nil {
		query.Cursor = &audit.TimeTrackingAuditLogCursor{OccurredAt: filter.Cursor.OccurredAt, Source: filter.Cursor.Source, EntryID: filter.Cursor.EntryID}
	}
	rows, err := r.records.ListEntries(ctx, query)
	if rows == nil {
		return nil, err
	}
	result := make([]*active.TimeTrackingAuditEntry, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		result[i] = &active.TimeTrackingAuditEntry{
			OccurredAt: row.OccurredAt, Source: row.Source, EntryID: row.EntryID,
			StaffID: row.StaffID, ActorStaffID: row.ActorStaffID, ActorIsSystem: row.ActorIsSystem,
			Reason: row.Reason, Detail: row.Detail,
		}
	}
	return result, err
}

func auditFeedDate(date *timezone.Date) *audit.Date {
	if date == nil {
		return nil
	}
	if date.IsZero() {
		value := audit.Date("")
		return &value
	}
	value := audit.NewDate(date.Year(), date.Month(), date.Day())
	return &value
}
