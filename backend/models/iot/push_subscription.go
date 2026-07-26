package iot

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Push subscription portal constants: through which portal the device
// registered. Tenant/admin-scoped pushes go only to staff devices, guardian
// pushes only to parent devices.
const (
	PushPortalStaff  = "staff"
	PushPortalParent = "parent"
)

// PushSubscription is one browser/device registration for Web Push
// (RFC 8030). endpoint/p256dh/auth come verbatim from the browser's
// PushSubscription JSON. One account can hold many rows (multiple devices);
// a guardian linked to several schools holds one row per school for the
// same endpoint.
type PushSubscription struct {
	base.Model `bun:"schema:iot,table:push_subscriptions"`
	base.TenantModel
	AccountID int64  `bun:"account_id,notnull" json:"account_id"`
	Portal    string `bun:"portal,notnull" json:"portal"`
	Endpoint  string `bun:"endpoint,notnull" json:"endpoint"`
	P256dh    string `bun:"p256dh,notnull" json:"-"`
	Auth      string `bun:"auth,notnull" json:"-"`
	UserAgent string `bun:"user_agent,notnull,default:''" json:"user_agent"`
}

// Validate ensures push subscription data is valid.
func (s *PushSubscription) Validate() error {
	if s.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	if s.Portal != PushPortalStaff && s.Portal != PushPortalParent {
		return errors.New("portal must be 'staff' or 'parent'")
	}
	if strings.TrimSpace(s.Endpoint) == "" {
		return errors.New("endpoint is required")
	}
	if !strings.HasPrefix(s.Endpoint, "https://") {
		return errors.New("endpoint must be an https URL")
	}
	if strings.TrimSpace(s.P256dh) == "" || strings.TrimSpace(s.Auth) == "" {
		return errors.New("subscription keys are required")
	}
	return nil
}

// PushSubscriptionRepository defines operations for Web Push subscriptions.
// All reads/writes are tenant-scoped (RLS); callers outside tenant middleware
// must wrap calls in tenant.WithTenantTx.
type PushSubscriptionRepository interface {
	base.CRUDRepository[*PushSubscription]

	// Upsert inserts or refreshes a subscription keyed by (tenant_id, endpoint).
	Upsert(ctx context.Context, sub *PushSubscription) error
	// DeleteByEndpoint removes the caller's subscription for the current tenant.
	DeleteByEndpoint(ctx context.Context, accountID int64, endpoint string) error
	// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
	FindForTenantStaff(ctx context.Context) ([]*PushSubscription, error)
	// FindForTenantAdmins returns staff-portal subscriptions of accounts holding
	// the admin role in the current tenant.
	FindForTenantAdmins(ctx context.Context) ([]*PushSubscription, error)
	// FindForGuardian returns parent-portal subscriptions of one guardian account
	// in the current tenant.
	FindForGuardian(ctx context.Context, guardianAccountID int64) ([]*PushSubscription, error)
}
