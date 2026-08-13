package iot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
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

var trustedPushServiceHosts = map[string]struct{}{
	"fcm.googleapis.com":                {},
	"updates.push.services.mozilla.com": {},
	"web.push.apple.com":                {},
}

// ValidatePushEndpoint restricts outbound delivery to browser-managed push
// services. Subscription endpoints are otherwise attacker-controlled URLs and
// must never turn the notification worker into a general-purpose HTTP client.
func ValidatePushEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint must be a valid URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" {
		return errors.New("endpoint must be an https URL")
	}
	if parsed.User != nil || parsed.Fragment != "" {
		return errors.New("endpoint must not contain user information or a fragment")
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return errors.New("endpoint must use the default https port")
	}

	host := strings.ToLower(parsed.Hostname())
	if _, ok := trustedPushServiceHosts[host]; ok {
		return nil
	}
	if strings.HasSuffix(host, ".notify.windows.com") &&
		host != "notify.windows.com" {
		return nil
	}
	return errors.New("endpoint host is not a trusted push service")
}

// PushSubscription is one browser/device registration for Web Push
// (RFC 8030). endpoint/p256dh/auth come verbatim from the browser's
// PushSubscription JSON. One account can hold many rows (multiple devices);
// a guardian linked to several schools holds one row per school for the
// same endpoint.
type PushSubscription struct {
	base.Model `bun:"schema:iot,table:push_subscriptions"`
	base.TenantModel
	AccountID     int64  `bun:"account_id,notnull" json:"account_id"`
	Portal        string `bun:"portal,notnull" json:"portal"`
	Endpoint      string `bun:"endpoint,notnull" json:"endpoint"`
	P256dh        string `bun:"p256dh,notnull" json:"-"`
	Auth          string `bun:"auth,notnull" json:"-"`
	UserAgent     string `bun:"user_agent,notnull,default:''" json:"user_agent"`
	TokenFamilyID string `bun:"token_family_id,notnull,default:''" json:"-"`
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
	if err := ValidatePushEndpoint(s.Endpoint); err != nil {
		return err
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
	// DeleteExpiredIfUnchanged removes a subscription only if its persisted
	// ownership, key material, and update timestamp still match the sent snapshot.
	DeleteExpiredIfUnchanged(ctx context.Context, sub *PushSubscription) (bool, error)
	// DeleteParentByEndpoint removes every parent-portal binding for an endpoint
	// across tenants. Callers must use an admin transaction.
	DeleteParentByEndpoint(ctx context.Context, endpoint string) error
	// DeleteStaffByAccountID removes every staff-portal subscription for an
	// account across tenants. Callers must use an admin transaction and
	// reserve this for account-wide session revocation, not a single-device
	// logout.
	DeleteStaffByAccountID(ctx context.Context, accountID int64) error
	// DeleteParentByAccountID removes every parent-portal subscription for an
	// account across tenants. Callers must use an admin transaction and
	// reserve this for account-wide session revocation, not a single-device
	// logout.
	DeleteParentByAccountID(ctx context.Context, accountID int64) error
	// DeleteByTokenFamilyID removes subscriptions registered by one
	// refresh-token family, any portal, across tenants. Callers must use an
	// admin transaction. An empty family ID is a no-op so pre-binding rows
	// are not swept by an unbound logout.
	DeleteByTokenFamilyID(ctx context.Context, accountID int64, familyID string) error
	// DeleteStaffUnboundByAccount removes staff-portal subscriptions that were
	// never bound to a token family. tenantID > 0 limits the delete to that
	// school. Callers must use an admin transaction.
	DeleteStaffUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	// DeleteParentUnboundByAccount removes parent-portal subscriptions that
	// were never bound to a token family. tenantID > 0 limits the delete to
	// that school. Callers must use an admin transaction.
	DeleteParentUnboundByAccount(ctx context.Context, accountID, tenantID int64) error
	// DeleteOrphanedSubscriptions removes push rows whose token family is gone
	// or whose account has no live session left for that portal. Unbound
	// parent rows stay if any parent session exists on the account.
	DeleteOrphanedSubscriptions(ctx context.Context) error
	// FindForTenantStaff returns all staff-portal subscriptions of the current tenant.
	FindForTenantStaff(ctx context.Context) ([]*PushSubscription, error)

	// FindForStaffAccounts returns the staff-portal subscriptions of the given
	// accounts in the current tenant. Same eligibility rules as
	// FindForTenantStaff, narrowed to named recipients — the addressing
	// primitive for personal notifications.
	FindForStaffAccounts(ctx context.Context, accountIDs []int64) ([]*PushSubscription, error)
	// FindForTenantAdmins returns staff-portal subscriptions of accounts with
	// effective admin scope in the current tenant.
	FindForTenantAdmins(ctx context.Context) ([]*PushSubscription, error)
	// FindForGuardians returns parent-portal subscriptions of guardian accounts
	// with active guardian access in the current tenant. Pending-enrollment-only
	// recipients are deliberately excluded from Web Push.
	//
	// A non-empty studentIDs additionally requires parent_portal.access for at
	// least one of those children on the caller's users.students_guardians row,
	// so a notification about a child reaches only the accounts that may still
	// hear about it at delivery time. Pass nil for events about no child.
	FindForGuardians(ctx context.Context, guardianAccountIDs []int64, studentIDs []int64) ([]*PushSubscription, error)
}
