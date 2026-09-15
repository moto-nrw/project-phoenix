package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type deletionAudit struct {
	writer interface {
		Create(context.Context, *audit.DataDeletion) error
	}
}

// NewDeletionAudit connects retention evidence to the existing audit writer.
func NewDeletionAudit(writer interface {
	Create(context.Context, *audit.DataDeletion) error
}) active.DeletionAudit {
	if writer == nil {
		return nil
	}
	return deletionAudit{writer: writer}
}

func (a deletionAudit) Create(ctx context.Context, event *active.DeletionEvent) error {
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
