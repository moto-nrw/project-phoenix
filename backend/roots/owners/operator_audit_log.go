package owners

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
)

// NewOperatorAuditLogFromEntries serves the operator ledger the root records
// operator actions on over the Audit owner's platform ledger (#2720), which
// the Audit composition builds (ADR 0049).
func NewOperatorAuditLogFromEntries(entries OperatorAuditEntries) OperatorAuditEntries {
	if entries == nil {
		panic("operator audit log repository: audit ledger is required")
	}
	return operatorAuditLogRepository{entries: entries}
}

// operatorAuditLogRepository keeps the operator ledger's error reports over
// the Audit owner's ledger.
type operatorAuditLogRepository struct {
	entries OperatorAuditEntries
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
