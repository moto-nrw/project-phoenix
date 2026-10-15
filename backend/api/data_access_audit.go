package api

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The adapters below record the access evidence of Student Presence and
// Workforce in the Audit Platform's data access log (#4013).

type dataAccessAudit struct {
	writer services.DataAccessAppender
}

// NewDataAccessAudit connects attendance access evidence to the audit writer.
func NewDataAccessAudit(writer services.DataAccessAppender) presenceservice.DataAccessAudit {
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
func NewTimeTrackingDataAccessAudit(writer services.DataAccessAppender) workforceCompose.DataAccessAudit {
	if writer == nil {
		return nil
	}
	return timeTrackingDataAccessAudit{writer: writer}
}

type timeTrackingDataAccessAudit struct {
	writer services.DataAccessAppender
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
