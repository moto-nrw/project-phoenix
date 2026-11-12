package care

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChildFeaturesCloseTodayAtCareDayEnd pins #5710: from the school's care
// day end (operations.session_end_time) the portal closes today's pickup
// change, with the same rule that ends "Im Unterricht". The end minute itself
// is closed; without a configured end, or with the pickup change off, nothing
// closes.
func TestChildFeaturesCloseTodayAtCareDayEnd(t *testing.T) {
	t.Parallel()

	// 14:00 in Berlin (summer time).
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	child := &Child{GuardianPermissions: map[string]bool{GuardianPermissionPickupManage: true}}
	flags := func(enabled bool, careDayEnd string) ChildFeatureFlags {
		return childFeatureSettings{
			pickupChange: enabled,
			pickupCutoff: SameDayCutoff{Now: now},
			careDayEnd:   careDayEnd,
		}.activeChildFlags(child, false)
	}

	assert.False(t, flags(true, "14:01").PickupChangeCareDayEnded, "before the end")
	assert.True(t, flags(true, "14:00").PickupChangeCareDayEnded, "at the end")
	assert.True(t, flags(true, "13:00").PickupChangeCareDayEnded, "after the end")
	assert.False(t, flags(true, "").PickupChangeCareDayEnded, "no configured end")
	assert.False(t, flags(false, "13:00").PickupChangeCareDayEnded, "pickup change off")
}
