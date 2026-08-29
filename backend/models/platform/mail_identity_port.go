package platform

import "context"

// TenantMailIdentity is the outward mail identity of one school for
// tenant-bound e-mail (#1936).
//
// It deliberately carries ONLY a reply address. The visible From stays the
// central authenticated sender: putting a school's own domain in From without
// a verified domain or a connected mailbox would be sender spoofing and would
// break the SPF/DKIM/DMARC alignment we are still repairing (#1215). Moving
// the return path is the part that needs no verification and already fixes
// the reported pain — a parent's answer reaches the OGS instead of moto.
type TenantMailIdentity struct {
	ReplyToName    string
	ReplyToAddress string
}

// IsZero reports whether no reply address is configured. A zero identity must
// emit no Reply-To header at all, so the mail behaves exactly as it did
// before this feature.
func (i TenantMailIdentity) IsZero() bool { return i.ReplyToAddress == "" }

// TenantMailIdentityResolver resolves the mail identity of one tenant.
//
// Declared here (models/platform is a leaf package) so services/auth,
// services/enrollment, services/announcement and services/messaging can
// depend on it without importing services/platform — the same shape as
// OutboxEnqueuer in outbox_port.go.
type TenantMailIdentityResolver interface {
	ResolveTenantMailIdentity(ctx context.Context, tenantID int64) (TenantMailIdentity, error)
}
