package api

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
)

// caregiverCapabilityAudit records People Directory's caregiver capability
// changes as authentication audit events (#3754).
type caregiverCapabilityAudit struct {
	events AuthEventAppender
}

// newCaregiverCapabilityAudit binds the audit port. A nil repository leaves
// the port unbound, so the capability refuses the change instead of
// committing it without an audit event.
func newCaregiverCapabilityAudit(events AuthEventAppender) peoplePorts.CaregiverCapabilityAudit {
	if events == nil {
		return nil
	}
	return caregiverCapabilityAudit{events: events}
}

func (a caregiverCapabilityAudit) RecordCaregiverCapabilityChange(ctx context.Context, accountID int64, enabled bool, metadata map[string]any) error {
	event := auditModels.NewCaregiverCapabilityAuthEvent(accountID, enabled)
	for key, value := range metadata {
		event.SetMetadata(key, value)
	}
	return a.events.Create(ctx, event)
}
