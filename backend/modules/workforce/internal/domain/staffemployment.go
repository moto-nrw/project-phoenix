package domain

// AppendStaffNotes joins a new paragraph onto existing staff notes.
func AppendStaffNotes(existing, notes string) string {
	if existing == "" {
		return notes
	}
	return existing + "\n" + notes
}

// StaffPlanningProfile is one membership's template binding and every schedule
// version intersecting the requested calendar window, read atomically.
type StaffPlanningProfile struct {
	MembershipID       int64
	WorkTimeModelID    *int64
	RotationAnchorDate string
	Schedules          []StaffWorkSchedule
}
