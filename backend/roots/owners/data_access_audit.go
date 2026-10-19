package owners

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type dataAccessAudit struct {
	writer DataAccessAppender
}

// NewDataAccessAudit connects attendance access evidence to the audit writer.
func NewDataAccessAudit(writer DataAccessAppender) presenceservice.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return dataAccessAudit{writer: writer}
}

func (a dataAccessAudit) Create(ctx context.Context, event *presenceservice.DataAccessEvent) error {
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

// NewTimeTrackingDataAccessAudit connects the retained time export's access
// evidence to the audit writer.
func NewTimeTrackingDataAccessAudit(writer DataAccessAppender) workforceCompose.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingDataAccessAudit{writer: writer}
}

type timeTrackingDataAccessAudit struct {
	writer DataAccessAppender
}

func (a timeTrackingDataAccessAudit) Create(ctx context.Context, event *workforceCompose.DataAccessEvent) error {
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
