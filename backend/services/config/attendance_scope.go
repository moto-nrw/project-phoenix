package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

func isAttendanceScopeKey(key string) bool {
	return key == config.KeyAttendanceEditScope || key == config.KeyOperationalOverviewScope
}

// Both write directions hold the same lock until commit. Direct service
// callers get a transaction too; an autocommit advisory lock cannot guard
// the read/validate/write sequence.
func (s *settingsService) lockAttendanceScopePair(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return ErrRuntimeUnavailable
	}
	if err := s.runtime.AcquireLock(ctx, fmt.Sprintf("attendance-scope:%d", s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock attendance scope pair: %w", err)
	}
	return nil
}

func (s *settingsService) validateAttendanceScopePair(ctx context.Context, key string, value any) error {
	if err := s.lockAttendanceScopePair(ctx); err != nil {
		return err
	}
	sibling := config.KeyOperationalOverviewScope
	if key == config.KeyOperationalOverviewScope {
		sibling = config.KeyAttendanceEditScope
	}
	// Bypass immutable read-path snapshots as well as the request cache.
	current, err := s.ResolveStringForTenantInTx(ctx, s.tenantID(ctx), sibling)
	if err != nil {
		return fmt.Errorf("resolve paired attendance scope: %w", err)
	}
	visibility, editing := current, value
	if key == config.KeyOperationalOverviewScope {
		visibility, editing = value.(string), current
	}
	if editing == config.AttendanceEditScopeAllStaff && visibility != config.OverviewScopeAllStaff {
		return &InvalidValueError{
			Key:    key,
			Reason: "Überall an- und abmelden geht nur mit Sicht auf alle Gruppen und Blöcke. Erweitern Sie zuerst den Sichtbereich. Oder beschränken Sie zuerst das An- und Abmelden auf eigene Zuständigkeiten.",
		}
	}
	return nil
}
