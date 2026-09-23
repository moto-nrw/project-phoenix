package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type deletionAudit struct {
	writer interface {
		Create(context.Context, *audit.DataDeletion) error
	}
}

// NewDeletionAudit connects retention evidence to the existing audit writer.
func NewDeletionAudit(writer interface {
	Create(context.Context, *audit.DataDeletion) error
}) presenceservice.DeletionAudit {
	if writer == nil {
		return nil
	}
	return deletionAudit{writer: writer}
}

func (a deletionAudit) Create(ctx context.Context, event *presenceservice.DeletionEvent) error {
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

// NewTimeTrackingRetentionAudit connects the retained time-tracking cleanup's
// retention evidence to the same audit writer.
func NewTimeTrackingRetentionAudit(writer interface {
	Create(context.Context, *audit.DataDeletion) error
}) timetracking.DeletionAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingRetentionAudit{writer: writer}
}

type timeTrackingRetentionAudit struct {
	writer interface {
		Create(context.Context, *audit.DataDeletion) error
	}
}

func (a timeTrackingRetentionAudit) Create(ctx context.Context, event *timetracking.DeletionEvent) error {
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
