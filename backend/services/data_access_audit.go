package services

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type dataAccessAudit struct {
	writer interface {
		Create(context.Context, *auditModels.DataAccessLog) error
	}
}

// NewDataAccessAudit connects attendance access evidence to the audit writer.
func NewDataAccessAudit(writer interface {
	Create(context.Context, *auditModels.DataAccessLog) error
}) active.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return dataAccessAudit{writer: writer}
}

func (a dataAccessAudit) Create(ctx context.Context, event *active.DataAccessEvent) error {
	if event == nil {
		return a.writer.Create(ctx, nil)
	}
	return a.writer.Create(ctx, &auditModels.DataAccessLog{
		ActorAccountID: event.ActorAccountID, ActorRole: event.ActorRole,
		ResourceType: event.ResourceType, StudentID: event.StudentID,
		RangeStart: event.RangeStart, RangeEnd: event.RangeEnd,
		AccessedAt: event.AccessedAt, Metadata: event.Metadata,
	})
}
