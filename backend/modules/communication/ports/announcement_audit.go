package ports

import (
	"context"
	"net"
)

type AuditEntry struct {
	OperatorID int64
	Action     string
	ResourceID int64
	RequestIP  net.IP
	Changes    map[string]any
}

type Audit interface {
	AppendAnnouncementAudit(context.Context, AuditEntry) error
}

type AuditFunc func(context.Context, AuditEntry) error

func (f AuditFunc) AppendAnnouncementAudit(ctx context.Context, entry AuditEntry) error {
	return f(ctx, entry)
}
