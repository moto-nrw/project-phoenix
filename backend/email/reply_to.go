package email

import (
	"context"
	"log/slog"
)

// ReplyToIdentity is the reply address for a tenant-bound email.
type ReplyToIdentity struct {
	Name    string
	Address string
}

// IsZero reports whether no reply address is configured.
func (i ReplyToIdentity) IsZero() bool { return i.Address == "" }

// ReplyToResolver resolves the reply address of one tenant.
type ReplyToResolver interface {
	ResolveReplyTo(ctx context.Context, tenantID int64) (ReplyToIdentity, error)
}

// ResolveReplyToIdentity degrades a failed lookup to no reply address so a
// missing return path cannot prevent a message from being sent.
func ResolveReplyToIdentity(ctx context.Context, resolver ReplyToResolver, tenantID int64, logger *slog.Logger) ReplyToIdentity {
	if resolver == nil {
		return ReplyToIdentity{}
	}
	identity, err := resolver.ResolveReplyTo(ctx, tenantID)
	if err != nil {
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("failed to resolve tenant reply-to, sending without it",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return ReplyToIdentity{}
	}
	return identity
}
