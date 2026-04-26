package platform

import "time"

// SchoolYearBoundsForTest exposes the unexported schoolYearBounds helper to
// _test packages. The function itself stays unexported because it's purely
// an internal implementation detail of the provisioning seed.
func SchoolYearBoundsForTest(now time.Time) (int, int) {
	return schoolYearBounds(now)
}
