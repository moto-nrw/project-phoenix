// Package careexit is the application workflow behind the Care Plan writes
// that reach several owners: ending and restoring a child's care across the
// Timetable roster, the Student Presence attendance and the Care Plan offering
// bookings behind an Enrollment application, and adding a child to the blocks
// of a later pickup decision.
//
// The workflow owns no tables. It declares the owner surfaces it coordinates
// as consumer-owned ports, which the root binds to the owners' public
// facades. Every call joins the caller's tenant transaction: the care-exit
// transaction of the Care Plan lifecycle, or the pickup decision's, stays the
// one unit of work, so a failed owner write rolls back the others.
package careexit

import (
	"slices"
)

func uniqueSortedIDs(ids []int64) []int64 {
	result := slices.Clone(ids)
	slices.Sort(result)
	return slices.Compact(result)
}
