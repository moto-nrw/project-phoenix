package application

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// NewInstanceStaffRecords serves the retained instance staff repository over
// the Timetable capability alone (#2743).
func NewInstanceStaffRecords(capability timetable.Capability) ports.InstanceStaffRecords {
	return timetableInstanceStaffRepository{timetable: capability}
}
