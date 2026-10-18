package api

import (
	"context"
	"fmt"

	"encoding/json"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
)

// NewOperatorAuditLogFromEntries serves the operator ledger the root records
// operator actions on over the Audit owner's platform ledger (#2720), which
// the Audit composition builds (ADR 0049).
func NewOperatorAuditLogFromEntries(entries operatorAuditEntries) operatorAuditEntries {
	if entries == nil {
		panic("operator audit log repository: audit ledger is required")
	}
	return operatorAuditLogRepository{entries: entries}
}

// operatorAuditLogRepository keeps the operator ledger's error reports over
// the Audit owner's ledger.
type operatorAuditLogRepository struct {
	entries operatorAuditEntries
}

func (r operatorAuditLogRepository) Create(ctx context.Context, entry *auditModels.OperatorAuditEntry) error {
	if entry == nil {
		return fmt.Errorf("database error during create audit log entry: audit log entry is required")
	}
	if err := r.entries.Create(ctx, entry); err != nil {
		return fmt.Errorf("database error during create audit log entry: %w", err)
	}
	return nil
}

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
func NewCommunicationAnnouncementAudit(create func(context.Context, *auditModels.OperatorAuditEntry) error) communicationCompose.AuditFunc {
	return func(ctx context.Context, input communicationCompose.AuditEntry) error {
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
