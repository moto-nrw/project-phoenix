package staffoffboarding

// StaffDeletion is the manual data deletion record of one offboarded staff
// member, in the values the Audit Platform stores: the removed row counts
// per kind as metadata.
type StaffDeletion struct {
	TenantID       int64
	StaffID        int64
	RecordsDeleted int
	DeletedBy      string
	Reason         string
	Counts         map[string]int64
}

// NewStaffDeletion is the data deletion record of the offboarding result,
// attributed to actor.
func NewStaffDeletion(actor Actor, result Result) StaffDeletion {
	counts := map[string]int64{
		"group_teacher": result.Membership.GroupAssignments, "class_teachers": result.Membership.ClassAssignments,
		"activity_supervisors": result.Timetable.PlannedSupervisors, "timetable_instance_staff": result.Timetable.InstanceAssignments,
		"group_substitutions": result.Workforce.Substitutions, "staff_shifts": result.Workforce.Shifts,
		"staff_shift_series": result.Workforce.Series, "staff_absences": result.Workforce.Absences,
	}
	records := 1
	for _, count := range counts {
		records += int(count)
	}
	deletedBy := actor.Username
	if deletedBy == "" {
		// Preserve the legacy provider's audit value for authenticated
		// sessions without a display name, including the API seeder.
		deletedBy = "system"
	}
	reason := "staff offboarding"
	if actor.PlannedExitDay != "" {
		reason = "planned staff exit after last working day " + actor.PlannedExitDay
	}
	return StaffDeletion{
		TenantID: actor.TenantID, StaffID: result.StaffID, RecordsDeleted: records,
		DeletedBy: deletedBy, Reason: reason, Counts: counts,
	}
}
