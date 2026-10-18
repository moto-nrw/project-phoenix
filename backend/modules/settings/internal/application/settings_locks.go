package config

import (
	"context"
	"fmt"
	"strings"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// flushRequestCacheForLock drops every request-cache entry for the ambient
// tenant. All Lock* helpers call it first: which keys a cross-field guard
// re-reads under its lock is deliberately not enumerated here — a whole-tenant
// flush stays correct when the next guard adds another key (#1565/#1663).
func (s *settingsService) flushRequestCacheForLock(ctx context.Context) {
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		cache.EvictTenant(s.tenantID(ctx))
	}
}

// LockSlotListCutoffPairShared takes the SHARED variant of the Ganztag cutoff
// lock for read paths (the class-day projection, modules/classday). Shared holders never
// block one another, so concurrent /options, pickup-preview and export requests
// no longer serialize behind a single exclusive lock — a slow export scanning
// rosters or rendering a PDF/XLSX cannot stall every other reader in the tenant.
// It still conflicts with the exclusive writer lock, so a cutoff update can never
// commit a partial pair while a reader observes the two cutoffs (#1565 review).
// Flushes the tenant's request-cache bucket like the exclusive variant — the
// slotlists reader resolves both cutoffs after taking this lock and must see
// the writer's committed pair, not memoized pre-lock values.
func (s *settingsService) LockSlotListCutoffPairShared(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, slotListCutoffLockKey(s.tenantID(ctx)), true); err != nil {
		return fmt.Errorf("lock Ganztag cutoff pair (shared): %w", err)
	}
	return nil
}

// slotListCutoffLockKey is the per-tenant advisory-lock key shared by the
// exclusive writer lock and the shared reader lock so the two conflict.
func slotListCutoffLockKey(tenantID int64) string {
	return fmt.Sprintf("slot-list-cutoff:%d", tenantID)
}

// LockClassCollectionPair takes the per-tenant transaction-scoped advisory lock
// that guards the enrollment class-restriction / class-collection invariant.
// Two writes can otherwise race: one disabling concrete-class collection
// (validateClassCollectionGuard here) and one activating a class-restricted
// phase (validateEligibleClassesCollectable in modules/enrollment). Under READ
// COMMITTED each reads the other's pre-commit state, both pass, and they commit
// an active restricted phase with class collection off — every submission then
// fails class_not_eligible. Both sides take THIS lock on the same key, so the
// second writer blocks, re-reads the first's committed state, and rejects.
// Best-effort: without an ambient transaction the xact lock is meaningless (the
// settings and phase write paths always run inside a tenant tx), so it is
// skipped rather than failing.
// Flushes the tenant's request-cache bucket first: both the settings-side
// guard and the enrollment-side phase guard re-read settings (including
// enrollment.grade_level_max) under this lock and must observe committed
// state, not memoized pre-lock values.
func (s *settingsService) LockClassCollectionPair(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, classCollectionLockKey(s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock class-collection pair: %w", err)
	}
	return nil
}

// classCollectionLockKey is the per-tenant advisory-lock key shared by the
// settings-side class-collection guard and the enrollment-side phase
// eligibility guard so the two conflict.
func classCollectionLockKey(tenantID int64) string {
	return fmt.Sprintf("enrollment-class-collection:%d", tenantID)
}

// LockMFAPolicy takes the exclusive per-tenant advisory lock on
// security.mfa_mode for the WRITE side — see the interface doc and
// lockMFAPolicy.
func (s *settingsService) LockMFAPolicy(ctx context.Context) error {
	return s.lockMFAPolicy(ctx, s.tenantID(ctx), false)
}

// LockMFAPolicySharedForTenant takes the shared variant for the token-mint READ
// side, with the tenant passed explicitly because the login flows have no
// tenant context.
func (s *settingsService) LockMFAPolicySharedForTenant(ctx context.Context, tenantID int64) error {
	return s.lockMFAPolicy(ctx, tenantID, true)
}

