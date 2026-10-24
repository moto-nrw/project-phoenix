package config

import (
	"context"
	"fmt"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// validateModuleBooking keeps the module prerequisites of the module list
// (#4892) in both directions: booking a module needs every module it
// requires, and cancelling a module is refused while a booked module still
// requires it. A reset restores the registry default (booked), so it is
// checked as a booking.
//
// The module lock is taken before the sibling switches are read, so two
// concurrent writes (cancel Online-Anmeldung, book Ferienbetreuung) cannot
// both pass on the other's pre-commit state.
func (s *settingsService) validateModuleBooking(ctx context.Context, key string, module config.ModuleDefinition, value any) error {
	booked, ok := value.(bool)
	if !ok {
		// A non-boolean value is already rejected by validateValue.
		return nil
	}
	if err := s.lockModuleBooking(ctx); err != nil {
		return err
	}
	candidates := module.Requires
	if !booked {
		candidates = nil
		for _, dependent := range config.ModuleDependents(module.Key) {
			candidates = append(candidates, dependent.Key)
		}
	}
	conflicts, err := s.moduleConflicts(ctx, candidates, !booked)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	return &config.ModuleDependencyError{Key: key, Module: module, Booking: booked, Conflicts: conflicts}
}

// moduleConflicts returns the candidates whose booking state equals
// conflictWhenBooked: for a booking, the prerequisites not booked; for a
// cancellation, the dependents still booked.
func (s *settingsService) moduleConflicts(ctx context.Context, candidates []config.ModuleKey, conflictWhenBooked bool) ([]config.ModuleDefinition, error) {
	var conflicts []config.ModuleDefinition
	for _, key := range candidates {
		candidate, ok := config.LookupModule(key)
		if !ok {
			return nil, fmt.Errorf("module list names unknown module %q", key)
		}
		booked, err := s.ResolveBool(ctx, candidate.SettingKey)
		if err != nil {
			// Operational failure stays a plain error (500), not a 400.
			return nil, fmt.Errorf("resolve module %s: %w", candidate.Key, err)
		}
		if booked == conflictWhenBooked {
			conflicts = append(conflicts, candidate)
		}
	}
	return conflicts, nil
}

// lockModuleBooking takes the exclusive per-tenant advisory lock of the
// module switches. Like the other Lock* helpers it flushes the tenant's
// request cache first, so the prerequisite reads after the lock observe a
// concurrent writer's committed switch instead of a memoized value. Without
// an ambient transaction the xact lock is meaningless and is skipped; the
// operator write path always runs in the school's tenant transaction.
func (s *settingsService) lockModuleBooking(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return nil
	}
	if err := s.runtime.AcquireLock(ctx, moduleBookingLockKey(s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock module booking: %w", err)
	}
	return nil
}

// moduleBookingLockKey is per tenant: one school's bookings never wait for
// another school's.
func moduleBookingLockKey(tenantID int64) string {
	return fmt.Sprintf("module-booking:%d", tenantID)
}
