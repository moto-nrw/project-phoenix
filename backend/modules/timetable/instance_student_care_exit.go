package timetable

func validCareExitAssignments(removals []InstanceStudent) bool {
	for _, row := range removals {
		if row.TenantID <= 0 || row.StudentID <= 0 || row.InstanceID <= 0 {
			return false
		}
	}
	return true
}
