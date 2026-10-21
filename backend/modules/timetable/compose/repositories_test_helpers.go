//go:build testhooks

package compose

import "github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"

// InstanceReads serves Workforce planning and shift-plan-sync in the owner's public vocabulary.
func (r Repositories) InstanceReads() application.TimetableInstanceReads {
	return application.NewTimetableInstanceReads(r.ActivityInstance.Reads)
}

func (r Repositories) InstanceStaffReads() application.TimetableInstanceStaffReads {
	return application.NewTimetableInstanceStaffReads(r.InstanceStaff)
}
