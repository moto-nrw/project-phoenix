package services

import (
	"context"
	"fmt"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/compose/vocabulary"
	platform "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
)

// newOperatorAuditLog serves the retained operator audit-log contract over
// the Audit owner's platform ledger (#2720), which the Audit composition
// builds (ADR 0049).
func newOperatorAuditLog(entries auditModels.OperatorAuditLogRepository) platform.OperatorAuditLogRepository {
	if entries == nil {
		panic("operator audit log repository: audit ledger is required")
	}
	return operatorAuditLogRepository{entries: entries}
}

// operatorAuditLogRepository is the compatibility adapter behind
// platform.OperatorAuditLogRepository over the Audit owner's ledger.
type operatorAuditLogRepository struct {
	entries auditModels.OperatorAuditLogRepository
}

func (r operatorAuditLogRepository) Create(ctx context.Context, entry *platform.OperatorAuditLog) error {
	if entry == nil {
		return fmt.Errorf("database error during create audit log entry: audit log entry is required")
	}
	stored := auditEntry(entry)
	if err := r.entries.Create(ctx, stored); err != nil {
		return fmt.Errorf("database error during create audit log entry: %w", err)
	}
	entry.ID = stored.ID
	entry.CreatedAt = stored.CreatedAt
	return nil
}

func (r operatorAuditLogRepository) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	entries, err := r.entries.FindByDateRange(ctx, start, end, limit)
	if err != nil {
		return nil, fmt.Errorf("database error during find audit logs by date range: %w", err)
	}
	return auditLogModels(entries), nil
}

func auditEntry(entry *platform.OperatorAuditLog) *auditModels.OperatorAuditEntry {
	return &auditModels.OperatorAuditEntry{
		ID: entry.ID, OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, Changes: entry.Changes, RequestIP: entry.RequestIP, CreatedAt: entry.CreatedAt,
	}
}

func auditLogModels(entries []*auditModels.OperatorAuditEntry) []*platform.OperatorAuditLog {
	result := make([]*platform.OperatorAuditLog, 0, len(entries))
	for _, entry := range entries {
		result = append(result, &platform.OperatorAuditLog{
			ID: entry.ID, OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
			ResourceID: entry.ResourceID, Changes: entry.Changes, RequestIP: entry.RequestIP, CreatedAt: entry.CreatedAt,
		})
	}
	return result
}
