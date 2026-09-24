package authorize

import (
	"context"
	"fmt"
)

// SchoolWideActionScope reports whether an action scope setting opens its
// action to every verified staff member of the school: attendance editing
// (#3180) or starting a planned block (#3622). Its all_staff value counts only
// with the all_staff overview, so nobody acts on a block they cannot see.
//
// The caller decides who the question applies to (not portals, not admins),
// checks the staff profile, and names the blocks the action reaches.
func SchoolWideActionScope(ctx context.Context, settings OverviewSettingsResolver, scopeKey string) (bool, error) {
	if settings == nil {
		return false, fmt.Errorf("resolve %s: settings unavailable", scopeKey)
	}
	for _, key := range []string{scopeKey, operationalOverviewScopeKey} {
		value, err := settings.ResolveString(ctx, key)
		if err != nil {
			return false, fmt.Errorf("resolve %s: %w", key, err)
		}
		if value != OverviewScopeAllStaff {
			return false, nil
		}
	}
	return true, nil
}
