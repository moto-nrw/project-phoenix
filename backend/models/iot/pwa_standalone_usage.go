package iot

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// PWAStandaloneUsage records that an account used the app in PWA
// standalone display mode (installed to the home screen). One row per
// (tenant, account, portal); last_seen_at advances on every report. No
// device identifier is stored on purpose — the metric counts users, not
// devices (#2189).
type PWAStandaloneUsage struct {
	base.Model `bun:"schema:iot,table:pwa_standalone_usage"`
	base.TenantModel
	AccountID   int64     `bun:"account_id,notnull" json:"account_id"`
	Portal      string    `bun:"portal,notnull" json:"portal"`
	FirstSeenAt time.Time `bun:"first_seen_at,nullzero,notnull,default:current_timestamp" json:"first_seen_at"`
	LastSeenAt  time.Time `bun:"last_seen_at,nullzero,notnull,default:current_timestamp" json:"last_seen_at"`
}

// Validate ensures usage report data is valid.
func (u *PWAStandaloneUsage) Validate() error {
	if u.AccountID <= 0 {
		return errors.New("account_id is required")
	}
	if u.Portal != PushPortalStaff && u.Portal != PushPortalParent {
		return errors.New("portal must be 'staff' or 'parent'")
	}
	return nil
}

// PWAStandaloneUsageRepository defines operations for PWA standalone-usage
// rows. Reads/writes are tenant-scoped (RLS); callers outside tenant
// middleware must wrap calls in tenant.WithTenantTx / WithAdminTx.
type PWAStandaloneUsageRepository interface {
	base.CRUDRepository[*PWAStandaloneUsage]

	// RecordSeen inserts or refreshes a usage row keyed by
	// (tenant_id, account_id, portal), advancing last_seen_at.
	RecordSeen(ctx context.Context, usage *PWAStandaloneUsage) error
	// DeleteLastSeenBefore removes rows of the current tenant whose
	// last_seen_at is before cutoff and returns the number of deleted rows.
	// Deliberately NOT the generic DeleteOlderThan: that one targets DATE
	// columns via timezone.Date, while last_seen_at is a TIMESTAMPTZ instant.
	DeleteLastSeenBefore(ctx context.Context, cutoff time.Time) (int, error)
}
