package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type timeTrackingDeletionAudit struct {
	writer audit.TimeTrackingDeletionRepository
}

// NewTimeTrackingDeletionAudit keeps audit persistence outside the producer.
func NewTimeTrackingDeletionAudit(writer audit.TimeTrackingDeletionRepository) active.TimeTrackingDeletionAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingDeletionAudit{writer: writer}
}

func (a timeTrackingDeletionAudit) Create(ctx context.Context, event *active.TimeTrackingDeletionEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	return a.writer.Create(ctx, &audit.TimeTrackingDeletion{
		StaffID: event.StaffID, Source: event.Source, SourceID: event.SourceID,
		DeletedBy: event.DeletedBy, Payload: event.Payload, Note: event.Note, OccurredAt: event.OccurredAt,
	})
}
