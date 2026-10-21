package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The instance lifecycle, its automatic start and end, the status slots and
// the staff deviations live in internal/application (#4084). The roots build
// them through the constructors below; their dependency types are in
// modules/timetable/ports.

func NewInstanceLifecycle(deps ports.InstanceLifecycleDependencies) (ports.InstanceLifecycle, error) {
	return application.NewInstanceLifecycle(deps)
}

func NewInstanceAutoEnd(instances ports.LegacyInstanceReads, completion timetable.InstanceLifecycle) (timetable.InstanceAutoEnd, error) {
	return application.NewInstanceAutoEnd(instances, completion)
}

func NewInstanceStatusSlots(rows ports.StatusSlotRows) ports.InstanceStatusSlots {
	return application.NewInstanceStatusSlots(rows)
}

func NewStaffDeviations(deps ports.StaffDeviationDependencies) (timetable.StaffDeviations, error) {
	return application.NewStaffDeviations(deps)
}
