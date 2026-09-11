package audit

import (
	"context"
	"encoding/json"
	"net"
	"time"
)

// OperatorAuditEntry is one platform-operator action in the Audit-owned
// platform.operator_audit_log ledger. Operators act platform-wide, so the
// entry carries no tenant; the affected school, if any, is part of Changes.
type OperatorAuditEntry struct {
	ID           int64           `bun:"id,pk,autoincrement" json:"id"`
	OperatorID   int64           `bun:"operator_id,notnull" json:"operator_id"`
	Action       string          `bun:"action,notnull" json:"action"`
	ResourceType string          `bun:"resource_type,notnull" json:"resource_type"`
	ResourceID   *int64          `bun:"resource_id" json:"resource_id,omitempty"`
	Changes      json.RawMessage `bun:"changes,type:jsonb" json:"changes,omitempty"`
	RequestIP    net.IP          `bun:"request_ip,type:inet,nullzero" json:"request_ip,omitempty"`
	CreatedAt    time.Time       `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
}

// PlatformScoped marks the entry as tenantless: the Audit adapter appends it
// without the tenant handshake every tenant-scoped ledger requires.
func (*OperatorAuditEntry) PlatformScoped() bool { return true }

// OperatorAuditLogRepository appends and reads platform-operator actions.
type OperatorAuditLogRepository interface {
	Create(ctx context.Context, entry *OperatorAuditEntry) error
	FindByOperatorID(ctx context.Context, operatorID int64, limit int) ([]*OperatorAuditEntry, error)
	FindByDateRange(ctx context.Context, start, end time.Time, limit int) ([]*OperatorAuditEntry, error)
}
