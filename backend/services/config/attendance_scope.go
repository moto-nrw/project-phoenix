package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// schoolWideActionScopes are the action scopes whose all_staff value needs
// the all_staff overview, with the reason a rejected write shows.
var schoolWideActionScopes = []struct{ key, reason string }{
	{config.KeyAttendanceEditScope, "Überall an- und abmelden geht nur mit Sicht auf alle Gruppen und Blöcke. Erweitern Sie zuerst den Sichtbereich. Oder beschränken Sie zuerst das An- und Abmelden auf eigene Zuständigkeiten."},
	{config.KeyBlockStartScope, "Das ganze Team darf nur starten, wenn es alle Gruppen und Blöcke sieht. Erweitern Sie zuerst den Sichtbereich. Oder erlauben Sie das Starten zuerst nur eingeplanten Kräften."},
	{config.KeyBlockCompleteScope, "Das ganze Team darf nur beenden, wenn es alle Gruppen und Blöcke sieht. Erweitern Sie zuerst den Sichtbereich. Oder erlauben Sie das Beenden zuerst nur eingeplanten Kräften."},
}

func isOverviewScopeKey(key string) bool {
	if key == config.KeyOperationalOverviewScope {
		return true
	}
	for _, scope := range schoolWideActionScopes {
		if scope.key == key {
			return true
		}
	}
	return false
}

// Both write directions hold the same lock until commit. Direct service
// callers get a transaction too; an autocommit advisory lock cannot guard
// the read/validate/write sequence.
func (s *settingsService) lockOverviewScopeDependents(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return ErrRuntimeUnavailable
	}
	if err := s.runtime.AcquireLock(ctx, fmt.Sprintf("attendance-scope:%d", s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock overview scope dependents: %w", err)
	}
	return nil
}

func (s *settingsService) validateOverviewScopeDependents(ctx context.Context, key string, value any) error {
	if err := s.lockOverviewScopeDependents(ctx); err != nil {
		return err
	}
	// Bypass immutable read-path snapshots as well as the request cache.
	resolve := func(key string) (string, error) {
		current, err := s.ResolveStringForTenantInTx(ctx, s.tenantID(ctx), key)
		if err != nil {
			return "", fmt.Errorf("resolve overview scope dependent: %w", err)
		}
		return current, nil
	}
	visibility := value
	if key != config.KeyOperationalOverviewScope {
		current, err := resolve(config.KeyOperationalOverviewScope)
		if err != nil {
			return err
		}
		visibility = current
	}
	if visibility == config.OverviewScopeAllStaff {
		return nil
	}
	for _, scope := range schoolWideActionScopes {
		if key != config.KeyOperationalOverviewScope && key != scope.key {
			continue
		}
		action := value
		if key == config.KeyOperationalOverviewScope {
			current, err := resolve(scope.key)
			if err != nil {
				return err
			}
			action = current
		}
		// Every action scope names its school-wide value all_staff.
		if action == config.AttendanceEditScopeAllStaff {
			return &InvalidValueError{Key: key, Reason: scope.reason}
		}
	}
	return nil
}
