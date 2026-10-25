package notifications

import (
	"context"
	"fmt"
)

// Pushes of a module the school has not booked are not sent (#4895). A push
// to parents belongs to Eltern-App, a push to a device of the school portal
// to Unterricht am Vormittag. The router decides before any channel runs,
// so in-app and push deliveries agree.

// applyModuleBooking reports whether event may be delivered at all and marks
// it to skip the school portal when the school has not booked that portal.
// A failed read is an error, never a booking decision.
func (r *router) applyModuleBooking(ctx context.Context, event *Event) (bool, error) {
	tenantID := event.Audience.TenantID
	if event.Audience.Scope == ScopeGuardian {
		booked, err := r.settings.ResolveBoolForTenant(ctx, tenantID, settingModuleElternApp)
		if err != nil {
			return false, fmt.Errorf("resolving module Eltern-App: %w", err)
		}
		if !booked {
			return false, nil
		}
	}
	if deliversToSchoolPortal(*event) {
		booked, err := r.settings.ResolveBoolForTenant(ctx, tenantID, settingModuleUnterrichtVormittag)
		if err != nil {
			return false, fmt.Errorf("resolving module Unterricht am Vormittag: %w", err)
		}
		event.schoolPortalClosed = !booked
	}
	return true, nil
}

// requireEventModules applies the booking to one event. An event of a
// module the school has not booked answers ErrDisabled, the answer every
// producer already treats as a silent no-op.
func (r *router) requireEventModules(ctx context.Context, event *Event) error {
	deliver, err := r.applyModuleBooking(ctx, event)
	if err != nil {
		return err
	}
	if !deliver {
		return ErrDisabled
	}
	return nil
}

// keepBookedEvents drops the events of a module the school has not booked
// from a batch; ErrDisabled when none is left.
func (r *router) keepBookedEvents(ctx context.Context, events []Event) ([]Event, error) {
	kept := events[:0]
	for _, event := range events {
		deliver, err := r.applyModuleBooking(ctx, &event)
		if err != nil {
			return nil, err
		}
		if deliver {
			kept = append(kept, event)
		}
	}
	if len(kept) == 0 {
		return nil, ErrDisabled
	}
	return kept, nil
}
