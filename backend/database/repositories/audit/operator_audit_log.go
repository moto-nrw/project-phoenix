package audit

import (
	"context"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

// OperatorAuditLogRepository appends and reads the platform-operator action
// ledger (platform.operator_audit_log). Appends go through the shared
// Appender so they join the caller's transaction exactly like every other
// Audit ledger; the operator flows rely on that for revocation evidence.
type OperatorAuditLogRepository struct {
	runtime Runtime
}

func NewOperatorAuditLogRepository(runtime Runtime) *OperatorAuditLogRepository {
	return &OperatorAuditLogRepository{runtime: requireRuntime(runtime)}
}

func (r *OperatorAuditLogRepository) Create(ctx context.Context, entry *auditModels.OperatorAuditEntry) error {
	return NewAppender(r.runtime).Append(ctx, entry)
}

func (r *OperatorAuditLogRepository) FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*auditModels.OperatorAuditEntry, error) {
	var entries []*auditModels.OperatorAuditEntry
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&entries).
		ModelTableExpr(`platform.operator_audit_log AS "operator_audit_entry"`).
		Where(`"operator_audit_entry".operator_id = ?`, operatorID).
		OrderExpr(`"operator_audit_entry".created_at DESC`)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("find operator audit entries by operator id", err)
	}
	return entries, nil
}

func (r *OperatorAuditLogRepository) FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*auditModels.OperatorAuditEntry, error) {
	var entries []*auditModels.OperatorAuditEntry
	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&entries).
		ModelTableExpr(`platform.operator_audit_log AS "operator_audit_entry"`).
		Where(`"operator_audit_entry".created_at >= ?`, start).
		Where(`"operator_audit_entry".created_at <= ?`, end).
		OrderExpr(`"operator_audit_entry".created_at DESC`)
	if limit > 0 {
		query = query.Limit(limit)
	}
	if err := query.Scan(ctx); err != nil {
		return nil, wrapDatabase("find operator audit entries by date range", err)
	}
	return entries, nil
}
