package compose

// InstanceReads are the block reads of the Workforce planning and the
// shift-plan-sync workflow, in the owner's public vocabulary.
func (r Repositories) InstanceReads() TimetableInstanceReads {
	return NewTimetableInstanceReads(r.ActivityInstance)
}

// InstanceStaffReads are the staffing reads of the same consumers.
func (r Repositories) InstanceStaffReads() TimetableInstanceStaffReads {
	return NewTimetableInstanceStaffReads(r.InstanceStaff)
}
