package compose

import "github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"

// InstanceReads are the block reads of the Workforce planning and the
// shift-plan-sync workflow, in the owner's public vocabulary.
func (r Repositories) InstanceReads() application.TimetableInstanceReads {
	return application.NewTimetableInstanceReads(r.ActivityInstance)
}

// InstanceStaffReads are the staffing reads of the same consumers.
func (r Repositories) InstanceStaffReads() application.TimetableInstanceStaffReads {
	return application.NewTimetableInstanceStaffReads(r.InstanceStaff)
}
