package owners

import (
	"context"
	"slices"

	audit "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// workSessionEditTrail is the Audit Platform's trail of work-session edits.
type workSessionEditTrail interface {
	CreateBatch(context.Context, []*audit.WorkSessionEdit) error
	GetBySessionID(context.Context, int64) ([]*audit.WorkSessionEdit, error)
	CountBySessionIDs(context.Context, []int64) (map[int64]int, error)
	CountManualBySessionIDs(context.Context, []int64) (map[int64]int, error)
}

type workSessionAudit struct {
	workSessionEditTrail
}

// NewWorkSessionAudit projects audit storage onto the work-session contract.
func NewWorkSessionAudit(trail workSessionEditTrail) workforceCompose.WorkSessionAudit {
	if trail == nil {
		return nil
	}
	return workSessionAudit{trail}
}

func (a workSessionAudit) CreateBatch(ctx context.Context, edits []*workforceCompose.WorkSessionEdit) error {
	var rows []*audit.WorkSessionEdit
	if edits != nil {
		rows = make([]*audit.WorkSessionEdit, len(edits))
	}
	for i, edit := range edits {
		if edit == nil {
			continue
		}
		row := &audit.WorkSessionEdit{ID: edit.ID, SessionID: edit.SessionID, StaffID: edit.StaffID, EditedBy: edit.EditedBy, FieldName: edit.FieldName, OldValue: edit.OldValue, NewValue: edit.NewValue, Notes: edit.Notes, CreatedAt: edit.CreatedAt}
		row.SetTenantID(edit.TenantID)
		rows[i] = row
	}
	err := a.workSessionEditTrail.CreateBatch(ctx, rows)
	for i, row := range rows {
		if row != nil && edits[i] != nil {
			*edits[i] = *projectWorkSessionEdit(row)
		}
	}
	return err
}

func (a workSessionAudit) GetBySessionID(ctx context.Context, id int64) ([]*workforceCompose.WorkSessionEdit, error) {
	rows, err := a.workSessionEditTrail.GetBySessionID(ctx, id)
	if rows == nil {
		return nil, err
	}
	result := make([]*workforceCompose.WorkSessionEdit, len(rows))
	for i, row := range rows {
		result[i] = projectWorkSessionEdit(row)
	}
	return result, err
}

func projectWorkSessionEdit(row *audit.WorkSessionEdit) *workforceCompose.WorkSessionEdit {
	if row == nil {
		return nil
	}
	return &workforceCompose.WorkSessionEdit{ID: row.ID, TenantID: row.TenantID, SessionID: row.SessionID, StaffID: row.StaffID, EditedBy: row.EditedBy, FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue, Notes: row.Notes, CreatedAt: row.CreatedAt}
}

// timeTrackingDeletionTrail is the Audit Platform's trail of deleted
// time-tracking records.
type timeTrackingDeletionTrail interface {
	Create(context.Context, *audit.TimeTrackingDeletion) error
}

type timeTrackingDeletionAudit struct {
	writer timeTrackingDeletionTrail
}

// NewTimeTrackingDeletionAudit keeps audit persistence outside the producer.
func NewTimeTrackingDeletionAudit(writer timeTrackingDeletionTrail) workforceCompose.TimeTrackingDeletionAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingDeletionAudit{writer: writer}
}

func (a timeTrackingDeletionAudit) Create(ctx context.Context, event *workforceCompose.TimeTrackingDeletionEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	return a.writer.Create(ctx, &audit.TimeTrackingDeletion{
		StaffID: event.StaffID, Source: event.Source, SourceID: event.SourceID,
		DeletedBy: event.DeletedBy, Payload: event.Payload, Note: event.Note, OccurredAt: event.OccurredAt,
	})
}

// timeTrackingAuditFeed is the Audit Platform's merged time-tracking feed.
type timeTrackingAuditFeed interface {
	ListEntries(context.Context, audit.TimeTrackingAuditLogFilter) ([]*audit.TimeTrackingAuditLogEntry, error)
}

type timeTrackingAuditReader struct {
	records timeTrackingAuditFeed
}

// NewTimeTrackingAuditReader adapts the audit-owned merged query for its consumer.
func NewTimeTrackingAuditReader(records timeTrackingAuditFeed) workforceCompose.TimeTrackingAuditReader {
	return timeTrackingAuditReader{records: records}
}

func (r timeTrackingAuditReader) ValidSources() []string {
	return slices.Clone(audit.ValidAuditLogSources)
}

func (r timeTrackingAuditReader) ListEntries(ctx context.Context, filter workforceCompose.TimeTrackingAuditFilter) ([]*workforceCompose.TimeTrackingAuditEntry, error) {
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
	result := make([]*workforceCompose.TimeTrackingAuditEntry, len(rows))
	for i, row := range rows {
		if row == nil {
			continue
		}
		result[i] = &workforceCompose.TimeTrackingAuditEntry{
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