// lockMFAPolicy is the shared body of the two MFA-policy lock helpers.
//
// Without it, "is MFA required for this session?" is a read that commits
// nothing: a school login resolves security.mfa_mode as off, an admin switches
// it to required and commits, and the login — which has not written its refresh
// token yet — still mints a session that never saw a second factor (#2207
// review). Re-reading the mode inside the mint transaction narrows that window
// but cannot close it: under READ COMMITTED the re-read simply happens a few
// statements earlier than the concurrent write. Only a lock the two sides share
// orders them, so either the writer waits for the mint to commit (and the next
// login sees the new mode) or the mint's re-read observes the committed value
// and refuses.
//
// Best-effort like the sibling helpers: without an ambient transaction an xact
// lock is meaningless — both sides always run in one — so it is skipped rather
// than failing. A tenant-less caller (registry default, no per-school mode to
// guard) is skipped for the same reason.
func (s *settingsService) lockMFAPolicy(ctx context.Context, tenantID int64, shared bool) error {
	// Same freshness barrier as the other Lock* helpers: post-lock reads must
	// hit the database instead of replaying a memoized pre-lock value.
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		cache.EvictTenant(tenantID)
	}
	if tenantID <= 0 {
		return nil
	}
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, mfaPolicyLockKey(tenantID), shared); err != nil {
		return fmt.Errorf("lock mfa policy: %w", err)
	}
	return nil
}

// mfaPolicyLockKey is the per-tenant advisory-lock key shared by the
// security.mfa_mode writer and the token mints that re-decide the MFA gate, so
// the two conflict. Per tenant: one school's admin must not serialize another
// school's logins.
func mfaPolicyLockKey(tenantID int64) string {
	return fmt.Sprintf("mfa-policy:%d", tenantID)
}

// lockMFAPolicyForWrite takes the exclusive policy lock when the key being
// written is one a session mint reads. Called by SetValue and ResetValue
// BEFORE they read the current value, so the whole read-modify-write is
// ordered against an in-flight mint rather than just the final upsert.
func (s *settingsService) lockMFAPolicyForWrite(ctx context.Context, key string) error {
	if key != config.KeyMFAMode {
		return nil
	}
	return s.LockMFAPolicy(ctx)
}

// LockParentPickupChangePolicy takes the exclusive per-tenant policy lock on
// the parent pickup-change switch and its same-day cutoff. The corresponding
// guardian write takes the shared side before it re-reads both values.
func (s *settingsService) LockParentPickupChangePolicy(ctx context.Context) error {
	return s.lockParentPickupChangePolicy(ctx, s.tenantID(ctx), false)
}

// LockParentPickupChangePolicySharedForTenant takes the shared side of the
// parent pickup-change policy lock for a guardian write transaction.
func (s *settingsService) LockParentPickupChangePolicySharedForTenant(ctx context.Context, tenantID int64) error {
	return s.lockParentPickupChangePolicy(ctx, tenantID, true)
}

// lockParentPickupChangePolicy serializes settings writes with guardian
// pickup-change writes. Without the shared lock, the enabled flag and cutoff
// can be read on separate READ COMMITTED snapshots, or a settings write can
// commit after their re-read but before the parent write commits.
func (s *settingsService) lockParentPickupChangePolicy(ctx context.Context, tenantID int64, shared bool) error {
	if cache := config.RequestCacheFromContext(ctx); cache != nil {
		cache.EvictTenant(tenantID)
	}
	if tenantID <= 0 || s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, parentPickupChangePolicyLockKey(tenantID), shared); err != nil {
		return fmt.Errorf("lock parent pickup-change policy: %w", err)
	}
	return nil
}

// parentPickupChangePolicyLockKey is shared by writes to both settings and by
// guardian writes that consume their effective policy. It is per tenant so
// one school's settings update never stalls another school's families.
func parentPickupChangePolicyLockKey(tenantID int64) string {
	return fmt.Sprintf("parent-pickup-change-policy:%d", tenantID)
}

// lockParentPickupChangePolicyForWrite takes the exclusive policy lock for
// either setting before the write reads the old value or changes the override.
func (s *settingsService) lockParentPickupChangePolicyForWrite(ctx context.Context, key string) error {
	if key != config.KeyParentPickupChangeEnabled && key != config.KeyParentPickupChangeCutoffTime {
		return nil
	}
	return s.LockParentPickupChangePolicy(ctx)
}

// validateTimeFormat checks that a string is a valid HH:MM time.
func validateTimeFormat(s string) error {
	if _, err := time.Parse("15:04", strings.TrimSpace(s)); err != nil {
		return fmt.Errorf("expected time in HH:MM format")
	}
	return nil
}
