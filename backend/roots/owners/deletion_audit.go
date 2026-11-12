package owners

import (
	"context"

	audit "github.com/moto-ogs/project-phoenix/modules/auditlog/records"
	"github.com/moto-ogs/project-phoenix/modules/studentpresence/compose/presenceservice"
	workforceCompose "github.com/moto-ogs/project-phoenix/modules/workforce/compose"
)

type deletionAudit struct {
	writer DataDeletionBatchAppender
}

// NewDeletionAudit connects retention evidence to the existing audit writer.
func NewDeletionAudit(writer DataDeletionBatchAppender) presenceservice.DeletionAudit {
	if writer == nil {
		return nil
	}
	return deletionAudit{writer: writer}
}

// CreateAll appends the evidence of many students in one Audit append; a nil
// event reaches the writer as nil and fails there.
func (a deletionAudit) CreateAll(ctx context.Context, events []*presenceservice.DeletionEvent) error {
	entries := make([]*audit.DataDeletion, len(events))
	for i, event := range events {
		if event != nil {
			entries[i] = presenceDataDeletion(event)
		}
	}
	return a.writer.CreateAll(ctx, entries)
}

func presenceDataDeletion(event *presenceservice.DeletionEvent) *audit.DataDeletion {
	entry := &audit.DataDeletion{
		StudentID: event.StudentID, StaffID: event.StaffID, DeletionType: event.DeletionType,
		RecordsDeleted: event.RecordsDeleted, DeletionReason: event.DeletionReason,
		DeletedBy: event.DeletedBy, DeletedAt: event.DeletedAt, Metadata: event.Metadata,
	}
	entry.SetTenantID(event.TenantID)
	return entry
}

// NewTimeTrackingRetentionAudit connects the retained time-tracking cleanup's
// retention evidence to the same audit writer.
func NewTimeTrackingRetentionAudit(writer DataDeletionAppender) workforceCompose.DeletionAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingRetentionAudit{writer: writer}
}

type timeTrackingRetentionAudit struct {
	writer DataDeletionAppender
}

func (a timeTrackingRetentionAudit) Create(ctx context.Context, event *workforceCompose.DeletionEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	entry := &audit.DataDeletion{
		StudentID: event.StudentID, StaffID: event.StaffID, DeletionType: event.DeletionType,
		RecordsDeleted: event.RecordsDeleted, DeletionReason: event.DeletionReason,
		DeletedBy: event.DeletedBy, DeletedAt: event.DeletedAt, Metadata: event.Metadata,
	}
	entry.SetTenantID(event.TenantID)
	return a.writer.Create(ctx, entry)
}
