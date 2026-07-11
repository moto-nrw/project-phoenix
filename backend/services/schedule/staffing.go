package schedule

import scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"

// Understaffing detection (#1840). A block is "understaffed" when fewer people
// are actually present than the number of planned positions — OR when nobody is
// present at all. This is the single rule that /gaps, the deliberately-unstaffed
// acknowledgement, and the atomic deviations endpoint all share, so a partially
// covered block (e.g. two planned, one absent without replacement) is reported
// and acknowledgeable exactly like a fully empty one, and the acknowledgement is
// rejected only when the block is genuinely fully staffed.
//
// Counting:
//   - planned  = instance_staff rows that are NOT substitutes (the base-plan
//     positions, whether or not currently absent).
//   - present  = instance_staff rows that are NOT absent (planned people still
//     there PLUS any substitute covering an absence).
//
// A substitute counts toward `present` (it restores coverage for the absent
// planned person) but not toward `planned`, so a fully-substituted block reads
// as staffed, and an unreplaced absence reads as understaffed.

// IsUnderstaffedCounts reports whether a block with `planned` non-substitute
// positions and `present` non-absent people is below its planned staffing.
func IsUnderstaffedCounts(present, planned int) bool {
	return present == 0 || present < planned
}

// IsUnderstaffed reports whether the given instance_staff rows leave the block
// understaffed (see IsUnderstaffedCounts). nil rows are ignored.
func IsUnderstaffed(rows []*scheduleModel.InstanceStaff) bool {
	present, planned := 0, 0
	for _, r := range rows {
		if r == nil {
			continue
		}
		if !r.IsAbsent {
			present++
		}
		if !r.IsSubstitute {
			planned++
		}
	}
	return IsUnderstaffedCounts(present, planned)
}
