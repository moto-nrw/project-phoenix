package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/services"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
)

// caregiverCapabilityAuditIP stands in for the client address: the change is
// recorded by the service, not by a login request.
const caregiverCapabilityAuditIP = "0.0.0.0"

// caregiverCapabilityAudit records People Directory's caregiver capability
// changes as authentication audit events (#3754).
type caregiverCapabilityAudit struct {
	events services.AuthEventAppender
}

// newCaregiverCapabilityAudit binds the audit port. A nil repository leaves
// the port unbound, so the capability refuses the change instead of
// committing it without an audit event.
func newCaregiverCapabilityAudit(events services.AuthEventAppender) peopleCompose.CaregiverCapabilityAudit {
	if events == nil {
		return nil
	}
	return caregiverCapabilityAudit{events: events}
}

func (a caregiverCapabilityAudit) RecordCaregiverCapabilityChange(ctx context.Context, accountID int64, enabled bool, metadata map[string]any) error {
	eventType := auditModels.EventTypeCaregiverCapabilityDisabled
	if enabled {
		eventType = auditModels.EventTypeCaregiverCapabilityEnabled
	}
	event := auditModels.NewAuthEvent(accountID, eventType, true, caregiverCapabilityAuditIP)
	for key, value := range metadata {
		event.SetMetadata(key, value)
	}
	return a.events.Create(ctx, event)
}
