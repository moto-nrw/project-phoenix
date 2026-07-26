// Package schedule — Betreuungsplan capacity computation (issue #1838).
//
// Computes the Betreuungsschlüssel-derived staffing requirement for a
// materialized Betreuungsplan block (activity instance). The list endpoints
// (instances_list.go, templates_list.go) already have instance_staff/
// instance_students rows loaded per row, so they call RequiredStaffForChildren
// directly on the counts they already hold instead of a dedicated service —
// that would mean an extra DB round-trip per list request for no benefit.
package schedule

// RequiredStaffForChildren returns the number of staff required to supervise
// childrenCount children at the given ratio (max children per staff member),
// rounded up. ratio < 1 is defensively clamped to 1 so a corrupted override
// can never divide by zero or invert the relationship; the registry
// Validation (1-30) already prevents this in practice.
func RequiredStaffForChildren(childrenCount, ratio int) int {
	if childrenCount <= 0 {
		return 0
	}
	if ratio < 1 {
		ratio = 1
	}
	return (childrenCount + ratio - 1) / ratio
}

// EffectiveRequiredStaff returns the required staffing level for a block: the
// manual Personalbedarf override when one is set (issue #1839 — a set value
// wins over the Betreuungsschlüssel-derived figure), otherwise the derived
// RequiredStaffForChildren value. A NULL override (nil) means "derive"; a
// negative override is defensively treated as 0 (Validate already rejects it
// on write, so this only guards a corrupted stored value).
func EffectiveRequiredStaff(override *int, childrenCount, ratio int) int {
	if override != nil {
		if *override < 0 {
			return 0
		}
		return *override
	}
	return RequiredStaffForChildren(childrenCount, ratio)
}
