package api

import (
	timetableAPI "github.com/moto-nrw/project-phoenix/api/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// The timetable routes announce their staffing saves (#1844) through the
// instance lifecycle's broadcaster, which the composition binds to the
// realtime hub: one tenant-wide staffing_deviation_changed event whose
// Source names the emitting flow.
var _ timetableAPI.StaffingAnnouncer = (*timetableCompose.InstanceLifecycleService)(nil)

// timetableStaffingAnnouncer is the lifecycle as the routes' staffing
// announcer; a lifecycle that cannot announce leaves the routes silent.
func timetableStaffingAnnouncer(lifecycle timetable.InstanceLifecycleCapability) timetableAPI.StaffingAnnouncer {
	announcer, _ := lifecycle.(timetableAPI.StaffingAnnouncer)
	return announcer
}
