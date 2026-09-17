package platform

import (
	"context"
	"time"
)

// The operator identity rows and their refresh sessions are owned by the
// Identity & Access module (#2720, #3252); the retained flows in
// services/platform reach them through their own consumer-owned ports. The
// same holds for the operator MFA records (#2723), the operator invitation
// and e-mail change links (#2722) and the operator passkey records (#2724).

// OperatorAuditLogRepository defines operations for the audit log
type OperatorAuditLogRepository interface {
	// Create a new audit log entry
	Create(ctx context.Context, entry *OperatorAuditLog) error

	// Query audit logs
	FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*OperatorAuditLog, error)
}
