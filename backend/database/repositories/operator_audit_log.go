package repositories

import (
	"context"
	"errors"
	"time"

	auditRepo "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/platform"
	authRepo "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/authpostgres"
)

// NewOperatorAuditLogRepository serves the retained operator audit-log
// contract over the Audit owner's platform ledger (#2720).
func NewOperatorAuditLogRepository(entries auditModels.OperatorAuditLogRepository) platform.OperatorAuditLogRepository {
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
		return authRepo.DatabaseError("create audit log entry", errors.New("audit log entry is required"))
	}
	stored := auditEntry(entry)
	if err := r.entries.Create(ctx, stored); err != nil {
		return authRepo.DatabaseError("create audit log entry", err)
	}
	entry.ID = stored.ID
	entry.CreatedAt = stored.CreatedAt
	return nil
}

func (r operatorAuditLogRepository) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*platform.OperatorAuditLog, error) {
	entries, err := r.entries.FindByDateRange(ctx, start, end, limit)
	if err != nil {
		return nil, authRepo.DatabaseError("find audit logs by date range", err)
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

// newOperatorAuditLog binds the Audit-owned operator ledger for the factory.
func newOperatorAuditLog(runtime auditRepo.Runtime) platform.OperatorAuditLogRepository {
	return NewOperatorAuditLogRepository(auditRepo.NewOperatorAuditLogRepository(runtime))
}
