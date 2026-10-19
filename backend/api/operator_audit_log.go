package api

import (
	"context"

	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"

	"encoding/json"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// setOperatorAuditChanges stores the change summary on an operator ledger
// entry as JSON; on an encoding error the entry keeps its changes.
func setOperatorAuditChanges(entry *auditModels.OperatorAuditEntry, changes map[string]any) error {
	encoded, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	entry.Changes = encoded
	return nil
}

// NewCommunicationAnnouncementAudit adapts the retained operator audit repository
// at the Root binding to the existing public owner facade.
func NewCommunicationAnnouncementAudit(create func(context.Context, *auditModels.OperatorAuditEntry) error) recordports.AuditFunc {
	return func(ctx context.Context, input recordports.AuditEntry) error {
		resourceID := input.ResourceID
		entry := &auditModels.OperatorAuditEntry{
			OperatorID: input.OperatorID, Action: input.Action,
			ResourceType: communication.TypeAnnouncement, ResourceID: &resourceID,
			RequestIP: input.RequestIP,
		}
		if input.Changes != nil {
			if err := setOperatorAuditChanges(entry, input.Changes); err != nil {
				return err
			}
		}
		return create(ctx, entry)
	}
}
